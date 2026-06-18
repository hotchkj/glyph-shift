package pipeline_test

import (
	"context"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
)

const (
	benchPipelineBlocksLineCountLarge = 55_000
	benchPipelineBlocksLineLen        = 32
	benchPipelineBlocksSmallIters     = 800
)

func perfBlocksBenchRoot() string {
	return filepath.Join(string([]rune{filepath.Separator}), "glyph-shift-perf-test-root-blocks")
}

func buildBenchPipelineBlocksManySmall(iterations int) []byte {
	line := []byte("```go\nx\n```\n")
	out := make([]byte, 0, len(line)*iterations)

	for range iterations {
		out = append(out, line...)
	}

	return out
}

func reportPipelineBlocksBenchMetrics(b *testing.B, measurement *testutil.BlocksPipelinePerfMeasurement) {
	b.Helper()

	if measurement == nil {
		return
	}

	b.ReportMetric(float64(measurement.SourceBytesRead), "source_bytes_read/op")
	b.ReportMetric(float64(measurement.SourceReadCalls), "source_reads/op")
	b.ReportMetric(float64(measurement.SourceSeekCalls), "source_seeks/op")
	b.ReportMetric(float64(measurement.SourceOpens), "source_opens/op")
	b.ReportMetric(float64(measurement.DestinationBytesWritten), "dest_bytes_written/op")
	b.ReportMetric(float64(measurement.DestinationOpens), "dest_opens/op")
	b.ReportMetric(float64(measurement.DestinationMkdirAllCalls), "dest_mkdir_all_calls/op")
	b.ReportMetric(float64(measurement.PlannedOutputCount), "planned_outputs/op")
	b.ReportMetric(float64(measurement.TotalAllocDelta), "total_alloc_delta/op")
	b.ReportMetric(float64(measurement.HeapAllocDelta), "heap_alloc_delta/op")
}

func BenchmarkRunBlocksLargeSingleContentBody(b *testing.B) {
	ctx := context.Background()

	srcBytes := testutil.BuildLargeBlocksSingleBodySource(
		nil,
		[]byte("```go\n"),
		[]byte("```\n"),
		benchPipelineBlocksLineCountLarge,
		benchPipelineBlocksLineLen,
	)

	root := perfBlocksBenchRoot()
	srcAbs := filepath.Join(root, "bench-runblocks-large-src.txt")
	outAbs := filepath.Join(root, "bench-runblocks-large-out-dir")

	start := regexp.MustCompile("^```go$")
	finish := regexp.MustCompile("^```$")

	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	params := pipeline.BlocksParams{
		SrcPath:        srcAbs,
		OutDir:         outAbs,
		Root:           root,
		StartDelimiter: start,
		EndDelimiter:   finish,
		Naming:         fileops.Sequential,
		Extension:      ".txt",
		Mkdir:          true,
		MaxFiles:       2000,
	}

	b.ReportAllocs()

	b.SetBytes(int64(len(srcBytes)))

	b.ResetTimer()

	var last testutil.BlocksPipelinePerfMeasurement

	for b.Loop() {
		b.StopTimer()

		memFs, resolver, prepErr := benchPreparePipelineSplitMemWorkspace(b, srcAbs, srcBytes, srcOp)
		if prepErr != nil {
			b.Fatalf("bench workspace: %v", prepErr)
		}

		b.StartTimer()

		meas, _, runErr := testutil.MeasurePipelineBlocksCountingSrcMem(ctx, srcOp, memFs, resolver, params)
		if runErr != nil {
			b.Fatalf("MeasurePipelineBlocksCountingSrcMem: %v", runErr)
		}

		last = meas
	}

	reportPipelineBlocksBenchMetrics(b, &last)
}

func BenchmarkRunBlocksManySmallBlocks(b *testing.B) {
	ctx := context.Background()

	srcBytes := buildBenchPipelineBlocksManySmall(benchPipelineBlocksSmallIters)

	root := perfBlocksBenchRoot()
	srcAbs := filepath.Join(root, "bench-runblocks-multi-src.txt")
	outAbs := filepath.Join(root, "bench-runblocks-multi-out-dir")

	start := regexp.MustCompile("^```go$")
	finish := regexp.MustCompile("^```$")

	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	params := pipeline.BlocksParams{
		SrcPath:        srcAbs,
		OutDir:         outAbs,
		Root:           root,
		StartDelimiter: start,
		EndDelimiter:   finish,
		Naming:         fileops.Sequential,
		Extension:      ".txt",
		Mkdir:          true,
		MaxFiles:       2000,
	}

	b.ReportAllocs()

	b.SetBytes(int64(len(srcBytes)))

	b.ResetTimer()

	var last testutil.BlocksPipelinePerfMeasurement

	for b.Loop() {
		b.StopTimer()

		memFs, resolver, prepErr := benchPreparePipelineSplitMemWorkspace(b, srcAbs, srcBytes, srcOp)
		if prepErr != nil {
			b.Fatalf("bench workspace: %v", prepErr)
		}

		b.StartTimer()

		meas, _, runErr := testutil.MeasurePipelineBlocksCountingSrcMem(ctx, srcOp, memFs, resolver, params)
		if runErr != nil {
			b.Fatalf("MeasurePipelineBlocksCountingSrcMem: %v", runErr)
		}

		last = meas
	}

	reportPipelineBlocksBenchMetrics(b, &last)
}

func BenchmarkRunBlocksPreviewBaseline(b *testing.B) {
	ctx := context.Background()

	root := perfBlocksBenchRoot()
	srcAbs := filepath.Join(root, "bench-runblocks-preview-src.txt")
	outAbs := filepath.Join(root, "bench-runblocks-preview-out-dir")

	srcBytes := []byte("```go\nx\n```\n")

	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	params := pipeline.BlocksParams{
		SrcPath:        srcAbs,
		OutDir:         outAbs,
		Root:           root,
		StartDelimiter: regexp.MustCompile("^```go$"),
		EndDelimiter:   regexp.MustCompile("^```$"),
		Naming:         fileops.Sequential,
		Extension:      ".txt",
		Preview:        true,
	}

	b.ReportAllocs()

	b.SetBytes(int64(len(srcBytes)))

	b.ResetTimer()

	var last testutil.BlocksPipelinePerfMeasurement

	for b.Loop() {
		b.StopTimer()

		memFs, resolver, prepErr := benchPreparePipelineSplitMemWorkspace(b, srcAbs, srcBytes, srcOp)
		if prepErr != nil {
			b.Fatalf("bench workspace: %v", prepErr)
		}

		b.StartTimer()

		meas, _, runErr := testutil.MeasurePipelineBlocksCountingSrcMem(ctx, srcOp, memFs, resolver, params)
		if runErr != nil {
			b.Fatalf("MeasurePipelineBlocksCountingSrcMem: %v", runErr)
		}

		last = meas
	}

	reportPipelineBlocksBenchMetrics(b, &last)
}
