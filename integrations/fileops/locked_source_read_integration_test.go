//go:build integration

// Real-OS justification: exercises streaming reads against a large on-disk file opened via os.Open,
// complementing MemFileSession-based unit tests that cannot use real paths without violating test isolation rules.

package fileops_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

type openReadPathOnlyFS struct{}

var errOpenReadPathOnlyStub = errors.New("openReadPathOnlyFS: method not supported")

func (openReadPathOnlyFS) OpenRead(path string) (fileops.SessionReadHandle, error) {
	return fileops.NewOSFileSession().OpenRead(path)
}

func (openReadPathOnlyFS) OpenRDWR(string) (fileops.SessionRDWRHandle, error) {
	return nil, errOpenReadPathOnlyStub
}

func (openReadPathOnlyFS) CreateTemp(string, string) (fileops.SessionTempHandle, error) {
	return nil, errOpenReadPathOnlyStub
}

func (openReadPathOnlyFS) Remove(string) error {
	return errOpenReadPathOnlyStub
}

func (openReadPathOnlyFS) Rename(string, string) error {
	return errOpenReadPathOnlyStub
}

func (openReadPathOnlyFS) Chmod(string, fs.FileMode) error {
	return errOpenReadPathOnlyStub
}

// TestOpenLockedSourceRead_OSPathStreamsSmallRead proves the helper completes after a bounded read
// against a large on-disk file (OpenRead streams via *os.File; fileops never calls io.ReadAll on it).
func TestOpenLockedSourceRead_OSPathStreamsSmallRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "large.bin")

	const total = 8 * 1024 * 1024
	if err := os.WriteFile(path, bytes.Repeat([]byte("Q"), total), 0o600); err != nil {
		t.Fatal(err)
	}

	ls, err := fileops.OpenLockedSourceRead(path, openReadPathOnlyFS{})
	if err != nil {
		t.Fatalf("OpenLockedSourceRead: %v", err)
	}

	if ls.State.Size != total {
		t.Fatalf("State.Size: got %d want %d", ls.State.Size, total)
	}

	const prefix = 4096
	copiedBytes, err := io.CopyN(io.Discard, ls, prefix)
	if err != nil {
		t.Fatalf("CopyN: %v", err)
	}

	if copiedBytes != prefix {
		t.Fatalf("copied %d want %d", copiedBytes, prefix)
	}

	if closeErr := ls.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}
}
