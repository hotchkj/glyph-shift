package fileops_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

var errCommitWriterTestCallback = errors.New("commit writer callback failed")

func TestOpenForModifyLocked_CommitFromWriter_CopiesBytesWhenHashMatches(t *testing.T) {
	t.Parallel()

	fs := testutil.NewMemFileSession()
	original := []byte("original content\n")
	if err := afero.WriteFile(fs.Fs, "/source.txt", original, 0o644); err != nil {
		t.Fatalf("seed memfs: %v", err)
	}

	mod, err := fileops.OpenForModifyLocked("/source.txt", fs)
	if err != nil {
		t.Fatalf("OpenForModifyLocked: %v", err)
	}
	fileops.TestingSetModifierHash(mod, sha256.Sum256(original))

	if commitErr := mod.CommitFromWriter(func(dest io.Writer) ([sha256.Size]byte, error) {
		return replayLockedSourceTo(dest, mod)
	}); commitErr != nil {
		t.Fatalf("CommitFromWriter: %v", commitErr)
	}

	got, err := afero.ReadFile(fs.Fs, "/source.txt")
	if err != nil {
		t.Fatalf("read committed file: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("committed bytes: got %q want %q", got, original)
	}
}

func TestCommitFromWriter_ReturnedSourceDigestMismatch_AbortsBeforeCAS(t *testing.T) {
	t.Parallel()

	fs := testutil.NewMemFileSession()
	original := []byte("original content\n")
	if err := afero.WriteFile(fs.Fs, "/digest-mismatch.txt", original, 0o644); err != nil {
		t.Fatalf("seed memfs: %v", err)
	}

	mod, err := fileops.OpenForModify("/digest-mismatch.txt", fs)
	if err != nil {
		t.Fatalf("OpenForModify: %v", err)
	}

	commitErr := mod.CommitFromWriter(func(dest io.Writer) ([sha256.Size]byte, error) {
		if _, writeErr := dest.Write([]byte("replacement\n")); writeErr != nil {
			return [sha256.Size]byte{}, writeErr
		}

		return sha256.Sum256([]byte("different source digest")), nil
	})
	if !errors.Is(commitErr, fileops.ErrFileModifiedExternally) {
		t.Fatalf("want ErrFileModifiedExternally, got %v", commitErr)
	}

	got, err := afero.ReadFile(fs.Fs, "/digest-mismatch.txt")
	if err != nil {
		t.Fatalf("read original file: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("file changed after digest mismatch: got %q want %q", got, original)
	}
}

func TestCommitFromWriter_WriteCallbackError_RemovesTempAndLeavesOriginal(t *testing.T) {
	t.Parallel()

	fs := testutil.NewMemFileSession()
	original := []byte("original content\n")
	if err := afero.WriteFile(fs.Fs, "/callback-error.txt", original, 0o644); err != nil {
		t.Fatalf("seed memfs: %v", err)
	}

	mod, err := fileops.OpenForModify("/callback-error.txt", fs)
	if err != nil {
		t.Fatalf("OpenForModify: %v", err)
	}

	commitErr := mod.CommitFromWriter(func(io.Writer) ([sha256.Size]byte, error) {
		return [sha256.Size]byte{}, errCommitWriterTestCallback
	})
	if !errors.Is(commitErr, errCommitWriterTestCallback) {
		t.Fatalf("want callback error, got %v", commitErr)
	}

	got, err := afero.ReadFile(fs.Fs, "/callback-error.txt")
	if err != nil {
		t.Fatalf("read original file: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("file changed after callback error: got %q want %q", got, original)
	}
	if len(fs.Files()) != 1 {
		t.Fatalf("temp cleanup left files: %#v", fs.Files())
	}
}

func TestCommitFromWriter_DoubleInvoke_ErrModifierAlreadyDone(t *testing.T) {
	t.Parallel()

	fs := testutil.NewMemFileSession()
	if err := afero.WriteFile(fs.Fs, "/double.txt", []byte("original content\n"), 0o644); err != nil {
		t.Fatalf("seed memfs: %v", err)
	}

	mod, err := fileops.OpenForModify("/double.txt", fs)
	if err != nil {
		t.Fatalf("OpenForModify: %v", err)
	}

	if commitErr := mod.CommitFromWriter(func(dest io.Writer) ([sha256.Size]byte, error) {
		return replayLockedSourceTo(dest, mod)
	}); commitErr != nil {
		t.Fatalf("first CommitFromWriter: %v", commitErr)
	}

	err = mod.CommitFromWriter(func(io.Writer) ([sha256.Size]byte, error) {
		t.Fatal("second callback must not run")

		return [sha256.Size]byte{}, nil
	})
	if !errors.Is(err, fileops.ErrModifierAlreadyDone) {
		t.Fatalf("want ErrModifierAlreadyDone, got %v", err)
	}
}

// Exercises post-temp-write CAS on the guarded source: the write callback returns a digest that
// matches the pre-open snapshot (m.hash), but mutates the live handle before callback return so a
// naive "callback digest only" check would still pass while verifyCAS sees new bytes.
func TestCommitFromWriter_PostWriteCASDetectsSourceTamperAfterReplayRead(t *testing.T) {
	t.Parallel()

	fs := testutil.NewMemFileSession()
	if err := afero.WriteFile(fs.Fs, "/guarded.txt", []byte("original content\n"), 0o644); err != nil {
		t.Fatalf("seed memfs: %v", err)
	}

	mod, err := fileops.OpenForModify("/guarded.txt", fs)
	if err != nil {
		t.Fatalf("OpenForModify: %v", err)
	}

	commitErr := mod.CommitFromWriter(func(dest io.Writer) ([sha256.Size]byte, error) {
		src := fileops.TestingLockedReplaySource(mod)

		if _, seekErr := src.Seek(0, io.SeekStart); seekErr != nil {
			return [sha256.Size]byte{}, seekErr
		}

		hasher := sha256.New()
		tr := io.TeeReader(src, hasher)
		if _, copyErr := io.Copy(dest, tr); copyErr != nil {
			return [sha256.Size]byte{}, copyErr
		}

		if _, writeErr := src.WriteAt([]byte("tampered!!!\n"), 0); writeErr != nil {
			return [sha256.Size]byte{}, writeErr
		}

		var digest [sha256.Size]byte
		copy(digest[:], hasher.Sum(nil))

		return digest, nil
	})

	if !errors.Is(commitErr, fileops.ErrFileModifiedExternally) {
		t.Fatalf("want ErrFileModifiedExternally, got %v", commitErr)
	}

	// Callback deliberately mutates the guarded source after replay read; invariant under test is
	// post-write CAS rejection, not preserving initial bytes after the hook.
}

func replayLockedSourceTo(dest io.Writer, mod *fileops.Modifier) ([sha256.Size]byte, error) {
	src := fileops.TestingLockedReplaySource(mod)
	if _, seekErr := src.Seek(0, io.SeekStart); seekErr != nil {
		return [sha256.Size]byte{}, seekErr
	}

	hasher := sha256.New()
	tr := io.TeeReader(src, hasher)
	if _, copyErr := io.Copy(dest, tr); copyErr != nil {
		return [sha256.Size]byte{}, copyErr
	}

	var digest [sha256.Size]byte
	copy(digest[:], hasher.Sum(nil))

	return digest, nil
}
