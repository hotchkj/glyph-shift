package fileops

// ValidateExtractRange returns an error when the normalized line range cannot be extracted
// (ErrEmptyRange or ErrRangeExceedsFile). Intended for validating before opening the destination.
func ValidateExtractRange(lr LineRange, lineCount int) error {
	return normalizeExtractRange(lr, lineCount)
}

// normalizeExtractRange applies 1-based LineRange rules and returns an error when the range
// is empty or the end line exceeds lineCount.
func normalizeExtractRange(lr LineRange, lineCount int) error {
	start := lr.Start
	end := lr.End

	if start == 0 {
		start = 1
	}

	if end == 0 {
		end = lineCount
	}

	if start > end {
		return &EmptyRangeError{Start: start, End: end}
	}

	if end > lineCount {
		return &RangeExceedsFileError{
			FileLines:  lineCount,
			RangeStart: start,
			RangeEnd:   end,
		}
	}

	return nil
}
