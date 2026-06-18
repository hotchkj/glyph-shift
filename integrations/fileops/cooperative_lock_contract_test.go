//go:build integration

// Real-OS justification: this contract test exercises cooperative shared/exclusive
// lock behavior through the OS-backed file session.
package fileops_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

func cooperativeExclusiveWaitsForSharedRelease(
	t *testing.T,
	fileName string,
	payload []byte,
	acquireShared func(path string) (closeShared func() error, err error),
	sharedCloseDesc string,
) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	closeShared, err := acquireShared(path)
	if err != nil {
		t.Fatalf("acquire shared: %v", err)
	}

	modifyEntered := make(chan struct{})
	modifyDone := make(chan error, 1)

	go func() {
		close(modifyEntered)
		modifier, mErr := fileops.OpenForModify(path, fileops.NewOSFileSession())
		if mErr != nil {
			modifyDone <- mErr

			return
		}

		modifier.Abort()
		modifyDone <- nil
	}()

	<-modifyEntered

	select {
	case err := <-modifyDone:
		closeErr := closeShared()
		if err != nil {
			t.Fatalf(
				"unexpected exclusive-open error while shared lock held: %v (%s err=%v)",
				err, sharedCloseDesc, closeErr,
			)
		}

		t.Fatalf(
			"exclusive modify completed while shared read lock should still be held (%s err=%v)",
			sharedCloseDesc, closeErr,
		)
	default:
	}

	if closeErr := closeShared(); closeErr != nil {
		t.Fatalf("%s: %v", sharedCloseDesc, closeErr)
	}

	select {
	case err := <-modifyDone:
		if err != nil {
			t.Fatalf("OpenForModify after releasing shared lock: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for exclusive modify after shared release")
	}
}

// TestSafeIOSourceLock_ExclusiveModifyDoesNotCompleteWhileSharedReadLockHeld verifies cooperative locking:
// OpenForModify cannot finish until ReadHandle from OpenForRead releases the shared lock.
func TestSafeIOSourceLock_ExclusiveModifyDoesNotCompleteWhileSharedReadLockHeld(t *testing.T) {
	t.Parallel()

	cooperativeExclusiveWaitsForSharedRelease(
		t,
		"cooperative-lock.bin",
		[]byte("locked-by-shared"),
		func(path string) (func() error, error) {
			_, readHandle, err := fileops.OpenForRead(path, fileops.NewOSFileSession())
			if err != nil {
				return nil, err
			}

			return func() error { return readHandle.Close() }, nil
		},
		"ReadHandle Close",
	)
}

// TestSafeIOSourceLock_ExclusiveModifyBlockedWhileLockedSourceReadHeld verifies streaming reads:
// OpenForModify cannot finish until LockedSourceRead.Close releases the shared lock.
func TestSafeIOSourceLock_ExclusiveModifyBlockedWhileLockedSourceReadHeld(t *testing.T) {
	t.Parallel()

	cooperativeExclusiveWaitsForSharedRelease(
		t,
		"cooperative-lock-stream.bin",
		[]byte("locked-by-shared-stream"),
		func(path string) (func() error, error) {
			ls, err := fileops.OpenLockedSourceRead(path, fileops.NewOSFileSession())
			if err != nil {
				return nil, err
			}

			return func() error { return ls.Close() }, nil
		},
		"LockedSourceRead Close",
	)
}
