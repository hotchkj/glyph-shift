package pipeline_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

const (
	benchRunExtractLineCountLarge = 100_000
	benchRunExtractLineLength     = 32
)

func perfPipelineBenchRoot() string {
	return filepath.Join(string([]rune{filepath.Separator}), "glyph-shift-perf-test-root")
}

// benchPreparePipelineExtractMemWorkspace returns a fresh mem FS with source bytes materialized
// at srcAbs (for MemPathResolver Lstat semantics mirroring BDD extract performance scenarios),
// a resolver over that FS, and clears srcOp counters. Destination paths are absent until the
// pipeline writes them; setup is intended for b.StopTimer regions.
func benchPreparePipelineExtractMemWorkspace(
	b *testing.B,
	srcAbs string,
	srcBytes []byte,
	srcOp *testutil.CountingSourceOpener,
) (afero.Fs, validate.PathResolver, error) {
	b.Helper()

	memFs := afero.NewMemMapFs()

	srcDir := filepath.Dir(srcAbs)
	if srcDir != "." && srcDir != "" {
		if mkErr := memFs.MkdirAll(srcDir, 0o755); mkErr != nil {
			return nil, nil, fmt.Errorf("mkdir source parent: %w", mkErr)
		}
	}

	if wErr := afero.WriteFile(memFs, srcAbs, srcBytes, 0o644); wErr != nil {
		return nil, nil, fmt.Errorf("write bench source: %w", wErr)
	}

	resolver := testutil.NewMemPathResolverWithFS(memFs)
	srcOp.ResetCounters()

	return memFs, resolver, nil
}

func reportPipelineExtractMetrics(b *testing.B, measurement *testutil.ExtractMeasurement) {
	b.Helper()

	if measurement == nil {
		return
	}

	b.ReportMetric(float64(measurement.SourceBytesRead), "source_bytes_read/op")
	b.ReportMetric(float64(measurement.SourceReadCalls), "source_reads/op")
	b.ReportMetric(float64(measurement.SourceSeekCalls), "source_seeks/op")
	b.ReportMetric(float64(measurement.SourceOpens), "source_opens/op")
	b.ReportMetric(float64(measurement.DestinationOpens), "dest_opens/op")
	b.ReportMetric(float64(measurement.DestinationBytesWritten), "dest_bytes_written/op")
	b.ReportMetric(float64(measurement.DestinationMkdirAllCalls), "dest_mkdir_all_calls/op")
	b.ReportMetric(float64(measurement.TotalAllocDelta), "total_alloc_delta/op")
	b.ReportMetric(float64(measurement.HeapAllocDelta), "heap_alloc_delta/op")
}

func BenchmarkRunExtractClosedEarlyRange(b *testing.B) {
	ctx := context.Background()

	fx := testutil.ExtractFixture{
		LineCount:  benchRunExtractLineCountLarge,
		LineLength: benchRunExtractLineLength,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: 1, End: 1},
	}

	srcBytes, golden, err := testutil.BuildExtractFixture(ctx, fx)
	if err != nil {
		b.Fatalf("BuildExtractFixture: %v", err)
	}

	root := perfPipelineBenchRoot()
	srcAbs := filepath.Join(root, "bench-runextract-early-src.txt")
	destAbs := filepath.Join(root, "bench-runextract-early-dest.txt")

	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	params := pipeline.ExtractParams{
		SrcPath:  srcAbs,
		DestPath: destAbs,
		Root:     root,
		Lines:    fileops.LineRange{Start: 1, End: 1},
	}

	b.ReportAllocs()

	b.SetBytes(int64(len(golden)))

	b.ResetTimer()

	var last testutil.ExtractMeasurement

	for b.Loop() {
		b.StopTimer()
		memFs, resolver, prepErr := benchPreparePipelineExtractMemWorkspace(b, srcAbs, srcBytes, srcOp)
		if prepErr != nil {
			b.Fatalf("bench workspace: %v", prepErr)
		}
		b.StartTimer()

		var runErr error

		last, _, runErr = testutil.MeasurePipelineExtractCountingSrcMem(ctx, srcOp, memFs, resolver, params)
		if runErr != nil {
			b.Fatalf("MeasurePipelineExtractCountingSrcMem: %v", runErr)
		}
	}

	reportPipelineExtractMetrics(b, &last)
}

func BenchmarkRunExtractMidFileClosedRange(b *testing.B) {
	ctx := context.Background()

	const midStart, midEnd = 500, 510

	fx := testutil.ExtractFixture{
		LineCount:  benchRunExtractLineCountLarge,
		LineLength: benchRunExtractLineLength,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: midStart, End: midEnd},
	}

	srcBytes, golden, err := testutil.BuildExtractFixture(ctx, fx)
	if err != nil {
		b.Fatalf("BuildExtractFixture: %v", err)
	}

	root := perfPipelineBenchRoot()
	srcAbs := filepath.Join(root, "bench-runextract-mid-src.txt")
	destAbs := filepath.Join(root, "bench-runextract-mid-dest.txt")

	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	params := pipeline.ExtractParams{
		SrcPath:  srcAbs,
		DestPath: destAbs,
		Root:     root,
		Lines:    fileops.LineRange{Start: midStart, End: midEnd},
	}

	b.ReportAllocs()

	b.SetBytes(int64(len(golden)))

	b.ResetTimer()

	var last testutil.ExtractMeasurement

	for b.Loop() {
		b.StopTimer()
		memFs, resolver, prepErr := benchPreparePipelineExtractMemWorkspace(b, srcAbs, srcBytes, srcOp)
		if prepErr != nil {
			b.Fatalf("bench workspace: %v", prepErr)
		}
		b.StartTimer()

		var runErr error

		last, _, runErr = testutil.MeasurePipelineExtractCountingSrcMem(ctx, srcOp, memFs, resolver, params)
		if runErr != nil {
			b.Fatalf("MeasurePipelineExtractCountingSrcMem: %v", runErr)
		}
	}

	reportPipelineExtractMetrics(b, &last)
}

func BenchmarkRunExtractOpenEndedRange(b *testing.B) {
	ctx := context.Background()

	fx := testutil.ExtractFixture{
		LineCount:  benchRunExtractLineCountLarge,
		LineLength: benchRunExtractLineLength,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: 2, End: 0},
	}

	srcBytes, golden, err := testutil.BuildExtractFixture(ctx, fx)
	if err != nil {
		b.Fatalf("BuildExtractFixture: %v", err)
	}

	root := perfPipelineBenchRoot()
	srcAbs := filepath.Join(root, "bench-runextract-open-src.txt")
	destAbs := filepath.Join(root, "bench-runextract-open-dest.txt")

	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	params := pipeline.ExtractParams{
		SrcPath:  srcAbs,
		DestPath: destAbs,
		Root:     root,
		Lines:    fileops.LineRange{Start: 2, End: 0},
	}

	b.ReportAllocs()

	b.SetBytes(int64(len(golden)))

	b.ResetTimer()

	var last testutil.ExtractMeasurement

	for b.Loop() {
		b.StopTimer()
		memFs, resolver, prepErr := benchPreparePipelineExtractMemWorkspace(b, srcAbs, srcBytes, srcOp)
		if prepErr != nil {
			b.Fatalf("bench workspace: %v", prepErr)
		}
		b.StartTimer()

		var runErr error

		last, _, runErr = testutil.MeasurePipelineExtractCountingSrcMem(ctx, srcOp, memFs, resolver, params)
		if runErr != nil {
			b.Fatalf("MeasurePipelineExtractCountingSrcMem: %v", runErr)
		}
	}

	reportPipelineExtractMetrics(b, &last)
}

func BenchmarkRunExtractClosedEarlyRangePreview(b *testing.B) {
	ctx := context.Background()

	fx := testutil.ExtractFixture{
		LineCount:  benchRunExtractLineCountLarge,
		LineLength: benchRunExtractLineLength,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: 1, End: 1},
	}

	srcBytes, golden, err := testutil.BuildExtractFixture(ctx, fx)
	if err != nil {
		b.Fatalf("BuildExtractFixture: %v", err)
	}

	root := perfPipelineBenchRoot()
	srcAbs := filepath.Join(root, "bench-runextract-prev-src.txt")
	destAbs := filepath.Join(root, "bench-runextract-prev-dest.txt")

	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	params := pipeline.ExtractParams{
		SrcPath:  srcAbs,
		DestPath: destAbs,
		Root:     root,
		Lines:    fileops.LineRange{Start: 1, End: 1},
		Preview:  true,
	}

	b.ReportAllocs()

	// Preview does not write destination bytes; SetBytes uses golden (non-preview) output size as a
	// comparable "would have written" throughput scale for the same line selection.
	b.SetBytes(int64(len(golden)))

	b.ResetTimer()

	var last testutil.ExtractMeasurement

	for b.Loop() {
		b.StopTimer()
		memFs, resolver, prepErr := benchPreparePipelineExtractMemWorkspace(b, srcAbs, srcBytes, srcOp)
		if prepErr != nil {
			b.Fatalf("bench workspace: %v", prepErr)
		}
		b.StartTimer()

		var runErr error

		last, _, runErr = testutil.MeasurePipelineExtractCountingSrcMem(ctx, srcOp, memFs, resolver, params)
		if runErr != nil {
			b.Fatalf("MeasurePipelineExtractCountingSrcMem: %v", runErr)
		}
	}

	reportPipelineExtractMetrics(b, &last)
}
