package linparse

import (
	"errors"
	"testing"
)

func TestLineRangeParseErrorNilReceiverUsesSentinel(t *testing.T) {
	t.Parallel()

	var parseErr *LineRangeParseError
	if !errors.Is(parseErr.Unwrap(), ErrLineRangeParse) {
		t.Fatal("Unwrap should return ErrLineRangeParse")
	}
	if NewLineRangeParseError(nil) != nil {
		t.Fatal("NewLineRangeParseError(nil) should return nil")
	}
}

func TestLineRangeParseError_nonNumericUsesStableHint(t *testing.T) {
	t.Parallel()

	_, _, err := ParseCLIRange("not-a-range")
	if err == nil {
		t.Fatal("expected parse error")
	}

	const want = `parse lines: parse line range: invalid integer "not"`
	got := NewLineRangeParseError(err).Error()
	if got != want {
		t.Fatalf("hint = %q, want %q", got, want)
	}
}
