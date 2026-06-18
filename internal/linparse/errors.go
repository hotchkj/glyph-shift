package linparse

import (
	"errors"
	"strconv"
)

// ErrEmptyLineRange is returned when the --lines flag is empty.
var ErrEmptyLineRange = errors.New("empty line range")

// ErrInvalidLineRange is returned when the --lines flag cannot be parsed.
var ErrInvalidLineRange = errors.New("invalid line range")

// ErrLineRangeParse is returned when a public lines argument cannot be parsed.
var ErrLineRangeParse = errors.New("line range parse error")

// LineRangeParseError preserves the parse detail while giving callers a stable
// public sentinel for CLI/MCP error classification.
type LineRangeParseError struct {
	Err error
}

func (e *LineRangeParseError) Error() string {
	if e == nil || e.Err == nil {
		return ErrLineRangeParse.Error()
	}

	return "parse lines: " + e.Err.Error()
}

func (e *LineRangeParseError) Unwrap() error {
	if e == nil || e.Err == nil {
		return ErrLineRangeParse
	}

	return errors.Join(ErrLineRangeParse, e.Err)
}

// InvalidLineRangeIntegerError reports a non-numeric token in a --lines value.
type InvalidLineRangeIntegerError struct {
	Token string
}

func (e *InvalidLineRangeIntegerError) Error() string {
	return "parse line range: invalid integer " + strconv.Quote(e.Token)
}

func invalidLineRangeInteger(token string) error {
	return &InvalidLineRangeIntegerError{Token: token}
}

// NewLineRangeParseError wraps errors returned by ParseCLIRange for public
// operation error classification.
func NewLineRangeParseError(err error) error {
	if err == nil {
		return nil
	}

	return &LineRangeParseError{Err: err}
}
