package ssh

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
)

// MakeDirectoryCommand describes a directory to create on the remote host.
type MakeDirectoryCommand struct {
	Path          string
	User          string
	Group         string
	UID           *int
	GID           *int
	Mode          string
	CreateParents bool
}

// Validate checks required fields and mutual exclusivity of name/ID pairs.
func (c *MakeDirectoryCommand) Validate() error {
	if c.Path == "" {
		return fmt.Errorf("path is required")
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

// Directory holds the metadata of a remote directory as returned by stat.
type Directory struct {
	Path     string
	User     string
	Group    string
	Mode     string
	UID      int
	GID      int
	Basename string
	Dirname  string
}

// MakeDirectory creates a directory on the remote host. When CreateParents is
// true, intermediate directories are created as needed (mkdir -p). Ownership
// and permissions are applied after creation.
func (c *Client) MakeDirectory(ctx context.Context, cmd *MakeDirectoryCommand) (*Directory, error) {
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("make directory: %w", err)
	}

	env := map[string]string{"DIR": cmd.Path}

	mkdirCmd := `mkdir "$DIR"`
	if cmd.CreateParents {
		mkdirCmd = `mkdir -p "$DIR"`
	}

	_, err := c.Run(ctx, mkdirCmd, env, nil)
	if err != nil {
		return nil, fmt.Errorf("make directory: mkdir %q: %w", cmd.Path, err)
	}

	ownerSpec := chownSpec(cmd.User, cmd.Group, cmd.UID, cmd.GID)
	if ownerSpec != "" {
		_, err = c.Run(ctx, `chown "$OWNER" "$DIR"`, map[string]string{"OWNER": ownerSpec, "DIR": cmd.Path}, nil)
		if err != nil {
			return nil, fmt.Errorf("make directory: chown %q: %w", cmd.Path, err)
		}
	}

	if cmd.Mode != "" {
		_, err = c.Run(ctx, `chmod "$MODE" "$DIR"`, map[string]string{"MODE": cmd.Mode, "DIR": cmd.Path}, nil)
		if err != nil {
			return nil, fmt.Errorf("make directory: chmod %q: %w", cmd.Path, err)
		}
	}

	return c.GetDirectory(ctx, cmd.Path)
}

// GetDirectory reads the metadata of a remote directory.
func (c *Client) GetDirectory(ctx context.Context, dirPath string) (*Directory, error) {
	env := map[string]string{"DIR": dirPath}

	_, err := c.Run(ctx, `test -d "$DIR"`, env, nil)
	if err != nil {
		if _, ok := errors.AsType[*RunError](err); ok {
			return nil, fmt.Errorf("get directory: %q does not exist or is not a directory: %w", dirPath, ErrNotFound)
		}
		return nil, fmt.Errorf("get directory: %w", err)
	}

	res, err := c.Run(ctx, "stat -c '%U\n%G\n%u\n%g\n%a' \"$DIR\"", env, nil)
	if err != nil {
		return nil, fmt.Errorf("get directory: stat %q: %w", dirPath, err)
	}

	fields := strings.Split(strings.TrimSpace(string(res.Stdout)), "\n")
	if len(fields) != 5 {
		return nil, fmt.Errorf("get directory: stat %q: unexpected output (got %d fields, want 5)", dirPath, len(fields))
	}

	owner := fields[0]
	group := fields[1]
	uid, err := strconv.Atoi(fields[2])
	if err != nil {
		return nil, fmt.Errorf("get directory: stat %q: parse uid: %w", dirPath, err)
	}
	gid, err := strconv.Atoi(fields[3])
	if err != nil {
		return nil, fmt.Errorf("get directory: stat %q: parse gid: %w", dirPath, err)
	}
	modeNum, err := strconv.ParseUint(fields[4], 8, 32)
	if err != nil {
		return nil, fmt.Errorf("get directory: stat %q: parse mode: %w", dirPath, err)
	}
	mode := fmt.Sprintf("%04o", modeNum)

	return &Directory{
		Path:     dirPath,
		User:     owner,
		Group:    group,
		Mode:     mode,
		UID:      uid,
		GID:      gid,
		Basename: path.Base(dirPath),
		Dirname:  path.Dir(dirPath),
	}, nil
}

// UpdateDirectory applies ownership and permission changes to an existing directory.
func (c *Client) UpdateDirectory(ctx context.Context, cmd *MakeDirectoryCommand) (*Directory, error) {
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("update directory: %w", err)
	}

	env := map[string]string{"DIR": cmd.Path}

	_, err := c.Run(ctx, `test -d "$DIR"`, env, nil)
	if err != nil {
		if _, ok := errors.AsType[*RunError](err); ok {
			return nil, fmt.Errorf("update directory: %q does not exist or is not a directory: %w", cmd.Path, ErrNotFound)
		}
		return nil, fmt.Errorf("update directory: %w", err)
	}

	ownerSpec := chownSpec(cmd.User, cmd.Group, cmd.UID, cmd.GID)
	if ownerSpec != "" {
		_, err = c.Run(ctx, `chown "$OWNER" "$DIR"`, map[string]string{"OWNER": ownerSpec, "DIR": cmd.Path}, nil)
		if err != nil {
			return nil, fmt.Errorf("update directory: chown %q: %w", cmd.Path, err)
		}
	}

	if cmd.Mode != "" {
		_, err = c.Run(ctx, `chmod "$MODE" "$DIR"`, map[string]string{"MODE": cmd.Mode, "DIR": cmd.Path}, nil)
		if err != nil {
			return nil, fmt.Errorf("update directory: chmod %q: %w", cmd.Path, err)
		}
	}

	return c.GetDirectory(ctx, cmd.Path)
}

// DeleteDirectory removes an empty directory on the remote host using rmdir.
// This intentionally does not use rm -rf to prevent accidental recursive
// deletion of directory trees.
func (c *Client) DeleteDirectory(ctx context.Context, dirPath string) error {
	env := map[string]string{"DIR": dirPath}

	_, err := c.Run(ctx, `test -e "$DIR"`, env, nil)
	if err != nil {
		if _, ok := errors.AsType[*RunError](err); ok {
			// Does not exist, consider it already deleted
			return nil
		}
		return fmt.Errorf("delete directory: check existence: %w", err)
	}

	_, err = c.Run(ctx, `test -d "$DIR"`, env, nil)
	if err != nil {
		if _, ok := errors.AsType[*RunError](err); ok {
			return fmt.Errorf("delete directory: %q is not a directory", dirPath)
		}
		return fmt.Errorf("delete directory: %w", err)
	}

	_, err = c.Run(ctx, `rmdir "$DIR"`, env, nil)
	if err != nil {
		return fmt.Errorf("delete directory: rmdir %q: %w", dirPath, err)
	}
	return nil
}
