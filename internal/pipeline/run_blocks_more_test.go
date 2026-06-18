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
	"github.com/spf13/afero"
)

func TestRunBlocks_SourceDerivedExtensionInvalidFailsValidation(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath, outDir := filepath.Join(root, "in.my_ext"), filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(runBlocksDualInnerSource))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	startRE, endRE := regexpBlocksStdFence()

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
			Extension:      "",
			Mkdir:          true,
		},
	)
	if err == nil {
		t.Fatal("expected error when Extension is omitted and filepath.Ext(src) violates ValidateExtension")
	}

	if !errors.Is(err, validate.ErrInvalidExtension) {
		t.Fatalf("expected ErrInvalidExtension, got: %v", err)
	}
}

func TestRunBlocks_NoBlocksFoundLeavesNoFiles(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath, outDir := filepath.Join(root, "in.txt"), filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte("plain\ntext\n"))

	out := testutil.NewMemOutputOpener()
	if err := out.Fs.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir out: %v", err)
	}

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
			StartDelimiter: regexp.MustCompile("^```go$"),
			EndDelimiter:   regexp.MustCompile("^```$"),
			Naming:         fileops.Sequential,
			Extension:      ".txt",
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, fileops.ErrNoBlocksFound) {
		t.Fatalf("expected ErrNoBlocksFound, got: %v", err)
	}

	entries, rdErr := afero.ReadDir(out.Fs, outDir)
	if rdErr != nil {
		t.Fatalf("ReadDir: %v", rdErr)
	}

	if len(entries) != 0 {
		t.Fatalf("out dir must have no files on no-blocks failure, got %d entries", len(entries))
	}
}

func TestRunBlocks_OmittedExtensionUsesSourceExtension(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath, outDir := filepath.Join(root, "in.md"), filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := runBlocksDualInnerSource
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	startRE, endRE := regexpBlocksStdFence()

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
			Naming:         fileops.Sequential,
			Extension:      "",
			Mkdir:          true,
		},
	)
	if err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}

	if pres.Files[0] != mustAbsPlannedOutputPath(t, outDir, "001.md") ||
		pres.Files[1] != mustAbsPlannedOutputPath(t, outDir, "002.md") {
		t.Fatalf("Files = %#v, want absolute 001.md / 002.md", pres.Files)
	}
}

func TestRunBlocks_InvalidDeclaredExtension(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath, outDir := filepath.Join(root, "in.md"), filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte("<<BEGIN>>\nx\n<<END>>\n"))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	startRE, endRE := regexpBlocksStdFence()

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
			Extension:      "..",
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, validate.ErrInvalidExtension) {
		t.Fatalf("expected ErrInvalidExtension, got: %v", err)
	}
}

func TestRunBlocks_MaxFilesExceeded(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath, outDir := filepath.Join(root, "in.txt"), filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := runBlocksDualInnerSource
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	startRE, endRE := regexpBlocksStdFence()

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
			MaxFiles:       1,
			Mkdir:          true,
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, pipeline.ErrMaxFilesExceeded) {
		t.Fatalf("want ErrMaxFilesExceeded, got %v", err)
	}
}
