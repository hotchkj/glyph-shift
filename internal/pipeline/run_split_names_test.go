package pipeline_test

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

func TestRunSplit_ExplicitNameRejectedPathSeparator(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := "---\nsolo\n"
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	re := regexp.MustCompile(`^---$`)

	_, err := pipeline.RunSplit(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.SplitParams{
			SrcPath:   srcPath,
			OutDir:    outDir,
			Root:      root,
			Delimiter: re,
			Naming:    fileops.Sequential,
			Extension: ".txt",
			Names:     []string{"sub/a"},
			Mkdir:     true,
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, pipeline.ErrInvalidExplicitName) {
		t.Fatalf("want ErrInvalidExplicitName, got %v", err)
	}
}

func TestRunSplit_ExplicitNameRejectedDuplicate(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := "---\nA\n---\nB\n"
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	re := regexp.MustCompile(`^---$`)

	_, err := pipeline.RunSplit(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.SplitParams{
			SrcPath:   srcPath,
			OutDir:    outDir,
			Root:      root,
			Delimiter: re,
			Naming:    fileops.Sequential,
			Extension: ".txt",
			Names:     []string{"x", "x"},
			Mkdir:     true,
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, pipeline.ErrDuplicateExplicitNames) {
		t.Fatalf("want ErrDuplicateExplicitNames, got %v", err)
	}
}

func TestRunSplitStopsBeforeTailWhenMaxFilesExceeded(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := testRoot()
	srcPath := filepath.Join(root, "bounded-split-in.txt")
	outDir := filepath.Join(root, "bounded-split-out")

	prefix := testutil.BuildMaxFilesExceededSplitPrefix('@', "---\n", 2)
	src := testutil.NewTailGuardSourceOpener(prefix, filepath.Clean(srcPath))
	out := testutil.NewCountingOutputOpener()
	re := regexp.MustCompile(`^---$`)

	publishFS := discardedPublishSession(t)

	_, err := pipeline.RunSplit(
		ctx,
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.SplitParams{
			SrcPath:   srcPath,
			OutDir:    outDir,
			Root:      root,
			Delimiter: re,
			Naming:    fileops.Sequential,
			Extension: ".txt",
			MaxFiles:  2,
			Mkdir:     true,
		},
	)

	if out.DestinationOpens() != 0 {
		t.Fatalf("boundedness: expected no output opens, got %d", out.DestinationOpens())
	}

	if out.BytesWritten() != 0 {
		t.Fatalf("boundedness: expected no output bytes written, got %d", out.BytesWritten())
	}

	if errors.Is(err, testutil.ErrBoundednessTailConsumptionForbidden) {
		t.Fatalf("boundedness (red): consumed logical tail after max-files violation was knowable: %v", err)
	}

	if !errors.Is(err, pipeline.ErrMaxFilesExceeded) {
		t.Fatalf("boundedness: want ErrMaxFilesExceeded, got %v", err)
	}
}

func TestRunSplit_ExplicitNameRejectedReservedStem(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := "---\nsolo\n"
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	re := regexp.MustCompile(`^---$`)

	_, err := pipeline.RunSplit(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.SplitParams{
			SrcPath:   srcPath,
			OutDir:    outDir,
			Root:      root,
			Delimiter: re,
			Naming:    fileops.Sequential,
			Extension: ".txt",
			Names:     []string{"CON"},
			Mkdir:     true,
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, pipeline.ErrInvalidExplicitName) || !errors.Is(err, validate.ErrReservedName) {
		t.Fatalf("want ErrInvalidExplicitName wrapping reserved name, got %v", err)
	}
}
