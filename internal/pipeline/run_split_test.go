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
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

// goconst: shared delimiter-separated sample reused across split tests.
const runSplitTripleSectionSource = "---\nB\n---\nC\n---\nD\n"

func TestRunSplit_Success(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := runSplitTripleSectionSource
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	re := regexp.MustCompile(`^---$`)

	pres, err := pipeline.RunSplit(
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
			Mkdir:     true,
		},
	)
	if err != nil {
		t.Fatalf("RunSplit: %v", err)
	}

	if len(pres.Files) != 3 {
		t.Fatalf("Files len = %d, want 3", len(pres.Files))
	}

	wantNames := []string{
		mustAbsPlannedOutputPath(t, outDir, "001.txt"),
		mustAbsPlannedOutputPath(t, outDir, "002.txt"),
		mustAbsPlannedOutputPath(t, outDir, "003.txt"),
	}
	for i, name := range wantNames {
		if pres.Files[i] != name {
			t.Fatalf("Files[%d] = %q, want %q", i, pres.Files[i], name)
		}
	}

	p1 := filepath.Join(outDir, "001.txt")
	got1 := out.FileContent(p1)
	want1 := []byte("---\nB\n")
	if !bytes.Equal(got1, want1) {
		t.Fatalf("001 content = %q, want %q", got1, want1)
	}

	p2 := filepath.Join(outDir, "002.txt")
	got2 := out.FileContent(p2)
	want2 := []byte("---\nC\n")
	if !bytes.Equal(got2, want2) {
		t.Fatalf("002 content = %q, want %q", got2, want2)
	}

	p3 := filepath.Join(outDir, "003.txt")
	got3 := out.FileContent(p3)
	want3 := []byte("---\nD\n")
	if !bytes.Equal(got3, want3) {
		t.Fatalf("003 content = %q, want %q", got3, want3)
	}
}

func TestRunSplit_SourceNotFound(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "missing.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
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
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, pipeline.ErrSourceNotFound) {
		t.Fatalf("expected ErrSourceNotFound, got: %v", err)
	}
}

func TestRunSplit_BinarySource(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "bin.dat")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte{0x00, 0x01, 0x02})

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
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, pipeline.ErrBinarySource) {
		t.Fatalf("expected ErrBinarySource, got: %v", err)
	}
}

func TestRunSplit_DestExists_NoForce(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := runSplitTripleSectionSource
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	firstOut := filepath.Join(outDir, "001.txt")
	mustWriteAferoFile(t, out.Fs, firstOut, []byte("exists\n"))

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
			Mkdir:     true,
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, pipeline.ErrDestinationExists) {
		t.Fatalf("expected ErrDestinationExists, got: %v", err)
	}
}

func TestRunSplit_Preview_plannedOutputCollisionNoForce_destinationExistsWritesNothing(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(runSplitTripleSectionSource))

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
			Mkdir:     true,
			Preview:   true,
			Force:     false,
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
		got := make([]string, 0, len(entriesAfter))
		for _, e := range entriesAfter {
			got = append(got, e.Name())
		}
		t.Fatalf("preview must create no new output files on collision failure; before %d after %d got=%v",
			len(entriesBefore), len(entriesAfter), got)
	}
}

func TestRunSplit_SourceDerivedExtensionInvalidFailsValidation(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.my_ext")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(runSplitTripleSectionSource))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)

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
			Delimiter: regexp.MustCompile(`^---$`),
			Naming:    fileops.Sequential,
			Extension: "",
			Mkdir:     true,
		},
	)
	if err == nil {
		t.Fatal("expected error when Extension is omitted and filepath.Ext(src) violates ValidateExtension")
	}

	if !errors.Is(err, validate.ErrInvalidExtension) {
		t.Fatalf("expected ErrInvalidExtension, got: %v", err)
	}
}

func TestRunSplit_OmittedExtensionUsesSourceExtension(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.md")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := runSplitTripleSectionSource
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	re := regexp.MustCompile(`^---$`)

	pres, err := pipeline.RunSplit(
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
			Extension: "",
			Mkdir:     true,
		},
	)
	if err != nil {
		t.Fatalf("RunSplit: %v", err)
	}

	wantNames := []string{
		mustAbsPlannedOutputPath(t, outDir, "001.md"),
		mustAbsPlannedOutputPath(t, outDir, "002.md"),
		mustAbsPlannedOutputPath(t, outDir, "003.md"),
	}
	for i, name := range wantNames {
		if pres.Files[i] != name {
			t.Fatalf("Files[%d] = %q, want %q", i, pres.Files[i], name)
		}
	}
}

func TestRunSplit_InvalidDeclaredExtension(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.md")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte("a\n"))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)

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
			Delimiter: regexp.MustCompile(`^$`),
			Naming:    fileops.Sequential,
			Extension: "..",
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, validate.ErrInvalidExtension) {
		t.Fatalf("expected ErrInvalidExtension, got: %v", err)
	}
}

func TestRunSplit_UnknownNaming(t *testing.T) {
	t.Parallel()

	_, err := pipeline.ParseNamingStrategy("bogus")
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, pipeline.ErrUnknownNamingStrategy) {
		t.Fatalf("expected ErrUnknownNamingStrategy, got: %v", err)
	}
}

func TestRunSplit_MaxFilesExceeded(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := runSplitTripleSectionSource
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
			MaxFiles:  2,
			Mkdir:     true,
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, pipeline.ErrMaxFilesExceeded) {
		t.Fatalf("want ErrMaxFilesExceeded, got %v", err)
	}
}

func TestRunSplit_NoDelimiterMatch(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte("plain\ntext\n"))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)

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
			Delimiter: regexp.MustCompile(`^---$`),
			Naming:    fileops.Sequential,
			Extension: ".txt",
			Mkdir:     true,
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, fileops.ErrNoDelimiterMatch) {
		t.Fatalf("want ErrNoDelimiterMatch, got %v", err)
	}
}

func TestRunSplit_ExplicitNames(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := runSplitTripleSectionSource
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	re := regexp.MustCompile(`^---$`)

	pres, err := pipeline.RunSplit(
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
			Naming:    fileops.FromContent,
			Extension: ".txt",
			Names:     []string{"one", "two", "three"},
			Mkdir:     true,
		},
	)
	if err != nil {
		t.Fatalf("RunSplit: %v", err)
	}

	want := []string{
		mustAbsPlannedOutputPath(t, outDir, "one.txt"),
		mustAbsPlannedOutputPath(t, outDir, "two.txt"),
		mustAbsPlannedOutputPath(t, outDir, "three.txt"),
	}
	for i := range want {
		if pres.Files[i] != want[i] {
			t.Fatalf("Files[%d] = %q want %q", i, pres.Files[i], want[i])
		}
	}
}

func TestRunSplit_NamesCountMismatch(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()
	content := runSplitTripleSectionSource
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
			Names:     []string{"only"},
			Mkdir:     true,
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, pipeline.ErrNamesCountMismatch) {
		t.Fatalf("want ErrNamesCountMismatch, got %v", err)
	}
}
