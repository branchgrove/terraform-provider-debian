package ssh

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func verifyDirectoryFields(t *testing.T, client *Client, dir *Directory) {
	t.Helper()

	assert.Equal(t, dir.Path, fmt.Sprintf("%s/%s", dir.Dirname, dir.Basename),
		"Path should equal Dirname/Basename")

	gotOwner := hostRun(t, client, fmt.Sprintf("stat -c '%%U' %s", dir.Path))
	assert.Equal(t, gotOwner, dir.User, "Owner mismatch")

	gotGroup := hostRun(t, client, fmt.Sprintf("stat -c '%%G' %s", dir.Path))
	assert.Equal(t, gotGroup, dir.Group, "Group mismatch")

	gotUID := hostRun(t, client, fmt.Sprintf("stat -c '%%u' %s", dir.Path))
	assert.Equal(t, gotUID, fmt.Sprintf("%d", dir.UID), "UID mismatch")

	gotGID := hostRun(t, client, fmt.Sprintf("stat -c '%%g' %s", dir.Path))
	assert.Equal(t, gotGID, fmt.Sprintf("%d", dir.GID), "GID mismatch")

	gotMode := hostRun(t, client, fmt.Sprintf("stat -c '%%a' %s", dir.Path))
	assert.Equal(t, fmt.Sprintf("0%s", gotMode), dir.Mode, "Mode mismatch")

	gotBasename := hostRun(t, client, fmt.Sprintf("basename %s", dir.Path))
	assert.Equal(t, gotBasename, dir.Basename, "Basename mismatch")

	gotDirname := hostRun(t, client, fmt.Sprintf("dirname %s", dir.Path))
	assert.Equal(t, gotDirname, dir.Dirname, "Dirname mismatch")
}

func TestMakeDirectory(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	t.Run("basic with mode", func(t *testing.T) {
		dirPath := "/tmp/test_mkdir_basic"
		t.Cleanup(func() { client.DeleteDirectory(ctx, dirPath) })

		dir, err := client.MakeDirectory(ctx, &MakeDirectoryCommand{
			Path: dirPath,
			Mode: "0755",
		})
		require.NoError(t, err)

		assert.Equal(t, dirPath, dir.Path)
		assert.Equal(t, "test_mkdir_basic", dir.Basename)
		assert.Equal(t, "/tmp", dir.Dirname)
		assert.Equal(t, "0755", dir.Mode)

		verifyDirectoryFields(t, client, dir)
	})

	t.Run("restrictive mode", func(t *testing.T) {
		dirPath := "/tmp/test_mkdir_mode700"
		t.Cleanup(func() { client.DeleteDirectory(ctx, dirPath) })

		dir, err := client.MakeDirectory(ctx, &MakeDirectoryCommand{
			Path: dirPath,
			Mode: "0700",
		})
		require.NoError(t, err)

		assert.Equal(t, "0700", dir.Mode)
		verifyDirectoryFields(t, client, dir)
	})

	t.Run("owner and group by name", func(t *testing.T) {
		dirPath := "/tmp/test_mkdir_owner_name"
		t.Cleanup(func() { client.DeleteDirectory(ctx, dirPath) })

		dir, err := client.MakeDirectory(ctx, &MakeDirectoryCommand{
			Path:  dirPath,
			Mode:  "0755",
			User:  "nobody",
			Group: "nogroup",
		})
		require.NoError(t, err)

		assert.Equal(t, "nobody", dir.User)
		assert.Equal(t, "nogroup", dir.Group)
		verifyDirectoryFields(t, client, dir)
	})

	t.Run("create parents", func(t *testing.T) {
		dirPath := "/tmp/test_mkdir_nested/sub/dir"
		t.Cleanup(func() {
			client.Run(ctx, "rm -rf /tmp/test_mkdir_nested", nil, nil)
		})

		dir, err := client.MakeDirectory(ctx, &MakeDirectoryCommand{
			Path:          dirPath,
			Mode:          "0755",
			CreateParents: true,
		})
		require.NoError(t, err)

		assert.Equal(t, dirPath, dir.Path)
		assert.Equal(t, "dir", dir.Basename)
		assert.Equal(t, "/tmp/test_mkdir_nested/sub", dir.Dirname)
		verifyDirectoryFields(t, client, dir)
	})

	t.Run("without create_parents fails for nested", func(t *testing.T) {
		dirPath := "/tmp/test_mkdir_no_parents/sub/dir"

		_, err := client.MakeDirectory(ctx, &MakeDirectoryCommand{
			Path: dirPath,
			Mode: "0755",
		})
		assert.Error(t, err)
		assert.ErrorContains(t, err, "mkdir")
	})
}

func TestGetDirectory(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	t.Run("returns all fields", func(t *testing.T) {
		dirPath := "/tmp/test_getdir_all"
		t.Cleanup(func() { client.DeleteDirectory(ctx, dirPath) })

		_, err := client.MakeDirectory(ctx, &MakeDirectoryCommand{
			Path: dirPath,
			Mode: "0750",
		})
		require.NoError(t, err)

		dir, err := client.GetDirectory(ctx, dirPath)
		require.NoError(t, err)

		assert.Equal(t, dirPath, dir.Path)
		assert.Equal(t, "test_getdir_all", dir.Basename)
		assert.Equal(t, "/tmp", dir.Dirname)
		assert.Equal(t, "0750", dir.Mode)
		verifyDirectoryFields(t, client, dir)
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		_, err := client.GetDirectory(ctx, "/tmp/test_getdir_nonexistent")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("rejects regular file", func(t *testing.T) {
		filePath := "/tmp/test_getdir_file"
		t.Cleanup(func() { client.DeleteFile(ctx, filePath) })

		hostRun(t, client, fmt.Sprintf("touch %s", filePath))

		_, err := client.GetDirectory(ctx, filePath)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestDeleteDirectory(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	t.Run("removes empty directory", func(t *testing.T) {
		dirPath := "/tmp/test_rmdir_basic"

		_, err := client.MakeDirectory(ctx, &MakeDirectoryCommand{
			Path: dirPath,
			Mode: "0755",
		})
		require.NoError(t, err)

		err = client.DeleteDirectory(ctx, dirPath)
		require.NoError(t, err)

		_, err = client.GetDirectory(ctx, dirPath)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("fails on non-empty directory", func(t *testing.T) {
		dirPath := "/tmp/test_rmdir_notempty"
		t.Cleanup(func() {
			client.Run(ctx, "rm -rf /tmp/test_rmdir_notempty", nil, nil)
		})

		hostRun(t, client, fmt.Sprintf("mkdir -p %s", dirPath))
		hostRun(t, client, fmt.Sprintf("touch %s/file", dirPath))

		err := client.DeleteDirectory(ctx, dirPath)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "rmdir")
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		err := client.DeleteDirectory(ctx, "/tmp/test_rmdir_nonexistent")
		assert.Error(t, err)
		assert.ErrorContains(t, err, "not a directory")
	})
}

func TestMakeDirectoryValidation(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	t.Run("missing path", func(t *testing.T) {
		_, err := client.MakeDirectory(ctx, &MakeDirectoryCommand{})
		assert.ErrorContains(t, err, "path is required")
	})

	t.Run("owner and uid conflict", func(t *testing.T) {
		uid := 1000
		_, err := client.MakeDirectory(ctx, &MakeDirectoryCommand{
			Path: "/tmp/x",
			User: "root",
			UID:  &uid,
		})
		assert.ErrorContains(t, err, "user and uid cannot be set at the same time")
	})

	t.Run("group and gid conflict", func(t *testing.T) {
		gid := 1000
		_, err := client.MakeDirectory(ctx, &MakeDirectoryCommand{
			Path:  "/tmp/x",
			Group: "root",
			GID:   &gid,
		})
		assert.ErrorContains(t, err, "group and gid cannot be set at the same time")
	})

	t.Run("invalid mode", func(t *testing.T) {
		_, err := client.MakeDirectory(ctx, &MakeDirectoryCommand{
			Path: "/tmp/x",
			Mode: "not-a-mode",
		})
		assert.ErrorContains(t, err, "invalid mode")
	})
}
