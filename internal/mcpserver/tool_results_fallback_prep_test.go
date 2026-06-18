package mcpserver

import (
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

func operationErrorHintQuotedPairs(t *testing.T, hint string) map[string]string {
	t.Helper()

	hintPairMap, err := pipeline.ParseClassificationDiagnosticQuotedPairs(hint)
	if err != nil {
		t.Fatalf("parse diagnostic hint: %v", err)
	}

	return hintPairMap
}

func TestOperationErrorMap_prepare_primary_path_failure_reconcile(t *testing.T) {
	t.Parallel()

	srv, _, _ := newServer(t, testWorkspaceRoot())

	lexPrimary := filepath.Join(srv.WorkspaceRoot, string([]byte{'z', 0x00})+"nul-suffix.bin")

	if _, err := pipeline.PreparePath(lexPrimary, srv.WorkspaceRoot); err == nil {
		t.Skip("PreparePath unexpectedly accepted lexical primary containing NUL bytes; cannot exercise MCP reconcile branch")
	}

	payload := srv.operationErrorMap(pipeline.ErrBinarySource, lexPrimary)
	if validateErr := pipeline.ValidateOperationErrorPayload(payload); validateErr != nil {
		t.Fatalf("operation error map contract: %v", validateErr)
	}

	if payload["error"] != "internal_error" {
		t.Fatalf(`error sentinel: got %v`, payload["error"])
	}

	hintPayload := payload["hint"].(string)
	diagQuotedPairs := operationErrorHintQuotedPairs(t, hintPayload)

	if got := diagQuotedPairs["_tag"]; got != pipeline.TagMCPToolPrimaryPathPrepFailure {
		t.Fatalf("_tag=%q", got)
	}

	if _, hasSrcPlaceholder := diagQuotedPairs["src"]; hasSrcPlaceholder {
		t.Fatalf("expected internal_error base variant without stray src placeholder, got hint=%s", hintPayload)
	}
}
