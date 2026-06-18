package fileops

import (
	"errors"
	"fmt"
)

// ErrEmptyRange is returned when the requested 1-based line range is empty after normalization.
var ErrEmptyRange = errors.New("empty range")

// ErrRangeExceedsFile is returned when the requested end line exceeds the number of lines in the source.
var ErrRangeExceedsFile = errors.New("range exceeds file")

// ErrUnclosedBlock is returned when a start delimiter matched but no end delimiter matched before EOF.
var ErrUnclosedBlock = errors.New("unclosed block")

// EmptyRangeError carries inclusive 1-based line endpoints for contract classification while
// preserving errors.Is(.., ErrEmptyRange).
type EmptyRangeError struct {
	Start int
	End   int
}

func (e *EmptyRangeError) Error() string {
	return fmt.Sprintf("%v: start=%d end=%d", ErrEmptyRange, e.Start, e.End)
}

func (e *EmptyRangeError) Unwrap() error {
	return ErrEmptyRange
}

// RangeExceedsFileError carries structured range metadata for contract classification while
// preserving errors.Is(.., ErrRangeExceedsFile).
type RangeExceedsFileError struct {
	FileLines  int
	RangeStart int
	RangeEnd   int
}

func (e *RangeExceedsFileError) Error() string {
	return fmt.Sprintf(
		"%v: file has %d lines but requested range start %d end %d cannot be satisfied",
		ErrRangeExceedsFile,
		e.FileLines,
		e.RangeStart,
		e.RangeEnd,
	)
}

func (e *RangeExceedsFileError) Unwrap() error {
	return ErrRangeExceedsFile
}

// UnclosedBlockDetailError carries the 1-based start delimiter line for the unclosed-block variant.
type UnclosedBlockDetailError struct {
	StartLine int
}

func (e *UnclosedBlockDetailError) Error() string {
	if e.StartLine > 0 {
		return fmt.Sprintf("%v: block started at line %d has no matching end delimiter", ErrUnclosedBlock, e.StartLine)
	}

	return ErrUnclosedBlock.Error()
}

func (e *UnclosedBlockDetailError) Unwrap() error {
	return ErrUnclosedBlock
}
