package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

const testJSONErrDestinationExists = "destination_exists"

func testCmdWorkspaceRoot() string {
	raw := filepath.Join(string([]rune{filepath.Separator}), "cmd-operations-test-root")
	absRoot, err := filepath.Abs(raw)
	if err != nil {
		panic(err)
	}

	return filepath.Clean(absRoot)
}

type operationRunnerFunc func([]string, string, io.Writer, io.Writer, pipeline.Runner) int

func newTestOperationRunner(t *testing.T, src *testutil.MemSourceOpener,
	out *testutil.MemOutputOpener,
) pipeline.Runner {
	t.Helper()

	runner, err := pipeline.NewDefaultRunner(
		src,
		out,
		testutil.NewMemFileStater(),
		testutil.NoSymlinkPathResolver{},
		testutil.NewMemPublishSessionForOutput(out),
	)
	if err != nil {
		t.Fatalf("NewDefaultRunner: %v", err)
	}

	return runner
}

func runOperationWithBuffers(
	t *testing.T,
	args []string,
	dir string,
	src *testutil.MemSourceOpener,
	out *testutil.MemOutputOpener,
	run operationRunnerFunc,
) (code int, stdoutText string, gotErr errorJSONOutput) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code = run(args, dir, &stdout, &stderr, newTestOperationRunner(t, src, out))

	if stderr.Len() > 0 {
		_ = json.Unmarshal(stderr.Bytes(), &gotErr)
	}

	return code, stdout.String(), gotErr
}

func runSplitWithBuffers(
	t *testing.T,
	args []string,
	dir string,
	src *testutil.MemSourceOpener,
	out *testutil.MemOutputOpener,
) (code int, stdoutText string, gotErr errorJSONOutput) {
	return runOperationWithBuffers(t, args, dir, src, out, runSplit)
}

func runBlocksWithBuffers(
	t *testing.T,
	args []string,
	dir string,
	src *testutil.MemSourceOpener,
	out *testutil.MemOutputOpener,
) (code int, stdoutText string, gotErr errorJSONOutput) {
	return runOperationWithBuffers(t, args, dir, src, out, runBlocks)
}

func runExtractWithBuffers(
	t *testing.T,
	args []string,
	dir string,
	src *testutil.MemSourceOpener,
	out *testutil.MemOutputOpener,
) (code int, stdoutText string, gotErr errorJSONOutput) {
	return runOperationWithBuffers(t, args, dir, src, out, runExtract)
}

//nolint:gocyclo,cyclop // JSON operation matrix enumerates orthogonal flags and assertions.
func TestDispatchCLI_splitApplySuccessFlatJSONFilesCreated(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	srcMem := testutil.NewMemSourceOpener()
	outMem := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	// Content before the first "^---$" line is the first section (001), then 002, 003.
	doc := "a\n---\nb\n---\nc\n"
	if err := afero.WriteFile(srcMem.Fs, srcPath, []byte(doc), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	outDir := filepath.Join(root, "out")
	if err := outMem.Fs.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runSplit(
		[]string{"--source", "doc.md", "--delimiter", "^---$", "--output-dir", "out"},
		root,
		&stdout,
		&stderr,
		newTestOperationRunner(t, srcMem, outMem),
	)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr: want empty, got %q", stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v", err)
	}
	wantKeys := map[string]struct{}{"files_created": {}}
	if len(got) != len(wantKeys) {
		t.Fatalf("stdout JSON must stay flat keys=%v (%s)", gotKeysStable(got), stdout.String())
	}
	for k := range got {
		if _, ok := wantKeys[k]; !ok {
			t.Fatalf("unexpected key %q in %s", k, stdout.String())
		}
	}
	arr, ok := got["files_created"].([]any)
	if !ok {
		t.Fatalf("files_created type: got %T", got["files_created"])
	}
	if len(arr) != 3 {
		t.Fatalf("files_created len: got %d want 3 (%s)", len(arr), stdout.String())
	}
	wantPaths := []string{
		filepath.Join(outDir, "001.md"),
		filepath.Join(outDir, "002.md"),
		filepath.Join(outDir, "003.md"),
	}
	requireAbsoluteNativePathSlice(t, "files_created", wantPaths, arr)
}

//nolint:gocyclo,cyclop // JSON preview path mirrors apply matrix with orthogonal assertions.
func TestDispatchCLI_splitPreviewSuccessFlatJSONWouldCreate(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	srcMem := testutil.NewMemSourceOpener()
	outMem := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	if err := afero.WriteFile(srcMem.Fs, srcPath, []byte("x\n---\ny\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	outDir := filepath.Join(root, "out")
	if err := outMem.Fs.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runSplit(
		[]string{"--source", "doc.md", "--delimiter", "^---$", "--output-dir", "out", "--preview"},
		root,
		&stdout,
		&stderr,
		newTestOperationRunner(t, srcMem, outMem),
	)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr: want empty, got %q", stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v", err)
	}
	wantKeys := map[string]struct{}{"would_create": {}}
	if len(got) != len(wantKeys) {
		t.Fatalf("stdout JSON must stay flat keys=%v (%s)", gotKeysStable(got), stdout.String())
	}
	for k := range got {
		if _, ok := wantKeys[k]; !ok {
			t.Fatalf("unexpected key %q in %s", k, stdout.String())
		}
	}
	arr, ok := got["would_create"].([]any)
	if !ok {
		t.Fatalf("would_create type: got %T", got["would_create"])
	}
	if len(arr) != 2 {
		t.Fatalf("would_create len: got %d want 2 (%s)", len(arr), stdout.String())
	}
	wantPaths := []string{
		filepath.Join(outDir, "001.md"),
		filepath.Join(outDir, "002.md"),
	}
	requireAbsoluteNativePathSlice(t, "would_create", wantPaths, arr)
}

func gotKeysStable(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)

	return out
}

func TestDispatchCLI_splitDestinationExistsReportsDestination(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	if err := afero.WriteFile(src.Fs, srcPath, []byte("---\nbody\n"), 0o600); err != nil {
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

	code, stdout, gotErr := runSplitWithBuffers(
		t,
		[]string{"--source", "doc.md", "--delimiter", "^---$", "--output-dir", "out"},
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

//nolint:gocyclo,cyclop // JSON blocks apply matrix covers fences, previews, counts, and file paths.
func TestDispatchCLI_blocksCountsContentAndEmptyBlocks(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	srcMem := testutil.NewMemSourceOpener()
	outMem := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	src := "```go\nfmt.Println()\n```\n```go\n```\n"
	if err := afero.WriteFile(srcMem.Fs, srcPath, []byte(src), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	outDir := filepath.Join(root, "out")
	if err := outMem.Fs.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runBlocks(
		[]string{"--source", "doc.md", "--start-line", "^```go$", "--end-line", "^```$", "--output-dir", "out"},
		root,
		&stdout,
		&stderr,
		newTestOperationRunner(t, srcMem, outMem),
	)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr: want empty, got %q", stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v", err)
	}
	wantKeys := map[string]struct{}{
		"content_blocks_found": {},
		"empty_blocks_found":   {},
		"files_created":        {},
	}
	if len(got) != len(wantKeys) {
		t.Fatalf("stdout JSON must stay flat keys=%v (%s)", gotKeysStable(got), stdout.String())
	}
	for k := range got {
		if _, ok := wantKeys[k]; !ok {
			t.Fatalf("unexpected key %q in %s", k, stdout.String())
		}
	}
	if got["content_blocks_found"] != float64(1) {
		t.Fatalf("content_blocks_found: got %v want 1", got["content_blocks_found"])
	}
	if got["empty_blocks_found"] != float64(1) {
		t.Fatalf("empty_blocks_found: got %v want 1", got["empty_blocks_found"])
	}

	rawFiles, ok := got["files_created"].([]any)
	if !ok {
		t.Fatalf("files_created: want []any, got %T", got["files_created"])
	}
	if len(rawFiles) != 1 {
		t.Fatalf("files_created: want len 1, got %d (%v)", len(rawFiles), rawFiles)
	}
	wantPaths := []string{filepath.Join(outDir, "001.md")}
	requireAbsoluteNativePathSlice(t, "files_created", wantPaths, rawFiles)
}

func TestDispatchCLI_blocksApplySuccessOmitsEmptyBlocksWhenZeroFlatJSON(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	srcMem := testutil.NewMemSourceOpener()
	outMem := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	document := "```go\npackage p\n```\n"
	if err := afero.WriteFile(srcMem.Fs, srcPath, []byte(document), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	outDir := filepath.Join(root, "out")
	if err := outMem.Fs.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runBlocks(
		[]string{"--source", "doc.md", "--start-line", "^```go$", "--end-line", "^```$", "--output-dir", "out"},
		root,
		&stdout,
		&stderr,
		newTestOperationRunner(t, srcMem, outMem),
	)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr: want empty, got %q", stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v", err)
	}
	wantKeys := map[string]struct{}{
		"content_blocks_found": {},
		"files_created":        {},
	}
	if len(got) != len(wantKeys) {
		t.Fatalf("stdout JSON flat keys mismatch got=%v (%s)", gotKeysStable(got), stdout.String())
	}
	for k := range got {
		if _, ok := wantKeys[k]; !ok {
			t.Fatalf("unexpected key %q in %s", k, stdout.String())
		}
	}
	if _, has := got["empty_blocks_found"]; has {
		t.Fatalf("empty_blocks_found should be absent when zero: %s", stdout.String())
	}
}

//nolint:gocyclo // JSON blocks preview mirrors apply matrix with orthogonal assertions.
func TestDispatchCLI_blocksPreviewSuccessFlatJSON(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	srcMem := testutil.NewMemSourceOpener()
	outMem := testutil.NewMemOutputOpener()
	outDir := filepath.Join(root, "out")
	srcPath := filepath.Join(root, "doc.md")
	document := "```go\na\n```\n```go\n```\n"
	if err := afero.WriteFile(srcMem.Fs, srcPath, []byte(document), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runBlocks(
		[]string{"--source", "doc.md", "--start-line", "^```go$", "--end-line", "^```$", "--output-dir", "out", "--preview"},
		root,
		&stdout,
		&stderr,
		newTestOperationRunner(t, srcMem, outMem),
	)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr: want empty, got %q", stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v", err)
	}
	wantKeys := map[string]struct{}{
		"content_blocks_found": {},
		"empty_blocks_found":   {},
		"would_create":         {},
	}
	if len(got) != len(wantKeys) {
		t.Fatalf("stdout JSON flat keys mismatch got=%v (%s)", gotKeysStable(got), stdout.String())
	}
	for k := range got {
		if _, ok := wantKeys[k]; !ok {
			t.Fatalf("unexpected key %q in %s", k, stdout.String())
		}
	}

	would, ok := got["would_create"].([]any)
	if !ok || len(would) != 1 {
		t.Fatalf("would_create: got %#v (%s)", got["would_create"], stdout.String())
	}
	wantPaths := []string{filepath.Join(outDir, "001.md")}
	requireAbsoluteNativePathSlice(t, "would_create", wantPaths, would)
}
