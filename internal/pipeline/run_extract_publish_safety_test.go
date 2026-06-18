package pipeline_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

// TestRunExtract_ForceWriteFailureMustPreservePreviousDestinationBytes
// Invariant: extract --force write failure must leave the previous destination bytes unchanged.
func TestRunExtract_ForceWriteFailureMustPreservePreviousDestinationBytes(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "out.txt")

	orig := []byte("prior destination bytes\n")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte("line1\nline2\n"))

	through := testutil.NewThroughMemOutputOpener()
	if err := afero.WriteFile(through.Fs, destPath, orig, 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

	publishFS := testutil.NewMemStagingPublishSession(through.Fs, func(stagingPath string, w io.Writer) io.Writer {
		_ = stagingPath

		return &failAfterWriter{inner: w, after: 4, err: errExtractInjectedFault}
	})

	_, err := pipeline.RunExtract(
		context.Background(),
		src,
		through,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 1},
			Force:    true,
			Mkdir:    true,
		},
	)
	if err == nil {
		t.Fatal("expected injected extract fault")
	}

	if !errors.Is(err, errExtractInjectedFault) {
		t.Fatalf("want injected fault, got %v", err)
	}

	got := through.FileContent(destPath)
	if !bytes.Equal(got, orig) {
		t.Fatalf("destination must remain unchanged on force failure; got %q want %q", got, orig)
	}
}

// TestRunExtract_AppendWriteFailureMustPreservePreviousDestinationBytes
// Invariant: extract --append write failure must leave the previous destination bytes unchanged.
func TestRunExtract_AppendWriteFailureMustPreservePreviousDestinationBytes(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "out.txt")

	orig := []byte("prior tail\n")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte("line1\n"))

	through := testutil.NewThroughMemOutputOpener()
	if err := afero.WriteFile(through.Fs, destPath, orig, 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

	publishFS := testutil.NewMemStagingPublishSession(through.Fs, func(stagingPath string, w io.Writer) io.Writer {
		_ = stagingPath

		return &failAfterWriter{inner: w, after: 1, err: errExtractInjectedFault}
	})

	_, err := pipeline.RunExtract(
		context.Background(),
		src,
		through,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 1},
			Append:   true,
			Mkdir:    true,
		},
	)
	if err == nil {
		t.Fatal("expected injected extract fault")
	}

	if !errors.Is(err, errExtractInjectedFault) {
		t.Fatalf("want injected fault, got %v", err)
	}

	got := through.FileContent(destPath)
	if !bytes.Equal(got, orig) {
		t.Fatalf("destination must remain unchanged on append failure; got %q want %q", got, orig)
	}
}

// TestRunExtract_FinalizePublishFailureMustSurfaceWithoutPublishedDestination verifies that when the
// atomic publication finalize step (rename/replace onto DestPath) fails, the error surfaces and the
// destination path is not left published (no false success).
func TestRunExtract_FinalizePublishFailureMustSurfaceWithoutPublishedDestination(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "out.txt")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte("only\n"))

	through := testutil.NewThroughMemOutputOpener()

	publishFS := &renameFailExtractPublishSession{MemTestSession: newMemPublishSession(t, through.Fs)}

	_, err := pipeline.RunExtract(
		context.Background(),
		src,
		through,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 1},
			Mkdir:    true,
		},
	)
	if err == nil {
		t.Fatal("expected error when atomic publish rename fails")
	}

	if !errors.Is(err, errExtractInjectedFault) {
		t.Fatalf("want injected fault in chain, got %v", err)
	}

	if through.FileExists(destPath) {
		t.Fatalf("destination must not be published when rename fails; got %q", through.FileContent(destPath))
	}
}

// TestRunExtract_MixedLineTerminators_ClosedRange verifies planning vs replay SHA256 alignment when logical
// lines use CRLF, lone CR, and a final line without a newline (nil terminator): hashes digest serialized
// content+terminator bytes only (bounded RAM via streaming).
func TestRunExtract_MixedLineTerminators_ClosedRange(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "out.txt")

	srcRaw := []byte("alpha\r\nbeta\rgamma")
	wantLines2Through3 := []byte("beta\rgamma")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, srcRaw)

	out := testutil.NewMemOutputOpener()

	res, err := pipeline.RunExtract(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		newMemPublishSession(t, out.Fs),
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     root,
			Lines:    fileops.LineRange{Start: 2, End: 3},
		},
	)
	if err != nil {
		t.Fatalf("RunExtract: %v", err)
	}

	if res.LinesExtracted != 2 {
		t.Fatalf("LinesExtracted = %d, want 2", res.LinesExtracted)
	}

	got := out.FileContent(destPath)
	if !bytes.Equal(got, wantLines2Through3) {
		t.Fatalf("dest = %q (%[1]x), want %q (%[2]x)", got, wantLines2Through3)
	}
}

// TestRunExtract_MixedLineTerminators_FullRange echoes byte-stable hashing across all diversified lines.
func TestRunExtract_MixedLineTerminators_FullRange(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "out.txt")

	srcRaw := []byte("alpha\r\nbeta\rgamma")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, srcRaw)

	out := testutil.NewMemOutputOpener()

	res, err := pipeline.RunExtract(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		newMemPublishSession(t, out.Fs),
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 3},
		},
	)
	if err != nil {
		t.Fatalf("RunExtract: %v", err)
	}

	if res.LinesExtracted != 3 {
		t.Fatalf("LinesExtracted = %d, want 3", res.LinesExtracted)
	}

	got := out.FileContent(destPath)
	if !bytes.Equal(got, srcRaw) {
		t.Fatalf("dest = %q (%[1]x), want %q (%[2]x)", got, srcRaw)
	}
}
