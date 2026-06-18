package testutil_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

func perfTestRoot() string {
	// POSIX-style root keeps afero keys stable when validate.Path applies filepath.Abs on Windows.
	return "/glyph-shift-perf-test-root"
}

func mustPrepareMemExtractWithPreexistingDestination(
	t *testing.T,
	memFs afero.Fs,
	srcAbs, destAbs string,
	srcBytes, existingDest []byte,
) {
	t.Helper()

	if mkErr := memFs.MkdirAll(filepath.Dir(srcAbs), 0o755); mkErr != nil {
		t.Fatalf("seed source parent dirs: %v", mkErr)
	}
	if wErr := afero.WriteFile(memFs, srcAbs, srcBytes, 0o644); wErr != nil {
		t.Fatalf("seed source file: %v", wErr)
	}
	if err := afero.WriteFile(memFs, destAbs, existingDest, 0o644); err != nil {
		t.Fatalf("seed destination: %v", err)
	}
}

func assertDestinationBytesUnchangedAfterFailedExtract(t *testing.T, memFs afero.Fs, destAbs string, want []byte) {
	t.Helper()

	gotDest, readErr := afero.ReadFile(memFs, destAbs)
	if readErr != nil {
		t.Fatalf("read destination after failed extract: %v", readErr)
	}

	if !bytes.Equal(gotDest, want) {
		t.Fatalf("destination content changed; want %q got %q", string(want), string(gotDest))
	}

	afterExist, existErr := afero.Exists(memFs, destAbs)
	if existErr != nil {
		t.Fatalf("Exists dest after failed extract: %v", existErr)
	}

	if !afterExist {
		t.Fatal("destination path must remain present after ErrDestinationExists")
	}
}

// discardExtractMeasurementForBDDBinding: BDD-facing ExtractMeasurement wiring (avoids ST1023 redundant locals).
func discardExtractMeasurementForBDDBinding(m *testutil.ExtractMeasurement) {
	_ = m
}

// discardHeapAllocDeltaForBDDBinding keeps HeapAllocDelta field access wired to the BDD-facing measurement type.
func discardHeapAllocDeltaForBDDBinding(heapAllocDelta int64) {
	_ = heapAllocDelta
}

// discardTotalAllocDeltaForBDDBinding keeps TotalAllocDelta field access wired to the BDD-facing measurement type.
//
// The value is not asserted non-zero here: MemStats.TotalAlloc delta can be zero under timing,
// allocation elision, or future runtime optimizations; BDD steps and benchmarks still record it.
func discardTotalAllocDeltaForBDDBinding(totalAllocDelta uint64) {
	_ = totalAllocDelta
}

func TestBuildExtractFixture_GoldenMatchesFileopsExtract(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fx := testutil.ExtractFixture{
		LineCount:  40,
		LineLength: 8,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: 5, End: 12},
	}

	src, want, err := testutil.BuildExtractFixture(ctx, fx)
	if err != nil {
		t.Fatalf("BuildExtractFixture: %v", err)
	}

	var buf bytes.Buffer

	res, exErr := fileops.Extract(ctx, fileops.ExtractOptions{
		Source: bytes.NewReader(src),
		Lines:  fx.Lines,
	}, &buf)
	if exErr != nil {
		t.Fatalf("fileops.Extract direct: %v", exErr)
	}

	requireBytesEqual(t, buf.Bytes(), want, "extract output")

	requireEQ(t, "LinesExtracted", res.LinesExtracted, 12-5+1)
}

func TestBuildExtractFixture_RejectsLineCountZero(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, _, err := testutil.BuildExtractFixture(ctx, testutil.ExtractFixture{
		LineCount:  0,
		LineLength: 1,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: 1, End: 1},
	})
	if err == nil {
		t.Fatal("want error for LineCount < 1")
	}
}

func TestBuildExtractFixture_RejectsNegativeLineLength(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, _, err := testutil.BuildExtractFixture(ctx, testutil.ExtractFixture{
		LineCount:  5,
		LineLength: -1,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: 1, End: 1},
	})
	if err == nil {
		t.Fatal("want error for negative LineLength")
	}
}

func TestBuildExtractFixture_RejectsUnknownTerminator(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, _, err := testutil.BuildExtractFixture(ctx, testutil.ExtractFixture{
		LineCount:  5,
		LineLength: 2,
		Terminator: testutil.ExtractLineTerminator(99),
		Lines:      fileops.LineRange{Start: 1, End: 1},
	})
	if err == nil {
		t.Fatal("want error for unknown terminator enum")
	}
}

func TestMeasureFileopsExtract_RecordsInstrumentation(t *testing.T) {
	t.Parallel()

	fx := testutil.ExtractFixture{
		LineCount:  20,
		LineLength: 4,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: 1, End: 3},
	}

	src, _ := mustBuildExtractFixture(t, fx)

	meas, res := mustMeasureFileopsExtract(t, src, fileops.LineRange{Start: 1, End: 3}, false)

	requirePositive(t, "SourceReadCalls", meas.SourceReadCalls)

	requirePositive(t, "SourceBytesRead", meas.SourceBytesRead)

	requireZero(t, "SourceOpens", meas.SourceOpens)

	requireZero(t, "DestinationOpens", meas.DestinationOpens)

	requireEQ(t, "LinesExtracted", meas.LinesExtracted, res.LinesExtracted)

	requireZero(t, "DestinationMkdirAllCalls", meas.DestinationMkdirAllCalls)
}

func TestMeasurePipelineExtract_NilResolverErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	src := &testutil.CountingSourceOpener{Immutable: []byte("a\n")}
	out := testutil.NewCountingOutputOpener()
	sess := testutil.NewMemFileSession()

	_, _, err := testutil.MeasurePipelineExtract(ctx, src, out, sess, nil, pipeline.ExtractParams{})
	if err == nil {
		t.Fatal("want error for nil resolver")
	}
}

func TestMeasurePipelineExtract_MatchesGolden(t *testing.T) {
	t.Parallel()

	fx := testutil.ExtractFixture{
		LineCount:  25,
		LineLength: 6,
		Terminator: testutil.ExtractLineTerminatorCRLF,
		Lines:      fileops.LineRange{Start: 3, End: 7},
	}

	srcBytes, golden := mustBuildExtractFixture(t, fx)

	root := perfTestRoot()
	srcAbs := filepath.Join(root, "in.txt")
	destAbs := filepath.Join(root, "out.txt")

	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	outOp := testutil.NewCountingOutputOpener()

	params := pipeline.ExtractParams{
		SrcPath:  srcAbs,
		DestPath: destAbs,
		Root:     root,
		Lines:    fileops.LineRange{Start: 3, End: 7},
		Force:    true,
	}

	meas, res := mustMeasurePipelineExtract(t, srcOp, outOp, params)

	requireBytesEqual(t, meas.OutputBytes, golden, "OutputBytes golden")

	requireEQ(t, "LinesExtracted", res.LinesExtracted, 5)

	requireEQ(t, "SourceOpens", int(meas.SourceOpens), 1)

	requireEQ(t, "DestinationOpens", int(meas.DestinationOpens), 1)

	closeCount := srcOp.AggregateSourceCloseCalls()
	if closeCount != 1 {
		t.Fatalf("pipeline should Close source opener reader once, closes=%d", closeCount)
	}
}

func TestMeasurePipelineExtract_PreviewSkipsPhysicalDestinationOpens(t *testing.T) {
	t.Parallel()

	fx := testutil.ExtractFixture{
		LineCount:  10,
		LineLength: 2,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: 1, End: 2},
	}

	srcBytes, _ := mustBuildExtractFixture(t, fx)

	root := perfTestRoot()
	srcAbs := filepath.Join(root, "in-prev.txt")
	destAbs := filepath.Join(root, "out-prev.txt")

	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}
	outOp := testutil.NewCountingOutputOpener()

	meas, res := mustMeasurePipelineExtract(t, srcOp, outOp, pipeline.ExtractParams{
		SrcPath:  srcAbs,
		DestPath: destAbs,
		Root:     root,
		Lines:    fileops.LineRange{Start: 1, End: 2},
		Preview:  true,
	})

	requireEQ(t, "LinesExtracted", res.LinesExtracted, 2)

	requireZero(t, "Preview DestinationOpens", meas.DestinationOpens)

	if len(meas.OutputBytes) != 0 {
		t.Fatalf("Preview OutputBytes should be empty snapshot got len %d", len(meas.OutputBytes))
	}

	requireZero(t, "Preview DestinationBytesWritten", meas.DestinationBytesWritten)
}

func TestMeasurePipelineExtractCountingSrcMem_PreviewSkipsPhysicalDestinationWrites(t *testing.T) {
	t.Parallel()

	fx := testutil.ExtractFixture{
		LineCount:  10,
		LineLength: 2,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: 1, End: 2},
	}

	srcBytes, _ := mustBuildExtractFixture(t, fx)

	root := perfTestRoot()
	srcAbs := filepath.Join(root, "in-prev-mem.txt")
	destAbs := filepath.Join(root, "out-prev-mem.txt")

	memFs := afero.NewMemMapFs()
	if mkErr := memFs.MkdirAll(filepath.Dir(srcAbs), 0o755); mkErr != nil {
		t.Fatalf("seed source parent dirs: %v", mkErr)
	}
	if wErr := afero.WriteFile(memFs, srcAbs, srcBytes, 0o644); wErr != nil {
		t.Fatalf("seed source file: %v", wErr)
	}

	resolver := testutil.NewMemPathResolverWithFS(memFs)
	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	meas, res := mustMeasurePipelineExtractCountingSrcMem(t, srcOp, memFs, resolver, pipeline.ExtractParams{
		SrcPath:  srcAbs,
		DestPath: destAbs,
		Root:     root,
		Lines:    fileops.LineRange{Start: 1, End: 2},
		Preview:  true,
	})

	requireEQ(t, "LinesExtracted", res.LinesExtracted, 2)

	requireZero(t, "Preview DestinationOpens", meas.DestinationOpens)

	if len(meas.OutputBytes) != 0 {
		t.Fatalf("Preview OutputBytes should be empty snapshot got len %d", len(meas.OutputBytes))
	}

	requireZero(t, "Preview DestinationBytesWritten", meas.DestinationBytesWritten)
}

func TestMeasurePipelineExtract_AllowedPathMismatchIsSourceNotFound(t *testing.T) {
	t.Parallel()

	fx := testutil.ExtractFixture{
		LineCount:  3,
		LineLength: 1,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: 1, End: 1},
	}

	srcBytes, _ := mustBuildExtractFixture(t, fx)

	root := perfTestRoot()
	srcWrong := filepath.Join(root, "other.txt")

	srcOp := &testutil.CountingSourceOpener{
		Immutable:   srcBytes,
		AllowedPath: srcWrong,
	}
	outOp := testutil.NewCountingOutputOpener()

	memFs := afero.NewMemMapFs()
	sess := testutil.NewMemFileSession()
	sess.SetFs(memFs)

	_, _, runErr := testutil.MeasurePipelineExtract(
		context.Background(),
		srcOp,
		outOp,
		sess,
		testutil.NewSyntheticAbsentPathResolver(),
		pipeline.ExtractParams{
			SrcPath:  filepath.Join(root, "expected.txt"),
			DestPath: filepath.Join(root, "out.txt"),
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 1},
			Force:    true,
		},
	)
	if runErr == nil {
		t.Fatal("want error")
	}

	if !errors.Is(runErr, pipeline.ErrSourceNotFound) {
		t.Fatalf("want ErrSourceNotFound wrapping, got %v", runErr)
	}
}

func TestMeasurePipelineExtractCountingSrcMem_NilResolverErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	src := &testutil.CountingSourceOpener{Immutable: []byte("a\n")}
	memFs := afero.NewMemMapFs()

	_, _, err := testutil.MeasurePipelineExtractCountingSrcMem(ctx, src, memFs, nil, pipeline.ExtractParams{})
	if err == nil {
		t.Fatal("want error for nil resolver")
	}
}

func TestMeasurePipelineExtractCountingSrcMem_NilDepsErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resolver := testutil.NewMemPathResolverWithFS(afero.NewMemMapFs())

	_, _, err := testutil.MeasurePipelineExtractCountingSrcMem(
		ctx,
		nil,
		afero.NewMemMapFs(),
		resolver,
		pipeline.ExtractParams{},
	)
	if err == nil {
		t.Fatal("want error for nil source opener")
	}

	src := &testutil.CountingSourceOpener{Immutable: []byte("a\n")}

	_, _, err = testutil.MeasurePipelineExtractCountingSrcMem(
		ctx,
		src,
		nil,
		resolver,
		pipeline.ExtractParams{},
	)
	if err == nil {
		t.Fatal("want error for nil mem FS")
	}
}

func TestMeasurePipelineExtractCountingSrcMem_RecordsContractInstrumentation(t *testing.T) {
	t.Parallel()

	fx := testutil.ExtractFixture{
		LineCount:  25,
		LineLength: 6,
		Terminator: testutil.ExtractLineTerminatorCRLF,
		Lines:      fileops.LineRange{Start: 3, End: 7},
	}

	srcBytes, golden := mustBuildExtractFixture(t, fx)

	root := perfTestRoot()
	srcAbs := filepath.Join(root, "in-mem-contract.txt")
	destAbs := filepath.Join(root, "out-mem-contract.txt")

	memFs := afero.NewMemMapFs()
	// Resolver Lstat must see the source in the mem FS so validation matches production/BDD source-exists
	// semantics; CountingSourceOpener still supplies the measured reader.
	if mkErr := memFs.MkdirAll(filepath.Dir(srcAbs), 0o755); mkErr != nil {
		t.Fatalf("seed source parent dirs: %v", mkErr)
	}
	if wErr := afero.WriteFile(memFs, srcAbs, srcBytes, 0o644); wErr != nil {
		t.Fatalf("seed source file: %v", wErr)
	}

	resolver := testutil.NewMemPathResolverWithFS(memFs)
	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	params := pipeline.ExtractParams{
		SrcPath:  srcAbs,
		DestPath: destAbs,
		Root:     root,
		Lines:    fileops.LineRange{Start: 3, End: 7},
		// Absent destination, no overwrite: aligns with performance-contract extract (Force omitted/false).
	}

	meas, res := mustMeasurePipelineExtractCountingSrcMem(t, srcOp, memFs, resolver, params)

	// ExtractMeasurement matches the struct stored on tc.LastPerfExtract / PerfExtractBySource in BDD.
	discardExtractMeasurementForBDDBinding(&meas)

	requireBytesEqual(t, meas.OutputBytes, golden, "OutputBytes golden")

	requireEQ(t, "LinesExtracted", res.LinesExtracted, 5)

	requireEQ(t, "SourceOpens", int(meas.SourceOpens), 1)

	requirePositive(t, "SourceBytesRead", meas.SourceBytesRead)

	requirePositive(t, "SourceReadCalls", meas.SourceReadCalls)

	requireEQ(t, "DestinationOpens", int(meas.DestinationOpens), 1)

	// Pipeline calls OutputOpener.MkdirAll only when params.Mkdir is true; mem commit still mkdirs via Fs.
	requireZero(t, "DestinationMkdirAllCalls", meas.DestinationMkdirAllCalls)

	if int(meas.DestinationBytesWritten) != len(golden) {
		t.Fatalf(
			"DestinationBytesWritten = %d, want len(golden)=%d",
			meas.DestinationBytesWritten,
			len(golden),
		)
	}

	closeCount := srcOp.AggregateSourceCloseCalls()
	if closeCount != 1 {
		t.Fatalf("pipeline should Close source opener reader once, closes=%d", closeCount)
	}

	discardTotalAllocDeltaForBDDBinding(meas.TotalAllocDelta)

	// HeapAllocDelta uses the same MemStats bracket as TotalAllocDelta but is intentionally not
	// asserted non-zero here: HeapAlloc between ReadMemStats calls is timing- and GC-dependent
	// (often zero or negative when the collector frees). Retained-heap delta math is covered by
	// deterministic unit tests on heapAllocDeltaSigned in package testutil.
	//
	// This test still binds meas.HeapAllocDelta to the ExtractMeasurement wiring used by BDD.
	discardHeapAllocDeltaForBDDBinding(meas.HeapAllocDelta)
}

func TestMeasurePipelineExtractCountingSrcMem_MkdirCreatesParentsRecordsMkdirAll(t *testing.T) {
	t.Parallel()

	fx := testutil.ExtractFixture{
		LineCount:  25,
		LineLength: 6,
		Terminator: testutil.ExtractLineTerminatorCRLF,
		Lines:      fileops.LineRange{Start: 3, End: 7},
	}

	srcBytes, golden := mustBuildExtractFixture(t, fx)

	root := perfTestRoot()
	srcAbs := filepath.Join(root, "in-mkdir.txt")
	destAbs := filepath.Join(root, "nested", "out-mkdir.txt")

	memFs := afero.NewMemMapFs()

	if mkErr := memFs.MkdirAll(filepath.Dir(srcAbs), 0o755); mkErr != nil {
		t.Fatalf("seed source parent dirs: %v", mkErr)
	}
	if wErr := afero.WriteFile(memFs, srcAbs, srcBytes, 0o644); wErr != nil {
		t.Fatalf("seed source file: %v", wErr)
	}

	nestedParent := filepath.Dir(destAbs)
	existsBefore, existErr := afero.Exists(memFs, nestedParent)
	if existErr != nil {
		t.Fatalf("Exists nested parent before extract: %v", existErr)
	}
	if existsBefore {
		t.Fatal("nested destination parent must be absent before extract")
	}

	resolver := testutil.NewMemPathResolverWithFS(memFs)
	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	params := pipeline.ExtractParams{
		SrcPath:  srcAbs,
		DestPath: destAbs,
		Root:     root,
		Lines:    fileops.LineRange{Start: 3, End: 7},
		Mkdir:    true,
	}

	meas, res := mustMeasurePipelineExtractCountingSrcMem(t, srcOp, memFs, resolver, params)

	discardExtractMeasurementForBDDBinding(&meas)

	requireBytesEqual(t, meas.OutputBytes, golden, "OutputBytes golden")

	requireEQ(t, "LinesExtracted", res.LinesExtracted, 5)

	requireEQ(t, "DestinationMkdirAllCalls", int(meas.DestinationMkdirAllCalls), 1)

	requireEQ(t, "DestinationOpens", int(meas.DestinationOpens), 1)

	if int(meas.DestinationBytesWritten) != len(golden) {
		t.Fatalf(
			"DestinationBytesWritten = %d, want len(golden)=%d",
			meas.DestinationBytesWritten,
			len(golden),
		)
	}

	existsAfter, existAfterErr := afero.Exists(memFs, destAbs)
	if existAfterErr != nil {
		t.Fatalf("Exists dest after extract: %v", existAfterErr)
	}
	if !existsAfter {
		t.Fatal("destination file must exist after successful extract with Mkdir")
	}

	discardTotalAllocDeltaForBDDBinding(meas.TotalAllocDelta)
	discardHeapAllocDeltaForBDDBinding(meas.HeapAllocDelta)
}

func TestMeasurePipelineExtractCountingSrcMem_DestinationExistsNoForceLeavesFile(t *testing.T) {
	t.Parallel()

	fx := testutil.ExtractFixture{
		LineCount:  3,
		LineLength: 1,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: 1, End: 1},
	}

	srcBytes, _ := mustBuildExtractFixture(t, fx)

	root := perfTestRoot()
	srcAbs := filepath.Join(root, "in-exists.txt")
	destAbs := filepath.Join(root, "out-exists.txt")

	memFs := afero.NewMemMapFs()
	existing := []byte("pre-existing-bytes\n")
	mustPrepareMemExtractWithPreexistingDestination(t, memFs, srcAbs, destAbs, srcBytes, existing)

	resolver := testutil.NewMemPathResolverWithFS(memFs)
	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	meas, _, runErr := testutil.MeasurePipelineExtractCountingSrcMem(
		context.Background(),
		srcOp,
		memFs,
		resolver,
		pipeline.ExtractParams{
			SrcPath:  srcAbs,
			DestPath: destAbs,
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 1},
			Force:    false,
			Append:   false,
		},
	)
	if runErr == nil {
		t.Fatal("want error when destination exists and Force is false")
	}

	if !errors.Is(runErr, pipeline.ErrDestinationExists) {
		t.Fatalf("want ErrDestinationExists, got %v", runErr)
	}

	assertDestinationBytesUnchangedAfterFailedExtract(t, memFs, destAbs, existing)

	if meas.DestinationBytesWritten != 0 {
		t.Fatalf(
			"want no payload bytes through Write on ErrDestinationExists, got %d",
			meas.DestinationBytesWritten,
		)
	}

	requireBytesEqual(t, meas.OutputBytes, existing, "OutputBytes snapshot matches pre-existing dest")

	requireEQ(t, "DestinationOpens", int(meas.DestinationOpens), 1)
}
