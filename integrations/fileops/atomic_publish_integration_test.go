//go:build integration

// Real-OS justification: AtomicPublish exercises the production FileSession boundary
// with CreateTemp beside the destination path, buffered write + Sync + Close + Chmod
// on real handles, and final rename through the backend seam. These behaviors are not
// faithfully modeled by MemFileSession alone.

package fileops_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

func TestAtomicPublishIntegration_CreateWritesContentAndDoesNotLeaveTemp(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	err := fileops.AtomicPublish(fileops.NewOSFileSession(), fileops.AtomicPublishOptions{
		Path: path,
		Perm: 0o640,
		Mode: fileops.AtomicPublishCreate,
	}, func(w io.Writer) error {
		_, err := w.Write([]byte("created\n"))
		return err
	})
	if err != nil {
		t.Fatalf("AtomicPublish: %v", err)
	}

	got, err := os.ReadFile(path) //nolint:gosec // G304: path under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("created\n")) {
		t.Fatalf("content: got %q want %q", got, "created\n")
	}

	if runtime.GOOS != "windows" {
		fi, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatal(statErr)
		}
		if fi.Mode().Perm() != 0o640 {
			t.Fatalf("perm: got %o want %o", fi.Mode().Perm(), 0o640)
		}
	}

	assertNoAtomicPublishTempArtifacts(t, dir)
}

func TestAtomicPublishIntegration_CreateRejectsExistingDestination(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(path, []byte("already\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := fileops.AtomicPublish(fileops.NewOSFileSession(), fileops.AtomicPublishOptions{
		Path: path,
		Perm: 0o600,
		Mode: fileops.AtomicPublishCreate,
	}, func(w io.Writer) error {
		_, err := w.Write([]byte("new\n"))
		return err
	})
	if !errors.Is(err, fileops.ErrAtomicDestinationExists) {
		t.Fatalf("want ErrAtomicDestinationExists, got %v", err)
	}

	got, err := os.ReadFile(path) //nolint:gosec // G304: path under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("already\n")) {
		t.Fatalf("destination must be unchanged: got %q", got)
	}

	assertNoAtomicPublishTempArtifacts(t, dir)
}

func TestAtomicPublishIntegration_ReplaceOverwritesAtomically(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := fileops.AtomicPublish(fileops.NewOSFileSession(), fileops.AtomicPublishOptions{
		Path: path,
		Perm: 0o600,
		Mode: fileops.AtomicPublishReplace,
	}, func(w io.Writer) error {
		_, err := w.Write([]byte("fresh\n"))
		return err
	})
	if err != nil {
		t.Fatalf("AtomicPublish: %v", err)
	}

	got, err := os.ReadFile(path) //nolint:gosec // G304: path under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("fresh\n")) {
		t.Fatalf("content: got %q want %q", got, "fresh\n")
	}

	assertNoAtomicPublishTempArtifacts(t, dir)
}

func TestAtomicPublishIntegration_AppendPreservesPrefix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := fileops.AtomicPublish(fileops.NewOSFileSession(), fileops.AtomicPublishOptions{
		Path: path,
		Perm: 0o600,
		Mode: fileops.AtomicPublishAppend,
	}, func(w io.Writer) error {
		_, err := w.Write([]byte("new\n"))
		return err
	})
	if err != nil {
		t.Fatalf("AtomicPublish: %v", err)
	}

	got, err := os.ReadFile(path) //nolint:gosec // G304: path under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("old\nnew\n")) {
		t.Fatalf("content: got %q want %q", got, "old\nnew\n")
	}

	assertNoAtomicPublishTempArtifacts(t, dir)
}

func assertNoAtomicPublishTempArtifacts(t *testing.T, dir string) {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(dir, ".glyph-shift-*"))
	if err != nil {
		t.Fatalf("glob temp artifacts: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("unexpected publish temp files left behind: %s", strings.Join(matches, ", "))
	}
}
