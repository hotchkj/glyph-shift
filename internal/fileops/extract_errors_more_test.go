package fileops_test

import (
	"errors"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

func TestExtractStructuredErrorsFormatAndUnwrap(t *testing.T) {
	t.Parallel()

	empty := &fileops.EmptyRangeError{Start: 3, End: 2}
	if empty.Error() == "" {
		t.Fatal("empty range error message must be non-empty")
	}
	if !errors.Is(empty, fileops.ErrEmptyRange) {
		t.Fatal("empty range error should unwrap ErrEmptyRange")
	}

	exceeds := &fileops.RangeExceedsFileError{FileLines: 2, RangeStart: 3, RangeEnd: 4}
	if exceeds.Error() == "" {
		t.Fatal("range-exceeds-file error message must be non-empty")
	}
	if !errors.Is(exceeds, fileops.ErrRangeExceedsFile) {
		t.Fatal("range-exceeds-file error should unwrap ErrRangeExceedsFile")
	}

	unclosed := &fileops.UnclosedBlockDetailError{}
	if unclosed.Error() == "" {
		t.Fatal("unclosed block error message must be non-empty")
	}
	if !errors.Is(unclosed, fileops.ErrUnclosedBlock) {
		t.Fatal("unclosed block detail should unwrap ErrUnclosedBlock")
	}
}
