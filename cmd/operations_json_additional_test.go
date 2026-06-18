package cmd

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

func TestDispatchCLI_extractPreviewSuccessFlatJSONWouldExtractLinesAndCreate(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	srcMem := testutil.NewMemSourceOpener()
	outMem := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.txt")
	if err := afero.WriteFile(srcMem.Fs, srcPath, []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	code, stdout, gotErr := runExtractWithBuffers(
		t,
		[]string{"--source", "doc.txt", "--lines", "1-2", "--destination", "out.txt", "--preview"},
		root,
		srcMem,
		outMem,
	)
	if code != 0 {
		t.Fatalf("exit code: got %d error=%#v", code, gotErr)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v", err)
	}
	wantKeys := map[string]struct{}{
		"would_extract_lines": {},
		"would_create":        {},
	}
	if len(got) != len(wantKeys) {
		t.Fatalf("stdout JSON flat keys mismatch got=%v (%s)", gotKeysStable(got), stdout)
	}
	for k := range got {
		if _, ok := wantKeys[k]; !ok {
			t.Fatalf("unexpected key %q in %s", k, stdout)
		}
	}
	if got["would_extract_lines"] != float64(2) {
		t.Fatalf("would_extract_lines: got %v want 2", got["would_extract_lines"])
	}

	wantDest := filepath.Clean(filepath.Join(root, "out.txt"))
	wc, ok := got["would_create"].(string)
	if !ok {
		t.Fatalf("would_create type: got %T", got["would_create"])
	}

	requireAbsoluteNativePathEquals(t, "would_create", wantDest, wc)
}

func TestDispatchCLI_splitNoDelimiterMatch_JSONError(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	if err := afero.WriteFile(src.Fs, srcPath, []byte("plain\ntext\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	code, stdout, gotErr := runSplitWithBuffers(
		t,
		[]string{"--source", "doc.md", "--delimiter", "^---$", "--output-dir", "out"},
		root,
		src,
		out,
	)
	if code != exitValidation {
		t.Fatalf("exit code: got %d want %d", code, exitValidation)
	}
	if stdout != "" {
		t.Fatalf("stdout: want empty, got %q", stdout)
	}
	if gotErr.Error != "no_delimiter_match" {
		t.Fatalf("error: got %q want no_delimiter_match", gotErr.Error)
	}
}

func TestDispatchCLI_blocksNoBlocksFound_JSONError(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	if err := afero.WriteFile(src.Fs, srcPath, []byte("plain\ntext\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	code, stdout, gotErr := runBlocksWithBuffers(
		t,
		[]string{"--source", "doc.md", "--start-line", "^```go$", "--end-line", "^```$", "--output-dir", "out"},
		root,
		src,
		out,
	)
	if code != exitValidation {
		t.Fatalf("exit code: got %d want %d", code, exitValidation)
	}
	if stdout != "" {
		t.Fatalf("stdout: want empty, got %q", stdout)
	}
	if gotErr.Error != "no_blocks_found" {
		t.Fatalf("error: got %q want no_blocks_found", gotErr.Error)
	}
}
