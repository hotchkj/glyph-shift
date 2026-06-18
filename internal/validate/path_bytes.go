package validate

import (
	"errors"
	"strings"
)

// ErrPathContainsNUL is returned when a path argument contains an embedded NUL byte.
var ErrPathContainsNUL = errors.New("validate: path contains NUL byte")

func rejectNULByteInPath(path string) error {
	if strings.IndexByte(path, 0) >= 0 {
		return ErrPathContainsNUL
	}

	return nil
}
