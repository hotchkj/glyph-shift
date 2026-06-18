package testutil

import (
	"math"
	"testing"
)

// Tests for heapAllocDeltaSigned verify retained-heap delta conversion (signed subtraction on
// uint64 with G115-safe saturation) independent of GC timing. Integration tests bracketing real
// pipeline runs avoid asserting HeapAllocDelta non-zero because snapshots are best-effort.
func TestHeapAllocDeltaSigned_EqualYieldsZero(t *testing.T) {
	t.Parallel()

	if got := heapAllocDeltaSigned(42, 42); got != 0 {
		t.Fatalf("equal before/after: got %d want 0", got)
	}
}

func TestHeapAllocDeltaSigned_PositiveDelta(t *testing.T) {
	t.Parallel()

	if got := heapAllocDeltaSigned(10, 25); got != 15 {
		t.Fatalf("got %d want 15", got)
	}
}

func TestHeapAllocDeltaSigned_NegativeDelta(t *testing.T) {
	t.Parallel()

	if got := heapAllocDeltaSigned(200, 50); got != -150 {
		t.Fatalf("got %d want -150", got)
	}
}

func TestHeapAllocDeltaSigned_PositiveExactlyMaxInt64(t *testing.T) {
	t.Parallel()

	before := uint64(100)
	after := before + uint64(math.MaxInt64)
	if got := heapAllocDeltaSigned(before, after); got != math.MaxInt64 {
		t.Fatalf("got %d want MaxInt64", got)
	}
}

func TestHeapAllocDeltaSigned_PositiveExceedMaxInt64Saturates(t *testing.T) {
	t.Parallel()

	before := uint64(0)
	after := uint64(math.MaxInt64) + 1
	if got := heapAllocDeltaSigned(before, after); got != math.MaxInt64 {
		t.Fatalf("got %d want MaxInt64", got)
	}
}

func TestHeapAllocDeltaSigned_NegativeExactlyMinRepresentableDelta(t *testing.T) {
	t.Parallel()

	// Magnitude math.MaxInt64: after - before logic does not apply; magnitude before-after.
	before := uint64(math.MaxInt64) + 123
	after := uint64(123)
	want := -(int64(math.MaxInt64)) // magnitude MaxInt64, negative branch
	if got := heapAllocDeltaSigned(before, after); got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}

func TestHeapAllocDeltaSigned_NegativeExceedMinInt64MagnitudeSaturates(t *testing.T) {
	t.Parallel()

	before := uint64(math.MaxUint64)
	after := uint64(0)
	if got := heapAllocDeltaSigned(before, after); got != math.MinInt64 {
		t.Fatalf("got %d want MinInt64", got)
	}
}

func TestHeapAllocDeltaSigned_PositiveNearlyFullUint64NoWrap(t *testing.T) {
	t.Parallel()

	const step = uint64(7)
	before := uint64(math.MaxUint64) - step
	after := uint64(math.MaxUint64)
	if got := heapAllocDeltaSigned(before, after); got != int64(step) {
		t.Fatalf("got %d want %d", got, step)
	}
}
