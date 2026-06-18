package fileops_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

var (
	errAtomicPublishInjectedWrite  = errors.New("atomic publish test: injected write failure")
	errAtomicPublishInjectedRename = errors.New("atomic publish test: injected rename failure")
)

func TestAtomicPublishCreateWritesOnlyAfterSuccessfulPublish(t *testing.T) {
	t.Parallel()

	session := testutil.NewMemFileSession()

	err := fileops.AtomicPublish(session, fileops.AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: fileops.AtomicPublishCreate,
	}, writeBytes([]byte("created\n")))
	if err != nil {
		t.Fatalf("AtomicPublish: %v", err)
	}

	got := session.Files()["/out.txt"]
	if !bytes.Equal(got, []byte("created\n")) {
		t.Fatalf("published bytes: got %q want %q", got, "created\n")
	}
}

func TestAtomicPublishCreateExistingDestinationFailsWithoutMutation(t *testing.T) {
	t.Parallel()

	session := testutil.NewMemFileSession()
	original := []byte("already here\n")
	if err := afero.WriteFile(session.Fs, "/out.txt", original, 0o600); err != nil {
		t.Fatal(err)
	}

	err := fileops.AtomicPublish(session, fileops.AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: fileops.AtomicPublishCreate,
	}, writeBytes([]byte("replacement\n")))
	if !errors.Is(err, fileops.ErrAtomicDestinationExists) {
		t.Fatalf("want ErrAtomicDestinationExists, got %v", err)
	}

	got := session.Files()["/out.txt"]
	if !bytes.Equal(got, original) {
		t.Fatalf("destination mutated: got %q want %q", got, original)
	}
}

func TestAtomicPublishReplaceWriteFailurePreservesExistingDestination(t *testing.T) {
	t.Parallel()

	session := testutil.NewMemFileSession()
	original := []byte("original\n")
	if err := afero.WriteFile(session.Fs, "/out.txt", original, 0o600); err != nil {
		t.Fatal(err)
	}

	err := fileops.AtomicPublish(session, fileops.AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: fileops.AtomicPublishReplace,
	}, func(writer io.Writer) error {
		if _, writeErr := writer.Write([]byte("partial")); writeErr != nil {
			return writeErr
		}

		return errAtomicPublishInjectedWrite
	})
	if !errors.Is(err, errAtomicPublishInjectedWrite) {
		t.Fatalf("want injected write error, got %v", err)
	}

	got := session.Files()["/out.txt"]
	if !bytes.Equal(got, original) {
		t.Fatalf("destination mutated: got %q want %q", got, original)
	}
}

func TestAtomicPublishAppendStagesExistingBytesAndNewBytes(t *testing.T) {
	t.Parallel()

	session := testutil.NewMemFileSession()
	if err := afero.WriteFile(session.Fs, "/out.txt", []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := fileops.AtomicPublish(session, fileops.AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: fileops.AtomicPublishAppend,
	}, writeBytes([]byte("new\n")))
	if err != nil {
		t.Fatalf("AtomicPublish append: %v", err)
	}

	got := session.Files()["/out.txt"]
	if !bytes.Equal(got, []byte("old\nnew\n")) {
		t.Fatalf("append bytes: got %q want %q", got, "old\nnew\n")
	}
}

func TestAtomicPublishAppendFailurePreservesExistingDestination(t *testing.T) {
	t.Parallel()

	session := testutil.NewMemFileSession()
	original := []byte("old\n")
	if err := afero.WriteFile(session.Fs, "/out.txt", original, 0o600); err != nil {
		t.Fatal(err)
	}

	err := fileops.AtomicPublish(session, fileops.AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: fileops.AtomicPublishAppend,
	}, func(writer io.Writer) error {
		if _, writeErr := writer.Write([]byte("new")); writeErr != nil {
			return writeErr
		}

		return errAtomicPublishInjectedWrite
	})
	if !errors.Is(err, errAtomicPublishInjectedWrite) {
		t.Fatalf("want injected write error, got %v", err)
	}

	got := session.Files()["/out.txt"]
	if !bytes.Equal(got, original) {
		t.Fatalf("destination mutated: got %q want %q", got, original)
	}
}

func TestAtomicPublishRenameFailurePreservesExistingDestination(t *testing.T) {
	t.Parallel()

	session := &renameFailPublishSession{MemTestSession: testutil.NewMemFileSession()}
	original := []byte("original\n")
	if err := afero.WriteFile(session.Fs, "/out.txt", original, 0o600); err != nil {
		t.Fatal(err)
	}

	err := fileops.AtomicPublish(session, fileops.AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: fileops.AtomicPublishReplace,
	}, writeBytes([]byte("replacement\n")))
	if !errors.Is(err, errAtomicPublishInjectedRename) {
		t.Fatalf("want rename error, got %v", err)
	}

	got := session.Files()["/out.txt"]
	if !bytes.Equal(got, original) {
		t.Fatalf("destination mutated: got %q want %q", got, original)
	}
}

func TestAtomicPublishNilFileSession(t *testing.T) {
	t.Parallel()

	err := fileops.AtomicPublish(nil, fileops.AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: fileops.AtomicPublishCreate,
	}, writeBytes([]byte("x")))
	if !errors.Is(err, fileops.ErrNilFileSession) {
		t.Fatalf("want ErrNilFileSession, got %v", err)
	}
}

func TestAtomicPublishNilWriteContentCallback(t *testing.T) {
	t.Parallel()

	session := testutil.NewMemFileSession()
	err := fileops.AtomicPublish(session, fileops.AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: fileops.AtomicPublishCreate,
	}, nil)
	if !errors.Is(err, fileops.ErrAtomicNilWriteContent) {
		t.Fatalf("want ErrAtomicNilWriteContent, got %v", err)
	}
}

func writeBytes(data []byte) func(io.Writer) error {
	return func(writer io.Writer) error {
		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf("write test bytes: %w", err)
		}

		return nil
	}
}

type renameFailPublishSession struct {
	*testutil.MemTestSession
}

func (*renameFailPublishSession) Rename(_, _ string) error {
	return errAtomicPublishInjectedRename
}
