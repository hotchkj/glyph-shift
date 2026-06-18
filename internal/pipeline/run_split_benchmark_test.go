package pipeline_test

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

const (
	benchPipelineSplitLineCountLarge = 55_000
	benchPipelineSplitLineLen        = 32
	benchPipelineSplitDelimLines     = 900
	benchPipelineSplitBodyLineLen    = 72
)

func perfSplitBenchRoot() string {
	return filepath.Join(string([]rune{filepath.Separator}), "glyph-shift-perf-test-root-split")
}

// benchPreparePipelineSplitMemWorkspace materializes immutable source bytes for split pipeline benches.
func benchPreparePipelineSplitMemWorkspace(
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

func buildBenchPipelineSplitAlternating(delimiterLines, bodyLen int) []byte {
	payload := make([]byte, bodyLen)
	for i := range payload {
		payload[i] = 'y'
	}

	out := append([]byte(nil), []byte("preamble\n")...)

	for range delimiterLines {
		out = append(out, []byte("---\n")...)
		out = append(out, payload...)
		out = append(out, '\n')
	}

	return out
}

func reportPipelineSplitBenchMetrics(b *testing.B, measurement *testutil.SplitPipelinePerfMeasurement) {
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

func BenchmarkRunSplitLargeTrailingSection(b *testing.B) {
	ctx := context.Background()

	delimPrefix := []byte("---\n")
	srcBytes := testutil.BuildLargeSplitSingleSectionSource(
		benchPipelineSplitLineCountLarge,
		benchPipelineSplitLineLen,
		delimPrefix,
	)

	root := perfSplitBenchRoot()
	srcAbs := filepath.Join(root, "bench-runsplit-large-src.txt")
	outAbs := filepath.Join(root, "bench-runsplit-large-out-dir")

	delimiter := regexp.MustCompile(`^---$`)

	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	params := pipeline.SplitParams{
		SrcPath:   srcAbs,
		OutDir:    outAbs,
		Root:      root,
		Delimiter: delimiter,
		Naming:    fileops.Sequential,
		Extension: ".txt",
		Mkdir:     true,
		MaxFiles:  2000,
	}

	b.ReportAllocs()

	b.SetBytes(int64(len(srcBytes)))

	b.ResetTimer()

	var last testutil.SplitPipelinePerfMeasurement

	for b.Loop() {
		b.StopTimer()

		memFs, resolver, prepErr := benchPreparePipelineSplitMemWorkspace(b, srcAbs, srcBytes, srcOp)
		if prepErr != nil {
			b.Fatalf("bench workspace: %v", prepErr)
		}

		b.StartTimer()

		meas, _, runErr := testutil.MeasurePipelineSplitCountingSrcMem(ctx, srcOp, memFs, resolver, params)
		if runErr != nil {
			b.Fatalf("MeasurePipelineSplitCountingSrcMem: %v", runErr)
		}

		last = meas
	}

	reportPipelineSplitBenchMetrics(b, &last)
}

func BenchmarkRunSplitManyDelimiterSections(b *testing.B) {
	ctx := context.Background()

	srcBytes := buildBenchPipelineSplitAlternating(benchPipelineSplitDelimLines, benchPipelineSplitBodyLineLen)

	root := perfSplitBenchRoot()
	srcAbs := filepath.Join(root, "bench-runsplit-multi-src.txt")
	outAbs := filepath.Join(root, "bench-runsplit-multi-out-dir")

	delimiter := regexp.MustCompile(`^---$`)

	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	params := pipeline.SplitParams{
		SrcPath:   srcAbs,
		OutDir:    outAbs,
		Root:      root,
		Delimiter: delimiter,
		Naming:    fileops.Sequential,
		Extension: ".txt",
		Mkdir:     true,
		MaxFiles:  2000,
	}

	b.ReportAllocs()

	b.SetBytes(int64(len(srcBytes)))

	b.ResetTimer()

	var last testutil.SplitPipelinePerfMeasurement

	for b.Loop() {
		b.StopTimer()

		memFs, resolver, prepErr := benchPreparePipelineSplitMemWorkspace(b, srcAbs, srcBytes, srcOp)
		if prepErr != nil {
			b.Fatalf("bench workspace: %v", prepErr)
		}

		b.StartTimer()

		meas, _, runErr := testutil.MeasurePipelineSplitCountingSrcMem(ctx, srcOp, memFs, resolver, params)
		if runErr != nil {
			b.Fatalf("MeasurePipelineSplitCountingSrcMem: %v", runErr)
		}

		last = meas
	}

	reportPipelineSplitBenchMetrics(b, &last)
}

func BenchmarkRunSplitPreviewBaseline(b *testing.B) {
	ctx := context.Background()

	root := perfSplitBenchRoot()
	srcAbs := filepath.Join(root, "bench-runsplit-preview-src.txt")
	outAbs := filepath.Join(root, "bench-runsplit-preview-out-dir")

	srcBytes := []byte("lead\n---\nbody\n")

	srcOp := &testutil.CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	params := pipeline.SplitParams{
		SrcPath:   srcAbs,
		OutDir:    outAbs,
		Root:      root,
		Delimiter: regexp.MustCompile(`^---$`),
		Naming:    fileops.Sequential,
		Extension: ".txt",
		Preview:   true,
	}

	b.ReportAllocs()

	b.SetBytes(int64(len(srcBytes)))

	b.ResetTimer()

	var last testutil.SplitPipelinePerfMeasurement

	for b.Loop() {
		b.StopTimer()

		memFs, resolver, prepErr := benchPreparePipelineSplitMemWorkspace(b, srcAbs, srcBytes, srcOp)
		if prepErr != nil {
			b.Fatalf("bench workspace: %v", prepErr)
		}

		b.StartTimer()

		meas, _, runErr := testutil.MeasurePipelineSplitCountingSrcMem(ctx, srcOp, memFs, resolver, params)
		if runErr != nil {
			b.Fatalf("MeasurePipelineSplitCountingSrcMem: %v", runErr)
		}

		last = meas
	}

	reportPipelineSplitBenchMetrics(b, &last)
}
