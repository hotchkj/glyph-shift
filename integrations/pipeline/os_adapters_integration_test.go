//go:build integration

// Real-OS justification: production pipeline adapters must surface OS error branches
// (missing paths, invalid path syntax, missing parent directories) without unit tests
// constructing zero-value adapters that perform host filesystem I/O.
package pipeline_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

func TestProductionAdaptersOSErrorBranches(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	out := pipeline.NewOSOutputOpener()
	stater := pipeline.NewOSFileStater()

	missing := filepath.Join(dir, "no-such-file.dat")
	if _, err := stater.Stat(missing); err == nil {
		t.Fatal("Stat missing path: expected error")
	}

	if err := out.MkdirAll(mkdirAllMustFailPath(t, dir), pipeline.DirPerm); err == nil {
		t.Fatal("MkdirAll invalid path: expected error")
	}

	dest := filepath.Join(dir, "nested", "out.txt")
	_, openErr := out.OpenFile(dest, pipeline.OutputCreateExclusive, pipeline.FilePerm)
	if openErr == nil {
		t.Fatal("OpenFile missing parent: expected error")
	}
}

func TestProductionAdaptersRejectNULPath(t *testing.T) {
	t.Parallel()

	invalidPath := string([]byte{0})
	out := pipeline.NewOSOutputOpener()
	stater := pipeline.NewOSFileStater()

	if err := out.MkdirAll(invalidPath, pipeline.DirPerm); !errors.Is(err, pipeline.ErrPathContainsNUL) {
		t.Fatalf("MkdirAll NUL path: got %v want ErrPathContainsNUL", err)
	}

	_, openErr := out.OpenFile(invalidPath, pipeline.OutputCreateExclusive, pipeline.FilePerm)
	if !errors.Is(openErr, pipeline.ErrPathContainsNUL) {
		t.Fatalf("OpenFile NUL path: got %v want ErrPathContainsNUL", openErr)
	}

	if _, err := stater.Stat(invalidPath); !errors.Is(err, pipeline.ErrPathContainsNUL) {
		t.Fatalf("Stat NUL path: got %v want ErrPathContainsNUL", err)
	}
}

// mkdirAllMustFailPath returns a path MkdirAll must reject on the current OS.
func mkdirAllMustFailPath(t *testing.T, dir string) string {
	t.Helper()

	if runtime.GOOS == "windows" {
		return `bad|path`
	}

	blockingFile := filepath.Join(dir, "blocking-file")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}

	return filepath.Join(blockingFile, "child")
}
