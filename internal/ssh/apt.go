package ssh

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// InstalledPackage holds the name and installed version of a dpkg package.
type InstalledPackage struct {
	Name    string
	Version string
}

// isAptLockError returns true if the error output indicates that apt/dpkg is locked.
func isAptLockError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Could not get lock") || strings.Contains(msg, "Unable to acquire the dpkg frontend lock") || strings.Contains(msg, "dpkg status database is locked")
}

// runWithAptLockRetry executes the given function, retrying with exponential backoff if an apt lock error is encountered.
func runWithAptLockRetry(ctx context.Context, operation func() (*RunResult, error)) (*RunResult, error) {
	const maxRetries = 30
	baseDelay := 1 * time.Second
	maxDelay := 10 * time.Second

	var res *RunResult
	var err error

	for i := 0; i < maxRetries; i++ {
		res, err = operation()
		if !isAptLockError(err) {
			return res, err
		}

		delay := baseDelay * time.Duration(1<<i)
		if delay > maxDelay {
			delay = maxDelay
		}

		select {
		case <-time.After(delay):
			// retry
		case <-ctx.Done():
			return nil, fmt.Errorf("context canceled while waiting for apt lock: %w (last error: %v)", ctx.Err(), err)
		}
	}

	return nil, fmt.Errorf("timeout waiting for apt lock after %d retries. last error: %w", maxRetries, err)
}

// GetInstalledPackages queries dpkg for the install status of the given package
// names. Returns a map from package name to installed version. Packages that are
// not installed are omitted from the map.
func (c *Client) GetInstalledPackages(ctx context.Context, names []string) (map[string]string, error) {
	if len(names) == 0 {
		return map[string]string{}, nil
	}

	env := map[string]string{"PKGS": strings.Join(names, " ")}
	res, err := c.Run(ctx, `dpkg-query -W -f='${Package}\t${Version}\t${Status}\n' $PKGS 2>/dev/null || true`, env, nil)
	if err != nil {
		return nil, fmt.Errorf("get installed packages: %w", err)
	}

	installed := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(res.Stdout)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue
		}
		// Status field is e.g. "install ok installed"
		if strings.Contains(fields[2], "installed") && !strings.Contains(fields[2], "not-installed") {
			installed[fields[0]] = fields[1]
		}
	}

	return installed, nil
}

// AptUpdate runs apt-get update to refresh the package index.
func (c *Client) AptUpdate(ctx context.Context) (*RunResult, error) {
	c.LockApt()
	defer c.UnlockApt()

	return runWithAptLockRetry(ctx, func() (*RunResult, error) {
		return c.Run(ctx, `apt-get update`, nil, nil)
	})
}

// AptUpgrade runs apt-get upgrade (or dist-upgrade) to upgrade all installed packages.
func (c *Client) AptUpgrade(ctx context.Context, distUpgrade bool) (*RunResult, error) {
	c.LockApt()
	defer c.UnlockApt()

	cmd := `DEBIAN_FRONTEND=noninteractive apt-get upgrade -y`
	if distUpgrade {
		cmd = `DEBIAN_FRONTEND=noninteractive apt-get dist-upgrade -y`
	}
	return runWithAptLockRetry(ctx, func() (*RunResult, error) {
		return c.Run(ctx, cmd, nil, nil)
	})
}

// AptInstall runs apt-get install for the given packages. Each entry in the
// packages map is name -> version. An empty version string means latest.
func (c *Client) AptInstall(ctx context.Context, packages map[string]string, update bool) error {
	if len(packages) == 0 {
		return nil
	}

	c.LockApt()
	defer c.UnlockApt()

	if update {
		_, err := runWithAptLockRetry(ctx, func() (*RunResult, error) {
			return c.Run(ctx, `apt-get update -qq`, nil, nil)
		})
		if err != nil {
			return fmt.Errorf("apt install: apt-get update: %w", err)
		}
	}

	var args []string
	envIdx := 0
	env := make(map[string]string)
	for name, version := range packages {
		pkgKey := fmt.Sprintf("PKG_%d", envIdx)
		if version != "" && version != "*" {
			verKey := fmt.Sprintf("VER_%d", envIdx)
			env[pkgKey] = name
			env[verKey] = version
			args = append(args, fmt.Sprintf(`"$%s=$%s"`, pkgKey, verKey))
		} else {
			env[pkgKey] = name
			args = append(args, fmt.Sprintf(`"$%s"`, pkgKey))
		}
		envIdx++
	}

	cmd := fmt.Sprintf(`DEBIAN_FRONTEND=noninteractive apt-get install -y -qq %s`, strings.Join(args, " "))
	_, err := runWithAptLockRetry(ctx, func() (*RunResult, error) {
		return c.Run(ctx, cmd, env, nil)
	})
	if err != nil {
		return fmt.Errorf("apt install: %w", err)
	}

	return nil
}

// AptRemove runs apt-get remove (or purge) for the given package names.
func (c *Client) AptRemove(ctx context.Context, names []string, purge bool) error {
	if len(names) == 0 {
		return nil
	}

	c.LockApt()
	defer c.UnlockApt()

	env := make(map[string]string)
	var args []string
	for i, name := range names {
		key := fmt.Sprintf("PKG_%d", i)
		env[key] = name
		args = append(args, fmt.Sprintf(`"$%s"`, key))
	}

	action := "remove"
	if purge {
		action = "purge"
	}

	cmd := fmt.Sprintf(`DEBIAN_FRONTEND=noninteractive apt-get %s -y -qq %s`, action, strings.Join(args, " "))
	_, err := runWithAptLockRetry(ctx, func() (*RunResult, error) {
		return c.Run(ctx, cmd, env, nil)
	})
	if err != nil {
		return fmt.Errorf("apt %s: %w", action, err)
	}

	return nil
}
