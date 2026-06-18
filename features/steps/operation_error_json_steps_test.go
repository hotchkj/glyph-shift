package steps

import (
	"errors"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

const (
	wantEstablishedOperation  = "operation"
	wantEstablishedMCPContent = "MCP content"
)

func TestEstablishedOperationErrorJSONSources_ignoresSuccessShapedMCPStructured(t *testing.T) {
	t.Parallel()

	tc := NewTestContext()
	defer tc.Cleanup()

	tc.LastOperationError = pipeline.ErrNoTransformSpecified
	tc.MCPStructuredContent = map[string]interface{}{"would_create_count": float64(2)}
	tc.MCPError = nil

	got := establishedOperationErrorJSONSources(tc)
	if len(got) != 1 || got[0] != wantEstablishedOperation {
		t.Fatalf("want single operation source, got %#v", got)
	}
}

func TestEstablishedOperationErrorJSONSources_mcpContentTextWhenStructuredNotOpError(t *testing.T) {
	t.Parallel()

	tc := NewTestContext()
	defer tc.Cleanup()

	tc.MCPContentJSON = map[string]interface{}{
		"error": "invalid_input",
		"hint":  "x",
	}
	tc.MCPError = nil
	tc.MCPStructuredContent = map[string]interface{}{"ok": true}

	got := establishedOperationErrorJSONSources(tc)
	if len(got) != 1 || got[0] != wantEstablishedMCPContent {
		t.Fatalf("want MCP content only, got %#v", got)
	}
}

func TestResolveOmittedOperationErrorJSON_ambiguousOperationAndStderr(t *testing.T) {
	t.Parallel()

	tc := NewTestContext()
	defer tc.Cleanup()

	tc.LastOperationError = pipeline.ErrNoTransformSpecified
	tc.Stderr = `{"error":"invalid_input","hint":"from stderr"}`

	_, err := resolveOmittedOperationErrorJSON(tc)
	if err == nil {
		t.Fatal("expected error when both operation and CLI stderr JSON are established")
	}
	if !errors.Is(err, errOperationErrorJSONSourceAmbiguous) {
		t.Fatalf("want %v, got %v", errOperationErrorJSONSourceAmbiguous, err)
	}
}

func TestResolveOmittedOperationErrorJSON_singleOperation(t *testing.T) {
	t.Parallel()

	tc := NewTestContext()
	defer tc.Cleanup()

	tc.LastOperationError = pipeline.ErrDestinationExists
	tc.LastOperationErrorFallbackPath = tc.Ws.Join("exists.txt")

	payload, perr := resolveOmittedOperationErrorJSON(tc)
	if perr != nil {
		t.Fatal(perr)
	}
	ev, ok := payload["error"].(string)
	if !ok || ev != "destination_exists" {
		t.Fatalf("want destination_exists, got %#v", payload)
	}
}
