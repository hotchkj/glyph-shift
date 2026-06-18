package pipeline_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
)

const wantClassifierSourceFingerprintMismatch = "source_fingerprint_mismatch"

func assertSpanFingerprintMismatchClassified(t *testing.T, err error, primaryPath string) {
	t.Helper()
	if !errors.Is(err, fileops.ErrSpanFingerprintMismatch) {
		t.Fatalf("errors.Is SpanFingerprintMismatch: got %v", err)
	}

	co := pipeline.ClassifyOperationError(err, primaryPath)
	if co.Error != wantClassifierSourceFingerprintMismatch {
		t.Fatalf("ClassifyOperationError.Error = %q, want %s", co.Error, wantClassifierSourceFingerprintMismatch)
	}
}

func assertGoldenSplitHugeLineOutputs(
	t *testing.T,
	pres pipeline.SplitPipelineResult,
	out *testutil.MemOutputOpener,
	outDir string,
	want1, want2, want3 []byte,
) {
	t.Helper()

	for i := range pres.Sections {
		if len(pres.Sections[i].Lines) != 0 {
			t.Fatalf("production SplitSection[%d].Lines must be unset (got len=%d)", i, len(pres.Sections[i].Lines))
		}
	}

	if !bytes.Equal(out.FileContent(filepath.Join(outDir, "001.txt")), want1) {
		t.Fatalf("section 001 bytes mismatch")
	}

	if !bytes.Equal(out.FileContent(filepath.Join(outDir, "002.txt")), want2) {
		t.Fatalf("section 002 bytes mismatch")
	}

	if !bytes.Equal(out.FileContent(filepath.Join(outDir, "003.txt")), want3) {
		t.Fatalf("section 003 bytes mismatch")
	}
}

// Split/blocks organic ErrSpanFingerprintMismatch is asserted via divergent seekable sources (bounded
// scan vs span replay on the same handle)—the intentional production-path proof.
// [fileops.AtomicPublishStagingDecorator] staging is used only to exercise propagation and
// ClassifyOperationError: fileops.CopySpanToWriterWithSHA256Verify hashes the same buffers passed to
// io.MultiWriter(dst, digestHasher), so mutating only the staging-wrapped writer does not necessarily
// alter digest input or yield organic mismatch.
func runDivergentSplitHugeFingerprintCheck(
	t *testing.T,
	root, srcPath, outDir string,
	re *regexp.Regexp,
) {
	t.Helper()

	huge2 := strings.Repeat("P", 10000)
	scanBody := "---\n" + huge2 + "\n---\nC\n---\nD\n"
	copyBody := "---\n" + huge2 + "\n---\nX\n---\nD\n"
	if len(scanBody) != len(copyBody) {
		t.Fatal("divergent split buffers must stay equal length")
	}

	pad := strings.Repeat("@\n", 6000)
	scanBytes := append([]byte(pad), []byte(scanBody)...)
	copyBytes := append([]byte(pad), []byte(copyBody)...)

	srcDiv := &divergentSplitSourceOpener{allowedPath: srcPath, scan: scanBytes, copyBuf: copyBytes}

	outDiv := testutil.NewMemOutputOpener()
	pubDiv := newMemPublishSession(t, outDiv.Fs)

	_, err := pipeline.RunSplit(
		context.Background(),
		srcDiv,
		outDiv,
		testutil.NoSymlinkPathResolver{},
		pubDiv,
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
		t.Fatal("expected span fingerprint mismatch when scan bytes and replay bytes diverge")
	}

	assertSpanFingerprintMismatchClassified(t, err, srcPath)
}

func assertGoldenManyInnerBlocksOutputs(
	t *testing.T,
	pres *pipeline.BlocksPipelineResult,
	out *testutil.MemOutputOpener,
	outDir string,
	want1, want2 []byte,
) {
	t.Helper()

	if pres.BlocksFound != 2 {
		t.Fatalf("BlocksFound = %d, want 2", pres.BlocksFound)
	}

	for i := range pres.Blocks {
		if len(pres.Blocks[i].Lines) != 0 {
			t.Fatalf("production Block[%d].Lines must be unset (got len=%d)", i, len(pres.Blocks[i].Lines))
		}
	}

	if !bytes.Equal(out.FileContent(filepath.Join(outDir, "001.txt")), want1) {
		t.Fatalf("block 001 bytes mismatch")
	}

	if !bytes.Equal(out.FileContent(filepath.Join(outDir, "002.txt")), want2) {
		t.Fatalf("block 002 bytes mismatch")
	}
}

func runDivergentBlocksManyInnerFingerprintCheck(
	t *testing.T,
	root, srcPath, outDir string,
	startRE, endRE *regexp.Regexp,
) {
	t.Helper()

	var scanB strings.Builder
	scanB.WriteString("header\n<<BEGIN>>\n")
	for range 500 {
		scanB.WriteString("inner\n")
	}
	scanB.WriteString("<<END>>\n<<BEGIN>>\nsecond\n<<END>>\n")

	var copyB strings.Builder
	copyB.WriteString("header\n<<BEGIN>>\n")
	for range 500 {
		copyB.WriteString("inner\n")
	}
	copyB.WriteString("<<END>>\n<<BEGIN>>\nzecund\n<<END>>\n")

	scanTriple := scanB.String()
	copyTriple := copyB.String()
	if len(scanTriple) != len(copyTriple) {
		t.Fatal("divergent blocks buffers must stay equal length")
	}

	pad := strings.Repeat("@\n", 6000)
	scanBytes := append([]byte(pad), []byte(scanTriple)...)
	copyBytes := append([]byte(pad), []byte(copyTriple)...)

	srcDiv := &divergentSplitSourceOpener{allowedPath: srcPath, scan: scanBytes, copyBuf: copyBytes}

	outDiv := testutil.NewMemOutputOpener()
	pubDiv := newMemPublishSession(t, outDiv.Fs)

	_, err := pipeline.RunBlocks(
		context.Background(),
		srcDiv,
		outDiv,
		testutil.NoSymlinkPathResolver{},
		pubDiv,
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
		t.Fatal("expected span fingerprint mismatch when scan bytes and replay bytes diverge")
	}

	assertSpanFingerprintMismatchClassified(t, err, srcPath)
}

// TestProductionBoundary_Extract_FingerprintMismatchClassifiesAndSkipsPublish uses the seekable extract
// validation/replay divergence seam: planning observes pass1 bytes while span replay reads pass2 bytes.
func TestProductionBoundary_Extract_FingerprintMismatchClassifiesAndSkipsPublish(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "out.txt")

	pass1 := []byte("a\nb\n")
	pass2 := []byte("x\nb\n")

	src := &divergentExtractSourceOpener{allowedPath: srcPath, pass1: pass1, pass2: pass2}

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)

	_, err := pipeline.RunExtract(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 1},
		},
	)
	if err == nil {
		t.Fatal("expected span fingerprint mismatch when validation and replay bytes diverge")
	}

	assertSpanFingerprintMismatchClassified(t, err, srcPath)

	if out.FileExists(destPath) {
		t.Fatalf("destination must not be reported as successfully written; path exists: %q", destPath)
	}
}

// TestProductionBoundary_Split_HugeLogicalLineUsesSpanPublishAndRejectsCorruptWrite builds a delimiter split
// where one section contains a long non-delimiter logical line; production results carry names only (no Lines).
func TestProductionBoundary_Split_HugeLogicalLineUsesSpanPublishAndRejectsCorruptWrite(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	hugeLine := strings.Repeat("P", 10000)
	content := "---\n" + hugeLine + "\n---\nC\n---\nD\n"

	want1 := []byte("---\n" + hugeLine + "\n")
	want2 := []byte("---\nC\n")
	want3 := []byte("---\nD\n")

	src := testutil.NewMemSourceOpener()
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
		t.Fatalf("RunSplit success path: %v", err)
	}

	assertGoldenSplitHugeLineOutputs(t, pres, out, outDir, want1, want2, want3)

	// Fingerprint mismatch with bounded scan metadata vs replay cannot be triggered by mutating bytes only
	// at the AtomicPublish staging writer: CopySpanToWriterWithSHA256Verify hashes the same source buffers that
	// pass through io.MultiWriter to the digest. Use the scan vs replay seekable source seam instead.
	runDivergentSplitHugeFingerprintCheck(t, root, srcPath, outDir, re)
}

// TestProductionBoundary_Blocks_ManyInnerLinesUsesSpanPublishAndRejectsCorruptWrite writes multiple inner
// lines inside a block; production Blocks carry names only (no Lines materialization). Fingerprint failure
// uses the same seekable scan/replay divergence seam as split (see split test comment).
func TestProductionBoundary_Blocks_ManyInnerLinesUsesSpanPublishAndRejectsCorruptWrite(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	var srcb strings.Builder
	srcb.WriteString("header\n<<BEGIN>>\n")
	for range 500 {
		srcb.WriteString("inner\n")
	}
	srcb.WriteString("<<END>>\n<<BEGIN>>\nsecond\n<<END>>\n")
	content := srcb.String()

	var want1b strings.Builder
	for range 500 {
		want1b.WriteString("inner\n")
	}
	want1 := []byte(want1b.String())
	want2 := []byte("second\n")

	src := testutil.NewMemSourceOpener()
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
			Naming:         fileops.Sequential,
			Extension:      ".txt",
			Mkdir:          true,
		},
	)
	if err != nil {
		t.Fatalf("RunBlocks success path: %v", err)
	}

	assertGoldenManyInnerBlocksOutputs(t, &pres, out, outDir, want1, want2)

	runDivergentBlocksManyInnerFingerprintCheck(t, root, srcPath, outDir, startRE, endRE)
}

type injectFingerprintMismatchWriter struct{}

func (injectFingerprintMismatchWriter) Write(_ []byte) (int, error) {
	return 0, fmt.Errorf("staging writer: %w", fileops.ErrSpanFingerprintMismatch)
}

// TestProductionBoundary_Split_StagingFingerprintMismatchClassification documents that staged span copy
// can surface ErrSpanFingerprintMismatch through ClassifyOperationError. (Byte corruption at the staging
// writer alone does not change the parallel digest in fileops.CopySpanToWriterWithSHA256Verify;
// organic mismatch is covered by the divergent seekable source seam above.)
func TestProductionBoundary_Split_StagingFingerprintMismatchClassification(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	firstClean := filepath.Clean(filepath.Join(outDir, "001.txt"))

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(runSplitTripleSectionSource))

	out := testutil.NewMemOutputOpener()
	publishFS := testutil.NewMemStagingPublishSession(out.Fs, func(destPath string, w io.Writer) io.Writer {
		if filepath.Clean(destPath) != firstClean {
			return w
		}

		return injectFingerprintMismatchWriter{}
	})
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
		t.Fatal("expected error from staging-injected fingerprint mismatch")
	}

	assertSpanFingerprintMismatchClassified(t, err, srcPath)
}

// TestProductionBoundary_Blocks_StagingFingerprintMismatchClassification mirrors the split staging
// classification probe for RunBlocks.
func TestProductionBoundary_Blocks_StagingFingerprintMismatchClassification(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	firstClean := filepath.Clean(filepath.Join(outDir, "001.txt"))

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(runBlocksDualInnerSource))

	out := testutil.NewMemOutputOpener()
	publishFS := testutil.NewMemStagingPublishSession(out.Fs, func(destPath string, w io.Writer) io.Writer {
		if filepath.Clean(destPath) != firstClean {
			return w
		}

		return injectFingerprintMismatchWriter{}
	})
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
			Mkdir:          true,
		},
	)
	if err == nil {
		t.Fatal("expected error from staging-injected fingerprint mismatch")
	}

	assertSpanFingerprintMismatchClassified(t, err, srcPath)
}
