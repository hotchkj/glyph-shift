package mcpserver

import (
	"errors"
	"fmt"
)

// ErrUnknownToolForOutputSchema signals that ToolOutputSchema does not recognize toolName.
var ErrUnknownToolForOutputSchema = errors.New("unknown MCP tool for output schema")

// ToolOutputSchema returns the JSON Schema map used as the MCP tool outputSchema declaration
// for supported glyph-shift MCP tools ("extract", "split", "blocks", "transform").
func ToolOutputSchema(toolName string) (map[string]any, error) {
	switch toolName {
	case toolExtract:
		return extractOutputSchema(), nil
	case toolSplit:
		return splitOutputSchema(), nil
	case toolBlocks:
		return blocksOutputSchema(), nil
	case toolTransform:
		return transformOutputSchema(), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownToolForOutputSchema, toolName)
	}
}
