package mcpserver

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func assertUnexpectedArgumentHint(t *testing.T, tool, hint string) {
	t.Helper()

	want := acceptedFieldsHintForTool(tool)
	if hint != want {
		t.Fatalf("unexpected_argument hint: got %q want %q", hint, want)
	}
}

func assertUnexpectedArgumentToolResult(t *testing.T, tool string, result *mcp.CallToolResult) map[string]any {
	t.Helper()

	if result == nil {
		t.Fatal("expected non-nil CallToolResult")
	}

	if !result.IsError {
		t.Fatalf("tool result must use isError: true for %s unexpected_argument failures", tool)
	}

	mustAssertToolStructuredJSONMatchesText(t, result)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != unexpectedArgumentSentinel {
		t.Fatalf("error sentinel: got %q want %s", opErrMustString(t, payload, "error"), unexpectedArgumentSentinel)
	}

	assertUnexpectedArgumentHint(t, tool, opErrMustString(t, payload, "hint"))

	return payload
}

func TestInvokeRegisteredTool_ExtractSurplusJSONPropertyUsesUnexpectedArgument(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	srv, srcMem, _ := newServer(t, testWorkspaceRoot())
	mustWriteSrvSourceBytes(t, srv, srcMem.Fs, "doc.txt", []byte("a\n"))

	argsJSON := []byte(`{"source":"doc.txt","lines":"1-1","destination":"out.txt","also_not_a_field":true}`)

	result, err := srv.InvokeRegisteredTool(ctx, toolExtract, argsJSON)
	if err != nil {
		t.Fatalf(
			"InvokeRegisteredTool must return in-band MCP tool error "+
				"(not Go decode error) for surplus properties: %v",
			err,
		)
	}

	if result == nil {
		t.Fatal("expected CallToolResult")
	}

	assertUnexpectedArgumentToolResult(t, toolExtract, result)
}

func TestInvokeRegisteredTool_ExtractJSONTypeMismatchUsesUnexpectedArgument(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	srv, _, _ := newServer(t, testWorkspaceRoot())

	argsJSON := []byte(`{"source":"doc.txt","lines":1,"destination":"out.txt"}`)

	result, err := srv.InvokeRegisteredTool(ctx, toolExtract, argsJSON)
	if err != nil {
		t.Fatalf(
			"InvokeRegisteredTool must return in-band MCP tool error "+
				"(not stdlib JSON type error) for argument-shape mismatch: %v",
			err,
		)
	}

	if result == nil {
		t.Fatal("expected CallToolResult")
	}

	assertUnexpectedArgumentToolResult(t, toolExtract, result)
}

// TestInvokeRegisteredTool_ExtractNonObjectTopLevelJSONUsesUnexpectedArgumentWithFieldJSON covers MCP decode
// failures where stdlib JSON cannot name a concrete member (whole-document / shape mismatch). In
// that case unexpected_argument uses logical field "json" (docs/glyph-shift-json-contract.md).
func TestInvokeRegisteredTool_ExtractNonObjectTopLevelJSONUsesUnexpectedArgumentWithFieldJSON(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	srv, _, _ := newServer(t, testWorkspaceRoot())

	argsJSON := []byte(`true`)

	result, err := srv.InvokeRegisteredTool(ctx, toolExtract, argsJSON)
	if err != nil {
		t.Fatalf("InvokeRegisteredTool must return in-band MCP tool error for non-object top-level JSON: %v", err)
	}

	if result == nil {
		t.Fatal("expected CallToolResult")
	}

	payload := assertUnexpectedArgumentToolResult(t, toolExtract, result)

	if opErrMustString(t, payload, "field") != "json" {
		t.Fatalf(
			"unexpected_argument.field: got %q want %q (no recoverable member path in decode error)",
			opErrMustString(t, payload, "field"),
			"json",
		)
	}
}

func TestInvokeRegisteredTool_SplitSurplusJSONPropertyUsesUnexpectedArgument(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	srv, srcMem, _ := newServer(t, testWorkspaceRoot())
	srcPath := mustToolPath(t, srv, "in.txt")
	mustWriteMem(t, srcMem.Fs, srcPath, []byte("hello\n---\nbody\n"))

	argsJSON := []byte(`{"source":"in.txt","delimiter":"^---$","output_dir":"out","extra_bravo_field":false}`)

	result, err := srv.InvokeRegisteredTool(ctx, toolSplit, argsJSON)
	if err != nil {
		t.Fatalf("InvokeRegisteredTool must return in-band MCP tool error for surplus properties: %v", err)
	}

	if result == nil {
		t.Fatal("expected CallToolResult")
	}

	assertUnexpectedArgumentToolResult(t, toolSplit, result)
}

func TestInvokeRegisteredTool_BlocksSurplusJSONPropertyUsesUnexpectedArgument(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	srv, _, _ := newServer(t, testWorkspaceRoot())

	argsJSON := []byte(`{"source":"doc.md","start_line":"^x$","end_line":"^y$","output_dir":"out",` +
		`"unknown_blocks_field":false}`)

	result, err := srv.InvokeRegisteredTool(ctx, toolBlocks, argsJSON)
	if err != nil {
		t.Fatalf("InvokeRegisteredTool must return in-band MCP tool error for surplus properties: %v", err)
	}

	if result == nil {
		t.Fatal("expected CallToolResult")
	}

	assertUnexpectedArgumentToolResult(t, toolBlocks, result)
}

func TestInvokeRegisteredTool_TransformSurplusJSONPropertyUsesUnexpectedArgument(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	srv, _, _, stMem, sessMem := newUnitGlyphShiftServerParts(t, testWorkspaceRoot())
	srcPath := mustToolPath(t, srv, "doc.txt")
	mustWriteMem(t, stMem.Fs, srcPath, []byte("a\r\n"))
	mustWriteMem(t, sessMem.Fs, srcPath, []byte("a\r\n"))

	argsJSON := []byte(`{"source":"doc.txt","line_endings":"lf","bonus_transform_hint":false}`)

	result, err := srv.InvokeRegisteredTool(ctx, toolTransform, argsJSON)
	if err != nil {
		t.Fatalf("InvokeRegisteredTool must return in-band MCP tool error for surplus properties: %v", err)
	}

	if result == nil {
		t.Fatal("expected CallToolResult")
	}

	assertUnexpectedArgumentToolResult(t, toolTransform, result)
}
