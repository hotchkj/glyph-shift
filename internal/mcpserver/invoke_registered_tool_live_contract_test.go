// User vision: registered MCP tools must emit structured payloads that satisfy declared output schemas
// and mirror text content blocks when exercised through tools/call (same path as transports use).
package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/afero"
)

const structuredLiveMarkdownGoFenceDocSample = "```go\npackage p\n```\n"

func liveStructuredMustSuccessToolEnvelope(tb testing.TB, result *mcp.CallToolResult) {
	tb.Helper()

	if result == nil || result.IsError {
		tb.Fatalf("want success envelope, got %+v", result)
	}
}

func runLiveStructuredExtractSuccess(t *testing.T, ctx context.Context, srv *GlyphShiftServer, srcFs afero.Fs) {
	t.Helper()

	mustWriteSrvSourceBytes(t, srv, srcFs, "source.txt", []byte("line1\nline2\nline3\n"))

	argsJSON := []byte(`{"source":"source.txt","lines":"1-2","destination":"output.txt"}`)
	result, err := srv.InvokeRegisteredTool(ctx, toolExtract, argsJSON)
	if err != nil {
		t.Fatalf("InvokeRegisteredTool: %v", err)
	}
	liveStructuredMustSuccessToolEnvelope(t, result)
	mustValidateStructuredContentAgainstSchema(t, toolExtract, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)
}

func runLiveStructuredSplitSuccess(t *testing.T, ctx context.Context, srv *GlyphShiftServer, srcFs, outFs afero.Fs) {
	t.Helper()

	srcPath, err := srv.validateToolPath("in.txt")
	if err != nil {
		t.Fatalf("validate source path: %v", err)
	}
	if writeErr := afero.WriteFile(srcFs, srcPath, []byte("hello\n---\nbody\n"), 0o600); writeErr != nil {
		t.Fatalf("write source: %v", writeErr)
	}

	outDir, err := srv.validateToolPath("out")
	if err != nil {
		t.Fatalf("validate out path: %v", err)
	}
	if mkdirErr := outFs.MkdirAll(outDir, 0o700); mkdirErr != nil {
		t.Fatalf("mkdir out: %v", mkdirErr)
	}

	argsJSON := []byte(`{"source":"in.txt","delimiter":"^---$","output_dir":"out","extension":".txt","mkdir":true}`)
	result, err := srv.InvokeRegisteredTool(ctx, toolSplit, argsJSON)
	if err != nil {
		t.Fatalf("InvokeRegisteredTool: %v", err)
	}

	liveStructuredMustSuccessToolEnvelope(t, result)

	mustValidateStructuredContentAgainstSchema(t, toolSplit, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	assertStringSlicePathsAbsoluteUnderResolvedDir(t, srv, "out", result.StructuredContent, "files_created")
}

func runLiveStructuredBlocksSuccess(t *testing.T, ctx context.Context, srv *GlyphShiftServer, srcFs, outFs afero.Fs) {
	t.Helper()

	srcPath, err := srv.validateToolPath("doc.md")
	if err != nil {
		t.Fatalf("validate source path: %v", err)
	}

	doc := structuredLiveMarkdownGoFenceDocSample
	if writeErr := afero.WriteFile(srcFs, srcPath, []byte(doc), 0o600); writeErr != nil {
		t.Fatalf("write source: %v", writeErr)
	}

	outDir, err := srv.validateToolPath("out")
	if err != nil {
		t.Fatalf("validate out path: %v", err)
	}
	if mkdirErr := outFs.MkdirAll(outDir, 0o700); mkdirErr != nil {
		t.Fatalf("mkdir out: %v", mkdirErr)
	}

	argsJSON, marshalErr := json.Marshal(BlocksInput{
		Source:    "doc.md",
		StartLine: "^```go$",
		EndLine:   "^```$",
		OutputDir: "out",
	})
	if marshalErr != nil {
		t.Fatalf("marshal blocks args: %v", marshalErr)
	}

	result, err := srv.InvokeRegisteredTool(ctx, toolBlocks, argsJSON)
	if err != nil {
		t.Fatalf("InvokeRegisteredTool: %v", err)
	}

	liveStructuredMustSuccessToolEnvelope(t, result)

	mustValidateStructuredContentAgainstSchema(t, toolBlocks, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)
}

func runLiveStructuredTransformApplySuccess(t *testing.T, ctx context.Context, srv *GlyphShiftServer) {
	t.Helper()

	argsJSON := []byte(`{"source":"doc.txt","line_endings":"lf"}`)
	result, err := srv.InvokeRegisteredTool(ctx, toolTransform, argsJSON)
	if err != nil {
		t.Fatalf("InvokeRegisteredTool: %v", err)
	}

	liveStructuredMustSuccessToolEnvelope(t, result)

	mustValidateStructuredContentAgainstSchema(t, toolTransform, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	raw, marshalStructuredErr := json.Marshal(result.StructuredContent)
	if marshalStructuredErr != nil {
		t.Fatalf("marshal structured: %v", marshalStructuredErr)
	}

	jsonRequireKeyPresent(t, json.RawMessage(raw), "changed")

	jsonRequireKeyAbsent(t, json.RawMessage(raw), "would_change")
}

func runLiveStructuredExtractMissingSourceEnvelope(t *testing.T, ctx context.Context, srv *GlyphShiftServer) {
	t.Helper()

	argsJSON := []byte(`{"source":"does-not-exist.txt","lines":"1-1","destination":"out.txt"}`)
	result, err := srv.InvokeRegisteredTool(ctx, toolExtract, argsJSON)
	if err != nil {
		t.Fatalf("InvokeRegisteredTool: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("want operation error envelope, got %+v", result)
	}

	mustValidateStructuredContentAgainstSchema(t, toolExtract, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != sourceNotFoundError {
		t.Fatalf("error: got %q want %s", opErrMustString(t, payload, "error"), sourceNotFoundError)
	}
}

func TestInvokeRegisteredTool_liveStructuredPayload_contract(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := testWorkspaceRoot()

	t.Run("extract_success", func(t *testing.T) {
		t.Parallel()

		srv, srcMem, _ := newServer(t, root)

		runLiveStructuredExtractSuccess(t, ctx, srv, srcMem.Fs)
	})

	t.Run("split_success", func(t *testing.T) {
		t.Parallel()

		srv, srcMem, outMem := newServer(t, root)

		runLiveStructuredSplitSuccess(t, ctx, srv, srcMem.Fs, outMem.Fs)
	})

	t.Run("blocks_success", func(t *testing.T) {
		t.Parallel()

		srv, srcMem, outMem := newServer(t, root)

		runLiveStructuredBlocksSuccess(t, ctx, srv, srcMem.Fs, outMem.Fs)
	})

	t.Run("transform_success_apply", func(t *testing.T) {
		t.Parallel()

		srv, _, _, stMem, sessMem := newUnitGlyphShiftServerParts(t, root)

		mustSeedTransformTwinFS(t, srv, stMem, sessMem, "doc.txt", []byte("a\n"))

		runLiveStructuredTransformApplySuccess(t, ctx, srv)
	})

	t.Run("extract_source_not_found_error", func(t *testing.T) {
		t.Parallel()

		srv, _, _ := newServer(t, root)

		runLiveStructuredExtractMissingSourceEnvelope(t, ctx, srv)
	})
}

func TestInvokeRegisteredTool_EmptyArgumentsDefaultToObject(t *testing.T) {
	t.Parallel()

	srv, _, _ := newServer(t, testWorkspaceRoot())
	result, err := srv.InvokeRegisteredTool(context.Background(), toolExtract, nil)
	if err != nil {
		t.Fatalf("InvokeRegisteredTool: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("want validation error envelope for empty default args, got %+v", result)
	}

	mustValidateStructuredContentAgainstSchema(t, toolExtract, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != invalidInputErrorName {
		t.Fatalf("error: got %q want %s", opErrMustString(t, payload, "error"), invalidInputErrorName)
	}
}
