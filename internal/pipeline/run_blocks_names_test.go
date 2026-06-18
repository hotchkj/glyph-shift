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
)

func TestRunBlocks_ExplicitNames(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := runBlocksDualInnerSource
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	startRE := regexp.MustCompile(`^<<BEGIN>>$`)
	endRE := regexp.MustCompile(`^<<END>>$`)

	pres, err := pipeline.RunBlocks(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.BlocksParams{
			SrcPath:        srcPath,
			OutDir:         outDir,
			Root:           root,
			StartDelimiter: startRE,
			EndDelimiter:   endRE,
			Naming:         fileops.FromContent,
			Extension:      ".txt",
			Names:          []string{"auth", "db"},
			Mkdir:          true,
		},
	)
	if err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}

	if pres.Files[0] != mustAbsPlannedOutputPath(t, outDir, "auth.txt") ||
		pres.Files[1] != mustAbsPlannedOutputPath(t, outDir, "db.txt") {
		t.Fatalf("Files = %#v", pres.Files)
	}
}

func TestRunBlocks_NamesBoundToNonEmptyFiles(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := "```go\n```\n```go\ninner\n```\n"
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	startRE := regexp.MustCompile("^```go")
	endRE := regexp.MustCompile("^```$")

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)

	_, err := pipeline.RunBlocks(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.BlocksParams{
			SrcPath:        srcPath,
			OutDir:         outDir,
			Root:           root,
			StartDelimiter: startRE,
			EndDelimiter:   endRE,
			Naming:         fileops.Sequential,
			Extension:      ".txt",
			Names:          []string{"a", "b"},
			Mkdir:          true,
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, pipeline.ErrNamesCountMismatch) {
		t.Fatalf("want ErrNamesCountMismatch, got %v", err)
	}
}

func TestRunBlocks_ExplicitNameRejectedPathSeparator(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := "header\n<<BEGIN>>\ninner1\n<<END>>\n"
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	startRE := regexp.MustCompile(`^<<BEGIN>>$`)
	endRE := regexp.MustCompile(`^<<END>>$`)

	_, err := pipeline.RunBlocks(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.BlocksParams{
			SrcPath:        srcPath,
			OutDir:         outDir,
			Root:           root,
			StartDelimiter: startRE,
			EndDelimiter:   endRE,
			Naming:         fileops.Sequential,
			Extension:      ".txt",
			Names:          []string{"sub/out"},
			Mkdir:          true,
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, pipeline.ErrInvalidExplicitName) {
		t.Fatalf("want ErrInvalidExplicitName, got %v", err)
	}
}

func TestRunBlocksStopsBeforeTailWhenMaxFilesExceeded(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := testRoot()
	srcPath := filepath.Join(root, "bounded-blocks-in.txt")
	outDir := filepath.Join(root, "bounded-blocks-out")

	prefix := testutil.BuildMaxFilesExceededBlocksPrefix(
		'@',
		"H\n",
		"<<BEGIN>>",
		"b",
		"<<END>>",
		3,
	)
	src := testutil.NewTailGuardSourceOpener(prefix, filepath.Clean(srcPath))
	out := testutil.NewCountingOutputOpener()
	startRE := regexp.MustCompile(`^<<BEGIN>>$`)
	endRE := regexp.MustCompile(`^<<END>>$`)

	publishFS := discardedPublishSession(t)

	_, err := pipeline.RunBlocks(
		ctx,
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.BlocksParams{
			SrcPath:        srcPath,
			OutDir:         outDir,
			Root:           root,
			StartDelimiter: startRE,
			EndDelimiter:   endRE,
			Naming:         fileops.Sequential,
			Extension:      ".txt",
			MaxFiles:       2,
			Mkdir:          true,
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

func TestRunBlocks_ExplicitNameRejectedDuplicate(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := runBlocksDualInnerSource
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	startRE := regexp.MustCompile(`^<<BEGIN>>$`)
	endRE := regexp.MustCompile(`^<<END>>$`)

	_, err := pipeline.RunBlocks(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.BlocksParams{
			SrcPath:        srcPath,
			OutDir:         outDir,
			Root:           root,
			StartDelimiter: startRE,
			EndDelimiter:   endRE,
			Naming:         fileops.Sequential,
			Extension:      ".txt",
			Names:          []string{"z", "z"},
			Mkdir:          true,
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, pipeline.ErrDuplicateExplicitNames) {
		t.Fatalf("want ErrDuplicateExplicitNames, got %v", err)
	}
}
