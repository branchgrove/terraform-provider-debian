package ssh

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAptInstall_Concurrent verifies that concurrent apt operations do not fail due
// to the global apt lock on the host. The Client.aptMu should serialize these calls.
func TestAptInstall_Concurrent(t *testing.T) {
	if os.Getenv("TEST_SSH_HOST") == "" {
		t.Skip("TEST_SSH_HOST not set, skipping integration test")
	}

	client := testClient(t)
	ctx := context.Background()

	// Ensure system is in a clean state before we start. We run update first
	// just in case, but without concurrency.
	err := client.AptInstall(ctx, map[string]string{}, true)
	require.NoError(t, err)

	packagesGroup1 := map[string]string{
		"curl": "",
	}
	packagesGroup2 := map[string]string{
		"wget": "",
	}

	var wg sync.WaitGroup
	var err1, err2 error

	wg.Add(2)

	go func() {
		defer wg.Done()
		err1 = client.AptInstall(ctx, packagesGroup1, true)
	}()

	go func() {
		defer wg.Done()
		err2 = client.AptInstall(ctx, packagesGroup2, true)
	}()

	// Wait for both concurrent operations to finish.
	// Without a lock, one of these would typically fail with:
	// "Could not get lock /var/lib/dpkg/lock-frontend"
	wg.Wait()

	assert.NoError(t, err1, "First concurrent apt-get install failed")
	assert.NoError(t, err2, "Second concurrent apt-get install failed")

	// Cleanup
	_ = client.AptRemove(ctx, []string{"curl", "wget"}, true)
}

func TestLockApt(t *testing.T) {
	// Unit test to verify the lock can be acquired and released without panicking.
	client := &Client{
		aptMu: &sync.Mutex{},
	}

	locked := false
	var wg sync.WaitGroup
	wg.Add(1)

	// Acquire lock in main goroutine
	client.LockApt()

	go func() {
		defer wg.Done()
		// This should block until the lock is released
		client.LockApt()
		locked = true
		client.UnlockApt()
	}()

	// Ensure the goroutine had a chance to run and block
	time.Sleep(50 * time.Millisecond)
	assert.False(t, locked, "Lock was acquired concurrently")

	client.UnlockApt()

	// Wait for goroutine to finish and acquire/release lock
	wg.Wait()
	assert.True(t, locked, "Lock was never acquired by goroutine")
}
