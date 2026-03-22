package ssh

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// User holds the metadata of a remote user as parsed from getent and id.
type User struct {
	Name   string
	UID    int
	GID    int
	Group  string
	Home   string
	Shell  string
	Groups []string
}

// CreateUserCommand describes a user to create on the remote host.
type CreateUserCommand struct {
	Name       string
	UID        *int
	GID        *int
	Group      string
	Home       string
	Shell      string
	System     bool
	CreateHome *bool
	Groups     []string
}

// CreateUser creates a new user on the remote host using useradd.
func (c *Client) CreateUser(ctx context.Context, cmd *CreateUserCommand) (*User, error) {
	if cmd.Name == "" {
		return nil, fmt.Errorf("create user: name is required")
	}

	useraddCmd := `useradd`
	env := map[string]string{"NAME": cmd.Name}

	if cmd.UID != nil {
		useraddCmd += ` --uid "$UID"`
		env["UID_VAL"] = strconv.Itoa(*cmd.UID)
		useraddCmd = strings.Replace(useraddCmd, `"$UID"`, `"$UID_VAL"`, 1)
	}
	if cmd.GID != nil {
		useraddCmd += ` --gid "$GID_VAL"`
		env["GID_VAL"] = strconv.Itoa(*cmd.GID)
	} else if cmd.Group != "" {
		useraddCmd += ` --gid "$GROUP"`
		env["GROUP"] = cmd.Group
	}
	if cmd.Home != "" {
		useraddCmd += ` --home-dir "$HOME_DIR"`
		env["HOME_DIR"] = cmd.Home
	}
	if cmd.Shell != "" {
		useraddCmd += ` --shell "$SHELL_VAL"`
		env["SHELL_VAL"] = cmd.Shell
	}
	if cmd.System {
		useraddCmd += ` --system`
	}
	if cmd.CreateHome != nil {
		if *cmd.CreateHome {
			useraddCmd += ` --create-home`
		} else {
			useraddCmd += ` --no-create-home`
		}
	}
	if len(cmd.Groups) > 0 {
		useraddCmd += ` --groups "$GROUPS"`
		env["GROUPS"] = strings.Join(cmd.Groups, ",")
	}

	useraddCmd += ` "$NAME"`

	_, err := c.Run(ctx, useraddCmd, env, nil)
	if err != nil {
		return nil, fmt.Errorf("create user: useradd %q: %w", cmd.Name, err)
	}

	return c.GetUser(ctx, cmd.Name)
}

// GetUser reads the metadata of a remote user using getent and id.
func (c *Client) GetUser(ctx context.Context, name string) (*User, error) {
	env := map[string]string{"NAME": name}

	res, err := c.Run(ctx, `getent passwd "$NAME"`, env, nil)
	if err != nil {
		if _, ok := errors.AsType[*RunError](err); ok {
			return nil, fmt.Errorf("get user: %q does not exist: %w", name, ErrNotFound)
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	user, err := parseGetentPasswd(strings.TrimSpace(string(res.Stdout)))
	if err != nil {
		return nil, err
	}

	// Get primary group name
	res, err = c.Run(ctx, `getent group "$GID"`, map[string]string{"GID": strconv.Itoa(user.GID)}, nil)
	if err != nil {
		return nil, fmt.Errorf("get user: resolve primary group for uid %d: %w", user.GID, err)
	}
	groupFields := strings.Split(strings.TrimSpace(string(res.Stdout)), ":")
	if len(groupFields) >= 1 {
		user.Group = groupFields[0]
	}

	// Get supplementary groups
	res, err = c.Run(ctx, `id -Gn "$NAME"`, env, nil)
	if err != nil {
		return nil, fmt.Errorf("get user: id -Gn %q: %w", name, err)
	}
	allGroups := strings.Fields(strings.TrimSpace(string(res.Stdout)))
	var suppGroups []string
	for _, g := range allGroups {
		if g != user.Group {
			suppGroups = append(suppGroups, g)
		}
	}
	user.Groups = suppGroups

	return user, nil
}

// parseGetentPasswd parses a single line of getent passwd output:
// name:x:uid:gid:gecos:home:shell
func parseGetentPasswd(line string) (*User, error) {
	fields := strings.Split(line, ":")
	if len(fields) < 7 {
		return nil, fmt.Errorf("get user: unexpected getent output: %q", line)
	}

	uid, err := strconv.Atoi(fields[2])
	if err != nil {
		return nil, fmt.Errorf("get user: parse uid: %w", err)
	}
	gid, err := strconv.Atoi(fields[3])
	if err != nil {
		return nil, fmt.Errorf("get user: parse gid: %w", err)
	}

	return &User{
		Name:  fields[0],
		UID:   uid,
		GID:   gid,
		Home:  fields[5],
		Shell: fields[6],
	}, nil
}

// UpdateUserCommand describes the fields to update on an existing user.
type UpdateUserCommand struct {
	Name   string
	UID    *int
	GID    *int
	Group  string
	Home   string
	Shell  string
	Groups *[]string
}

// UpdateUser modifies an existing user using usermod.
func (c *Client) UpdateUser(ctx context.Context, cmd *UpdateUserCommand) (*User, error) {
	if cmd.Name == "" {
		return nil, fmt.Errorf("update user: name is required")
	}

	usermodCmd := `usermod`
	env := map[string]string{"NAME": cmd.Name}
	hasChanges := false

	if cmd.UID != nil {
		usermodCmd += ` --uid "$UID_VAL"`
		env["UID_VAL"] = strconv.Itoa(*cmd.UID)
		hasChanges = true
	}
	if cmd.GID != nil {
		usermodCmd += ` --gid "$GID_VAL"`
		env["GID_VAL"] = strconv.Itoa(*cmd.GID)
		hasChanges = true
	} else if cmd.Group != "" {
		usermodCmd += ` --gid "$GROUP"`
		env["GROUP"] = cmd.Group
		hasChanges = true
	}
	if cmd.Home != "" {
		usermodCmd += ` --home "$HOME_DIR"`
		env["HOME_DIR"] = cmd.Home
		hasChanges = true
	}
	if cmd.Shell != "" {
		usermodCmd += ` --shell "$SHELL_VAL"`
		env["SHELL_VAL"] = cmd.Shell
		hasChanges = true
	}
	if cmd.Groups != nil {
		usermodCmd += ` --groups "$GROUPS"`
		env["GROUPS"] = strings.Join(*cmd.Groups, ",")
		hasChanges = true
	}

	if hasChanges {
		usermodCmd += ` "$NAME"`
		_, err := c.Run(ctx, usermodCmd, env, nil)
		if err != nil {
			return nil, fmt.Errorf("update user: usermod %q: %w", cmd.Name, err)
		}
	}

	return c.GetUser(ctx, cmd.Name)
}

// DeleteUser removes a user using userdel.
func (c *Client) DeleteUser(ctx context.Context, name string) error {
	env := map[string]string{"NAME": name}

	_, err := c.Run(ctx, `userdel "$NAME"`, env, nil)
	if err != nil {
		return fmt.Errorf("delete user: userdel %q: %w", name, err)
	}

	return nil
}
