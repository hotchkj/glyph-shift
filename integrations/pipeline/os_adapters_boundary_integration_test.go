//go:build integration

// Real-OS justification: NUL-byte paths must be rejected by production pipeline OS
// adapters (source opener with OS FileSession, output opener, file stater) without
// relying on in-memory testutil seams.
package pipeline_test

import (
	"errors"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

func assertOSSourceOpenerOpenInvalidPath(t *testing.T, invalidPath string) {
	t.Helper()

	src, err := pipeline.NewOSSourceOpener(fileops.NewOSFileSession())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := src.Open(invalidPath); !errors.Is(err, fileops.ErrPathContainsNUL) {
		t.Fatalf("Open invalid path: got %v want ErrPathContainsNUL", err)
	}
}

func assertOSOutputOpenerMkdirAllInvalidPath(t *testing.T, invalidPath string) {
	t.Helper()

	out := pipeline.NewOSOutputOpener()
	if err := out.MkdirAll(invalidPath, pipeline.DirPerm); !errors.Is(err, fileops.ErrPathContainsNUL) {
		t.Fatalf("MkdirAll invalid path: got %v want ErrPathContainsNUL", err)
	}
}

func assertOSOutputOpenerOpenFileInvalidPath(t *testing.T, invalidPath string) {
	t.Helper()

	out := pipeline.NewOSOutputOpener()
	_, err := out.OpenFile(invalidPath, pipeline.OutputCreateExclusive, pipeline.FilePerm)
	if !errors.Is(err, fileops.ErrPathContainsNUL) {
		t.Fatalf("OpenFile invalid path: got %v want ErrPathContainsNUL", err)
	}
}

func assertOSFileStaterStatInvalidPath(t *testing.T, invalidPath string) {
	t.Helper()

	st := pipeline.NewOSFileStater()
	if _, err := st.Stat(invalidPath); !errors.Is(err, fileops.ErrPathContainsNUL) {
		t.Fatalf("Stat invalid path: got %v want ErrPathContainsNUL", err)
	}
}

func TestOSPipelineAdapterMethodsInvalidPathErrors(t *testing.T) {
	t.Parallel()

	invalidPath := string([]byte{0})

	cases := []struct {
		name   string
		assert func(*testing.T, string)
	}{
		{name: "OSSourceOpener/Open", assert: assertOSSourceOpenerOpenInvalidPath},
		{name: "OSOutputOpener/MkdirAll", assert: assertOSOutputOpenerMkdirAllInvalidPath},
		{name: "OSOutputOpener/OpenFile", assert: assertOSOutputOpenerOpenFileInvalidPath},
		{name: "OSFileStater/Stat", assert: assertOSFileStaterStatInvalidPath},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.assert(t, invalidPath)
		})
	}
}
