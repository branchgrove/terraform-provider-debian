package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

var ErrNotFound = errors.New("not found")

// PutFileCommand describes a file to create or overwrite on the remote host.
// Owner/group can be specified by name or numeric ID but not both.
type PutFileCommand struct {
	// Absolute path to the file
	Path string
	// Name of the user that owns the file, conflicts with UID
	User string
	// Name of the group of the file, conflicts with GID
	Group string
	// Numeric user ID of the file owner, conflicts with User
	UID *int
	// Numeric group ID of the file group, conflicts with Group
	GID *int
	// File permission mode, e.g. "0644"
	Mode string
	// Content of the file
	Content io.Reader
	// Create parent directories if they don't exist
	CreateDirectories bool
}

// Validate checks required fields and mutual exclusivity of name/ID pairs.
func (c *PutFileCommand) Validate() error {
	if c.Path == "" {
		return fmt.Errorf("path is required")
	}

	if c.Content == nil {
		return fmt.Errorf("content is required")
	}

	if c.User != "" && c.UID != nil {
		return fmt.Errorf("user and uid cannot be set at the same time")
	}

	if c.Group != "" && c.GID != nil {
		return fmt.Errorf("group and gid cannot be set at the same time")
	}

	if c.Mode != "" {
		_, err := strconv.ParseUint(c.Mode, 8, 32)
		if err != nil {
			return fmt.Errorf("invalid mode: %w", err)
		}
	}

	return nil
}

// File holds the metadata of a remote file as returned by stat and sha256sum.
type File struct {
	Path     string
	User     string
	Group    string
	Mode     string
	UID      int
	GID      int
	Basename string
	Dirname  string
	SHA256   string
	Size     int
}

// PutFile creates or overwrites a file on the remote host using SSH commands
// (dd, chown, chmod, mv) rather than SFTP, keeping a single transport
// mechanism. The write is atomic: content goes to a temp file in the same
// directory, attributes are applied there, and the temp file is renamed into
// place so a partial failure never corrupts the target. Arguments are passed
// through env vars to avoid shell injection.
func (c *Client) PutFile(ctx context.Context, cmd *PutFileCommand) (*File, error) {
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("put file: %w", err)
	}

	dir := path.Dir(cmd.Path)
	pathEnv := map[string]string{"FILE": cmd.Path}

	if cmd.CreateDirectories {
		_, err := c.Run(ctx, `mkdir -p "$DIR"`, map[string]string{"DIR": dir}, nil)
		if err != nil {
			return nil, fmt.Errorf("put file: mkdir %q: %w", dir, err)
		}
	}

	// Reject symlinks, directories, and device nodes up front so we don't
	// follow a symlink to an unintended target or block on a device open.
	_, err := c.Run(ctx, `test ! -e "$FILE" || test -f "$FILE"`, pathEnv, nil)
	if err != nil {
		if _, ok := errors.AsType[*RunError](err); ok {
			return nil, fmt.Errorf("put file: %q exists but is not a regular file", cmd.Path)
		}
		return nil, fmt.Errorf("put file: check %q: %w", cmd.Path, err)
	}

	// Write to a temp file in the same directory, apply attrs, then atomically
	// rename into place so that a partial failure never corrupts the target.
	res, err := c.Run(ctx, `mktemp -p "$DIR" .tmp.XXXXXXXXXX`, map[string]string{"DIR": dir}, nil)
	if err != nil {
		return nil, fmt.Errorf("put file: mktemp: %w", err)
	}
	tmpPath := strings.TrimSpace(string(res.Stdout))
	tmpEnv := map[string]string{"TMP": tmpPath}
	defer func() { _, _ = c.Run(ctx, `rm -f "$TMP"`, tmpEnv, nil) }()

	_, err = c.Run(ctx, `dd of="$TMP" status=none`, tmpEnv, cmd.Content)
	if err != nil {
		return nil, fmt.Errorf("put file: write %q: %w", cmd.Path, err)
	}

	ownerSpec := chownSpec(cmd.User, cmd.Group, cmd.UID, cmd.GID)
	if ownerSpec != "" {
		_, err = c.Run(ctx, `chown "$OWNER" "$TMP"`, map[string]string{"OWNER": ownerSpec, "TMP": tmpPath}, nil)
		if err != nil {
			return nil, fmt.Errorf("put file: chown %q: %w", cmd.Path, err)
		}
	}

	if cmd.Mode != "" {
		_, err = c.Run(ctx, `chmod "$MODE" "$TMP"`, map[string]string{"MODE": cmd.Mode, "TMP": tmpPath}, nil)
		if err != nil {
			return nil, fmt.Errorf("put file: chmod %q: %w", cmd.Path, err)
		}
	}

	_, err = c.Run(ctx, `mv "$TMP" "$FILE"`, map[string]string{"TMP": tmpPath, "FILE": cmd.Path}, nil)
	if err != nil {
		return nil, fmt.Errorf("put file: rename %q: %w", cmd.Path, err)
	}

	return c.GetFile(ctx, cmd.Path)
}

// GetFile reads the metadata of a remote file. It collects owner, group,
// uid, gid, mode, and size in a single stat call, then hashes the content
// with sha256sum.
func (c *Client) GetFile(ctx context.Context, filePath string) (*File, error) {
	env := map[string]string{"FILE": filePath}

	_, err := c.Run(ctx, `test -f "$FILE"`, env, nil)
	if err != nil {
		if _, ok := errors.AsType[*RunError](err); ok {
			return nil, fmt.Errorf("get file: %q does not exist or is not a regular file: %w", filePath, ErrNotFound)
		}
		return nil, fmt.Errorf("get file: %w", err)
	}

	res, err := c.Run(ctx, "stat -c '%U\n%G\n%u\n%g\n%a\n%s' \"$FILE\"", env, nil)
	if err != nil {
		return nil, fmt.Errorf("get file: stat %q: %w", filePath, err)
	}

	fields := strings.Split(strings.TrimSpace(string(res.Stdout)), "\n")
	if len(fields) != 6 {
		return nil, fmt.Errorf("get file: stat %q: unexpected output (got %d fields, want 6)", filePath, len(fields))
	}

	owner := fields[0]
	group := fields[1]
	uid, err := strconv.Atoi(fields[2])
	if err != nil {
		return nil, fmt.Errorf("get file: stat %q: parse uid: %w", filePath, err)
	}
	gid, err := strconv.Atoi(fields[3])
	if err != nil {
		return nil, fmt.Errorf("get file: stat %q: parse gid: %w", filePath, err)
	}
	modeNum, err := strconv.ParseUint(fields[4], 8, 32)
	if err != nil {
		return nil, fmt.Errorf("get file: stat %q: parse mode: %w", filePath, err)
	}
	mode := fmt.Sprintf("%04o", modeNum)
	size, err := strconv.Atoi(fields[5])
	if err != nil {
		return nil, fmt.Errorf("get file: stat %q: parse size: %w", filePath, err)
	}

	res, err = c.Run(ctx, `sha256sum "$FILE"`, env, nil)
	if err != nil {
		return nil, fmt.Errorf("get file: sha256sum %q: %w", filePath, err)
	}
	sha256Fields := strings.Fields(strings.TrimSpace(string(res.Stdout)))
	if len(sha256Fields) == 0 {
		return nil, fmt.Errorf("get file: sha256sum %q: unexpected empty output", filePath)
	}
	sha256 := sha256Fields[0]

	return &File{
		Path:     filePath,
		User:     owner,
		Group:    group,
		Mode:     mode,
		UID:      uid,
		GID:      gid,
		Basename: path.Base(filePath),
		Dirname:  path.Dir(filePath),
		SHA256:   sha256,
		Size:     size,
	}, nil
}

// DeleteFile removes a regular file on the remote host. It refuses to operate
// on non-regular files (directories, symlinks, devices).
func (c *Client) DeleteFile(ctx context.Context, filePath string) error {
	env := map[string]string{"FILE": filePath}

	_, err := c.Run(ctx, `test -f "$FILE"`, env, nil)
	if err != nil {
		if _, ok := errors.AsType[*RunError](err); ok {
			return fmt.Errorf("delete file: %q does not exist or is not a regular file", filePath)
		}
		return fmt.Errorf("delete file: %w", err)
	}

	_, err = c.Run(ctx, `rm "$FILE"`, env, nil)
	if err != nil {
		return fmt.Errorf("delete file: remove %q: %w", filePath, err)
	}
	return nil
}

// chownSpec builds a chown OWNER[:GROUP] string from name or numeric ID
// fields, returning "" when neither owner nor group is set.
func chownSpec(owner, group string, uid, gid *int) string {
	var ownerPart, groupPart string
	if owner != "" {
		ownerPart = owner
	} else if uid != nil {
		ownerPart = strconv.Itoa(*uid)
	}
	if group != "" {
		groupPart = group
	} else if gid != nil {
		groupPart = strconv.Itoa(*gid)
	}

	switch {
	case ownerPart != "" && groupPart != "":
		return ownerPart + ":" + groupPart
	case ownerPart != "":
		return ownerPart
	case groupPart != "":
		return ":" + groupPart
	default:
		return ""
	}
}
