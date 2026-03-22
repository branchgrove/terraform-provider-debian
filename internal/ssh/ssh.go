// Package ssh provides a connection-pooling SSH client for the Terraform
// provider. A single Manager caches one connection per user@host:port and
// validates it with a keepalive before reuse. Each connection's concurrent
// sessions are capped by a semaphore sized to the host's MaxSessions (probed
// via sshd -T) minus a small headroom, so Terraform parallelism cannot
// exhaust the server's session capacity.
//
// Commands are executed through Client.Run, which uses Start/Wait with a
// select on ctx.Done to support Terraform timeouts. Run returns a *RunError
// for non-zero exit codes and a plain error for transport failures, letting
// callers distinguish "command failed" from "connection broke" via errors.As.
// Environment variables are injected as command-prefix
// shell assignments rather than session.Setenv (which most sshd configs
// reject via AcceptEnv).
package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/semaphore"
)

// Manager holds a pool of SSH connections keyed by user@host:port. A mutex
// serialises access; stale connections are detected via keepalive and replaced
// transparently.
type Manager struct {
	mu      sync.Mutex
	clients map[string]*Client
}

// Client wraps an SSH connection with a semaphore that limits concurrent
// sessions to stay within the host's MaxSessions.
type Client struct {
	ssh *ssh.Client
	sem *semaphore.Weighted
}

// NewManager creates a Manager that caches and reuses SSH connections keyed by user@host:port.
func NewManager() *Manager {
	return &Manager{
		mu:      sync.Mutex{},
		clients: make(map[string]*Client),
	}
}

// Close terminates all cached SSH connections and returns any errors encountered.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for key, v := range m.clients {
		if err := v.ssh.Close(); err != nil {
			errs = append(errs, err)
		}
		delete(m.clients, key)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to close ssh manager clients: %s", errors.Join(errs...))
	}

	return nil
}

type AuthMethod = ssh.AuthMethod

// PasswordAuth returns an AuthMethod that authenticates using the given password.
func PasswordAuth(password string) AuthMethod {
	return ssh.Password(password)
}

// PrivateKeyAuth parses a PEM-encoded private key and returns an AuthMethod for public-key authentication.
func PrivateKeyAuth(privateKey string) (AuthMethod, error) {
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return ssh.PublicKeys(signer), nil
}

// GetClient returns a cached or new SSH client. hostKey is an optional SSH
// public key in authorized_keys format; if non-empty the server's identity is
// verified against it, otherwise all host keys are accepted.
func (m *Manager) GetClient(ctx context.Context, host string, port int, user string, auth ssh.AuthMethod, hostKey string) (*Client, error) {
	key := fmt.Sprintf("%s@%s:%d", user, host, port)

	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[key]; ok {
		if client.isAlive() {
			return client, nil
		}
		_ = client.ssh.Close()
		delete(m.clients, key)
	}

	var hostKeyCallback ssh.HostKeyCallback
	if hostKey != "" {
		pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(hostKey))
		if err != nil {
			return nil, fmt.Errorf("parse host key: %w", err)
		}
		hostKeyCallback = ssh.FixedHostKey(pubKey)
	} else {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: hostKeyCallback,
	}

	// DialContext + NewClientConn instead of ssh.Dial so the TCP dial
	// respects context cancellation and timeouts.
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake %s: %w", addr, err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)

	hostMax, err := probeMaxSessions(client)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	// Reserve a few sessions for the server's own use (agent forwarding,
	// internal bookkeeping) so we don't saturate MaxSessions.
	sessions := hostMax - 3
	if sessions < 1 {
		sessions = 1
	}

	m.clients[key] = &Client{
		ssh: client,
		sem: semaphore.NewWeighted(int64(sessions)),
	}

	return m.clients[key], nil
}

// isAlive sends a keepalive request to check if the SSH connection is still usable.
func (c *Client) isAlive() bool {
	_, _, err := c.ssh.SendRequest("keepalive@openssh.com", true, nil)
	return err == nil
}

// acquireSession acquires a session slot and returns a new session. The caller must
// call the returned release function when done; it closes the session and releases the slot.
// The release function is safe to call multiple times.
func (c *Client) acquireSession(ctx context.Context) (*ssh.Session, func(), error) {
	if err := c.sem.Acquire(ctx, 1); err != nil {
		return nil, nil, fmt.Errorf("acquire session semaphore: %w", err)
	}
	session, err := c.ssh.NewSession()
	if err != nil {
		c.sem.Release(1)
		return nil, nil, fmt.Errorf("new session: %w", err)
	}

	var once sync.Once
	release := func() {
		once.Do(func() {
			session.Close()
			c.sem.Release(1)
		})
	}

	return session, release, nil
}

// RunResult holds the result of a remote command. ExitCode is the process exit
// status; Stdout and Stderr are captured separately.
type RunResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

type RunError struct {
	RunResult
	Command string
}

func (e *RunError) Error() string {
	return fmt.Sprintf("run command %q: exit code %d: %s", e.Command, e.ExitCode, strings.TrimSpace(string(e.Stderr)))
}

// Run executes a single command on the host and returns its stdout, stderr,
// and exit code.
//
// If ctx is canceled or its deadline is exceeded the session is closed
// immediately, which terminates the remote process. No PTY is allocated so
// output stays deterministic for provisioning.
//
// A non-zero exit code is returned as a *RunError (which implements error and
// embeds RunResult). Transport/session failures are returned as plain errors.
// Callers can use errors.AsType[*RunError](err) to distinguish the two.
func (c *Client) Run(ctx context.Context, cmd string, env map[string]string, stdin io.Reader) (*RunResult, error) {
	session, release, err := c.acquireSession(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	if len(env) > 0 {
		cmd = envPrefix(env) + cmd
	}

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if stdin != nil {
		session.Stdin = stdin
	}
	if err := session.Start(cmd); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}

	waitDone := make(chan error, 1)
	go func() { waitDone <- session.Wait() }()

	var waitErr error
	select {
	case waitErr = <-waitDone:
		// Command finished (normally or with exit error).
	case <-ctx.Done():
		release()
		<-waitDone
		return nil, context.Cause(ctx)
	}

	exitCode := 0
	if waitErr != nil {
		exitErr, ok := errors.AsType[*ssh.ExitError](waitErr)
		if ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return nil, fmt.Errorf("run command: %w", waitErr)
		}
	}

	runResult := &RunResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: exitCode,
	}

	if exitCode != 0 {
		return nil, &RunError{
			RunResult: *runResult,
			Command:   cmd,
		}
	}

	return runResult, nil
}

// probeMaxSessions runs sshd -T on the host and parses MaxSessions.
func probeMaxSessions(c *ssh.Client) (int, error) {
	session, err := c.NewSession()
	if err != nil {
		return 0, fmt.Errorf("create session to probe MaxSessions: %w", err)
	}
	defer session.Close()

	out, err := session.CombinedOutput("sshd -T | grep -i maxsessions")
	if err != nil {
		return 0, fmt.Errorf("run sshd -T on host to get MaxSessions: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	// sshd -T outputs "maxsessions 10"
	fields := strings.Fields(string(out))
	if len(fields) < 2 {
		return 0, fmt.Errorf("parse MaxSessions from host: unexpected output %q (expected \"maxsessions <n>\")", strings.TrimSpace(string(out)))
	}
	n, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, fmt.Errorf("parse MaxSessions from host: %q is not a number: %w", fields[1], err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("parse MaxSessions from host: value %d is invalid (must be positive)", n)
	}
	return n, nil
}

// envPrefix builds shell variable assignments (e.g. KEY='val'; ) prepended to
// the command so that $KEY expands in the command string. This avoids
// session.Setenv which most sshd configs reject via AcceptEnv, and avoids
// shell-quoting issues by single-quoting all values. Keys are sorted for
// deterministic output.
func envPrefix(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString("='")
		b.WriteString(strings.ReplaceAll(env[k], "'", `'\''`))
		b.WriteString("'; ")
	}
	return b.String()
}
