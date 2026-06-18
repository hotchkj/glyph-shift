package testutil_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

func TestTailGuardReadSeekCloser_IssuesSentinelAfterPrefix(t *testing.T) {
	t.Parallel()

	prefix := []byte("alpha\nbeta\n")
	o := testutil.NewTailGuardSourceOpener(prefix, "")

	rc, err := o.Open("any")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = rc.Close() })

	buf := make([]byte, 64)
	readCnt, err := rc.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if readCnt != len(prefix) || !bytes.Equal(buf[:readCnt], prefix) {
		t.Fatalf("first read: got %d bytes %q want full prefix %q", readCnt, buf[:readCnt], prefix)
	}

	_, err = rc.Read(buf)
	if err == nil {
		t.Fatal("expected error on read past prefix")
	}

	if !errors.Is(err, testutil.ErrBoundednessTailConsumptionForbidden) {
		t.Fatalf("want ErrBoundednessTailConsumptionForbidden, got %v", err)
	}
}

func TestTailGuardReadSeekCloser_BinaryWindowReadStaysInPrefix(t *testing.T) {
	t.Parallel()

	padLen := len([]byte("@\n"))
	repeat := testutil.BoundednessBinaryCheckReadWindow/padLen + 1
	pad := bytes.Repeat([]byte("@\n"), repeat)
	suffix := []byte("---\nx\n---\ny\n")
	prefix := append(append(make([]byte, 0, len(pad)+len(suffix)), pad...), suffix...)

	o := testutil.NewTailGuardSourceOpener(prefix, "")
	rc, err := o.Open("fixture")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = rc.Close() })

	window := make([]byte, testutil.BoundednessBinaryCheckReadWindow)
	readCnt, err := rc.Read(window)
	if err != nil {
		t.Fatalf("binary window read: %v", err)
	}

	if readCnt != len(window) {
		t.Fatalf("Read n=%d want %d", readCnt, len(window))
	}

	if _, seekErr := rc.Seek(0, io.SeekStart); seekErr != nil {
		t.Fatalf("Seek: %v", seekErr)
	}
}

func TestMeasurePipelineSplitCountingSrcMem(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := perfTestRoot()

	srcAbs := filepath.Join(root, "in-split-contract.txt")
	outDir := filepath.Join(root, "out-split-contract")

	content := "---\nB\n---\nC\n---\nD\n"

	memFs := afero.NewMemMapFs()

	if mkErr := memFs.MkdirAll(filepath.Dir(srcAbs), 0o755); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}

	if wErr := afero.WriteFile(memFs, srcAbs, []byte(content), 0o644); wErr != nil {
		t.Fatalf("write source: %v", wErr)
	}

	srcOp := &testutil.CountingSourceOpener{Immutable: []byte(content), AllowedPath: srcAbs}

	re := regexp.MustCompile(`^---$`)

	meas, res, runErr := testutil.MeasurePipelineSplitCountingSrcMem(ctx, srcOp, memFs,
		testutil.NewMemPathResolverWithFS(memFs), pipeline.SplitParams{
			SrcPath:   srcAbs,
			OutDir:    outDir,
			Root:      root,
			Delimiter: re,
			Naming:    fileops.Sequential,
			Mkdir:     true,
			MaxFiles:  50,
		})
	if runErr != nil {
		t.Fatalf("RunSplit: %v", runErr)
	}

	requirePositive(t, "SourceBytesRead", meas.SourceBytesRead)
	requirePositive(t, "DestinationBytesWritten", meas.DestinationBytesWritten)
	requireEQ(t, "planned outputs", meas.PlannedOutputCount, len(res.Files))
	requireEQ(t, "Files", len(res.Files), 3)
}

func TestMeasurePipelineBlocksCountingSrcMem(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := perfTestRoot()

	srcAbs := filepath.Join(root, "in-blocks-contract.go")
	outDir := filepath.Join(root, "out-blocks-contract")

	srcText := "```go\nalpha\n```\n"

	memFs := afero.NewMemMapFs()

	if mkErr := memFs.MkdirAll(filepath.Dir(srcAbs), 0o755); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}

	if wErr := afero.WriteFile(memFs, srcAbs, []byte(srcText), 0o644); wErr != nil {
		t.Fatalf("write source: %v", wErr)
	}

	srcOp := &testutil.CountingSourceOpener{Immutable: []byte(srcText), AllowedPath: srcAbs}

	meas, res, runErr := testutil.MeasurePipelineBlocksCountingSrcMem(ctx, srcOp, memFs,
		testutil.NewMemPathResolverWithFS(memFs), pipeline.BlocksParams{
			SrcPath:        srcAbs,
			OutDir:         outDir,
			Root:           root,
			StartDelimiter: regexp.MustCompile("^```go$"),
			EndDelimiter:   regexp.MustCompile("^```$"),
			Naming:         fileops.Sequential,
			Mkdir:          true,
			MaxFiles:       50,
		})
	if runErr != nil {
		t.Fatalf("RunBlocks: %v", runErr)
	}

	requirePositive(t, "SourceBytesRead", meas.SourceBytesRead)
	requirePositive(t, "DestinationBytesWritten", meas.DestinationBytesWritten)
	requireEQ(t, "planned outputs", meas.PlannedOutputCount, len(res.Files))
	requireEQ(t, "Files", len(res.Files), 1)
}

func TestMeasurePipelineSplitCountingSrcMem_NilResolverErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	memFs := afero.NewMemMapFs()
	src := &testutil.CountingSourceOpener{Immutable: []byte("x\n")}

	_, _, err := testutil.MeasurePipelineSplitCountingSrcMem(ctx, src, memFs, nil, pipeline.SplitParams{})
	if err == nil {
		t.Fatal("want error for nil resolver")
	}
}

func TestMeasurePipelineSplitCountingSrcMem_NilMemFSErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	src := &testutil.CountingSourceOpener{Immutable: []byte("x\n")}

	_, _, err := testutil.MeasurePipelineSplitCountingSrcMem(
		ctx,
		src,
		nil,
		testutil.NewMemPathResolverWithFS(afero.NewMemMapFs()),
		pipeline.SplitParams{},
	)
	if err == nil {
		t.Fatal("want error for nil mem Fs")
	}
}

func TestMeasurePipelineSplitCountingSrcMem_NilSourceErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, _, err := testutil.MeasurePipelineSplitCountingSrcMem(
		ctx,
		nil,
		afero.NewMemMapFs(),
		testutil.NewMemPathResolverWithFS(afero.NewMemMapFs()),
		pipeline.SplitParams{},
	)
	if err == nil {
		t.Fatal("want error for nil source opener")
	}
}

func TestMeasurePipelineBlocksCountingSrcMem_NilResolverErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	memFs := afero.NewMemMapFs()
	src := &testutil.CountingSourceOpener{Immutable: []byte("```\na\n```\n")}

	_, _, err := testutil.MeasurePipelineBlocksCountingSrcMem(ctx, src, memFs, nil, pipeline.BlocksParams{})
	if err == nil {
		t.Fatal("want error for nil resolver")
	}
}

func TestMeasurePipelineBlocksCountingSrcMem_NilMemFSErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	src := &testutil.CountingSourceOpener{Immutable: []byte("```\na\n```\n")}

	_, _, err := testutil.MeasurePipelineBlocksCountingSrcMem(
		ctx,
		src,
		nil,
		testutil.NewMemPathResolverWithFS(afero.NewMemMapFs()),
		pipeline.BlocksParams{},
	)
	if err == nil {
		t.Fatal("want error for nil mem Fs")
	}
}

func TestMeasurePipelineBlocksCountingSrcMem_NilSourceErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, _, err := testutil.MeasurePipelineBlocksCountingSrcMem(
		ctx,
		nil,
		afero.NewMemMapFs(),
		testutil.NewMemPathResolverWithFS(afero.NewMemMapFs()),
		pipeline.BlocksParams{},
	)
	if err == nil {
		t.Fatal("want error for nil source opener")
	}
}

func TestSplitBlocksPerformanceMeasurementsRecordPreviewDestinationBytes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := perfTestRoot()

	t.Run("SplitPreview", func(t *testing.T) {
		srcAbs := filepath.Join(root, "in-split-preview-contract.txt")
		outDir := filepath.Join(root, "out-split-preview-contract")

		body := "---\nline 1\n"

		memFs := afero.NewMemMapFs()

		if mkErr := memFs.MkdirAll(filepath.Dir(srcAbs), 0o755); mkErr != nil {
			t.Fatalf("mkdir: %v", mkErr)
		}

		if wErr := afero.WriteFile(memFs, srcAbs, []byte(body), 0o644); wErr != nil {
			t.Fatalf("write source: %v", wErr)
		}

		srcOp := &testutil.CountingSourceOpener{Immutable: []byte(body), AllowedPath: srcAbs}

		meas, _, runErr := testutil.MeasurePipelineSplitCountingSrcMem(ctx, srcOp, memFs,
			testutil.NewMemPathResolverWithFS(memFs), pipeline.SplitParams{
				SrcPath:   srcAbs,
				OutDir:    outDir,
				Root:      root,
				Delimiter: regexp.MustCompile(`^---$`),
				Naming:    fileops.Sequential,
				Preview:   true,
				MaxFiles:  50,
			})
		if runErr != nil {
			t.Fatalf("RunSplit preview: %v", runErr)
		}

		requireZero(t, "preview destination bytes written", meas.DestinationBytesWritten)
	})

	t.Run("BlocksPreview", func(t *testing.T) {
		srcAbs := filepath.Join(root, "in-blocks-preview-contract.go")
		outDir := filepath.Join(root, "out-blocks-preview-contract")

		body := "```go\nx\n```\n"

		memFs := afero.NewMemMapFs()

		if mkErr := memFs.MkdirAll(filepath.Dir(srcAbs), 0o755); mkErr != nil {
			t.Fatalf("mkdir: %v", mkErr)
		}

		if wErr := afero.WriteFile(memFs, srcAbs, []byte(body), 0o644); wErr != nil {
			t.Fatalf("write source: %v", wErr)
		}

		srcOp := &testutil.CountingSourceOpener{Immutable: []byte(body), AllowedPath: srcAbs}

		meas, _, runErr := testutil.MeasurePipelineBlocksCountingSrcMem(ctx, srcOp, memFs,
			testutil.NewMemPathResolverWithFS(memFs), pipeline.BlocksParams{
				SrcPath:        srcAbs,
				OutDir:         outDir,
				Root:           root,
				StartDelimiter: regexp.MustCompile("^```go$"),
				EndDelimiter:   regexp.MustCompile("^```$"),
				Naming:         fileops.Sequential,
				Preview:        true,
				MaxFiles:       50,
			})
		if runErr != nil {
			t.Fatalf("RunBlocks preview: %v", runErr)
		}

		requireZero(t, "preview destination bytes written", meas.DestinationBytesWritten)
	})
}

func TestNonRetainingOutputOpener_BytesCountedAcrossWrites(t *testing.T) {
	t.Parallel()

	nrOut := testutil.NewNonRetainingOutputOpener()
	path := filepath.Join(string([]rune{filepath.Separator}), "out", "a.txt")

	wc, openErr := nrOut.OpenFile(path, testutil.OutputWriteExclusiveCreate, 0o644)
	if openErr != nil {
		t.Fatalf("OpenFile: %v", openErr)
	}

	want := []byte("alpha-beta-gamma")

	n1, writeErr := wc.Write(want[:5])
	if writeErr != nil || n1 != 5 {
		t.Fatalf("Write first chunk: nn=%d err=%v", n1, writeErr)
	}

	n2, writeErr := wc.Write(want[5:])
	if writeErr != nil || n2 != len(want)-5 {
		t.Fatalf("Write second chunk: nn=%d err=%v", n2, writeErr)
	}

	if closeErr := wc.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	if got := nrOut.BytesWritten(); got != int64(len(want)) {
		t.Fatalf("BytesWritten: got %d want %d", got, len(want))
	}
}

func assertNonRetainingOutputDiscardsCommittedSnapshot(t *testing.T, path string, payload []byte, wantWritten int64) {
	t.Helper()

	nrOut := testutil.NewNonRetainingOutputOpener()

	wc, openErr := nrOut.OpenFile(path, testutil.OutputWriteExclusiveCreate, 0o644)
	if openErr != nil {
		t.Fatalf("OpenFile: %v", openErr)
	}

	if _, writeErr := wc.Write(payload); writeErr != nil {
		t.Fatalf("Write: %v", writeErr)
	}

	if closeErr := wc.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	if snap := nrOut.OutputBytesSnapshot(path); snap != nil {
		t.Fatalf("OutputBytesSnapshot: got %d bytes want nil", len(snap))
	}

	if nrOut.BytesWritten() != wantWritten {
		t.Fatalf("BytesWritten: got %d want %d", nrOut.BytesWritten(), wantWritten)
	}
}

func assertCountingOutputRetainsSnapshot(t *testing.T, path string, payload []byte) {
	t.Helper()

	countOp := testutil.NewCountingOutputOpener()

	cwc, openErr := countOp.OpenFile(path, testutil.OutputWriteExclusiveCreate, 0o644)
	if openErr != nil {
		t.Fatalf("OpenFile counting: %v", openErr)
	}

	if _, werr := cwc.Write(payload); werr != nil {
		t.Fatalf("Write counting: %v", werr)
	}

	if cerr := cwc.Close(); cerr != nil {
		t.Fatalf("Close counting: %v", cerr)
	}

	bs := countOp.OutputBytesSnapshot(path)
	if bs == nil || !bytes.Equal(bs, payload) {
		t.Fatal("CountingOutputOpener should retain an identical snapshot for content assertions")
	}
}

func TestNonRetainingOutputOpener_NoPayloadSnapshotAfterCommit(t *testing.T) {
	t.Parallel()

	payload := []byte("identifiable-retained-marker-xyzzy")
	path := filepath.Join(string([]rune{filepath.Separator}), "out", "committed.txt")

	assertNonRetainingOutputDiscardsCommittedSnapshot(t, path, payload, int64(len(payload)))
	assertCountingOutputRetainsSnapshot(t, path, payload)
}

func TestNonRetainingOutputOpener_OEXCLFailsAfterCommittedClose_OTRUNCClearsEnoughToReopenWriter(t *testing.T) {
	t.Parallel()

	nrOut := testutil.NewNonRetainingOutputOpener()
	path := filepath.Join(string([]rune{filepath.Separator}), "nr-excl-trunc-test.txt")

	wFirst, openFirstErr := nrOut.OpenFile(path, testutil.OutputWriteExclusiveCreate, 0o644)
	if openFirstErr != nil {
		t.Fatalf("first EXCL OpenFile: %v", openFirstErr)
	}

	if closeFirstErr := wFirst.Close(); closeFirstErr != nil {
		t.Fatalf("Close: %v", closeFirstErr)
	}

	_, secondExclErr := nrOut.OpenFile(path, testutil.OutputWriteExclusiveCreate, 0o644)
	if secondExclErr == nil {
		t.Fatal("expected second exclusive create to fail after logical commit")
	}

	if !errors.Is(secondExclErr, fs.ErrExist) {
		t.Fatalf("want fs.ErrExist wrapping, got %v", secondExclErr)
	}

	wTrunc, truncOpenErr := nrOut.OpenFile(path, testutil.OutputWriteTruncCreate, 0o644)
	if truncOpenErr != nil {
		t.Fatalf("TRUNC OpenFile after logical exists: %v", truncOpenErr)
	}

	if closeTruncErr := wTrunc.Close(); closeTruncErr != nil {
		t.Fatalf("Close after TRUNC session: %v", closeTruncErr)
	}

	wTrunc2, trunc2Err := nrOut.OpenFile(path, testutil.OutputWriteTruncCreate, 0o644)
	if trunc2Err != nil {
		t.Fatalf("second TRUNC OpenFile (reopen) after commit: %v", trunc2Err)
	}

	if closeTrunc2Err := wTrunc2.Close(); closeTrunc2Err != nil {
		t.Fatalf("Close second TRUNC session: %v", closeTrunc2Err)
	}

	if _, thriceExclErr := nrOut.OpenFile(path, testutil.OutputWriteExclusiveCreate, 0o644); thriceExclErr == nil {
		t.Fatal("expected EXCL to fail once logical destination exists again")
	}
}
