package fileops

import (
	"bytes"
	"context"
	"regexp"
	"testing"
)

// TestMatchLargeLine_HeapBudget keeps multi-megabyte line coverage without MemStats deltas (flaky under load).
// Invariant: MatchLineSpan only traverses offsets within the line content span (metered Seek/Read high-water mark
// stays at or before ContentEnd), so matching does not observe trailer bytes or materialize the full line body.
func TestMatchLargeLine_HeapBudget(t *testing.T) {
	needle := []byte("--MARKER--")

	smallLine := append(bytes.Repeat([]byte{'a'}, 8192), append(needle, '\n')...)
	largeLine := append(bytes.Repeat([]byte{'a'}, 6<<20), append(needle, '\n')...)

	smallSpan := singleLineSpanForLF(len(smallLine))
	largeSpan := singleLineSpanForLF(len(largeLine))

	re := regexp.MustCompile("--MARKER--")

	msSmall := newMeteringSeeker(smallLine)
	okSmall, err := MatchLineSpan(context.Background(), msSmall, smallSpan, re)
	if err != nil || !okSmall {
		t.Fatalf("small MatchLineSpan ok=%v err=%v", okSmall, err)
	}
	if msSmall.maxOffsetSeen > int64(smallSpan.ContentEnd) { //nolint:gosec // G115: test fixture span extents
		t.Fatalf("small: maxOffsetSeen %d > ContentEnd %d", msSmall.maxOffsetSeen, smallSpan.ContentEnd)
	}

	msLarge := newMeteringSeeker(largeLine)
	okLarge, err := MatchLineSpan(context.Background(), msLarge, largeSpan, re)
	if err != nil || !okLarge {
		t.Fatalf("large MatchLineSpan ok=%v err=%v", okLarge, err)
	}
	if msLarge.maxOffsetSeen > int64(largeSpan.ContentEnd) { //nolint:gosec // G115: test fixture span extents
		t.Fatalf(
			"large: maxOffsetSeen %d > ContentEnd %d (matcher must not read past line content)",
			msLarge.maxOffsetSeen,
			largeSpan.ContentEnd,
		)
	}
}

func singleLineSpanForLF(serializedLen int) LineSpan {
	serializedEnd := uint64FromPositiveInt(serializedLen)
	contentEnd := serializedEnd - 1

	return LineSpan{
		LineNum:         1,
		ContentStart:    0,
		ContentEnd:      contentEnd,
		SerializedStart: 0,
		SerializedEnd:   serializedEnd,
		Terminator:      LineTerminatorLF,
	}
}

func uint64FromPositiveInt(v int) uint64 {
	if v <= 0 {
		return 0
	}

	return uint64(v) //nolint:gosec // G115: test fixture lengths are positive and bounded by in-memory slices
}
