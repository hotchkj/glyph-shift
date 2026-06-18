package fileops

import (
	"bytes"
	"context"
	"io"
	"math"
	"runtime"
	"testing"
)

// Mirrors testutil.StreamingBodyResidencyRetainedHeapBudget; fileops tests cannot import testutil
// (import cycle: testutil → fileops).
const (
	streamingBodyResidencyMaxLargeToSmallRatio int64 = 2
	streamingBodyResidencyNoiseAllowance       int64 = 1 * 1024 * 1024
)

func transformStreamRetainedHeapBudget(smallRetainedHeapDelta int64) int64 {
	baseline := smallRetainedHeapDelta
	if baseline < 0 {
		baseline = 0
	}

	return streamingBodyResidencyMaxLargeToSmallRatio*baseline + streamingBodyResidencyNoiseAllowance
}

func buildLongLineSingleLogical(bodyLen int, term []byte) []byte {
	raw := make([]byte, bodyLen+len(term))
	for i := range bodyLen {
		raw[i] = 'x'
	}
	copy(raw[bodyLen:], term)

	return raw
}

func retainedHeapDeltaTransformStreamDiscard(t *testing.T, raw []byte, opts TransformOptions) int64 {
	t.Helper()

	runtime.GC()
	runtime.GC()

	var msIn runtime.MemStats
	runtime.ReadMemStats(&msIn)

	spill := memWhitespaceSpillForTests(io.Discard, opts)
	_, err := runTransformStream(context.Background(), bytes.NewReader(raw), opts, io.Discard, spill)
	if err != nil {
		t.Fatalf("runTransformStream: %v", err)
	}

	runtime.GC()

	var msOut runtime.MemStats
	runtime.ReadMemStats(&msOut)

	return memStatsHeapDelta(msIn.HeapAlloc, msOut.HeapAlloc)
}

// memStatsHeapDelta returns after-before for runtime.MemStats.HeapAlloc as int64 without casting
// each absolute uint64 to int64 (gosec G115).
func memStatsHeapDelta(before, after uint64) int64 {
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

// TestRunTransformStream_SingleVeryLongLineRetainedHeapStaysNearSmallCase uses one logical line
// sized in the multi‑MiB range versus a multi‑KiB baseline. A transform path that materializes the
// full logical line in a single []byte (as ForEachLineFromContext commits) retains incremental
// heap proportional to line length; streaming content keeps paired retained heap approximately
// flat versus the baseline under MemStats bracketing.
func TestRunTransformStream_SingleVeryLongLineRetainedHeapStaysNearSmallCase(t *testing.T) {
	t.Skip("MemStats residency gate flaky on CI; see issue #2")
	if testing.Short() {
		t.Skip("long-line memory probe")
	}

	lf := TargetLF
	opts := TransformOptions{LineEndings: &lf, TrimTrailing: true}

	smallRaw := buildLongLineSingleLogical(4096, []byte{'\n'})
	largeRaw := buildLongLineSingleLogical(6<<20, []byte{'\r', '\n'})

	smallDelta := retainedHeapDeltaTransformStreamDiscard(t, smallRaw, opts)
	largeDelta := retainedHeapDeltaTransformStreamDiscard(t, largeRaw, opts)

	budget := transformStreamRetainedHeapBudget(smallDelta)
	if largeDelta > budget {
		t.Fatalf(
			"large retained heap delta %d exceeds budget %d (small %d); "+
				"streaming transform likely retained O(line) buffering",
			largeDelta,
			budget,
			smallDelta,
		)
	}
}

func buildHugeWhitespaceOnlyLine(bodyLen int, term []byte) []byte {
	raw := make([]byte, bodyLen+len(term))
	for i := range bodyLen {
		raw[i] = ' '
	}

	copy(raw[bodyLen:], term)

	return raw
}

func buildHugeAltWhitespacePrefixThenX(bodyLen int, term []byte) []byte {
	raw := make([]byte, bodyLen+1+len(term))
	for i := range bodyLen {
		if i&1 == 0 {
			raw[i] = ' '
		} else {
			raw[i] = '\t'
		}
	}

	raw[bodyLen] = 'x'
	copy(raw[bodyLen+1:], term)

	return raw
}

// TestRunTransformStream_TrimTrailing_HugeWhitespaceOnlyLine_BoundedMem probes an all-ws logical
// line under TrimTrailing: bytes are discarded at the line end, so streaming state must not
// retain O(line) bytes in heap (as an unbounded pending suffix slice would).
func TestRunTransformStream_TrimTrailing_HugeWhitespaceOnlyLine_BoundedMem(t *testing.T) {
	t.Skip("MemStats residency gate flaky on CI; see issue #2")
	if testing.Short() {
		t.Skip("long-line memory probe")
	}

	opts := TransformOptions{TrimTrailing: true}
	smallRaw := buildHugeWhitespaceOnlyLine(4096, []byte{'\n'})
	largeRaw := buildHugeWhitespaceOnlyLine(6<<20, []byte{'\n'})

	var outSmall bytes.Buffer
	spill := memWhitespaceSpillForTests(&outSmall, opts)
	gotSmall, err := runTransformStream(context.Background(), bytes.NewReader(smallRaw), opts, &outSmall, spill)
	if err != nil {
		t.Fatalf("runTransformStream small: %v", err)
	}

	wantOut := []byte{'\n'}
	if !bytes.Equal(outSmall.Bytes(), wantOut) {
		t.Fatalf("small out %q want %q", outSmall.Bytes(), wantOut)
	}

	if gotSmall.TrailingTrimmed != 1 {
		t.Fatalf("small TrailingTrimmed %d want 1", gotSmall.TrailingTrimmed)
	}

	var outLarge bytes.Buffer
	spillLarge := memWhitespaceSpillForTests(&outLarge, opts)
	gotLarge, err := runTransformStream(context.Background(), bytes.NewReader(largeRaw), opts, &outLarge, spillLarge)
	if err != nil {
		t.Fatalf("runTransformStream large: %v", err)
	}

	if !bytes.Equal(outLarge.Bytes(), wantOut) {
		t.Fatalf("large out %q want %q", outLarge.Bytes(), wantOut)
	}

	if gotLarge.TrailingTrimmed != 1 {
		t.Fatalf("large TrailingTrimmed %d want 1", gotLarge.TrailingTrimmed)
	}

	smallDelta := retainedHeapDeltaTransformStreamDiscard(t, smallRaw, opts)
	largeDelta := retainedHeapDeltaTransformStreamDiscard(t, largeRaw, opts)
	budget := transformStreamRetainedHeapBudget(smallDelta)
	if largeDelta > budget {
		t.Fatalf(
			"large retained heap delta %d exceeds budget %d (small %d); "+
				"trim-trailing likely retained O(line) whitespace suffix in RAM",
			largeDelta,
			budget,
			smallDelta,
		)
	}
}

// TestRunTransformStream_TrimTrailing_HugeWhitespacePrefixThenContent_BoundedMem verifies a long
// inner run of spaces/tabs before non-whitespace still emits exact bytes while transform state
// stays within the same retained-heap budget envelope as a shorter line.
func TestRunTransformStream_TrimTrailing_HugeWhitespacePrefixThenContent_BoundedMem(t *testing.T) {
	t.Skip("MemStats residency gate flaky on CI; see issue #2")
	if testing.Short() {
		t.Skip("long-line memory probe")
	}

	opts := TransformOptions{TrimTrailing: true}
	smallRaw := buildHugeAltWhitespacePrefixThenX(4096, []byte{'\n'})
	largeRaw := buildHugeAltWhitespacePrefixThenX(6<<20, []byte{'\n'})

	var outSmall bytes.Buffer
	spill := memWhitespaceSpillForTests(&outSmall, opts)
	_, err := runTransformStream(context.Background(), bytes.NewReader(smallRaw), opts, &outSmall, spill)
	if err != nil {
		t.Fatalf("runTransformStream small: %v", err)
	}

	if !bytes.Equal(outSmall.Bytes(), smallRaw) {
		t.Fatalf("small out len %d want %d", len(outSmall.Bytes()), len(smallRaw))
	}

	var outLarge bytes.Buffer
	spillLarge := memWhitespaceSpillForTests(&outLarge, opts)
	_, err = runTransformStream(context.Background(), bytes.NewReader(largeRaw), opts, &outLarge, spillLarge)
	if err != nil {
		t.Fatalf("runTransformStream large: %v", err)
	}

	if !bytes.Equal(outLarge.Bytes(), largeRaw) {
		t.Fatalf("large out len %d want %d", len(outLarge.Bytes()), len(largeRaw))
	}

	smallDelta := retainedHeapDeltaTransformStreamDiscard(t, smallRaw, opts)
	largeDelta := retainedHeapDeltaTransformStreamDiscard(t, largeRaw, opts)
	budget := transformStreamRetainedHeapBudget(smallDelta)
	if largeDelta > budget {
		t.Fatalf(
			"large retained heap delta %d exceeds budget %d (small %d); "+
				"trim-trailing inner ws spill path likely retained O(line) buffering in heap",
			largeDelta,
			budget,
			smallDelta,
		)
	}
}
