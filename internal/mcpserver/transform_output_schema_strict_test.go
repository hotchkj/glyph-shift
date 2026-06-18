package mcpserver

import (
	"testing"
)

func TestTransformOutputSchema_DeclarationRejectsFinalNewlineNeededOnApplyShape(t *testing.T) {
	t.Parallel()

	schema := transformOutputSchema()

	illegalApply := map[string]any{
		"changed":              true,
		"final_newline_needed": true,
	}

	if err := validateStructuredContentAgainstOutputSchema(schema, illegalApply); err == nil {
		t.Fatal("strict transform outputSchema forbids preview-only field final_newline_needed in apply payloads " +
			"(see glyph-shift-json-contract.md mutually exclusive schemas)")
	}
}

func TestTransformOutputSchema_DeclarationRejectsFinalNewlineAddedOnPreviewShape(t *testing.T) {
	t.Parallel()

	schema := transformOutputSchema()

	illegalPreview := map[string]any{
		"would_change":        false,
		"final_newline_added": true,
	}

	if err := validateStructuredContentAgainstOutputSchema(schema, illegalPreview); err == nil {
		t.Fatal("strict transform MCP outputSchema must not allow apply-only field final_newline_added in preview payloads")
	}
}

func TestTransformOutputSchema_DeclarationRejectsPartialLineEndingBundleOnApply(t *testing.T) {
	t.Parallel()

	schema := transformOutputSchema()

	// When endings_changed appears, lf/cr/crlf counters must accompany it as one bundle (requested transform semantics).
	illegalPartial := map[string]any{
		"changed":         true,
		"endings_changed": 1,
	}

	if err := validateStructuredContentAgainstOutputSchema(schema, illegalPartial); err == nil {
		t.Fatal("current MCP outputSchema is overly permissive: partial line-ending metrics must not satisfy apply schema")
	}
}

func TestTransformOutputSchema_DeclarationRejectsPartialLineEndingBundleOnPreview(t *testing.T) {
	t.Parallel()

	schema := transformOutputSchema()

	illegalPartial := map[string]any{
		"would_change":    false,
		"endings_changed": 1,
	}

	if err := validateStructuredContentAgainstOutputSchema(schema, illegalPartial); err == nil {
		t.Fatal("current MCP outputSchema is overly permissive: partial line-ending metrics must not satisfy preview schema")
	}
}

func TestTransformOutputSchema_DeclarationRejectsChangedOnlyApplyShape(t *testing.T) {
	t.Parallel()

	schema := transformOutputSchema()

	illegal := map[string]any{
		"changed": true,
	}

	if err := validateStructuredContentAgainstOutputSchema(schema, illegal); err == nil {
		t.Fatal("strict transform outputSchema must reject apply payloads with only changed " +
			"(final-newline apply requires final_newline_added; trim requires trailing_trimmed; " +
			"line-endings require full counter bundle per glyph-shift-json-contract.md)")
	}
}

func TestTransformOutputSchema_DeclarationRejectsWouldChangeOnlyPreviewShape(t *testing.T) {
	t.Parallel()

	schema := transformOutputSchema()

	illegal := map[string]any{
		"would_change": false,
	}

	if err := validateStructuredContentAgainstOutputSchema(schema, illegal); err == nil {
		t.Fatal("strict transform outputSchema must reject preview payloads with only would_change " +
			"(preview final-newline requires final_newline_needed; trim requires trailing_trimmed; " +
			"line-endings require full counter bundle per glyph-shift-json-contract.md)")
	}
}

func TestTransformOutputSchema_DeclarationAcceptsFinalNewlineOnlyApplyDocShape(t *testing.T) {
	t.Parallel()

	schema := transformOutputSchema()
	legal := map[string]any{
		"changed":             true,
		"final_newline_added": true,
	}

	if err := validateStructuredContentAgainstOutputSchema(schema, legal); err != nil {
		t.Fatalf("expected doc apply final-newline-only shape to validate: %v", err)
	}
}

func TestTransformOutputSchema_DeclarationAcceptsFinalNewlineOnlyPreviewDocShape(t *testing.T) {
	t.Parallel()

	schema := transformOutputSchema()
	legal := map[string]any{
		"would_change":         false,
		"final_newline_needed": false,
	}

	if err := validateStructuredContentAgainstOutputSchema(schema, legal); err != nil {
		t.Fatalf("expected doc preview final-newline-only shape to validate: %v", err)
	}
}

func TestTransformOutputSchema_DeclarationAcceptsTrimOnlyApplyShape(t *testing.T) {
	t.Parallel()

	schema := transformOutputSchema()
	legal := map[string]any{
		"changed":          true,
		"trailing_trimmed": 0,
	}

	if err := validateStructuredContentAgainstOutputSchema(schema, legal); err != nil {
		t.Fatalf("expected trim-only apply shape (changed + trailing_trimmed) to validate: %v", err)
	}
}

func TestTransformOutputSchema_DeclarationAcceptsTrimOnlyPreviewShape(t *testing.T) {
	t.Parallel()

	schema := transformOutputSchema()
	legal := map[string]any{
		"would_change":     false,
		"trailing_trimmed": 0,
	}

	if err := validateStructuredContentAgainstOutputSchema(schema, legal); err != nil {
		t.Fatalf("expected trim-only preview shape (would_change + trailing_trimmed) to validate: %v", err)
	}
}
