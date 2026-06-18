package mcpserver

import (
	"context"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

func TestMCPPathPreparationPreservesPathWhitespace(t *testing.T) {
	t.Parallel()

	root := testWorkspaceRoot()
	srv, _, _ := newServer(t, root)

	got, err := srv.resolveToolPath(" spaced.txt ")
	if err != nil {
		t.Fatalf("resolveToolPath: %v", err)
	}

	want, prepErr := pipeline.PreparePath(" spaced.txt ", root)
	if prepErr != nil {
		t.Fatalf("PreparePath want: %v", prepErr)
	}
	if got != want {
		t.Fatalf("path: got %q want %q", got, want)
	}
}

func TestSplitToolEmptyDelimiterUsesInvalidPatternFieldError(t *testing.T) {
	t.Parallel()

	srv, _, _ := newServer(t, testWorkspaceRoot())

	result, _, err := srv.handleSplitTool(context.Background(), nil, SplitInput{
		Source:    "doc.txt",
		Delimiter: "",
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
	assertInvalidPatternFieldPayload(t, payload, "delimiter")
}

func TestBlocksToolEmptyStartOrEndUsesInvalidPatternFieldError(t *testing.T) {
	t.Parallel()

	srv, _, _ := newServer(t, testWorkspaceRoot())

	for _, tt := range []struct {
		name      string
		startLine string
		endLine   string
	}{
		{name: "start_line", startLine: "", endLine: "^```$"},
		{name: "end_line", startLine: "^```go$", endLine: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, _, err := srv.handleBlocksTool(context.Background(), nil, BlocksInput{
				Source:    "doc.md",
				StartLine: tt.startLine,
				EndLine:   tt.endLine,
				OutputDir: "out",
			})
			if err != nil {
				t.Fatalf("handleBlocksTool: %v", err)
			}
			if result == nil || !result.IsError {
				t.Fatal("expected non-nil IsError result")
			}

			mustValidateStructuredContentAgainstSchema(t, toolBlocks, result.StructuredContent)
			mustAssertToolStructuredJSONMatchesText(t, result)

			payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
			assertInvalidPatternFieldPayload(t, payload, tt.name)
		})
	}
}

func assertInvalidPatternFieldPayload(t *testing.T, payload map[string]any, wantField string) {
	t.Helper()

	if opErrMustString(t, payload, "error") != "invalid_pattern" {
		t.Fatalf("error: got %q want invalid_pattern", opErrMustString(t, payload, "error"))
	}
	if opErrMustString(t, payload, "field") != wantField {
		t.Fatalf("field: got %q want %s", opErrMustString(t, payload, "field"), wantField)
	}
	if opErrMustString(t, payload, "hint") != validate.ErrEmptyRegexpPattern.Error() {
		t.Fatalf("hint: got %q want %q", opErrMustString(t, payload, "hint"), validate.ErrEmptyRegexpPattern.Error())
	}
}
