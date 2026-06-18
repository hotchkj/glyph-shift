package mcpserver

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

// testSplitMultiSectionSource matches internal/pipeline/run_split_test fixtures: enough
// delimiter matches that a low MaxFiles limit triggers ErrMaxFilesExceeded.
const testSplitMultiSectionSource = "---\nB\n---\nC\n---\nD\n"

const (
	missingTextRelPath    = "missing.txt"
	sourceNotFoundError   = "source_not_found"
	invalidInputErrorName = "invalid_input"
)

func seedSplitToolMemLayout(
	t *testing.T,
	srv *GlyphShiftServer,
	srcMem *testutil.MemSourceOpener,
	outMem *testutil.MemOutputOpener,
	sourceBody []byte,
) {
	t.Helper()

	srcPath, err := srv.validateToolPath("in.txt")
	if err != nil {
		t.Fatalf("validate source path: %v", err)
	}
	if writeErr := afero.WriteFile(srcMem.Fs, srcPath, sourceBody, 0o600); writeErr != nil {
		t.Fatalf("write source: %v", writeErr)
	}

	outDir, err := srv.validateToolPath("out")
	if err != nil {
		t.Fatalf("validate out path: %v", err)
	}
	if mkdirErr := outMem.Fs.MkdirAll(outDir, 0o700); mkdirErr != nil {
		t.Fatalf("mkdir out: %v", mkdirErr)
	}
}

func newServer(tb testing.TB, root string) (*GlyphShiftServer, *testutil.MemSourceOpener, *testutil.MemOutputOpener) {
	srv, src, out, _, _ := newUnitGlyphShiftServerParts(tb, root)

	return srv, src, out
}

func newUnitGlyphShiftServerParts(
	tb testing.TB,
	root string,
) (
	*GlyphShiftServer,
	*testutil.MemSourceOpener,
	*testutil.MemOutputOpener,
	*testutil.MemFileStater,
	*testutil.MemTestSession,
) {
	tb.Helper()

	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	st := testutil.NewMemFileStater()
	sess := testutil.NewMemPublishSessionForOutput(out)

	return mustNewGlyphShiftServer(tb, root, src, out, st, sess), src, out, st, sess
}

func TestSplitToolErrorCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	explicitZero := 0
	srv, _, _ := newServer(t, testWorkspaceRoot())

	result, _, err := srv.handleSplitTool(context.Background(), nil, SplitInput{
		Source:    "in.txt",
		Delimiter: "^a$",
		OutputDir: "out",
		Mkdir:     true,
		MaxFiles:  intPtr(explicitZero),
	})
	if err != nil {
		t.Fatalf("handleSplitTool: %v", err)
	}
	if result == nil {
		t.Fatal("result must be non-nil")
	}
	if !result.IsError {
		t.Fatal("result IsError must be true")
	}

	mustValidateStructuredContentAgainstSchema(t, toolSplit, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != invalidInputErrorName {
		t.Fatalf("error: got %q want %s", opErrMustString(t, payload, "error"), invalidInputErrorName)
	}
	if opErrMustString(t, payload, "hint") != pipeline.ErrMaxFilesAtLeastOne.Error() {
		t.Fatalf("hint: got %q want %q", opErrMustString(t, payload, "hint"), pipeline.ErrMaxFilesAtLeastOne.Error())
	}
}

func TestSplitToolInvalidPatternCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, _, _ := newServer(t, testWorkspaceRoot())

	result, _, err := srv.handleSplitTool(context.Background(), nil, SplitInput{
		Source:    "in.txt",
		Delimiter: "[invalid",
		OutputDir: "out",
	})
	if err != nil {
		t.Fatalf("handleSplitTool: %v", err)
	}
	if result == nil {
		t.Fatal("result must be non-nil")
	}
	if !result.IsError {
		t.Fatal("result IsError must be true")
	}

	mustValidateStructuredContentAgainstSchema(t, toolSplit, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != "invalid_pattern" {
		t.Fatalf("error: got %q want invalid_pattern", opErrMustString(t, payload, "error"))
	}
}

func TestSplitToolNoDelimiterMatchCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, srcMem, _ := newServer(t, testWorkspaceRoot())

	srcPath, err := srv.validateToolPath("doc.txt")
	if err != nil {
		t.Fatalf("validate source path: %v", err)
	}
	if writeErr := afero.WriteFile(srcMem.Fs, srcPath, []byte("plain\ntext\n"), 0o600); writeErr != nil {
		t.Fatalf("write source: %v", writeErr)
	}

	result, _, err := srv.handleSplitTool(context.Background(), nil, SplitInput{
		Source:    "doc.txt",
		Delimiter: "^---$",
		OutputDir: "out",
		Mkdir:     true,
	})
	if err != nil {
		t.Fatalf("handleSplitTool: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected non-nil IsError result")
	}

	mustValidateStructuredContentAgainstSchema(t, toolSplit, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != "no_delimiter_match" {
		t.Fatalf("error: got %q want no_delimiter_match", opErrMustString(t, payload, "error"))
	}
	wantSrc := mustToolPath(t, srv, "doc.txt")
	src := opErrMustString(t, payload, "src")
	if filepath.Clean(src) != filepath.Clean(wantSrc) {
		t.Fatalf("Source: got %q want absolute native workspace path %q", src, wantSrc)
	}
	if !filepath.IsAbs(src) {
		t.Fatalf("src must be absolute native path per contract, got %q", src)
	}
}

func TestSplitToolSourceNotFoundCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, _, _ := newServer(t, testWorkspaceRoot())

	result, _, err := srv.handleSplitTool(context.Background(), nil, SplitInput{
		Source:    missingTextRelPath,
		Delimiter: "^---$",
		OutputDir: "out",
		Mkdir:     true,
	})
	if err != nil {
		t.Fatalf("handleSplitTool: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected non-nil IsError result")
	}

	mustValidateStructuredContentAgainstSchema(t, toolSplit, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != sourceNotFoundError {
		t.Fatalf("error: got %q want source_not_found", opErrMustString(t, payload, "error"))
	}
	wantSrc := mustToolPath(t, srv, missingTextRelPath)
	src := opErrMustString(t, payload, "src")
	if filepath.Clean(src) != filepath.Clean(wantSrc) {
		t.Fatalf("Source: got %q want absolute native workspace path %q", src, wantSrc)
	}
	if !filepath.IsAbs(src) {
		t.Fatalf("src must be absolute native path per contract, got %q", src)
	}
}

func TestSplitToolMaxFilesExceededCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, srcMem, outMem := newServer(t, testWorkspaceRoot())

	seedSplitToolMemLayout(t, srv, srcMem, outMem, []byte(testSplitMultiSectionSource))

	maxFiles := 2
	result, _, err := srv.handleSplitTool(context.Background(), nil, SplitInput{
		Source:    "in.txt",
		Delimiter: "^---$",
		OutputDir: "out",
		Extension: ".txt",
		Mkdir:     true,
		MaxFiles:  &maxFiles,
	})
	if err != nil {
		t.Fatalf("handleSplitTool: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected non-nil IsError result")
	}

	mustValidateStructuredContentAgainstSchema(t, toolSplit, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != "max_files_exceeded" {
		t.Fatalf("error: got %q want max_files_exceeded", opErrMustString(t, payload, "error"))
	}
	if got := opErrMustInt64(t, payload, "max_files"); got != int64(maxFiles) {
		t.Fatalf("max_files: got %d want %d", got, maxFiles)
	}
	// "---\nB\n---\nC\n---\nD\n" with delimiter ^---$: bounded split would emit 3 output segments.
	if got := opErrMustInt64(t, payload, "would_create_count"); got != 3 {
		t.Fatalf("would_create_count: got %d want 3", got)
	}
}

func TestSplitToolPatternTooLongCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, _, _ := newServer(t, testWorkspaceRoot())
	longPat := strings.Repeat("a", validate.MaxPatternLength+1)

	result, _, err := srv.handleSplitTool(context.Background(), nil, SplitInput{
		Source:    "in.txt",
		Delimiter: longPat,
		OutputDir: "out",
	})
	if err != nil {
		t.Fatalf("handleSplitTool: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected non-nil IsError result")
	}

	mustValidateStructuredContentAgainstSchema(t, toolSplit, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != "pattern_too_long" {
		t.Fatalf("error: got %q want pattern_too_long", opErrMustString(t, payload, "error"))
	}
}

func TestSplitToolControlCharsInDelimiterCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, _, _ := newServer(t, testWorkspaceRoot())

	result, _, err := srv.handleSplitTool(context.Background(), nil, SplitInput{
		Source:    "in.txt",
		Delimiter: "\x01bad",
		OutputDir: "out",
	})
	if err != nil {
		t.Fatalf("handleSplitTool: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected non-nil IsError result")
	}

	mustValidateStructuredContentAgainstSchema(t, toolSplit, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != "control_chars_in_input" {
		t.Fatalf("error: got %q want control_chars_in_input", opErrMustString(t, payload, "error"))
	}
	if opErrMustString(t, payload, "field") != "delimiter" {
		t.Fatalf("field: got %q want delimiter", opErrMustString(t, payload, "field"))
	}
}

func TestSplitToolSuccessStructuredContentMatchesOutputSchema(t *testing.T) {
	t.Parallel()

	srv, srcMem, outMem := newServer(t, testWorkspaceRoot())

	srcPath, err := srv.validateToolPath("in.txt")
	if err != nil {
		t.Fatalf("validate source path: %v", err)
	}
	if writeErr := afero.WriteFile(srcMem.Fs, srcPath, []byte("hello\n---\nbody\n"), 0o600); writeErr != nil {
		t.Fatalf("write source: %v", writeErr)
	}

	outDir, err := srv.validateToolPath("out")
	if err != nil {
		t.Fatalf("validate out path: %v", err)
	}
	if mkdirErr := outMem.Fs.MkdirAll(outDir, 0o700); mkdirErr != nil {
		t.Fatalf("mkdir out: %v", mkdirErr)
	}

	result, _, err := srv.handleSplitTool(context.Background(), nil, SplitInput{
		Source:    "in.txt",
		Delimiter: "^---$",
		OutputDir: "out",
		Extension: ".txt",
		Mkdir:     true,
	})
	if err != nil {
		t.Fatalf("handleSplitTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected success, isError=%v", result != nil && result.IsError)
	}

	mustValidateStructuredContentAgainstSchema(t, toolSplit, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)
	assertStringSlicePathsAbsoluteUnderResolvedDir(t, srv, "out", result.StructuredContent, "files_created")
}

func TestSplitToolPreviewStructuredContentMatchesOutputSchema(t *testing.T) {
	t.Parallel()

	srv, srcMem, _ := newServer(t, testWorkspaceRoot())

	srcPath, err := srv.validateToolPath("in.txt")
	if err != nil {
		t.Fatalf("validate source path: %v", err)
	}
	if writeErr := afero.WriteFile(srcMem.Fs, srcPath, []byte("hello\n---\nbody\n"), 0o600); writeErr != nil {
		t.Fatalf("write source: %v", writeErr)
	}

	result, _, err := srv.handleSplitTool(context.Background(), nil, SplitInput{
		Source:    "in.txt",
		Delimiter: "^---$",
		OutputDir: "out",
		Extension: ".txt",
		Preview:   true,
	})
	if err != nil {
		t.Fatalf("handleSplitTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected success, isError=%v", result != nil && result.IsError)
	}

	mustValidateStructuredContentAgainstSchema(t, toolSplit, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)
	assertStringSlicePathsAbsoluteUnderResolvedDir(t, srv, "out", result.StructuredContent, "would_create")
}

func TestBlocksToolNoBlocksFoundCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	root := testWorkspaceRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	st := testutil.NewMemFileStater()
	sess := testutil.NewMemPublishSessionForOutput(out)
	srv := mustNewGlyphShiftServer(t, root, src, out, st, sess)

	srcPath, err := srv.validateToolPath("in.txt")
	if err != nil {
		t.Fatalf("validate source path: %v", err)
	}
	if writeErr := afero.WriteFile(src.Fs, srcPath, []byte("plain\ntext\n"), 0o600); writeErr != nil {
		t.Fatalf("write source: %v", writeErr)
	}

	outDir, err := srv.validateToolPath("out")
	if err != nil {
		t.Fatalf("validate out path: %v", err)
	}
	if mkdirErr := out.Fs.MkdirAll(outDir, 0o700); mkdirErr != nil {
		t.Fatalf("mkdir out: %v", mkdirErr)
	}

	result, _, err := srv.handleBlocksTool(context.Background(), nil, BlocksInput{
		Source:    "in.txt",
		StartLine: "^```go$",
		EndLine:   "^```$",
		OutputDir: "out",
	})
	if err != nil {
		t.Fatalf("handleBlocksTool: %v", err)
	}
	if result == nil {
		t.Fatal("result must be non-nil")
	}
	if !result.IsError {
		t.Fatal("result IsError must be true")
	}

	mustValidateStructuredContentAgainstSchema(t, toolBlocks, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != "no_blocks_found" {
		t.Fatalf("error: got %q want no_blocks_found", opErrMustString(t, payload, "error"))
	}
}

func TestBlocksToolSuccessStructuredContentMatchesOutputSchema(t *testing.T) {
	t.Parallel()

	root := testWorkspaceRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	st := testutil.NewMemFileStater()
	sess := testutil.NewMemPublishSessionForOutput(out)
	srv := mustNewGlyphShiftServer(t, root, src, out, st, sess)

	srcPath, err := srv.validateToolPath("doc.md")
	if err != nil {
		t.Fatalf("validate source path: %v", err)
	}
	doc := "```go\npackage p\n```\n"
	if writeErr := afero.WriteFile(src.Fs, srcPath, []byte(doc), 0o600); writeErr != nil {
		t.Fatalf("write source: %v", writeErr)
	}

	outDir, err := srv.validateToolPath("out")
	if err != nil {
		t.Fatalf("validate out path: %v", err)
	}
	if mkdirErr := out.Fs.MkdirAll(outDir, 0o700); mkdirErr != nil {
		t.Fatalf("mkdir out: %v", mkdirErr)
	}

	result, _, err := srv.handleBlocksTool(context.Background(), nil, BlocksInput{
		Source:    "doc.md",
		StartLine: "^```go$",
		EndLine:   "^```$",
		OutputDir: "out",
	})
	if err != nil {
		t.Fatalf("handleBlocksTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected success, isError=%v", result != nil && result.IsError)
	}

	mustValidateStructuredContentAgainstSchema(t, toolBlocks, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)
	assertStringSlicePathsAbsoluteUnderResolvedDir(t, srv, "out", result.StructuredContent, "files_created")
}

func TestBlocksToolPreviewStructuredContentMatchesOutputSchema(t *testing.T) {
	t.Parallel()

	root := testWorkspaceRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	st := testutil.NewMemFileStater()
	sess := testutil.NewMemPublishSessionForOutput(out)
	srv := mustNewGlyphShiftServer(t, root, src, out, st, sess)

	srcPath, err := srv.validateToolPath("doc.md")
	if err != nil {
		t.Fatalf("validate source path: %v", err)
	}
	doc := "```go\npackage p\n```\n"
	if writeErr := afero.WriteFile(src.Fs, srcPath, []byte(doc), 0o600); writeErr != nil {
		t.Fatalf("write source: %v", writeErr)
	}

	result, _, err := srv.handleBlocksTool(context.Background(), nil, BlocksInput{
		Source:    "doc.md",
		StartLine: "^```go$",
		EndLine:   "^```$",
		OutputDir: "out",
		Preview:   true,
	})
	if err != nil {
		t.Fatalf("handleBlocksTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected success, isError=%v", result != nil && result.IsError)
	}

	mustValidateStructuredContentAgainstSchema(t, toolBlocks, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)
	assertStringSlicePathsAbsoluteUnderResolvedDir(t, srv, "out", result.StructuredContent, "would_create")
}

//nolint:gocyclo,cyclop // sequential assertions for JSON-RPC round-trip; splitting obscures the single scenario
func TestToolErrorSerializesAsJSONRPCResponse(t *testing.T) {
	t.Parallel()

	root := testWorkspaceRoot()
	payload, ferr := pipeline.FormatOperationErrorJSON(root, pipeline.ErrorOutcome{
		Error:    "no_delimiter_match",
		Src:      "doc.md",
		Hint:     "The delimiter pattern did not match any source lines.",
		ExitCode: pipeline.ExitValidation,
	})
	if ferr != nil {
		t.Fatalf("FormatOperationErrorJSON: %v", ferr)
	}

	wantSrc, werr := pipeline.PreparePath("doc.md", root)
	if werr != nil {
		t.Fatalf("PreparePath: %v", werr)
	}

	gotSrc, ok := payload["src"].(string)
	if !ok || filepath.Clean(gotSrc) != filepath.Clean(wantSrc) {
		t.Fatalf("expected src %q, got %v", wantSrc, payload["src"])
	}

	result, _, err := errorToolResult(payload)
	if err != nil {
		t.Fatalf("errorToolResult: %v", err)
	}

	mustValidateStructuredContentAgainstSchema(t, toolSplit, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	response := struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      int         `json:"id"`
		Result  interface{} `json:"result"`
	}{
		JSONRPC: "2.0",
		ID:      1,
		Result:  result,
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var got map[string]any
	if unmarshalErr := json.Unmarshal(encoded, &got); unmarshalErr != nil {
		t.Fatalf("unmarshal response: %v", unmarshalErr)
	}
	if got["jsonrpc"] != "2.0" {
		t.Fatalf("jsonrpc: got %v", got["jsonrpc"])
	}

	resultObj, ok := got["result"].(map[string]any)
	if !ok {
		t.Fatalf("result: got %T", got["result"])
	}
	if resultObj["isError"] != true {
		t.Fatalf("isError: got %v", resultObj["isError"])
	}
	structured, ok := resultObj["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent: got %T", resultObj["structuredContent"])
	}
	if structured["error"] != "no_delimiter_match" {
		t.Fatalf("structured error: got %v", structured["error"])
	}
}
