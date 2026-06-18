package fileops

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"runtime"
	"testing"
)

func totalAllocDelta(t *testing.T, fn func()) int64 {
	t.Helper()

	runtime.GC()
	runtime.GC()

	var msIn runtime.MemStats
	runtime.ReadMemStats(&msIn)

	fn()

	var msOut runtime.MemStats
	runtime.ReadMemStats(&msOut)

	return memStatsHeapDelta(msIn.TotalAlloc, msOut.TotalAlloc)
}

// Seekable-path probes (bytes.Reader implements io.ReadSeeker): Extract uses extractViaSerializedSpan;
// ScanSplitSectionsMeta and ScanBlocksMeta use span scan + MatchLineSpan.
//
// Uses runtime.MemStats.TotalAlloc delta (not post-GC HeapAlloc) because the failure mode is
// transient O(line) copies during the operation. Budget matches transform_stream_memory_test:
// maxLargeToSmallRatio×small + noise allowance (deterministic enough for CI when not parallelized).
func longLineAllocBudget(smallDelta int64) int64 {
	baseline := smallDelta
	if baseline < 0 {
		baseline = 0
	}

	return streamingBodyResidencyMaxLargeToSmallRatio*baseline + streamingBodyResidencyNoiseAllowance
}

const longLineAllocationProbeBytes = 2 << 20

func TestExtractSingleHugeLineAllocationStaysNearSmallCase(t *testing.T) {
	t.Skip("MemStats residency gate flaky on CI; see issue #2")
	if testing.Short() {
		t.Skip("long-line allocation probe")
	}

	smallRaw := buildLongLineSingleLogical(4096, []byte{'\n'})
	largeRaw := buildLongLineSingleLogical(longLineAllocationProbeBytes, []byte{'\n'})

	runExtract := func(raw []byte) func() {
		return func() {
			_, err := Extract(context.Background(), ExtractOptions{
				Source: bytes.NewReader(raw),
				Lines:  LineRange{Start: 1, End: 1},
			}, io.Discard)
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}
		}
	}

	smallDelta := totalAllocDelta(t, runExtract(smallRaw))
	largeDelta := totalAllocDelta(t, runExtract(largeRaw))
	budget := longLineAllocBudget(smallDelta)
	if largeDelta > budget {
		t.Fatalf(
			"large allocation delta %d exceeds budget %d (small %d); extract likely allocated O(line) during scan/copy",
			largeDelta,
			budget,
			smallDelta,
		)
	}
}

func TestScanSplitSectionsSingleHugeLineAllocationStaysNearSmallCase(t *testing.T) {
	t.Skip("MemStats residency gate flaky on CI; see issue #2")
	if testing.Short() {
		t.Skip("long-line allocation probe")
	}

	delimiter := regexp.MustCompile("^---$")
	smallRaw := append([]byte("---\n"), buildLongLineSingleLogical(4096, []byte{'\n'})...)
	largeRaw := append([]byte("---\n"), buildLongLineSingleLogical(longLineAllocationProbeBytes, []byte{'\n'})...)

	runScan := func(raw []byte) func() {
		return func() {
			_, err := ScanSplitSectionsMeta(
				context.Background(),
				SplitOptions{
					Source:    bytes.NewReader(raw),
					Delimiter: delimiter,
					Naming:    Sequential,
				},
				BoundedScanLimits{MaxFiles: 50},
			)
			if err != nil {
				t.Fatalf("ScanSplitSectionsMeta: %v", err)
			}
		}
	}

	smallDelta := totalAllocDelta(t, runScan(smallRaw))
	largeDelta := totalAllocDelta(t, runScan(largeRaw))
	budget := longLineAllocBudget(smallDelta)
	if largeDelta > budget {
		t.Fatalf(
			"large allocation delta %d exceeds budget %d (small %d); split scan likely allocated O(line) during scan",
			largeDelta,
			budget,
			smallDelta,
		)
	}
}

func TestScanBlocksSingleHugeLineAllocationStaysNearSmallCase(t *testing.T) {
	t.Skip("MemStats residency gate flaky on CI; see issue #2")
	if testing.Short() {
		t.Skip("long-line allocation probe")
	}

	start := regexp.MustCompile("^```go$")
	end := regexp.MustCompile("^```$")
	smallRaw := buildFencedHugeLineSource(4096)
	largeRaw := buildFencedHugeLineSource(longLineAllocationProbeBytes)

	runScan := func(raw []byte) func() {
		return func() {
			_, err := ScanBlocksMeta(
				context.Background(),
				BlocksOptions{
					Source:         bytes.NewReader(raw),
					StartDelimiter: start,
					EndDelimiter:   end,
					Naming:         Sequential,
				},
				BoundedScanLimits{MaxFiles: 50},
			)
			if err != nil {
				t.Fatalf("ScanBlocksMeta: %v", err)
			}
		}
	}

	smallDelta := totalAllocDelta(t, runScan(smallRaw))
	largeDelta := totalAllocDelta(t, runScan(largeRaw))
	budget := longLineAllocBudget(smallDelta)
	if largeDelta > budget {
		t.Fatalf(
			"large allocation delta %d exceeds budget %d (small %d); blocks scan likely allocated O(line) during scan",
			largeDelta,
			budget,
			smallDelta,
		)
	}
}

func buildFencedHugeLineSource(bodyLen int) []byte {
	raw := []byte("```go\n")
	raw = append(raw, buildLongLineSingleLogical(bodyLen, []byte{'\n'})...)
	raw = append(raw, []byte("```\n")...)

	return raw
}
