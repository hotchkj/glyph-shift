package harness

import "errors"

var (
	// ErrUnknownLineTerminator is returned when a terminator name is not LF, CRLF, or CR.
	ErrUnknownLineTerminator = errors.New("unknown line terminator")
	// ErrInvalidLineRange is returned when a line range is out of bounds.
	ErrInvalidLineRange = errors.New("invalid line range")
)
