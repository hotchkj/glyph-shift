package pipeline_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

// goconst: shared sample used by multiple blocks-facing tests.
const runBlocksDualInnerSource = "header\n<<BEGIN>>\ninner1\n<<END>>\n<<BEGIN>>\ninner2\n<<END>>\n"

// runBlocksSingleInnerSource is a minimal single-block fence fixture (one sequential output file).
const runBlocksSingleInnerSource = "<<BEGIN>>\na\n<<END>>\n"

func TestRunBlocks_SingleStdFenceWrites001WithInnerLine(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath, outDir := filepath.Join(root, "in.txt"), filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(runBlocksSingleInnerSource))

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
			Extension:      ".txt",
			Mkdir:          true,
		},
	)
	if err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}

	if pres.BlocksFound != 1 {
		t.Fatalf("BlocksFound = %d, want 1", pres.BlocksFound)
	}

	if len(pres.Files) != 1 {
		t.Fatalf("Files len = %d, want 1", len(pres.Files))
	}

	wantName := mustAbsPlannedOutputPath(t, outDir, "001.txt")
	if pres.Files[0] != wantName {
		t.Fatalf("Files[0] = %q, want %q", pres.Files[0], wantName)
	}

	p1 := filepath.Join(outDir, "001.txt")
	got := out.FileContent(p1)
	want := []byte("a\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("001 content = %q, want %q", got, want)
	}
}

func TestRunBlocks_Success(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath, outDir := filepath.Join(root, "in.txt"), filepath.Join(root, "out")

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
			Extension:      ".txt",
			Mkdir:          true,
		},
	)
	if err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}

	if pres.BlocksFound != 2 {
		t.Fatalf("BlocksFound = %d, want 2", pres.BlocksFound)
	}

	if len(pres.Files) != 2 {
		t.Fatalf("Files len = %d, want 2", len(pres.Files))
	}

	wantNames := []string{
		mustAbsPlannedOutputPath(t, outDir, "001.txt"),
		mustAbsPlannedOutputPath(t, outDir, "002.txt"),
	}
	for i, name := range wantNames {
		if pres.Files[i] != name {
			t.Fatalf("Files[%d] = %q, want %q", i, pres.Files[i], name)
		}
	}

	p1 := filepath.Join(outDir, "001.txt")
	got1 := out.FileContent(p1)
	want1 := []byte("inner1\n")
	if !bytes.Equal(got1, want1) {
		t.Fatalf("001 content = %q, want %q", got1, want1)
	}

	p2 := filepath.Join(outDir, "002.txt")
	got2 := out.FileContent(p2)
	want2 := []byte("inner2\n")
	if !bytes.Equal(got2, want2) {
		t.Fatalf("002 content = %q, want %q", got2, want2)
	}
}

func TestRunBlocks_SourceNotFound(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath, outDir := filepath.Join(root, "missing.txt"), filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
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
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, pipeline.ErrSourceNotFound) {
		t.Fatalf("expected ErrSourceNotFound, got: %v", err)
	}
}

func TestRunBlocks_BinarySource(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath, outDir := filepath.Join(root, "bin.dat"), filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte{0x00, 0x01, 0x02})

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
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, pipeline.ErrBinarySource) {
		t.Fatalf("expected ErrBinarySource, got: %v", err)
	}
}

func TestRunBlocks_DestExists_NoForce(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath, outDir := filepath.Join(root, "in.txt"), filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := runBlocksDualInnerSource
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	firstOut := filepath.Join(outDir, "001.txt")
	wantExisting := []byte("exists\n")
	mustWriteAferoFile(t, out.Fs, firstOut, wantExisting)

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
			Mkdir:          true,
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, pipeline.ErrDestinationExists) {
		t.Fatalf("expected ErrDestinationExists, got: %v", err)
	}

	got := out.FileContent(firstOut)
	if !bytes.Equal(got, wantExisting) {
		t.Fatalf("destination mutated: got %q, want %q", got, wantExisting)
	}
}

func TestRunBlocks_Preview_plannedOutputCollisionNoForce_destinationExistsWritesNothing(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath, outDir := filepath.Join(root, "in.txt"), filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(runBlocksDualInnerSource))

	out := testutil.NewMemOutputOpener()
	if mkErr := out.Fs.MkdirAll(outDir, 0o755); mkErr != nil {
		t.Fatalf("mkdir out: %v", mkErr)
	}

	publishFS := newMemPublishSession(t, out.Fs)
	firstOut := filepath.Join(outDir, "001.txt")
	wantCollision := []byte("exists\n")
	mustWriteAferoFile(t, out.Fs, firstOut, wantCollision)

	entriesBefore, errList := afero.ReadDir(out.Fs, outDir)
	if errList != nil {
		t.Fatalf("ReadDir before: %v", errList)
	}

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
			Mkdir:          true,
			Preview:        true,
			Force:          false,
		},
	)
	if err == nil {
		t.Fatal("expected preview to fail when a planned output path collides and Force is unset")
	}

	assertDestinationExistsOperationContract(t, err)

	gotPath, ok := pipeline.DestinationPathFromError(err)
	if !ok {
		t.Fatal("expected destination path on ErrDestinationExists")
	}

	if want := mustAbsCanonicalPath(t, firstOut); gotPath != want {
		t.Fatalf("DestinationPathFromError = %q want %q", gotPath, want)
	}

	got := out.FileContent(firstOut)
	if !bytes.Equal(got, wantCollision) {
		t.Fatalf("preview collision check must not mutate existing output file; got %q want %q", got, wantCollision)
	}

	entriesAfter, errList2 := afero.ReadDir(out.Fs, outDir)
	if errList2 != nil {
		t.Fatalf("ReadDir after: %v", errList2)
	}

	if len(entriesAfter) != len(entriesBefore) {
		gotNames := make([]string, 0, len(entriesAfter))
		for _, e := range entriesAfter {
			gotNames = append(gotNames, e.Name())
		}
		t.Fatalf("preview must create no new output files on collision failure; before %d after %d got=%v",
			len(entriesBefore), len(entriesAfter), gotNames)
	}
}

func TestRunBlocks_UnclosedBlock_reportsActionableStartLocation(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath, outDir := filepath.Join(root, "in.txt"), filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := "```gherkin\norphan line\n"
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	if err := out.Fs.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir out: %v", err)
	}

	publishFS := newMemPublishSession(t, out.Fs)

	startRE := regexp.MustCompile("^```gherkin")
	endRE := regexp.MustCompile("^```$")

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
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	assertUnclosedBlockErrorCarriesActionableStartLocation(t, err, srcPath)
}

func TestRunBlocks_UnclosedBlockLeavesNoFiles(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath, outDir := filepath.Join(root, "in.txt"), filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := "```gherkin\norphan line\n"
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	if err := out.Fs.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir out: %v", err)
	}

	publishFS := newMemPublishSession(t, out.Fs)

	startRE := regexp.MustCompile("^```gherkin")
	endRE := regexp.MustCompile("^```$")

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
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, fileops.ErrUnclosedBlock) {
		t.Fatalf("expected ErrUnclosedBlock, got: %v", err)
	}

	entries, rdErr := afero.ReadDir(out.Fs, outDir)
	if rdErr != nil {
		t.Fatalf("ReadDir: %v", rdErr)
	}

	if len(entries) != 0 {
		t.Fatalf("out dir must have no files on unclosed block failure, got %d entries", len(entries))
	}
}
