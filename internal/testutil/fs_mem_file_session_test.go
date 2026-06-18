package testutil

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/spf13/afero"
)

func TestMemFileSession_RenameTrackedTempWritesBackToMemoryFS(t *testing.T) {
	t.Parallel()

	session := NewMemFileSession()
	tmp, err := session.CreateTemp("/logical", ".glyph-shift-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, writeErr := io.WriteString(tmp, "published\n"); writeErr != nil {
		t.Fatalf("write temp: %v", writeErr)
	}
	if closeErr := tmp.Close(); closeErr != nil {
		t.Fatalf("close temp: %v", closeErr)
	}

	if renameErr := session.Rename(tmp.Name(), "/logical/out.txt"); renameErr != nil {
		t.Fatalf("Rename tracked temp: %v", renameErr)
	}

	got, err := afero.ReadFile(session.Fs, "/logical/out.txt")
	if err != nil {
		t.Fatalf("ReadFile published: %v", err)
	}
	if string(got) != "published\n" {
		t.Fatalf("published content = %q, want published newline", got)
	}
}

func TestMemFileSession_RemoveTrackedTempDeletesTempMapping(t *testing.T) {
	t.Parallel()

	session := NewMemFileSession()
	tmp, err := session.CreateTemp("/", ".glyph-shift-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	tmpName := tmp.Name()
	if err := tmp.Close(); err != nil {
		t.Fatalf("Close temp: %v", err)
	}

	if err := session.Remove(tmpName); err != nil {
		t.Fatalf("Remove tracked temp: %v", err)
	}

	if err := session.Rename(tmpName, "/should-not-exist.txt"); err == nil {
		t.Fatal("Rename removed temp error = nil, want error")
	}
}

func TestMemFileSession_RenameLogicalPathCopiesAndUnlinksSource(t *testing.T) {
	t.Parallel()

	session := NewMemFileSession()
	if err := afero.WriteFile(session.Fs, "/old.txt", []byte("logical"), 0o644); err != nil {
		t.Fatalf("seed logical file: %v", err)
	}

	if err := session.Rename("/old.txt", "/new.txt"); err != nil {
		t.Fatalf("Rename logical path: %v", err)
	}

	got, err := afero.ReadFile(session.Fs, "/new.txt")
	if err != nil {
		t.Fatalf("ReadFile new logical path: %v", err)
	}
	if string(got) != "logical" {
		t.Fatalf("new logical content = %q, want logical", got)
	}
	if _, err := afero.ReadFile(session.Fs, "/old.txt"); err == nil {
		t.Fatal("old logical path still exists after rename")
	}
}

func TestMemFileSession_RenameSameLogicalPathIsNoOp(t *testing.T) {
	t.Parallel()

	session := NewMemFileSession()
	if err := afero.WriteFile(session.Fs, "/same.txt", []byte("keep"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := session.Rename("/same.txt", "/same.txt"); err != nil {
		t.Fatalf("Rename same path: %v", err)
	}

	got, err := afero.ReadFile(session.Fs, "/same.txt")
	if err != nil {
		t.Fatalf("ReadFile after same-path rename: %v", err)
	}
	if string(got) != "keep" {
		t.Fatalf("content = %q want keep", string(got))
	}
}

func TestMemFileSession_OpenReadMaterializesContentAtStart(t *testing.T) {
	t.Parallel()

	session := NewMemFileSession()
	if err := afero.WriteFile(session.Fs, "/read.txt", []byte("abc"), 0o644); err != nil {
		t.Fatalf("seed read file: %v", err)
	}

	handle, err := session.OpenRead("/read.txt")
	if err != nil {
		t.Fatalf("OpenRead: %v", err)
	}
	defer func() { _ = handle.Close() }()

	got, err := io.ReadAll(handle)
	if err != nil {
		t.Fatalf("ReadAll materialized handle: %v", err)
	}
	if string(got) != "abc" {
		t.Fatalf("materialized content = %q, want abc", got)
	}
}

func TestMemFileSession_OpenRDWRPersistsOnClose(t *testing.T) {
	t.Parallel()

	session := NewMemFileSession()
	if err := afero.WriteFile(session.Fs, "/rdwr.txt", nil, 0o644); err != nil {
		t.Fatalf("seed rdwr file: %v", err)
	}

	handle, openErr := session.OpenRDWR("/rdwr.txt")
	if openErr != nil {
		t.Fatalf("OpenRDWR: %v", openErr)
	}
	if _, writeErr := handle.Write([]byte("after")); writeErr != nil {
		t.Fatalf("Write: %v", writeErr)
	}
	if closeErr := handle.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	got, err := afero.ReadFile(session.Fs, "/rdwr.txt")
	if err != nil {
		t.Fatalf("ReadFile after Close: %v", err)
	}
	if string(got) != "after" {
		t.Fatalf("persisted content = %q, want after", got)
	}
}

func TestMemRDWRHandle_ReadOnlyCloseDoesNotRewriteLogicalFile(t *testing.T) {
	t.Parallel()

	session := NewMemFileSession()
	seed := []byte("seed-bytes")
	if err := afero.WriteFile(session.Fs, "/readonly.txt", seed, 0o644); err != nil {
		t.Fatalf("seed readonly file: %v", err)
	}

	handle, openErr := session.OpenRDWR("/readonly.txt")
	if openErr != nil {
		t.Fatalf("OpenRDWR: %v", openErr)
	}

	buf := make([]byte, len(seed))
	if _, readErr := io.ReadFull(handle, buf); readErr != nil {
		t.Fatalf("ReadFull: %v", readErr)
	}
	if closeErr := handle.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	got, err := afero.ReadFile(session.Fs, "/readonly.txt")
	if err != nil {
		t.Fatalf("ReadFile after read-only Close: %v", err)
	}
	if !bytes.Equal(got, seed) {
		t.Fatalf("logical file after read-only Close = %q, want %q", got, seed)
	}
}

func TestMemRDWRHandle_SeekNegativeDoesNotCorruptOffset(t *testing.T) {
	t.Parallel()

	session := NewMemFileSession()
	if err := afero.WriteFile(session.Fs, "/seek.txt", []byte("abc"), 0o644); err != nil {
		t.Fatalf("seed seek file: %v", err)
	}

	handle, openErr := session.OpenRDWR("/seek.txt")
	if openErr != nil {
		t.Fatalf("OpenRDWR: %v", openErr)
	}
	defer func() { _ = handle.Close() }()

	if _, seekErr := handle.Seek(-1, io.SeekStart); !errors.Is(seekErr, errMemSeekNegativePos) {
		t.Fatalf("Seek negative: got %v want errMemSeekNegativePos", seekErr)
	}

	got, readErr := io.ReadAll(handle)
	if readErr != nil {
		t.Fatalf("ReadAll after failed seek: %v", readErr)
	}
	if string(got) != "abc" {
		t.Fatalf("content after failed seek = %q, want abc", got)
	}
}

func TestMemRDWRHandle_WriteAtRejectsNegativeOffset(t *testing.T) {
	t.Parallel()

	session := NewMemFileSession()
	if err := afero.WriteFile(session.Fs, "/writeat.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("seed writeat file: %v", err)
	}

	handle, err := session.OpenRDWR("/writeat.txt")
	if err != nil {
		t.Fatalf("OpenRDWR: %v", err)
	}
	defer func() { _ = handle.Close() }()

	if _, err := handle.WriteAt([]byte("z"), -1); !errors.Is(err, errMemWriteAtNegativeOffset) {
		t.Fatalf("WriteAt negative: got %v want errMemWriteAtNegativeOffset", err)
	}
}

func TestMemReadHandle_OperationsAfterCloseReturnErrClosed(t *testing.T) {
	t.Parallel()

	session := NewMemFileSession()
	if err := afero.WriteFile(session.Fs, "/read-closed.txt", []byte("seed"), 0o644); err != nil {
		t.Fatalf("seed read file: %v", err)
	}

	handle, err := session.OpenRead("/read-closed.txt")
	if err != nil {
		t.Fatalf("OpenRead: %v", err)
	}
	if closeErr := handle.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	if _, readErr := handle.Read(make([]byte, 1)); !errors.Is(readErr, fs.ErrClosed) {
		t.Fatalf("Read after Close: got %v want fs.ErrClosed", readErr)
	}
	if _, seekErr := handle.Seek(0, io.SeekStart); !errors.Is(seekErr, fs.ErrClosed) {
		t.Fatalf("Seek after Close: got %v want fs.ErrClosed", seekErr)
	}
	if _, statErr := handle.Stat(); !errors.Is(statErr, fs.ErrClosed) {
		t.Fatalf("Stat after Close: got %v want fs.ErrClosed", statErr)
	}
}

func TestMemRDWRHandle_LockAfterCloseReturnsErrClosed(t *testing.T) {
	t.Parallel()

	session := NewMemFileSession()
	if err := afero.WriteFile(session.Fs, "/lock-closed.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	handle, err := session.OpenRDWR("/lock-closed.txt")
	if err != nil {
		t.Fatalf("OpenRDWR: %v", err)
	}
	if closeErr := handle.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	locker, ok := handle.(fileops.AdvisoryLocker)
	if !ok {
		t.Fatal("OpenRDWR handle must implement AdvisoryLocker")
	}
	if lockErr := locker.LockExclusive(); !errors.Is(lockErr, fs.ErrClosed) {
		t.Fatalf("LockExclusive after Close: got %v want fs.ErrClosed", lockErr)
	}
}

func TestMemRDWRHandle_WriteAfterCloseReturnsErrClosed(t *testing.T) {
	t.Parallel()

	session := NewMemFileSession()
	if err := afero.WriteFile(session.Fs, "/closed.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("seed closed file: %v", err)
	}

	handle, err := session.OpenRDWR("/closed.txt")
	if err != nil {
		t.Fatalf("OpenRDWR: %v", err)
	}
	if closeErr := handle.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	if _, writeErr := handle.Write([]byte("y")); !errors.Is(writeErr, fs.ErrClosed) {
		t.Fatalf("Write after Close: got %v want fs.ErrClosed", writeErr)
	}
}
