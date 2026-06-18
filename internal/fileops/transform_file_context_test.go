package fileops_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

func TestTransformFileWithContext_ContextCanceledBeforeStreaming(t *testing.T) {
	t.Parallel()

	fs := testutil.NewMemFileSession()
	var sb strings.Builder
	for i := 0; i < 2500; i++ {
		_, _ = fmt.Fprintf(&sb, "line-%d\n", i)
	}

	if err := afero.WriteFile(fs.Fs, "/stream-cancel.txt", []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	lf := fileops.TargetLF
	_, err := fileops.TransformFileWithContext(
		ctx,
		"/stream-cancel.txt",
		fileops.TransformOptions{LineEndings: &lf},
		false,
		fs,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestTransformFileWithContext_BinaryMarkedSkippedWhenApplyTrue(t *testing.T) {
	t.Parallel()

	fs := seedTransformFile(t, "/binary.txt", []byte("text\x00binary"))
	lf := fileops.TargetLF

	got, err := fileops.TransformFileWithContext(
		context.Background(),
		"/binary.txt",
		fileops.TransformOptions{LineEndings: &lf},
		true,
		fs,
	)
	if err != nil {
		t.Fatalf("TransformFileWithContext: %v", err)
	}
	if !got.Skipped || got.SkipReason != "binary" {
		t.Fatalf("binary result: got %#v", got)
	}
	assertTransformFileBytes(t, fs, "/binary.txt", []byte("text\x00binary"))
}

func TestTransformFileWithContext_NoActiveOpts_ReturnsIdentityResultWithoutCommit(t *testing.T) {
	t.Parallel()

	original := []byte("line  \n")
	fs := seedTransformFile(t, "/identity.txt", original)

	got, err := fileops.TransformFileWithContext(
		context.Background(),
		"/identity.txt",
		fileops.TransformOptions{},
		true,
		fs,
	)
	if err != nil {
		t.Fatalf("TransformFileWithContext: %v", err)
	}
	if !got.Skipped || got.SkipReason != "no transform" || got.WouldChange {
		t.Fatalf("identity result: got %#v", got)
	}
	assertTransformFileBytes(t, fs, "/identity.txt", original)
}

func TestTransformFileWithContext_ActiveOptsApplyFalse_DoesNotRenameOutput(t *testing.T) {
	t.Parallel()

	original := []byte("line  \n")
	fs := seedTransformFile(t, "/preview.txt", original)

	got, err := fileops.TransformFileWithContext(
		context.Background(),
		"/preview.txt",
		fileops.TransformOptions{TrimTrailing: true},
		false,
		fs,
	)
	if err != nil {
		t.Fatalf("TransformFileWithContext: %v", err)
	}
	if !got.WouldChange || got.TrailingTrimmed != 1 {
		t.Fatalf("preview result: got %#v", got)
	}
	assertTransformFileBytes(t, fs, "/preview.txt", original)
}

func TestTransformFileWithContext_ActiveOptsApplyTrue_CommitsTransformedBytes(t *testing.T) {
	t.Parallel()

	fs := seedTransformFile(t, "/apply.txt", []byte("line  \n"))

	got, err := fileops.TransformFileWithContext(
		context.Background(),
		"/apply.txt",
		fileops.TransformOptions{TrimTrailing: true},
		true,
		fs,
	)
	if err != nil {
		t.Fatalf("TransformFileWithContext: %v", err)
	}
	if !got.WouldChange || got.TrailingTrimmed != 1 {
		t.Fatalf("apply result: got %#v", got)
	}
	assertTransformFileBytes(t, fs, "/apply.txt", []byte("line\n"))
}

func TestTransformFileWithContext_StreamErrorSurfacesWithoutCommit(t *testing.T) {
	t.Parallel()

	original := []byte("line  \n")
	fs := seedTransformFile(t, "/stream-error.txt", original)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fileops.TransformFileWithContext(
		ctx,
		"/stream-error.txt",
		fileops.TransformOptions{TrimTrailing: true},
		true,
		fs,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	assertTransformFileBytes(t, fs, "/stream-error.txt", original)
}

func seedTransformFile(t *testing.T, path string, content []byte) *testutil.MemTestSession {
	t.Helper()

	fs := testutil.NewMemFileSession()
	if err := afero.WriteFile(fs.Fs, path, content, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	return fs
}

func assertTransformFileBytes(t *testing.T, fs *testutil.MemTestSession, path string, want []byte) {
	t.Helper()

	got, err := afero.ReadFile(fs.Fs, path)
	if err != nil {
		t.Fatalf("read transformed file: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("file bytes: got %q want %q", got, want)
	}
}
