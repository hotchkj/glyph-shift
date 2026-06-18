//go:build integration

// Real-OS justification: production FileSession adapters must reject NUL paths before
// host syscalls; unit tests must not construct production sessions (forbidigo).
package session_test

import (
	"errors"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

func TestProductionFileSessionOpenReadRejectsNULPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
	}{
		{name: "nul_at_start", path: string([]byte{0})},
		{name: "nul_in_middle", path: "a\x00b"},
		{name: "nul_at_end", path: "path\x00"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := fileops.NewOSFileSession().OpenRead(tc.path)
			if !errors.Is(err, fileops.ErrPathContainsNUL) {
				t.Fatalf("OpenRead %q: got %v want ErrPathContainsNUL", tc.path, err)
			}
		})
	}
}

func TestProductionFileSessionRemoveRejectsNULPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
	}{
		{name: "nul_at_start", path: string([]byte{0})},
		{name: "nul_in_middle", path: "foo\x00bar"},
		{name: "nul_at_end", path: "foo\x00"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := fileops.NewOSFileSession().Remove(tc.path)
			if !errors.Is(err, fileops.ErrPathContainsNUL) {
				t.Fatalf("Remove %q: got %v want ErrPathContainsNUL", tc.path, err)
			}
		})
	}
}
