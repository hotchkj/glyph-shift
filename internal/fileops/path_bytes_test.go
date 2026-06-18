package fileops

import (
	"errors"
	"testing"
)

func TestRejectNULByteInPath(t *testing.T) {
	t.Parallel()

	if err := RejectNULByteInPath("ok"); err != nil {
		t.Fatalf("clean path: %v", err)
	}

	if err := RejectNULByteInPath(string([]byte{'a', 0, 'b'})); !errors.Is(err, ErrPathContainsNUL) {
		t.Fatalf("NUL path: got %v want ErrPathContainsNUL", err)
	}
}
