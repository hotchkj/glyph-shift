package fileops_test

import (
	"context"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/testutil"
)

const (
	benchExtractLineCountLarge = 100_000
	benchExtractLineLength     = 32
)

// reportFileopsExtractMetrics reports counters for BenchmarkExtract* in this package only.
// Those benchmarks diagnose fileops.Extract in isolation and are not BDD scenario contract evidence;
// unified BDD-shaped pipeline measurement lives in internal/pipeline/run_extract_benchmark_test.go.
//
// ReportMetric deliberately omits TotalAllocDelta, HeapAllocDelta, and DestinationMkdirAllCalls:
// fileops benches stay lightweight diagnostics; MeasureFileopsExtract still records those fields for
// consistency, while pipeline benchmarks in internal/pipeline publish alloc/mkdir contract metrics.
func reportFileopsExtractMetrics(b *testing.B, measurement *testutil.ExtractMeasurement) {
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
}

func BenchmarkExtractClosedEarlyRange(b *testing.B) {
	ctx := context.Background()

	fx := testutil.ExtractFixture{
		LineCount:  benchExtractLineCountLarge,
		LineLength: benchExtractLineLength,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: 1, End: 1},
	}

	src, golden, err := testutil.BuildExtractFixture(ctx, fx)
	if err != nil {
		b.Fatalf("BuildExtractFixture: %v", err)
	}

	lineRange := fileops.LineRange{Start: 1, End: 1}

	b.ReportAllocs()

	// Track throughput of emitted extract output; source is much larger than one line.
	b.SetBytes(int64(len(golden)))

	b.ResetTimer()

	var last testutil.ExtractMeasurement

	for b.Loop() {
		var runErr error

		last, _, runErr = testutil.MeasureFileopsExtract(ctx, src, lineRange, false)
		if runErr != nil {
			b.Fatalf("MeasureFileopsExtract: %v", runErr)
		}
	}

	reportFileopsExtractMetrics(b, &last)
}

func BenchmarkExtractMidFileClosedRange(b *testing.B) {
	ctx := context.Background()

	const midStart, midEnd = 500, 510

	fx := testutil.ExtractFixture{
		LineCount:  benchExtractLineCountLarge,
		LineLength: benchExtractLineLength,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: midStart, End: midEnd},
	}

	src, golden, err := testutil.BuildExtractFixture(ctx, fx)
	if err != nil {
		b.Fatalf("BuildExtractFixture: %v", err)
	}

	lineRange := fileops.LineRange{Start: midStart, End: midEnd}

	b.ReportAllocs()

	b.SetBytes(int64(len(golden)))

	b.ResetTimer()

	var last testutil.ExtractMeasurement

	for b.Loop() {
		var runErr error

		last, _, runErr = testutil.MeasureFileopsExtract(ctx, src, lineRange, false)
		if runErr != nil {
			b.Fatalf("MeasureFileopsExtract: %v", runErr)
		}
	}

	reportFileopsExtractMetrics(b, &last)
}

func BenchmarkExtractOpenEndedRange(b *testing.B) {
	ctx := context.Background()

	fx := testutil.ExtractFixture{
		LineCount:  benchExtractLineCountLarge,
		LineLength: benchExtractLineLength,
		Terminator: testutil.ExtractLineTerminatorLF,
		Lines:      fileops.LineRange{Start: 2, End: 0},
	}

	src, golden, err := testutil.BuildExtractFixture(ctx, fx)
	if err != nil {
		b.Fatalf("BuildExtractFixture: %v", err)
	}

	lineRange := fileops.LineRange{Start: 2, End: 0}

	b.ReportAllocs()

	// Open-ended range emits almost the full file; output bytes match that workload.
	b.SetBytes(int64(len(golden)))

	b.ResetTimer()

	var last testutil.ExtractMeasurement

	for b.Loop() {
		var runErr error

		last, _, runErr = testutil.MeasureFileopsExtract(ctx, src, lineRange, false)
		if runErr != nil {
			b.Fatalf("MeasureFileopsExtract: %v", runErr)
		}
	}

	reportFileopsExtractMetrics(b, &last)
}
