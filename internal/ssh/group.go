package ssh

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Group holds the metadata of a remote group as returned by getent.
type Group struct {
	Name    string
	GID     int
	Members []string
}

// CreateGroupCommand describes a group to create on the remote host.
type CreateGroupCommand struct {
	Name   string
	GID    *int
	System bool
}

// CreateGroup creates a new group on the remote host using groupadd.
func (c *Client) CreateGroup(ctx context.Context, cmd *CreateGroupCommand) (*Group, error) {
	if cmd.Name == "" {
		return nil, fmt.Errorf("create group: name is required")
	}

	groupaddCmd := `groupadd`
	env := map[string]string{"NAME": cmd.Name}

	if cmd.GID != nil {
		groupaddCmd += ` --gid "$GID"`
		env["GID"] = strconv.Itoa(*cmd.GID)
	}
	if cmd.System {
		groupaddCmd += ` --system`
	}
	groupaddCmd += ` "$NAME"`

	_, err := c.Run(ctx, groupaddCmd, env, nil)
	if err != nil {
		return nil, fmt.Errorf("create group: groupadd %q: %w", cmd.Name, err)
	}

	return c.GetGroup(ctx, cmd.Name)
}

// GetGroup reads the metadata of a remote group using getent.
func (c *Client) GetGroup(ctx context.Context, name string) (*Group, error) {
	env := map[string]string{"NAME": name}

	res, err := c.Run(ctx, `getent group "$NAME"`, env, nil)
	if err != nil {
		if _, ok := errors.AsType[*RunError](err); ok {
			return nil, fmt.Errorf("get group: %q does not exist: %w", name, ErrNotFound)
		}
		return nil, fmt.Errorf("get group: %w", err)
	}

	return parseGetentGroup(strings.TrimSpace(string(res.Stdout)))
}

// parseGetentGroup parses a single line of getent group output:
// name:x:gid:member1,member2,...
func parseGetentGroup(line string) (*Group, error) {
	fields := strings.Split(line, ":")
	if len(fields) < 3 {
		return nil, fmt.Errorf("get group: unexpected getent output: %q", line)
	}

	gid, err := strconv.Atoi(fields[2])
	if err != nil {
		return nil, fmt.Errorf("get group: parse gid: %w", err)
	}

	var members []string
	if len(fields) >= 4 && fields[3] != "" {
		members = strings.Split(fields[3], ",")
	}

	return &Group{
		Name:    fields[0],
		GID:     gid,
		Members: members,
	}, nil
}

// UpdateGroup modifies a group's GID using groupmod.
func (c *Client) UpdateGroup(ctx context.Context, name string, gid *int) (*Group, error) {
	if gid != nil {
		env := map[string]string{
			"NAME": name,
			"GID":  strconv.Itoa(*gid),
		}
		_, err := c.Run(ctx, `groupmod --gid "$GID" "$NAME"`, env, nil)
		if err != nil {
			return nil, fmt.Errorf("update group: groupmod %q: %w", name, err)
		}
	}

	return c.GetGroup(ctx, name)
}

// SetGroupMembers sets the full member list of a group using gpasswd -M.
// An empty slice removes all members.
func (c *Client) SetGroupMembers(ctx context.Context, name string, members []string) error {
	env := map[string]string{
		"NAME":    name,
		"MEMBERS": strings.Join(members, ","),
	}

	_, err := c.Run(ctx, `gpasswd -M "$MEMBERS" "$NAME"`, env, nil)
	if err != nil {
		return fmt.Errorf("set group members: gpasswd %q: %w", name, err)
	}

	return nil
}

// AddGroupMember adds a single user to a group using gpasswd -a.
func (c *Client) AddGroupMember(ctx context.Context, group, user string) error {
	env := map[string]string{
		"GROUP": group,
		"USER":  user,
	}

	_, err := c.Run(ctx, `gpasswd -a "$USER" "$GROUP"`, env, nil)
	if err != nil {
		return fmt.Errorf("add group member: gpasswd -a %q %q: %w", user, group, err)
	}

	return nil
}

// RemoveGroupMember removes a single user from a group using gpasswd -d.
func (c *Client) RemoveGroupMember(ctx context.Context, group, user string) error {
	env := map[string]string{
		"GROUP": group,
		"USER":  user,
	}

	_, err := c.Run(ctx, `gpasswd -d "$USER" "$GROUP"`, env, nil)
	if err != nil {
		return fmt.Errorf("remove group member: gpasswd -d %q %q: %w", user, group, err)
	}

	return nil
}

// DeleteGroup removes a group using groupdel.
func (c *Client) DeleteGroup(ctx context.Context, name string) error {
	env := map[string]string{"NAME": name}

	_, err := c.Run(ctx, `groupdel "$NAME"`, env, nil)
	if err != nil {
		return fmt.Errorf("delete group: groupdel %q: %w", name, err)
	}

	return nil
}
