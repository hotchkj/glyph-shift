package fileops_test

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/testutil"
)

const (
	benchBlocksLineCountLarge  = 55_000
	benchBlocksLineLength      = 32
	benchBlocksSmallIterations = 800
)

// benchCountBlocksReader wraps bytes.Reader to record streamed source reads during ExtractBlocks.
type benchCountBlocksReader struct {
	r         *bytes.Reader
	readCalls atomic.Int64
	bytesRead atomic.Int64
}

func newBenchCountBlocksReader(src []byte) *benchCountBlocksReader {
	return &benchCountBlocksReader{r: bytes.NewReader(src)}
}

//nolint:varnamelen // io.Reader convention uses `p` for the scratch slice.
func (c *benchCountBlocksReader) Read(p []byte) (int, error) {
	if c == nil || c.r == nil {
		return 0, io.EOF
	}

	c.readCalls.Add(1)

	n, err := c.r.Read(p)
	c.bytesRead.Add(int64(n))

	return n, err
}

func blocksMaterializedPayloadBytes(res fileops.BlocksResult) int64 {
	var sum int64

	for _, bl := range res.Blocks {
		for _, ln := range bl.Lines {
			sum += int64(len(ln.Content) + len(ln.Terminator))
		}
	}

	return sum
}

func reportFileopsBlocksBenchMetrics(
	b *testing.B,
	blocksReader *benchCountBlocksReader,
	res *fileops.BlocksResult,
	totalAllocDelta uint64,
	heapAllocDelta int64,
) {
	b.Helper()

	if blocksReader != nil {
		b.ReportMetric(float64(blocksReader.bytesRead.Load()), "source_bytes_read/op")
		b.ReportMetric(float64(blocksReader.readCalls.Load()), "source_reads/op")
	}

	if res != nil {
		b.ReportMetric(float64(res.BlocksFound), "blocks_found/op")
		b.ReportMetric(float64(len(res.Blocks)), "content_blocks/op")
		b.ReportMetric(float64(blocksMaterializedPayloadBytes(*res)), "materialized_payload_bytes/op")
	}

	b.ReportMetric(float64(totalAllocDelta), "total_alloc_delta/op")
	b.ReportMetric(float64(heapAllocDelta), "heap_alloc_delta/op")
}

func buildBenchBlocksManySmallSource(iterations int) []byte {
	var bb bytes.Buffer
	for range iterations {
		bb.WriteString("```go\nx\n```\n")
	}

	return bb.Bytes()
}

func BenchmarkExtractBlocksLargeSingleContentBody(b *testing.B) {
	ctx := context.Background()

	src := testutil.BuildLargeBlocksSingleBodySource(
		nil,
		[]byte("```go\n"),
		[]byte("```\n"),
		benchBlocksLineCountLarge,
		benchBlocksLineLength,
	)

	start := regexp.MustCompile("^```go$")
	finish := regexp.MustCompile("^```$")

	b.ReportAllocs()

	b.SetBytes(int64(len(src)))

	var (
		lastRes             fileops.BlocksResult
		lastTotalAllocDelta uint64
		lastHeapDelta       int64
		last                *benchCountBlocksReader
	)

	b.ResetTimer()

	for b.Loop() {
		blocksReader := newBenchCountBlocksReader(src)

		runtime.GC()

		var ms0, ms1 runtime.MemStats
		runtime.ReadMemStats(&ms0)

		res, runErr := fileops.ExtractBlocks(ctx, fileops.BlocksOptions{
			Source:         blocksReader,
			StartDelimiter: start,
			EndDelimiter:   finish,
			Naming:         fileops.Sequential,
			Extension:      ".md",
		})
		if runErr != nil {
			b.Fatalf("ExtractBlocks: %v", runErr)
		}

		runtime.ReadMemStats(&ms1)

		last = blocksReader
		lastRes = res
		lastTotalAllocDelta = ms1.TotalAlloc - ms0.TotalAlloc
		lastHeapDelta = benchHeapAllocDeltaSigned(ms0.HeapAlloc, ms1.HeapAlloc)
	}

	reportFileopsBlocksBenchMetrics(b, last, &lastRes, lastTotalAllocDelta, lastHeapDelta)
}

func BenchmarkExtractBlocksManySmallBlocks(b *testing.B) {
	ctx := context.Background()

	src := buildBenchBlocksManySmallSource(benchBlocksSmallIterations)
	start := regexp.MustCompile("^```go$")
	finish := regexp.MustCompile("^```$")

	b.ReportAllocs()

	b.SetBytes(int64(len(src)))

	var (
		lastRes             fileops.BlocksResult
		lastTotalAllocDelta uint64
		lastHeapDelta       int64
		last                *benchCountBlocksReader
	)

	b.ResetTimer()

	for b.Loop() {
		blocksReader := newBenchCountBlocksReader(src)

		runtime.GC()

		var ms0, ms1 runtime.MemStats
		runtime.ReadMemStats(&ms0)

		res, runErr := fileops.ExtractBlocks(ctx, fileops.BlocksOptions{
			Source:         blocksReader,
			StartDelimiter: start,
			EndDelimiter:   finish,
			Naming:         fileops.Sequential,
			Extension:      ".md",
		})
		if runErr != nil {
			b.Fatalf("ExtractBlocks: %v", runErr)
		}

		runtime.ReadMemStats(&ms1)

		last = blocksReader
		lastRes = res
		lastTotalAllocDelta = ms1.TotalAlloc - ms0.TotalAlloc
		lastHeapDelta = benchHeapAllocDeltaSigned(ms0.HeapAlloc, ms1.HeapAlloc)
	}

	reportFileopsBlocksBenchMetrics(b, last, &lastRes, lastTotalAllocDelta, lastHeapDelta)
}
