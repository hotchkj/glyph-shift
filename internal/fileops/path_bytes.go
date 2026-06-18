package fileops

import (
	"errors"
	"strings"
)

// ErrPathContainsNUL is returned when a path argument contains an embedded NUL byte.
var ErrPathContainsNUL = errors.New("fileops: path contains NUL byte")

func RejectNULByteInPath(path string) error {
	if strings.IndexByte(path, 0) >= 0 {
		return ErrPathContainsNUL
	}

	return nil
}
