package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

// Phase 1 contract tests (JSON paths, unexpected trailing positionals, blocks invariants,
// preview collision propagation). Intended to fail against pre-contract implementation.

const (
	phase1SplitDelimiterRegexp   = "^---$"
	phase1SplitDocThreeSegmentMd = "a\n---\nb\n---\nc\n"
)

func requireAbsoluteNativePathEquals(t *testing.T, label, wantAbs, got string) {
	t.Helper()

	if !filepath.IsAbs(got) {
		t.Fatalf("%s: path must be absolute native, got %q", label, got)
	}

	if filepath.Clean(got) != filepath.Clean(wantAbs) {
		t.Fatalf("%s: got %q want %q (cleaned)", label, got, wantAbs)
	}
}

func requireAbsoluteNativePathSlice(t *testing.T, label string, want []string, arr []any) {
	t.Helper()

	if len(arr) != len(want) {
		t.Fatalf("%s: len got %d want %d (%v)", label, len(arr), len(want), arr)
	}

	for idx := range arr {
		gotPath, ok := arr[idx].(string)
		if !ok {
			t.Fatalf("%s[%d]: want string, got %T", label, idx, arr[idx])
		}

		//nolint:gosec // len(arr) and len(want) are asserted equal above; idx is bounded by len(arr).
		requireAbsoluteNativePathEquals(t, fmt.Sprintf("%s[%d]", label, idx), want[idx], gotPath)
	}
}

func phase1SortedMemRegularFileNamesInDir(t *testing.T, fs afero.Fs, dir string) []string {
	t.Helper()

	infos, err := afero.ReadDir(fs, dir)
	if err != nil {
		t.Fatalf("ReadDir %q: %v", dir, err)
	}

	out := make([]string, 0, len(infos))
	for _, info := range infos {
		if info.IsDir() {
			continue
		}

		out = append(out, info.Name())
	}

	slices.Sort(out)

	return out
}

func decodeSingleStderrErrorJSON(t *testing.T, stderr *bytes.Buffer) errorJSONOutput {
	t.Helper()

	assertExactlyOneConsumerCommandErrorJSONOptionalWant(t, stderr)

	text := strings.TrimSpace(stderr.String())
	var firstLine string

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		if isSlogStructuredLine(raw) {
			continue
		}

		firstLine = line

		break
	}

	if firstLine == "" {
		t.Fatalf("no consumer error JSON line in stderr: %q", text)
	}

	var got errorJSONOutput
	if err := json.Unmarshal([]byte(firstLine), &got); err != nil {
		t.Fatalf("decode error JSON: %v line=%q", err, firstLine)
	}

	return got
}

func assertExactlyOneConsumerCommandErrorJSONOptionalWant(t *testing.T, stderr *bytes.Buffer) {
	t.Helper()

	text := stderr.String()
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")

	var consumer []errorJSONOutput

	for _, line := range lines {
		if payload, ok := tryParseConsumerCommandErrorLine(line); ok {
			consumer = append(consumer, payload)
		}
	}

	if len(consumer) != 1 {
		t.Fatalf("want exactly 1 consumer command-error JSON object, got %d: stderr=%q",
			len(consumer), text)
	}
}

func TestPhase1_extractPreview_wouldCreate_isAbsoluteNativePreparedPath(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	srcMem := testutil.NewMemSourceOpener()
	outMem := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.txt")
	if err := afero.WriteFile(srcMem.Fs, srcPath, []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	wantDest := filepath.Clean(filepath.Join(root, "out.txt"))

	code, stdout, gotErr := runExtractWithBuffers(
		t,
		[]string{"--source", "doc.txt", "--lines", "1-2", "--destination", "out.txt", "--preview"},
		root,
		srcMem,
		outMem,
	)
	if code != 0 {
		t.Fatalf("exit code: got %d err=%#v stderr should be empty on success path", code, gotErr)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode stdout: %v", err)
	}

	wc, ok := got["would_create"].(string)
	if !ok {
		t.Fatalf("would_create type: got %T", got["would_create"])
	}

	requireAbsoluteNativePathEquals(t, "would_create", wantDest, wc)
}

func TestPhase1_splitApply_filesCreated_absoluteNativePaths(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	srcMem := testutil.NewMemSourceOpener()
	outMem := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	if err := afero.WriteFile(srcMem.Fs, srcPath, []byte(phase1SplitDocThreeSegmentMd), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	outDir := filepath.Join(root, "out")
	if err := outMem.Fs.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runSplit(
		[]string{"--source", "doc.md", "--delimiter", phase1SplitDelimiterRegexp, "--output-dir", "out"},
		root,
		&stdout,
		&stderr,
		newTestOperationRunner(t, srcMem, outMem),
	)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%q", code, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode stdout: %v", err)
	}

	arr, ok := got["files_created"].([]any)
	if !ok {
		t.Fatalf("files_created type: got %T", got["files_created"])
	}

	want := []string{
		filepath.Join(outDir, "001.md"),
		filepath.Join(outDir, "002.md"),
		filepath.Join(outDir, "003.md"),
	}
	requireAbsoluteNativePathSlice(t, "files_created", want, arr)
}

func TestPhase1_splitPreview_wouldCreate_absoluteNativePaths(t *testing.T) {
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
		[]string{"--source", "doc.md", "--delimiter", phase1SplitDelimiterRegexp, "--output-dir", "out", "--preview"},
		root,
		&stdout,
		&stderr,
		newTestOperationRunner(t, srcMem, outMem),
	)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%q", code, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode stdout: %v", err)
	}

	arr, ok := got["would_create"].([]any)
	if !ok {
		t.Fatalf("would_create type: got %T", got["would_create"])
	}

	want := []string{
		filepath.Join(outDir, "001.md"),
		filepath.Join(outDir, "002.md"),
	}
	requireAbsoluteNativePathSlice(t, "would_create", want, arr)
}

func TestPhase1_blocksApply_filesCreated_absoluteNativePaths(t *testing.T) {
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
		t.Fatalf("exit code: got %d stderr=%q", code, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode stdout: %v", err)
	}

	arr, ok := got["files_created"].([]any)
	if !ok {
		t.Fatalf("files_created type: got %T", got["files_created"])
	}

	want := []string{filepath.Join(outDir, "001.md")}
	requireAbsoluteNativePathSlice(t, "files_created", want, arr)
}

func TestPhase1_blocksPreview_wouldCreate_absoluteNativePaths(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	srcMem := testutil.NewMemSourceOpener()
	outMem := testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	document := "```go\na\n```\n```go\n```\n"
	if err := afero.WriteFile(srcMem.Fs, srcPath, []byte(document), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	outDir := filepath.Join(root, "out")
	if err := outMem.Fs.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
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

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode stdout: %v", err)
	}

	arr, ok := got["would_create"].([]any)
	if !ok {
		t.Fatalf("would_create type: got %T", got["would_create"])
	}

	want := []string{filepath.Join(outDir, "001.md")}
	requireAbsoluteNativePathSlice(t, "would_create", want, arr)
}

type impossibleBlocksInvariantRunner struct {
	errorContractRunner
}

func (impossibleBlocksInvariantRunner) RunBlocks(
	_ context.Context,
	_ pipeline.BlocksParams, //nolint:gocritic // hugeParam: BlocksRunner uses non-pointer param shape
) (pipeline.BlocksPipelineResult, error) {
	// Impossible: more content output files recorded than total blocks found.
	return pipeline.BlocksPipelineResult{
		BlocksFound: 0,
		Files: []string{
			filepath.Join(testCmdWorkspaceRoot(), "inv", "001.md"),
			filepath.Join(testCmdWorkspaceRoot(), "inv", "002.md"),
		},
	}, nil
}

// TestPhase1_blocksApply_pipelineInvariant_BlocksFound_lt_lenFiles_failsLoudly requires a
// non-success outcome when the runner reports more output paths than the recorded block
// count can explain (structural invariant violation). The public error/exit taxonomy does
// not yet assign a dedicated pipeline.Exit* value to this case without choosing how cmd
// surfaces it (for example internal_error vs validation). Until that contract exists, the
// test accepts any non-zero exit with empty stdout and a non-empty stderr error JSON, and
// rejects success-shaped stdout keys.
func TestPhase1_blocksApply_pipelineInvariant_BlocksFound_lt_lenFiles_failsLoudly(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := runBlocks(
		[]string{
			"--source", "doc.md",
			"--start-line", "^```go$",
			"--end-line", "^```$",
			"--output-dir", "out",
		},
		testCmdWorkspaceRoot(),
		&stdout,
		&stderr,
		impossibleBlocksInvariantRunner{},
	)

	if code == 0 {
		t.Fatalf("expected non-success exit when BlocksFound < len(Files); stdout=%q stderr=%q",
			stdout.String(), stderr.String())
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout: want empty on failure, got %q", stdout.String())
	}

	got := decodeSingleStderrErrorJSON(t, &stderr)
	if got.Error == "" {
		t.Fatalf("error field empty: %#v", got)
	}
}

func phase1WriteSplitPreviewCollisionFiles(t *testing.T, root string) (
	src *testutil.MemSourceOpener,
	out *testutil.MemOutputOpener,
	outDir, existingPath string,
) {
	t.Helper()

	src = testutil.NewMemSourceOpener()
	out = testutil.NewMemOutputOpener()
	srcPath := filepath.Join(root, "doc.md")
	if err := afero.WriteFile(src.Fs, srcPath, []byte("---\nbody\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outDir = filepath.Join(root, "out")
	if err := out.Fs.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	existingPath = filepath.Join(outDir, "001.md")
	if err := afero.WriteFile(out.Fs, existingPath, []byte("exists\n"), 0o600); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	return src, out, outDir, existingPath
}

func phase1AssertSplitPreviewDestinationExistsPayload(
	t *testing.T, code int, stdout string, gotErr *errorJSONOutput, existingPath string,
) {
	t.Helper()

	if code != exitDestExists {
		t.Fatalf("exit code: got %d want %d (destination_exists category)", code, exitDestExists)
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

func phase1AssertFilesystemUnchangedApartFromReads(
	t *testing.T,
	outFs afero.Fs,
	outDir, existingPath string,
	beforeNames []string,
	before001 []byte,
) {
	t.Helper()

	afterNames := phase1SortedMemRegularFileNamesInDir(t, outFs, outDir)
	if !slices.Equal(beforeNames, afterNames) {
		t.Fatalf("preview collision must not create or remove outputs: before %v after %v",
			beforeNames, afterNames)
	}

	after001, err := afero.ReadFile(outFs, existingPath)
	if err != nil {
		t.Fatalf("read existing after preview: %v", err)
	}

	if !bytes.Equal(after001, before001) {
		t.Fatalf("preview must not mutate existing destination: got %q want %q",
			string(after001), string(before001))
	}
}

func TestPhase1_splitPreview_destinationExists_stderrJSON_emptyStdout(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	src, out, outDir, existingPath := phase1WriteSplitPreviewCollisionFiles(t, root)

	beforeNames := phase1SortedMemRegularFileNamesInDir(t, out.Fs, outDir)
	before001, err := afero.ReadFile(out.Fs, existingPath)
	if err != nil {
		t.Fatalf("read existing before preview: %v", err)
	}

	code, stdout, gotErr := runSplitWithBuffers(
		t,
		[]string{"--source", "doc.md", "--delimiter", phase1SplitDelimiterRegexp, "--output-dir", "out", "--preview"},
		root,
		src,
		out,
	)
	phase1AssertSplitPreviewDestinationExistsPayload(t, code, stdout, &gotErr, existingPath)
	phase1AssertFilesystemUnchangedApartFromReads(t, out.Fs, outDir, existingPath, beforeNames, before001)
}
