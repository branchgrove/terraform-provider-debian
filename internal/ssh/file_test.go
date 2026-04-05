package ssh

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	port = 22
	user = "root"
)

func testClient(t *testing.T) *Client {
	t.Helper()
	manager := NewManager()
	t.Cleanup(func() { manager.Close() })

	host := os.Getenv("TEST_SSH_HOST")
	require.NotEmpty(t, host, "TEST_SSH_HOST is not set")
	privateKey := os.Getenv("TEST_SSH_PRIVATE_KEY")
	require.NotEmpty(t, privateKey, "TEST_SSH_PRIVATE_KEY is not set")

	auth, err := PrivateKeyAuth(privateKey)
	require.NoError(t, err)

	client, err := manager.GetClient(context.Background(), host, port, user, auth, "")
	require.NoError(t, err)

	return client
}

func hostRun(t *testing.T, client *Client, cmd string) string {
	t.Helper()
	res, err := client.Run(context.Background(), cmd, nil, nil)
	require.NoError(t, err, "command: %s", cmd)
	require.Equal(t, 0, res.ExitCode, "command %q: stderr: %s", cmd, string(res.Stderr))
	return strings.TrimSpace(string(res.Stdout))
}

func verifyFileFields(t *testing.T, client *Client, file *File) {
	t.Helper()

	assert.Equal(t, file.Path, fmt.Sprintf("%s/%s", file.Dirname, file.Basename),
		"Path should equal Dirname/Basename")

	gotOwner := hostRun(t, client, fmt.Sprintf("stat -c '%%U' %s", file.Path))
	assert.Equal(t, gotOwner, file.User, "Owner mismatch")

	gotGroup := hostRun(t, client, fmt.Sprintf("stat -c '%%G' %s", file.Path))
	assert.Equal(t, gotGroup, file.Group, "Group mismatch")

	gotUID := hostRun(t, client, fmt.Sprintf("stat -c '%%u' %s", file.Path))
	assert.Equal(t, gotUID, fmt.Sprintf("%d", file.UID), "UID mismatch")

	gotGID := hostRun(t, client, fmt.Sprintf("stat -c '%%g' %s", file.Path))
	assert.Equal(t, gotGID, fmt.Sprintf("%d", file.GID), "GID mismatch")

	gotMode := hostRun(t, client, fmt.Sprintf("stat -c '%%a' %s", file.Path))
	assert.Equal(t, fmt.Sprintf("0%s", gotMode), file.Mode, "Mode mismatch")

	gotSHA := strings.Fields(hostRun(t, client, fmt.Sprintf("sha256sum %s", file.Path)))[0]
	assert.Equal(t, gotSHA, file.SHA256, "SHA256 mismatch")

	gotSize := hostRun(t, client, fmt.Sprintf("stat -c '%%s' %s", file.Path))
	assert.Equal(t, gotSize, fmt.Sprintf("%d", file.Size), "Size mismatch")

	gotBasename := hostRun(t, client, fmt.Sprintf("basename %s", file.Path))
	assert.Equal(t, gotBasename, file.Basename, "Basename mismatch")

	gotDirname := hostRun(t, client, fmt.Sprintf("dirname %s", file.Path))
	assert.Equal(t, gotDirname, file.Dirname, "Dirname mismatch")
}

func TestPutFile(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	t.Run("basic with mode", func(t *testing.T) {
		filePath := "/tmp/test_putfile_basic"
		content := "hello world\n"
		t.Cleanup(func() { client.DeleteFile(ctx, filePath) })

		file, err := client.PutFile(ctx, &PutFileCommand{
			Path:    filePath,
			Content: strings.NewReader(content),
			Mode:    "0644",
		})
		require.NoError(t, err)

		assert.Equal(t, filePath, file.Path)
		assert.Equal(t, "test_putfile_basic", file.Basename)
		assert.Equal(t, "/tmp", file.Dirname)
		assert.Equal(t, "0644", file.Mode)
		assert.Equal(t, len(content), file.Size)

		gotContent := hostRun(t, client, fmt.Sprintf("cat %s", filePath))
		assert.Equal(t, strings.TrimSpace(content), gotContent)

		verifyFileFields(t, client, file)
	})

	t.Run("restrictive mode", func(t *testing.T) {
		filePath := "/tmp/test_putfile_mode600"
		content := "secret data"
		t.Cleanup(func() { client.DeleteFile(ctx, filePath) })

		file, err := client.PutFile(ctx, &PutFileCommand{
			Path:    filePath,
			Content: strings.NewReader(content),
			Mode:    "0600",
		})
		require.NoError(t, err)

		assert.Equal(t, "0600", file.Mode)
		verifyFileFields(t, client, file)
	})

	t.Run("executable mode", func(t *testing.T) {
		filePath := "/tmp/test_putfile_mode755"
		content := "#!/bin/sh\necho hi\n"
		t.Cleanup(func() { client.DeleteFile(ctx, filePath) })

		file, err := client.PutFile(ctx, &PutFileCommand{
			Path:    filePath,
			Content: strings.NewReader(content),
			Mode:    "0755",
		})
		require.NoError(t, err)

		assert.Equal(t, "0755", file.Mode)
		verifyFileFields(t, client, file)
	})

	t.Run("owner and group by name", func(t *testing.T) {
		filePath := "/tmp/test_putfile_owner_name"
		content := "owned by name"
		t.Cleanup(func() { client.DeleteFile(ctx, filePath) })

		// "nobody" and "nogroup" exist on Debian
		file, err := client.PutFile(ctx, &PutFileCommand{
			Path:    filePath,
			Content: strings.NewReader(content),
			Mode:    "0644",
			User:    "nobody",
			Group:   "nogroup",
		})
		require.NoError(t, err)

		assert.Equal(t, "nobody", file.User)
		assert.Equal(t, "nogroup", file.Group)

		verifyFileFields(t, client, file)
	})

	t.Run("owner and group by id", func(t *testing.T) {
		filePath := "/tmp/test_putfile_owner_id"
		content := "owned by uid/gid"
		t.Cleanup(func() { client.DeleteFile(ctx, filePath) })

		// UID/GID 0 is skipped by PutFile (treated as "not set"), so use nobody's IDs.
		nobodyUID := hostRun(t, client, "id -u nobody")
		nobodyGID := hostRun(t, client, "getent group nogroup | cut -d: -f3")

		var uid, gid int
		fmt.Sscanf(nobodyUID, "%d", &uid)
		fmt.Sscanf(nobodyGID, "%d", &gid)

		file, err := client.PutFile(ctx, &PutFileCommand{
			Path:    filePath,
			Content: strings.NewReader(content),
			Mode:    "0644",
			UID:     &uid,
			GID:     &gid,
		})
		require.NoError(t, err)

		assert.Equal(t, uid, file.UID)
		assert.Equal(t, gid, file.GID)
		assert.Equal(t, "nobody", file.User)
		assert.Equal(t, "nogroup", file.Group)

		verifyFileFields(t, client, file)
	})

	t.Run("create directories", func(t *testing.T) {
		filePath := "/tmp/test_putfile_nested/sub/dir/file.txt"
		content := "nested content"
		t.Cleanup(func() {
			client.DeleteFile(ctx, filePath)
			client.Run(ctx, "rm -rf /tmp/test_putfile_nested", nil, nil)
		})

		file, err := client.PutFile(ctx, &PutFileCommand{
			Path:              filePath,
			Content:           strings.NewReader(content),
			Mode:              "0644",
			CreateDirectories: true,
		})
		require.NoError(t, err)

		assert.Equal(t, filePath, file.Path)
		assert.Equal(t, "file.txt", file.Basename)
		assert.Equal(t, "/tmp/test_putfile_nested/sub/dir", file.Dirname)

		hostRun(t, client, fmt.Sprintf("test -f %s", filePath))
		hostRun(t, client, "test -d /tmp/test_putfile_nested/sub/dir")

		verifyFileFields(t, client, file)
	})

	t.Run("empty content", func(t *testing.T) {
		filePath := "/tmp/test_putfile_empty"
		t.Cleanup(func() { client.DeleteFile(ctx, filePath) })

		file, err := client.PutFile(ctx, &PutFileCommand{
			Path:    filePath,
			Content: strings.NewReader(""),
			Mode:    "0644",
		})
		require.NoError(t, err)

		assert.Equal(t, 0, file.Size)
		verifyFileFields(t, client, file)
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		filePath := "/tmp/test_putfile_overwrite"
		t.Cleanup(func() { client.DeleteFile(ctx, filePath) })

		_, err := client.PutFile(ctx, &PutFileCommand{
			Path:    filePath,
			Content: strings.NewReader("first version"),
			Mode:    "0644",
		})
		require.NoError(t, err)

		file, err := client.PutFile(ctx, &PutFileCommand{
			Path:    filePath,
			Content: strings.NewReader("second version"),
			Mode:    "0600",
		})
		require.NoError(t, err)

		gotContent := hostRun(t, client, fmt.Sprintf("cat %s", filePath))
		assert.Equal(t, "second version", gotContent)
		assert.Equal(t, "0600", file.Mode)
		assert.Equal(t, len("second version"), file.Size)

		verifyFileFields(t, client, file)
	})
}

func TestGetFile(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	t.Run("returns all fields", func(t *testing.T) {
		filePath := "/tmp/test_getfile_all"
		content := "getfile test content\n"
		t.Cleanup(func() { client.DeleteFile(ctx, filePath) })

		_, err := client.PutFile(ctx, &PutFileCommand{
			Path:    filePath,
			Content: strings.NewReader(content),
			Mode:    "0640",
		})
		require.NoError(t, err)

		file, err := client.GetFile(ctx, filePath)
		require.NoError(t, err)

		assert.Equal(t, filePath, file.Path)
		assert.Equal(t, "test_getfile_all", file.Basename)
		assert.Equal(t, "/tmp", file.Dirname)
		assert.Equal(t, "0640", file.Mode)
		assert.Equal(t, len(content), file.Size)

		verifyFileFields(t, client, file)
	})

	t.Run("reflects external changes", func(t *testing.T) {
		filePath := "/tmp/test_getfile_external"
		t.Cleanup(func() { client.DeleteFile(ctx, filePath) })

		hostRun(t, client, fmt.Sprintf("echo -n 'external' > %s", filePath))
		hostRun(t, client, fmt.Sprintf("chmod 0751 %s", filePath))

		file, err := client.GetFile(ctx, filePath)
		require.NoError(t, err)

		assert.Equal(t, filePath, file.Path)
		assert.Equal(t, "0751", file.Mode)
		assert.Equal(t, len("external"), file.Size)

		verifyFileFields(t, client, file)
	})
}

func TestDeleteFile(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	t.Run("removes existing file", func(t *testing.T) {
		filePath := "/tmp/test_deletefile_basic"

		_, err := client.PutFile(ctx, &PutFileCommand{
			Path:    filePath,
			Content: strings.NewReader("to be deleted"),
			Mode:    "0644",
		})
		require.NoError(t, err)

		err = client.DeleteFile(ctx, filePath)
		require.NoError(t, err)

		_, err = client.GetFile(ctx, filePath)
		assert.Error(t, err)
	})

	t.Run("nonexistent file", func(t *testing.T) {
		err := client.DeleteFile(ctx, "/tmp/test_deletefile_nonexistent")
		assert.NoError(t, err)
	})

	t.Run("rejects directory", func(t *testing.T) {
		dirPath := "/tmp/test_deletefile_dir"
		t.Cleanup(func() { client.Run(ctx, fmt.Sprintf("rmdir %s", dirPath), nil, nil) })

		hostRun(t, client, fmt.Sprintf("mkdir -p %s", dirPath))

		err := client.DeleteFile(ctx, dirPath)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "not a regular file")
	})
}

func TestPutFileValidation(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	t.Run("missing path", func(t *testing.T) {
		_, err := client.PutFile(ctx, &PutFileCommand{
			Content: strings.NewReader("x"),
		})
		assert.ErrorContains(t, err, "path is required")
	})

	t.Run("missing content", func(t *testing.T) {
		_, err := client.PutFile(ctx, &PutFileCommand{
			Path: "/tmp/x",
		})
		assert.ErrorContains(t, err, "content is required")
	})

	t.Run("owner and uid conflict", func(t *testing.T) {
		_, err := client.PutFile(ctx, &PutFileCommand{
			Path:    "/tmp/x",
			Content: strings.NewReader("x"),
			User:    "root",
			UID:     new(1000),
		})
		assert.ErrorContains(t, err, "user and uid cannot be set at the same time")
	})

	t.Run("group and gid conflict", func(t *testing.T) {
		_, err := client.PutFile(ctx, &PutFileCommand{
			Path:    "/tmp/x",
			Content: strings.NewReader("x"),
			Group:   "root",
			GID:     new(1000),
		})
		assert.ErrorContains(t, err, "group and gid cannot be set at the same time")
	})

	t.Run("invalid mode", func(t *testing.T) {
		_, err := client.PutFile(ctx, &PutFileCommand{
			Path:    "/tmp/x",
			Content: strings.NewReader("x"),
			Mode:    "not-a-mode",
		})
		assert.ErrorContains(t, err, "invalid mode")
	})
}
