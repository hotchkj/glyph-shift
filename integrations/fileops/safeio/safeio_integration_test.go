//go:build integration

// Real-OS justification: tests exercise OS-level file locking semantics
// (exclusive/shared locks via LockFileEx on Windows, flock on Unix), CAS verification
// through concurrent file handle writes, atomic rename behavior, and file permission
// propagation — none of which can be substituted by in-memory fakes.
//
// Run: mage integration. Diagnostic: go test -tags integration ./integrations/...
package safeio_test

import (
	"errors"
	"os"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

func TestModifier_CASDetectsExternalChange(t *testing.T) {
	t.Parallel()

	path := tempTextFile(t, t.TempDir(), []byte("original"))
	modifier := openModify(t, path)

	if _, writeErr := fileops.IntegrationModifierWriteAt(modifier, []byte("tampered"), 0); writeErr != nil {
		modifier.Abort()
		t.Fatalf("WriteAt: %v", writeErr)
	}

	err := modifier.Commit([]byte("new content"))
	if !errors.Is(err, fileops.ErrFileModifiedExternally) {
		t.Fatalf("want ErrFileModifiedExternally, got %v", err)
	}
}

func TestOpenForRead_ReturnsContentAndReleasesLock(t *testing.T) {
	t.Parallel()

	path := tempTextFile(t, t.TempDir(), []byte("read me"))

	content, handle, err := fileops.OpenForRead(path, fileops.NewOSFileSession())
	if err != nil {
		t.Fatalf("OpenForRead: %v", err)
	}

	if string(content) != "read me" {
		t.Fatalf("content: got %q, want %q", content, "read me")
	}

	if closeErr := handle.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	openModify(t, path).Abort()
}

func TestReadHandle_DoubleCloseIsSafe(t *testing.T) {
	t.Parallel()

	path := tempTextFile(t, t.TempDir(), []byte("data"))

	_, handle, err := fileops.OpenForRead(path, fileops.NewOSFileSession())
	if err != nil {
		t.Fatalf("OpenForRead: %v", err)
	}

	if firstCloseErr := handle.Close(); firstCloseErr != nil {
		t.Fatalf("first Close: %v", firstCloseErr)
	}
	if secondCloseErr := handle.Close(); secondCloseErr != nil {
		t.Fatalf("second Close should be nil: %v", secondCloseErr)
	}
}

func TestModifier_TempFileInSameDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := tempTextFile(t, dir, []byte("hello"))
	modifier := openModify(t, path)

	if commitErr := modifier.Commit([]byte("world")); commitErr != nil {
		t.Fatalf("Commit: %v", commitErr)
	}

	assertDirContainsOnly(t, dir, safeioTestFileName)
	assertFileBytes(t, path, []byte("world"))
}

func TestModifier_CommitPreservesPermissions(t *testing.T) {
	t.Parallel()

	path := tempTextFile(t, t.TempDir(), []byte("data"))

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	perm := fi.Mode().Perm()

	modifier := openModify(t, path)
	if commitErr := modifier.Commit([]byte("updated")); commitErr != nil {
		t.Fatalf("Commit: %v", commitErr)
	}

	fi2, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi2.Mode().Perm() != perm {
		t.Fatalf("permissions changed: got %o, want %o", fi2.Mode().Perm(), perm)
	}
}
