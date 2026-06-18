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

func testRoot() string {
	p := filepath.Join(string([]rune{filepath.Separator}), "glyph-shift-test-root")
	abs, err := filepath.Abs(p)
	if err != nil {
		panic("testRoot Abs: " + err.Error())
	}

	return filepath.Clean(abs)
}

type failAfterWriter struct {
	inner     io.Writer
	after     int
	committed int
	err       error
}

func (f *failAfterWriter) Write(p []byte) (int, error) {
	written, werr := f.inner.Write(p)
	f.committed += written
	if f.committed >= f.after {
		return written, f.err
	}

	return written, werr
}

type renameFailExtractPublishSession struct {
	*testutil.MemTestSession
}

func (*renameFailExtractPublishSession) Rename(_, _ string) error {
	return errExtractInjectedFault
}

var errExtractInjectedFault = errors.New("pipeline_test: injected extract fault")

func TestRunExtract_Success(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "out.txt")

	src := testutil.NewMemSourceOpener()
	if err := afero.WriteFile(src.Fs, srcPath, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	out := testutil.NewMemOutputOpener()
	params := pipeline.ExtractParams{
		SrcPath:  srcPath,
		DestPath: destPath,
		Root:     root,
		Lines:    fileops.LineRange{Start: 1, End: 2},
	}

	res, err := pipeline.RunExtract(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		newMemPublishSession(t, out.Fs),
		params,
	)
	if err != nil {
		t.Fatalf("RunExtract: %v", err)
	}

	if res.LinesExtracted != 2 {
		t.Fatalf("LinesExtracted = %d, want 2", res.LinesExtracted)
	}

	if res.WouldCreatePath != "" {
		t.Fatalf("WouldCreatePath must be empty on apply, got %q", res.WouldCreatePath)
	}

	got := out.FileContent(destPath)
	want := []byte("line1\nline2\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("dest content = %q, want %q", got, want)
	}
}

func TestRunExtract_Append_AppendsToExisting(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "out.txt")

	src := testutil.NewMemSourceOpener()
	if err := afero.WriteFile(src.Fs, srcPath, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	out := testutil.NewMemOutputOpener()
	if err := afero.WriteFile(out.Fs, destPath, []byte("existing line\n"), 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

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
			Lines:    fileops.LineRange{Start: 1, End: 2},
			Append:   true,
		},
	)
	if err != nil {
		t.Fatalf("RunExtract: %v", err)
	}

	if res.LinesExtracted != 2 {
		t.Fatalf("LinesExtracted = %d, want 2", res.LinesExtracted)
	}

	got := out.FileContent(destPath)
	prefix := []byte("existing line\n")
	if len(got) < len(prefix) || !bytes.Equal(got[:len(prefix)], prefix) {
		t.Fatalf("dest content missing prefix %q, got %q", prefix, got)
	}
	rest := got[len(prefix):]
	wantRest := []byte("line1\nline2\n")
	if !bytes.Equal(rest, wantRest) {
		t.Fatalf("dest content after prefix = %q, want %q", rest, wantRest)
	}
}

func TestRunExtract_SourceNotFound(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "missing.txt")
	destPath := filepath.Join(root, "out.txt")

	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()

	_, err := pipeline.RunExtract(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		newMemPublishSession(t, out.Fs),
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 1},
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, pipeline.ErrSourceNotFound) {
		t.Fatalf("expected ErrSourceNotFound, got: %v", err)
	}
}

func TestRunExtract_DestExists_NoForce(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "out.txt")

	src := testutil.NewMemSourceOpener()
	if err := afero.WriteFile(src.Fs, srcPath, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	out := testutil.NewMemOutputOpener()
	if err := afero.WriteFile(out.Fs, destPath, []byte("exists\n"), 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

	_, err := pipeline.RunExtract(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		newMemPublishSession(t, out.Fs),
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 1},
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, pipeline.ErrDestinationExists) {
		t.Fatalf("expected ErrDestinationExists, got: %v", err)
	}
}

func TestRunExtract_Preview_destinationExistsNoForce_failsClassifiedDestinationExistsWritesNothing(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "out.txt")

	src := testutil.NewMemSourceOpener()
	if err := afero.WriteFile(src.Fs, srcPath, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	out := testutil.NewMemOutputOpener()
	existing := []byte("exists\n")
	if err := afero.WriteFile(out.Fs, destPath, existing, 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

	_, err := pipeline.RunExtract(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		newMemPublishSession(t, out.Fs),
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 1},
			Preview:  true,
			Force:    false,
			Append:   false,
		},
	)
	if err == nil {
		t.Fatal("expected preview to fail with destination_exists when dest exists and Force/Append are unset")
	}

	assertDestinationExistsOperationContract(t, err)

	gotPath, ok := pipeline.DestinationPathFromError(err)
	if !ok {
		t.Fatal("expected destination path on ErrDestinationExists")
	}

	if want := mustAbsCanonicalPath(t, destPath); gotPath != want {
		t.Fatalf("DestinationPathFromError = %q want %q", gotPath, want)
	}

	got := out.FileContent(destPath)
	if !bytes.Equal(got, existing) {
		t.Fatalf("preview must leave existing destination bytes unchanged; got %q want %q", got, existing)
	}
}

func TestRunExtract_Preview_SuccessCarriesAbsoluteWouldCreatePathAndWritesNoFile(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "preview-target.txt")

	src := testutil.NewMemSourceOpener()
	if err := afero.WriteFile(src.Fs, srcPath, []byte("only\n"), 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

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
			Lines:    fileops.LineRange{Start: 1, End: 1},
			Preview:  true,
		},
	)
	if err != nil {
		t.Fatalf("RunExtract preview: %v", err)
	}

	if res.LinesExtracted != 1 {
		t.Fatalf("LinesExtracted = %d, want 1", res.LinesExtracted)
	}

	wantAbs := mustAbsCanonicalPath(t, destPath)
	if res.WouldCreatePath != wantAbs {
		t.Fatalf("WouldCreatePath = %q want %q", res.WouldCreatePath, wantAbs)
	}

	exists, existsErr := afero.Exists(out.Fs, destPath)
	if existsErr != nil {
		t.Fatalf("Exists: %v", existsErr)
	}

	if exists {
		t.Fatal("preview must not create the destination file")
	}
}

func TestRunExtract_Force_Overwrites(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "out.txt")

	src := testutil.NewMemSourceOpener()
	if err := afero.WriteFile(src.Fs, srcPath, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	out := testutil.NewMemOutputOpener()
	if err := afero.WriteFile(out.Fs, destPath, []byte("original\n"), 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

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
			Lines:    fileops.LineRange{Start: 1, End: 2},
			Force:    true,
		},
	)
	if err != nil {
		t.Fatalf("RunExtract: %v", err)
	}

	if res.LinesExtracted != 2 {
		t.Fatalf("LinesExtracted = %d, want 2", res.LinesExtracted)
	}
}

func TestRunExtract_EmptyRangeLeavesDestinationAbsent(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "dest.txt")

	src := testutil.NewMemSourceOpener()
	if err := afero.WriteFile(src.Fs, srcPath, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	out := testutil.NewMemOutputOpener()

	_, err := pipeline.RunExtract(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		newMemPublishSession(t, out.Fs),
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     root,
			Lines:    fileops.LineRange{Start: 2, End: 1}, // empty range → ErrEmptyRange
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, fileops.ErrEmptyRange) {
		t.Fatalf("expected ErrEmptyRange, got: %v", err)
	}

	if out.FileExists(destPath) {
		t.Fatalf("destination must not be created on empty range failure")
	}
}

func TestRunExtract_RangeExceedsFileLeavesDestinationAbsent(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "dest.txt")

	src := testutil.NewMemSourceOpener()
	if err := afero.WriteFile(src.Fs, srcPath, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	out := testutil.NewMemOutputOpener()

	_, err := pipeline.RunExtract(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		newMemPublishSession(t, out.Fs),
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 99},
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, fileops.ErrRangeExceedsFile) {
		t.Fatalf("expected ErrRangeExceedsFile, got: %v", err)
	}

	if out.FileExists(destPath) {
		t.Fatalf("destination must not be created on range overflow failure")
	}
}

func TestRunExtract_BinarySource(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "bin.dat")
	destPath := filepath.Join(root, "out.txt")

	src := testutil.NewMemSourceOpener()
	if err := afero.WriteFile(src.Fs, srcPath, []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	out := testutil.NewMemOutputOpener()

	_, err := pipeline.RunExtract(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		newMemPublishSession(t, out.Fs),
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 1},
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, pipeline.ErrBinarySource) {
		t.Fatalf("expected ErrBinarySource, got: %v", err)
	}
}

// TestRunExtract_OpenEndedWriteFailureMustNotLeavePartialFinalDestination
// Invariant: open-ended extract write failure cannot leave a durable partial final destination.
func TestRunExtract_OpenEndedWriteFailureMustNotLeavePartialFinalDestination(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "out.txt")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte("line1\nline2\nline3\n"))

	through := testutil.NewThroughMemOutputOpener()

	publishFS := testutil.NewMemStagingPublishSession(through.Fs, func(stagingPath string, w io.Writer) io.Writer {
		_ = stagingPath

		return &failAfterWriter{inner: w, after: len("line1\n"), err: errExtractInjectedFault}
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
			Lines:    fileops.LineRange{Start: 1, End: 0},
			Mkdir:    true,
		},
	)
	if err == nil {
		t.Fatal("expected injected extract fault")
	}

	if !errors.Is(err, errExtractInjectedFault) {
		t.Fatalf("want injected fault, got %v", err)
	}

	if through.FileExists(destPath) {
		t.Fatalf("final destination must not exist after failed atomic staging write; got %q", through.FileContent(destPath))
	}
}
