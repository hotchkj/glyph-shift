package fileops

import (
	"errors"
	"math"
	"testing"
)

func TestSerializedSpanSeekAndLength_InvertedBounds(t *testing.T) {
	t.Parallel()

	_, _, err := SerializedSpanSeekAndLength(10, 5)
	if !errors.Is(err, errSerializedSpanBoundsInverted) {
		t.Fatalf("want errSerializedSpanBoundsInverted, got %v", err)
	}
}

func TestSerializedSpanSeekAndLength_LengthOverflowInt64(t *testing.T) {
	t.Parallel()

	start := uint64(0)
	end := uint64(math.MaxInt64) + 2 // half-open interval length overflows int64

	_, _, err := SerializedSpanSeekAndLength(start, end)
	if !errors.Is(err, ErrLineSpanContentTooLarge) {
		t.Fatalf("want ErrLineSpanContentTooLarge, got %v", err)
	}
}

func TestSerializedSpanSeekAndLength_StartOverflowInt64(t *testing.T) {
	t.Parallel()

	start := uint64(math.MaxInt64) + 1
	end := start + 50

	_, _, err := SerializedSpanSeekAndLength(start, end)
	if !errors.Is(err, ErrLineSpanOffsetTooLarge) {
		t.Fatalf("want ErrLineSpanOffsetTooLarge, got %v", err)
	}
}
