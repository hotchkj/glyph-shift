//go:build integration

package safeio_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

const safeioTestFileName = "test.txt"

func tempTextFile(t *testing.T, dir string, content []byte) string {
	t.Helper()

	path := filepath.Join(dir, safeioTestFileName)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func openModify(t *testing.T, path string) *fileops.Modifier {
	t.Helper()

	modifier, err := fileops.OpenForModify(path, fileops.NewOSFileSession())
	if err != nil {
		t.Fatalf("OpenForModify: %v", err)
	}

	return modifier
}

func assertDirContainsOnly(t *testing.T, dir, name string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("dir %q: got %d entries, want exactly 1 named %q", dir, len(entries), name)
	}

	if entries[0].Name() != name {
		t.Fatalf("dir %q: got entry %q, want %q", dir, entries[0].Name(), name)
	}
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()

	got, err := os.ReadFile(path) //nolint:gosec // G304: path under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("content: got %q, want %q", got, want)
	}
}
