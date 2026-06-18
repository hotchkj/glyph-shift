package mcpserver

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mustValidatedOperationErrorMap decodes MCP structured operation-error payloads as map[string]any,
// rejects unknown/unexpected JSON keys implicitly (contract validation uses exact key sets per sentinel),
// and enforces docs/glyph-shift-json-contract.md operation-error shape via ValidateOperationErrorPayload.
//
// The decoder enables UseNumber so integer contract fields survive as json.Number and can be asserted
// with opErrMustInt64 without silent float truncation surprises for large values.
func mustValidatedOperationErrorMap(t *testing.T, structuredContent any) map[string]any {
	t.Helper()

	if structuredContent == nil {
		t.Fatal("structuredContent is nil")
	}

	encoded, err := json.Marshal(structuredContent)
	if err != nil {
		t.Fatalf("marshal structuredContent for operation error: %v", err)
	}

	dec := json.NewDecoder(bytes.NewReader(encoded))
	dec.UseNumber()

	var payloadMap map[string]any
	if decodeErr := dec.Decode(&payloadMap); decodeErr != nil {
		t.Fatalf("decode operation error map: %v", decodeErr)
	}
	if payloadMap == nil {
		t.Fatal("operation error JSON decoded to nil map")
	}

	if validateErr := pipeline.ValidateOperationErrorPayload(payloadMap); validateErr != nil {
		t.Fatalf("operation error payload contract: %v\njson=%s", validateErr, encoded)
	}

	return payloadMap
}

func opErrMustString(t *testing.T, payload map[string]any, key string) string {
	t.Helper()

	rawVal, ok := payload[key]
	if !ok {
		t.Fatalf("operation error map missing string key %q (have keys: %v)", key, sortedMapKeysLite(payload))
	}

	str, ok := rawVal.(string)
	if !ok {
		t.Fatalf("operation error key %q: want string, got %T %#v", key, rawVal, rawVal)
	}

	return str
}

// opErrMustInt64 parses JSON numbers emitted as int-ish values (stdlib float64, json.Number from
// Decode with UseNumber, or small Whole integers as int/int64).
func opErrMustInt64(t *testing.T, payload map[string]any, key string) int64 {
	t.Helper()

	rawVal, ok := payload[key]
	if !ok {
		t.Fatalf("operation error map missing numeric key %q (have keys: %v)", key, sortedMapKeysLite(payload))
	}

	switch typed := rawVal.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case json.Number:
		i64, err := typed.Int64()
		if err != nil {
			t.Fatalf("%s: json.Number parse int: %v", key, err)
		}

		return i64
	default:
		t.Fatalf("operation error key %q: want JSON number-compatible type, got %T %#v", key, rawVal, rawVal)
		return 0
	}
}

func sortedMapKeysLite(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)

	return out
}

// mustAssertToolStructuredJSONMatchesText asserts the MCP tool result publishes the same operation
// JSON in structuredContent as in the text content block (glyph-shift-json-contract.md MCP section).
func mustAssertToolStructuredJSONMatchesText(t *testing.T, result *mcp.CallToolResult) {
	t.Helper()

	if result == nil {
		t.Fatal("nil CallToolResult")
	}

	if len(result.Content) != 1 {
		t.Fatalf("expected exactly one text content entry, got %d", len(result.Content))
	}

	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok || tc == nil {
		t.Fatalf("content[0] type: got %T", result.Content[0])
	}

	structBytes, marshalErr := json.Marshal(result.StructuredContent)
	if marshalErr != nil {
		t.Fatalf("marshal structured payload: %v", marshalErr)
	}

	var structuredAny any
	if unmarshalStructuredErr := json.Unmarshal(structBytes, &structuredAny); unmarshalStructuredErr != nil {
		t.Fatalf("unmarshal structured repr: %v", unmarshalStructuredErr)
	}

	var textAny any
	if unmarshalTextErr := json.Unmarshal([]byte(tc.Text), &textAny); unmarshalTextErr != nil {
		t.Fatalf("decode text JSON: %v", unmarshalTextErr)
	}

	if !reflect.DeepEqual(structuredAny, textAny) {
		t.Fatalf("structuredContent JSON != content text JSON\nstructured=%s\ntext=%s", structBytes, tc.Text)
	}
}

// --- Split path helpers scoped to MCP tests ---------------------------------

func absPathMustMatchToolPath(t *testing.T, srv *GlyphShiftServer, logical, abs, label string) {
	t.Helper()

	wantAbs := filepath.Clean(mustToolPath(t, srv, logical))
	if filepath.Clean(abs) != wantAbs {
		t.Fatalf("%s: got %q want absolute resolved workspace path %q", label, abs, wantAbs)
	}
	if !filepath.IsAbs(abs) {
		t.Fatalf("%s must be absolute native path per contract, got %q", label, abs)
	}
}

// mustOpErrOutDirMatchesPreparedLexical asserts structuredContent out_dir equals
// pipeline.PreparePath(lexicalRel, srv.WorkspaceRoot).
//
// lexicalRel may trip validateToolPath while still reflecting how contract paths render from JSON.
func mustOpErrOutDirMatchesPreparedLexical(
	t *testing.T, srv *GlyphShiftServer, payload map[string]any, lexicalRel string,
) {
	t.Helper()

	want, prepErr := pipeline.PreparePath(lexicalRel, srv.WorkspaceRoot)
	if prepErr != nil {
		t.Fatalf("PreparePath lexical %q: %v", lexicalRel, prepErr)
	}

	got := opErrMustString(t, payload, "out_dir")
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("out_dir: got %q want PreparePath(workspace, %q)=%q", got, lexicalRel, want)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("out_dir must be absolute native path per contract, got %q", got)
	}
}
