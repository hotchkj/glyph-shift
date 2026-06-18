package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

func TestDispatchCLI_extractDestinationExistsReportsDestination(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	if err := afero.WriteFile(src.Fs, srcPath, []byte("line1\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	destPath := filepath.Join(root, "out.txt")
	if err := afero.WriteFile(out.Fs, destPath, []byte("exists\n"), 0o600); err != nil {
		t.Fatalf("write existing dest: %v", err)
	}

	code, stdout, gotErr := runExtractWithBuffers(
		t,
		[]string{"--source", "doc.md", "--lines", "1-1", "--destination", "out.txt"},
		root,
		src,
		out,
	)
	if code != exitDestExists {
		t.Fatalf("exit code: got %d want %d", code, exitDestExists)
	}
	if stdout != "" {
		t.Fatalf("stdout: want empty, got %q", stdout)
	}
	if gotErr.Error != testJSONErrDestinationExists {
		t.Fatalf("error: got %q want %s", gotErr.Error, testJSONErrDestinationExists)
	}
	if filepath.Clean(gotErr.Dest) != filepath.Clean(destPath) {
		t.Fatalf("dest: got %q want %q", gotErr.Dest, destPath)
	}
}

func TestDispatchCLI_blocksDestinationExistsReportsDestination(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	doc := "```go\nx\n```\n"
	if err := afero.WriteFile(src.Fs, srcPath, []byte(doc), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outDir := filepath.Join(root, "out")
	if err := out.Fs.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existingPath := filepath.Join(outDir, "001.md")
	if err := afero.WriteFile(out.Fs, existingPath, []byte("exists\n"), 0o600); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	code, stdout, gotErr := runBlocksWithBuffers(
		t,
		[]string{"--source", "doc.md", "--start-line", "^```go$", "--end-line", "^```$", "--output-dir", "out"},
		root,
		src,
		out,
	)
	if code != exitDestExists {
		t.Fatalf("exit code: got %d want %d", code, exitDestExists)
	}
	if stdout != "" {
		t.Fatalf("stdout: want empty, got %q", stdout)
	}
	if gotErr.Error != testJSONErrDestinationExists {
		t.Fatalf("error: got %q want %s", gotErr.Error, testJSONErrDestinationExists)
	}
	if filepath.Clean(gotErr.Dest) != filepath.Clean(existingPath) {
		t.Fatalf("dest: got %q want %q", gotErr.Dest, existingPath)
	}
}

func TestDispatchCLI_extractForceOverwritesExistingDestination(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	if err := afero.WriteFile(src.Fs, srcPath, []byte("newonly\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	destPath := filepath.Join(root, "out.txt")
	if err := afero.WriteFile(out.Fs, destPath, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runExtract(
		[]string{"--source", "doc.md", "--lines", "1-1", "--destination", "out.txt", "--force"},
		root,
		&stdout,
		&stderr,
		newTestOperationRunner(t, src, out),
	)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%q", code, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v", err)
	}
	if got["lines_extracted"] != float64(1) {
		t.Fatalf("lines_extracted: got %v want 1", got["lines_extracted"])
	}

	gotBytes := out.FileContent(destPath)
	want := []byte("newonly\n")
	if !bytes.Equal(gotBytes, want) {
		t.Fatalf("dest content: got %q want %q", gotBytes, want)
	}
}

func TestDispatchCLI_splitForceOverwritesExistingDestination(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	srcMem := testutil.NewMemSourceOpener()
	outMem := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	doc := "a\n---\nb\n---\nc\n"
	if err := afero.WriteFile(srcMem.Fs, srcPath, []byte(doc), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outDir := filepath.Join(root, "out")
	if err := outMem.Fs.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing001 := filepath.Join(outDir, "001.md")
	if err := afero.WriteFile(outMem.Fs, existing001, []byte("stale\n"), 0o600); err != nil {
		t.Fatalf("seed existing 001: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runSplit(
		[]string{"--source", "doc.md", "--delimiter", "^---$", "--output-dir", "out", "--force"},
		root,
		&stdout,
		&stderr,
		newTestOperationRunner(t, srcMem, outMem),
	)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%q", code, stderr.String())
	}

	// Sequential naming emits preamble before first delimiter section as 001.md; delimiter sections follow.
	wantBody := []byte("a\n")
	if !bytes.Equal(outMem.FileContent(existing001), wantBody) {
		t.Fatalf("001.md after force: got %q want %q", outMem.FileContent(existing001), wantBody)
	}
}

func TestDispatchCLI_blocksForceOverwritesExistingDestination(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	srcMem := testutil.NewMemSourceOpener()
	outMem := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	doc := "```go\nfresh\n```\n"
	if err := afero.WriteFile(srcMem.Fs, srcPath, []byte(doc), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outDir := filepath.Join(root, "out")
	if err := outMem.Fs.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dest001 := filepath.Join(outDir, "001.md")
	if err := afero.WriteFile(outMem.Fs, dest001, []byte("stale\n"), 0o600); err != nil {
		t.Fatalf("seed existing 001: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runBlocks(
		[]string{"--source", "doc.md", "--start-line", "^```go$", "--end-line", "^```$", "--output-dir", "out", "--force"},
		root,
		&stdout,
		&stderr,
		newTestOperationRunner(t, srcMem, outMem),
	)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%q", code, stderr.String())
	}

	wantBody := []byte("fresh\n")
	if !bytes.Equal(outMem.FileContent(dest001), wantBody) {
		t.Fatalf("001.md after force: got %q want %q", outMem.FileContent(dest001), wantBody)
	}
}
