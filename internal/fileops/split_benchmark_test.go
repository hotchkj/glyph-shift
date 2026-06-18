package fileops_test

import (
	"bytes"
	"context"
	"math"
	"regexp"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/testutil"
)

const (
	benchSplitLineCountLarge = 55_000
	benchSplitLineLength     = 32
	benchSplitManyDelimiters = 900
	benchSplitBodyLineLen    = 72
)

// benchHeapAllocDeltaSigned matches testutil.heapAllocDeltaSigned semantics for bracketed MemStats deltas.
func benchHeapAllocDeltaSigned(before, after uint64) int64 {
	const maxInt64 = uint64(math.MaxInt64)

	if after >= before {
		diff := after - before
		if diff > maxInt64 {
			return math.MaxInt64
		}

		return int64(diff)
	}

	diff := before - after
	if diff > maxInt64 {
		return math.MinInt64
	}

	return -int64(diff)
}

// benchCountReadSeek wraps bytes.Reader to approximate extract-benchmark instrumentation for streaming paths.
type benchCountReadSeek struct {
	inner *bytes.Reader

	readCalls atomic.Int64
	seekCalls atomic.Int64
	bytesRead atomic.Int64
}

func newBenchCountReadSeek(src []byte) *benchCountReadSeek {
	return &benchCountReadSeek{inner: bytes.NewReader(src)}
}

//nolint:varnamelen // io.Reader convention uses `p` for the scratch slice.
func (c *benchCountReadSeek) Read(p []byte) (int, error) {
	c.readCalls.Add(1)

	n, err := c.inner.Read(p)
	c.bytesRead.Add(int64(n))

	return n, err
}

func (c *benchCountReadSeek) Seek(offset int64, whence int) (int64, error) {
	c.seekCalls.Add(1)

	return c.inner.Seek(offset, whence)
}

func sectionMaterializedPayloadBytes(res fileops.SplitResult) int64 {
	var sum int64

	for _, sec := range res.Sections {
		for _, ln := range sec.Lines {
			sum += int64(len(ln.Content) + len(ln.Terminator))
		}
	}

	return sum
}

func reportFileopsSplitBenchMetrics(
	b *testing.B,
	seekReader *benchCountReadSeek,
	res *fileops.SplitResult,
	totalAllocDelta uint64,
	heapAllocDelta int64,
) {
	b.Helper()

	if seekReader != nil {
		b.ReportMetric(float64(seekReader.bytesRead.Load()), "source_bytes_read/op")
		b.ReportMetric(float64(seekReader.readCalls.Load()), "source_reads/op")
		b.ReportMetric(float64(seekReader.seekCalls.Load()), "source_seeks/op")
	}

	if res != nil {
		b.ReportMetric(float64(len(res.Sections)), "sections/op")
		b.ReportMetric(float64(sectionMaterializedPayloadBytes(*res)), "materialized_payload_bytes/op")
	}

	b.ReportMetric(float64(totalAllocDelta), "total_alloc_delta/op")
	b.ReportMetric(float64(heapAllocDelta), "heap_alloc_delta/op")
}

func buildBenchSplitAlternatingSource(delimiterLines, bodyLineLen int) []byte {
	payload := bytes.Repeat([]byte{'z'}, bodyLineLen)

	var bb bytes.Buffer
	bb.WriteString("preamble\n")

	for range delimiterLines {
		bb.WriteString("---\n")
		bb.Write(payload)
		bb.WriteByte('\n')
	}

	return bb.Bytes()
}

func BenchmarkSplitLargeSingleTrailingSection(b *testing.B) {
	ctx := context.Background()

	delimPrefix := []byte("---\n")
	src := testutil.BuildLargeSplitSingleSectionSource(
		benchSplitLineCountLarge,
		benchSplitLineLength,
		delimPrefix,
	)
	re := regexp.MustCompile(`^---$`)

	b.ReportAllocs()

	b.SetBytes(int64(len(src)))

	var (
		lastRes             fileops.SplitResult
		lastTotalAllocDelta uint64
		lastHeapDelta       int64
		last                *benchCountReadSeek
	)

	b.ResetTimer()

	for b.Loop() {
		seekReader := newBenchCountReadSeek(src)

		runtime.GC()

		var ms0, ms1 runtime.MemStats
		runtime.ReadMemStats(&ms0)

		res, runErr := fileops.Split(ctx, fileops.SplitOptions{
			Source:    seekReader,
			Delimiter: re,
			Naming:    fileops.Sequential,
			Extension: ".md",
		})
		if runErr != nil {
			b.Fatalf("Split: %v", runErr)
		}

		runtime.ReadMemStats(&ms1)

		last = seekReader
		lastRes = res
		lastTotalAllocDelta = ms1.TotalAlloc - ms0.TotalAlloc
		lastHeapDelta = benchHeapAllocDeltaSigned(ms0.HeapAlloc, ms1.HeapAlloc)
	}

	reportFileopsSplitBenchMetrics(b, last, &lastRes, lastTotalAllocDelta, lastHeapDelta)
}

func BenchmarkSplitManyDelimiterSections(b *testing.B) {
	ctx := context.Background()

	src := buildBenchSplitAlternatingSource(benchSplitManyDelimiters, benchSplitBodyLineLen)
	re := regexp.MustCompile(`^---$`)

	b.ReportAllocs()

	b.SetBytes(int64(len(src)))

	var (
		lastRes             fileops.SplitResult
		lastTotalAllocDelta uint64
		lastHeapDelta       int64
		last                *benchCountReadSeek
	)

	b.ResetTimer()

	for b.Loop() {
		seekReader := newBenchCountReadSeek(src)

		runtime.GC()

		var ms0, ms1 runtime.MemStats
		runtime.ReadMemStats(&ms0)

		res, runErr := fileops.Split(ctx, fileops.SplitOptions{
			Source:    seekReader,
			Delimiter: re,
			Naming:    fileops.Sequential,
			Extension: ".md",
		})
		if runErr != nil {
			b.Fatalf("Split: %v", runErr)
		}

		runtime.ReadMemStats(&ms1)

		last = seekReader
		lastRes = res
		lastTotalAllocDelta = ms1.TotalAlloc - ms0.TotalAlloc
		lastHeapDelta = benchHeapAllocDeltaSigned(ms0.HeapAlloc, ms1.HeapAlloc)
	}

	reportFileopsSplitBenchMetrics(b, last, &lastRes, lastTotalAllocDelta, lastHeapDelta)
}
