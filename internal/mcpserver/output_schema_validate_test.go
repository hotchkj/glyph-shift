package mcpserver

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

var resolvedOutputSchemaMapCache sync.Map

func validateStructuredContentAgainstOutputSchema(schema map[string]any, structured any) error {
	raw, err := json.Marshal(schema)
	if err != nil {
		return err
	}

	cacheKey := string(raw)

	rsAny, ok := resolvedOutputSchemaMapCache.Load(cacheKey)
	if !ok {
		rs, resolveErr := resolveOutputSchemaMap(schema)
		if resolveErr != nil {
			return resolveErr
		}

		rsAny, _ = resolvedOutputSchemaMapCache.LoadOrStore(cacheKey, rs)
	}

	rs := rsAny.(*jsonschema.Resolved)

	inst, err := structuredContentAsMap(structured)
	if err != nil {
		return err
	}

	if err := rs.Validate(inst); err != nil {
		return err
	}

	return nil
}

func mustValidateStructuredContentAgainstSchema(t *testing.T, toolName string, structured any) {
	t.Helper()

	if err := ValidateToolStructuredOutput(toolName, structured); err != nil {
		var raw []byte

		switch v := structured.(type) {
		case json.RawMessage:
			raw = v
		default:
			structuredPayload, mErr := json.Marshal(structured)
			if mErr != nil {
				t.Fatalf("%v\n(marshal structured for debug failed: %v)", err, mErr)

				return
			}

			raw = structuredPayload
		}

		t.Fatalf("structured content does not match outputSchema: %v\npayload: %s", err, string(raw))
	}
}

func assertOutputSchemaRejectsJSON(t *testing.T, schema map[string]any, raw string) {
	t.Helper()

	var structured any
	if err := json.Unmarshal([]byte(raw), &structured); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	if err := validateStructuredContentAgainstOutputSchema(schema, structured); err == nil {
		t.Fatalf("expected outputSchema rejection for payload: %s", raw)
	}
}

func TestValidateToolStructuredOutput_unknownTool(t *testing.T) {
	t.Parallel()

	err := ValidateToolStructuredOutput("not-a-tool", map[string]any{"ok": true})
	if !errors.Is(err, ErrUnknownToolForOutputSchema) {
		t.Fatalf("ValidateToolStructuredOutput() error = %v, want %v", err, ErrUnknownToolForOutputSchema)
	}
}

func TestValidateToolStructuredOutput_invalidStructuredJSON(t *testing.T) {
	t.Parallel()

	err := ValidateToolStructuredOutput(toolExtract, json.RawMessage(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid structured JSON")
	}
}

func TestValidateToolStructuredOutput_rejectsSchemaViolation(t *testing.T) {
	t.Parallel()

	err := ValidateToolStructuredOutput(toolExtract, map[string]any{"surprise": true})
	if err == nil {
		t.Fatal("expected schema validation error")
	}
}

func TestValidateToolStructuredOutput_acceptsRawMessageSuccess(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{"error":"source_not_found","src":"/w/missing.md","hint":"Check path."}`)
	if err := ValidateToolStructuredOutput(toolExtract, raw); err != nil {
		t.Fatalf("ValidateToolStructuredOutput() error = %v", err)
	}
}

// TestAdvertisedToolOutputSchema_RejectsUnexpectedArgumentDocumentsHostBoundaries ensures the JSON Schema
// registered as MCP tool outputSchema does not admit unexpected_argument diagnostics (CLI `argument`
// or MCP decode `field`). Hosts reject invalid tool arguments at the protocol layer; those payloads
// are still valid operation errors from InvokeRegisteredTool and pass ValidateOperationErrorPayload.
func TestAdvertisedToolOutputSchema_RejectsUnexpectedArgumentDocumentsHostBoundaries(t *testing.T) {
	t.Parallel()

	tools := []string{toolExtract, toolSplit, toolBlocks, toolTransform}
	schemas := make(map[string]map[string]any, len(tools))
	for _, name := range tools {
		s, err := ToolOutputSchema(name)
		if err != nil {
			t.Fatalf("ToolOutputSchema(%q): %v", name, err)
		}

		schemas[name] = s
	}

	payloads := []struct {
		name string
		raw  string
	}{
		{
			name: "unexpected_argument_with_field",
			raw: `{"error":"unexpected_argument","field":"surplus_property",` +
				`"hint":"Accepted MCP fields include src and dest."}`,
		},
		{
			name: "unexpected_argument_with_argument",
			raw: `{"error":"unexpected_argument","argument":"extra",` +
				`"hint":"Trailing tokens are rejected on the CLI."}`,
		},
	}

	for _, toolName := range tools {
		for _, p := range payloads {
			t.Run(toolName+"/"+p.name, func(t *testing.T) {
				t.Parallel()

				assertOutputSchemaRejectsJSON(t, schemas[toolName], p.raw)
			})
		}
	}
}
