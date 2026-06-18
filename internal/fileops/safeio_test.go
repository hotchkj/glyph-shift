package fileops_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	iofs "io/fs"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

var (
	errSafeioTestChmod  = errors.New("safeio test chmod failed")
	errSafeioTestRename = errors.New("safeio test rename failed")
)

type safeioFaultFileSession struct {
	fileops.FileSession
	chmodErr  error
	renameErr error
}

func (s *safeioFaultFileSession) Chmod(string, iofs.FileMode) error {
	if s.chmodErr != nil {
		return s.chmodErr
	}

	return nil
}

func (s *safeioFaultFileSession) Rename(string, string) error {
	if s.renameErr != nil {
		return s.renameErr
	}

	return nil
}

func TestOpenForModify_NilFileSession_Fake(t *testing.T) {
	t.Parallel()

	_, err := fileops.OpenForModify("/any.txt", nil)
	if !errors.Is(err, fileops.ErrNilFileSession) {
		t.Fatalf("want ErrNilFileSession, got %v", err)
	}
}

func TestOpenForModifyLocked_NilFileSession_Fake(t *testing.T) {
	t.Parallel()

	_, err := fileops.OpenForModifyLocked("/any.txt", nil)
	if !errors.Is(err, fileops.ErrNilFileSession) {
		t.Fatalf("want ErrNilFileSession, got %v", err)
	}
}

func TestOpenForModifyLocked_ContentNilUntilBuffered_Fake(t *testing.T) {
	t.Parallel()

	fs := testutil.NewMemFileSession()
	if err := afero.WriteFile(fs.Fs, "/x.txt", []byte("only-locked\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	mod, err := fileops.OpenForModifyLocked("/x.txt", fs)
	if err != nil {
		t.Fatalf("OpenForModifyLocked: %v", err)
	}
	defer mod.Abort()

	if mod.Content() != nil {
		t.Fatalf("Content: want nil before buffered read, got %d bytes", len(mod.Content()))
	}
}

func TestWriteSyncCloseCommitTempWriteFailureReportsTempStillOpen(t *testing.T) {
	t.Parallel()

	fs := testutil.NewMemFileSession()
	tmp, err := fs.CreateTemp("/ignored", "glyph-shift-safeio-closed-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	tmpPath := tmp.Name()
	t.Cleanup(func() {
		_ = fs.Remove(tmpPath)
	})

	if closeErr := tmp.Close(); closeErr != nil {
		t.Fatalf("close temp before write: %v", closeErr)
	}

	closed, writeErr := fileops.TestingWriteSyncCloseCommitTemp(tmp, []byte("data"))
	if writeErr == nil {
		t.Fatal("want write error from closed temp")
	}
	if closed {
		t.Fatal("write failure must report temp as not closed by helper")
	}
}

func TestOpenForModify_ReadAndCommit_Fake(t *testing.T) {
	t.Parallel()

	fs := testutil.NewMemFileSession()
	if err := afero.WriteFile(fs.Fs, "/test.txt", []byte("hello world"), 0o600); err != nil {
		t.Fatal(err)
	}

	modifier, err := fileops.OpenForModify("/test.txt", fs)
	if err != nil {
		t.Fatalf("OpenForModify: %v", err)
	}

	if !bytes.Equal(modifier.Content(), []byte("hello world")) {
		t.Fatalf("Content: got %q, want %q", modifier.Content(), "hello world")
	}

	if commitErr := modifier.Commit([]byte("goodbye world")); commitErr != nil {
		t.Fatalf("Commit: %v", commitErr)
	}

	got := fs.Files()["/test.txt"]
	if !bytes.Equal(got, []byte("goodbye world")) {
		t.Fatalf("after commit: got %q, want %q", got, "goodbye world")
	}
}

func TestModifier_DoubleCommitFails_Fake(t *testing.T) {
	t.Parallel()

	fs := testutil.NewMemFileSession()
	if err := afero.WriteFile(fs.Fs, "/test.txt", []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	modifier, err := fileops.OpenForModify("/test.txt", fs)
	if err != nil {
		t.Fatalf("OpenForModify: %v", err)
	}

	if commitErr := modifier.Commit([]byte("updated")); commitErr != nil {
		t.Fatalf("first Commit: %v", commitErr)
	}

	err = modifier.Commit([]byte("again"))
	if !errors.Is(err, fileops.ErrModifierAlreadyDone) {
		t.Fatalf("want ErrModifierAlreadyDone, got %v", err)
	}
}

func TestModifier_AbortLeavesFileUnchanged_Fake(t *testing.T) {
	t.Parallel()

	fs := testutil.NewMemFileSession()
	original := []byte("untouched")
	if err := afero.WriteFile(fs.Fs, "/test.txt", original, 0o600); err != nil {
		t.Fatal(err)
	}

	modifier, err := fileops.OpenForModify("/test.txt", fs)
	if err != nil {
		t.Fatalf("OpenForModify: %v", err)
	}

	modifier.Abort()

	got := fs.Files()["/test.txt"]
	if !bytes.Equal(got, original) {
		t.Fatalf("after abort: got %q, want %q", got, original)
	}
}

func TestModifier_DoubleAbortIsSafe_Fake(t *testing.T) {
	t.Parallel()

	fs := testutil.NewMemFileSession()
	if err := afero.WriteFile(fs.Fs, "/test.txt", []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	modifier, err := fileops.OpenForModify("/test.txt", fs)
	if err != nil {
		t.Fatalf("OpenForModify: %v", err)
	}

	modifier.Abort()
	modifier.Abort() // should not panic
}

func TestOpenForModify_NonexistentFile_Fake(t *testing.T) {
	t.Parallel()

	_, err := fileops.OpenForModify("/does-not-exist.txt", testutil.NewMemFileSession())
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestOpenForRead_NilFileSession_Fake(t *testing.T) {
	t.Parallel()

	_, _, err := fileops.OpenForRead("/any.txt", nil)
	if !errors.Is(err, fileops.ErrNilFileSession) {
		t.Fatalf("want ErrNilFileSession, got %v", err)
	}
}

func TestOpenForRead_ReadAndClose_Fake(t *testing.T) {
	t.Parallel()

	fs := testutil.NewMemFileSession()
	want := []byte("hello read")
	if err := afero.WriteFile(fs.Fs, "/doc.txt", want, 0o600); err != nil {
		t.Fatal(err)
	}

	got, handle, err := fileops.OpenForRead("/doc.txt", fs)
	if err != nil {
		t.Fatalf("OpenForRead: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("content: got %q, want %q", got, want)
	}

	if closeErr := handle.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}
}

func TestOpenForRead_NonexistentFile_Fake(t *testing.T) {
	t.Parallel()

	_, _, err := fileops.OpenForRead("/missing.txt", testutil.NewMemFileSession())
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestModifier_CommitLargeFile_Fake(t *testing.T) {
	t.Parallel()

	fs := testutil.NewMemFileSession()
	original := bytes.Repeat([]byte("A"), 1024*1024)
	if err := afero.WriteFile(fs.Fs, "/large.bin", original, 0o600); err != nil {
		t.Fatal(err)
	}

	modifier, err := fileops.OpenForModify("/large.bin", fs)
	if err != nil {
		t.Fatalf("OpenForModify: %v", err)
	}

	if !bytes.Equal(modifier.Content(), original) {
		t.Fatal("large file content mismatch at read")
	}

	replacement := bytes.Repeat([]byte("B"), 1024*1024)
	if commitErr := modifier.Commit(replacement); commitErr != nil {
		t.Fatalf("Commit: %v", commitErr)
	}

	got := fs.Files()["/large.bin"]
	if !bytes.Equal(got, replacement) {
		t.Fatal("large file content mismatch after commit")
	}
}

func TestModifier_CommitSurfacesChmodError_Fake(t *testing.T) {
	t.Parallel()

	mem := testutil.NewMemFileSession()
	if err := afero.WriteFile(mem.Fs, "/test.txt", []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	session := &safeioFaultFileSession{FileSession: mem, chmodErr: errSafeioTestChmod}
	modifier, err := fileops.OpenForModify("/test.txt", session)
	if err != nil {
		t.Fatalf("OpenForModify: %v", err)
	}

	err = modifier.Commit([]byte("updated"))
	if !errors.Is(err, errSafeioTestChmod) {
		t.Fatalf("Commit error = %v, want %v", err, errSafeioTestChmod)
	}
}

func TestModifier_CommitSurfacesRenameError_Fake(t *testing.T) {
	t.Parallel()

	mem := testutil.NewMemFileSession()
	if err := afero.WriteFile(mem.Fs, "/test.txt", []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	session := &safeioFaultFileSession{FileSession: mem, renameErr: errSafeioTestRename}
	modifier, err := fileops.OpenForModify("/test.txt", session)
	if err != nil {
		t.Fatalf("OpenForModify: %v", err)
	}

	err = modifier.Commit([]byte("updated"))
	if !errors.Is(err, errSafeioTestRename) {
		t.Fatalf("Commit error = %v, want %v", err, errSafeioTestRename)
	}
}

func TestModifier_CommitFromWriterSurfacesRenameError_Fake(t *testing.T) {
	t.Parallel()

	mem := testutil.NewMemFileSession()
	original := []byte("data")
	if err := afero.WriteFile(mem.Fs, "/test.txt", original, 0o600); err != nil {
		t.Fatal(err)
	}

	session := &safeioFaultFileSession{FileSession: mem, renameErr: errSafeioTestRename}
	modifier, err := fileops.OpenForModifyLocked("/test.txt", session)
	if err != nil {
		t.Fatalf("OpenForModifyLocked: %v", err)
	}
	fileops.TestingSetModifierHash(modifier, sha256.Sum256(original))

	err = modifier.CommitFromWriter(func(w io.Writer) ([sha256.Size]byte, error) {
		if _, writeErr := w.Write([]byte("updated")); writeErr != nil {
			return [sha256.Size]byte{}, writeErr
		}

		return sha256.Sum256(original), nil
	})
	if !errors.Is(err, errSafeioTestRename) {
		t.Fatalf("CommitFromWriter error = %v, want %v", err, errSafeioTestRename)
	}
}
