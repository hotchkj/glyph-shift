package mcpserver

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
)

var (
	resolvedOutputSchemas    map[string]*jsonschema.Resolved
	resolveOutputSchemasOnce sync.Once
	resolveOutputSchemasErr  error
)

func init() {
	resolveOutputSchemasOnce.Do(func() {
		resolvedOutputSchemas, resolveOutputSchemasErr = buildResolvedOutputSchemas()
	})
	if resolveOutputSchemasErr != nil {
		panic(fmt.Sprintf("mcpserver: resolve output schemas: %v", resolveOutputSchemasErr))
	}
}

func resolvedOutputSchema(toolName string) (*jsonschema.Resolved, error) {
	if resolveOutputSchemasErr != nil {
		return nil, resolveOutputSchemasErr
	}

	rs, ok := resolvedOutputSchemas[toolName]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownToolForOutputSchema, toolName)
	}

	return rs, nil
}

func buildResolvedOutputSchemas() (map[string]*jsonschema.Resolved, error) {
	builders := map[string]func() map[string]any{
		toolExtract:   extractOutputSchema,
		toolSplit:     splitOutputSchema,
		toolBlocks:    blocksOutputSchema,
		toolTransform: transformOutputSchema,
	}

	out := make(map[string]*jsonschema.Resolved, len(builders))
	for name, builder := range builders {
		rs, err := resolveOutputSchemaMap(builder())
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", name, err)
		}

		out[name] = rs
	}

	return out, nil
}

func resolveOutputSchemaMap(toolSchema map[string]any) (*jsonschema.Resolved, error) {
	raw, err := json.Marshal(toolSchema)
	if err != nil {
		return nil, fmt.Errorf("marshal output schema: %w", err)
	}

	var schema jsonschema.Schema
	if unmarshalErr := json.Unmarshal(raw, &schema); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal output schema: %w", unmarshalErr)
	}

	rs, resolveErr := schema.Resolve(nil)
	if resolveErr != nil {
		return nil, fmt.Errorf("resolve output schema: %w", resolveErr)
	}

	return rs, nil
}

// ValidateToolStructuredOutput checks structured MCP tool payload against the pre-resolved
// outputSchema registered for toolName.
func ValidateToolStructuredOutput(toolName string, structured any) error {
	rs, err := resolvedOutputSchema(toolName)
	if err != nil {
		return err
	}

	inst, err := structuredContentAsMap(structured)
	if err != nil {
		return err
	}

	if err := rs.Validate(inst); err != nil {
		return fmt.Errorf("MCP structuredContent does not conform to declared outputSchema: %w", err)
	}

	return nil
}

func structuredContentAsMap(structured any) (map[string]any, error) {
	var raw []byte

	switch v := structured.(type) {
	case json.RawMessage:
		raw = v
	default:
		b, err := json.Marshal(structured)
		if err != nil {
			return nil, fmt.Errorf("marshal structured content: %w", err)
		}

		raw = b
	}

	var inst map[string]any
	if err := json.Unmarshal(raw, &inst); err != nil {
		return nil, fmt.Errorf("unmarshal structured content for schema validation: %w", err)
	}

	return inst, nil
}
