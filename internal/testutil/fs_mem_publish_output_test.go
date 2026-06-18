// fs_mem_publish_output_test.go covers memory publish session wiring and MemOutputOpener append
// behavior when the destination file is absent, using in-memory afero only (no workspace IO).
package testutil_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

func TestNewMemPublishSessionForOutput_SharesOutputFs(t *testing.T) {
	t.Parallel()

	backing := afero.NewMemMapFs()
	out := testutil.NewMemOutputOpenerWithFS(backing)
	sess := testutil.NewMemPublishSessionForOutput(out)

	if sess.Fs != out.Fs {
		t.Fatal("NewMemPublishSessionForOutput: MemFileSession.Fs must alias MemOutputOpener.Fs")
	}

	if sess.Fs != backing {
		t.Fatal("NewMemPublishSessionForOutput: expected session backed by the same afero.Fs as the output opener")
	}
}

func TestMemOutputOpener_OpenFile_AppendWithMissingFile(t *testing.T) {
	t.Parallel()

	out := testutil.NewMemOutputOpener()
	path := filepath.Join(string([]rune{filepath.Separator}), "mem-out", "append-missing.txt")

	wc, err := out.OpenFile(path, testutil.OutputWriteAppendCreate, 0o644)
	if err != nil {
		t.Fatalf("OpenFile append on missing path: %v", err)
	}

	payload := []byte("after-missing")
	if _, err := wc.Write(payload); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := wc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := out.FileContent(path)
	if !bytes.Equal(got, payload) {
		t.Fatalf("FileContent: got %q want %q (append must not prepend seed when file is absent)", got, payload)
	}
}
