package mcpserver

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSplitToolOutDirTraversalCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, srcMem, _ := newServer(t, testWorkspaceRoot())
	mustWriteSrvSourceBytes(t, srv, srcMem.Fs, "in.txt", []byte("a\n"))

	// Lexical out-dir outside workspace root (matches dest-escape style in operation_error_path_roles_test).
	outLexical := filepath.Join("..", "..", "..", "..", "..", "..", "..", "..", "nested", "out-rel")
	result, _, err := srv.handleSplitTool(context.Background(), nil, SplitInput{
		Source:    "in.txt",
		Delimiter: "^---$",
		OutputDir: outLexical,
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
	if opErrMustString(t, payload, "error") != invalidInputErrorName {
		t.Fatalf("error: got %q want invalid_input", opErrMustString(t, payload, "error"))
	}
	mustOpErrOutDirMatchesPreparedLexical(t, srv, payload, outLexical)
}
