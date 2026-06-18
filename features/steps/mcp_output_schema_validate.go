package steps

import (
	"github.com/hotchkj/glyph-shift/internal/mcpserver"
)

// ValidateMCPStructuredContentAgainstToolDeclaredOutputSchema verifies structuredContent against
// the same JSON Schema map the MCP server registers as the tool outputSchema. BDD-only.
func ValidateMCPStructuredContentAgainstToolDeclaredOutputSchema(toolName string, structured any) error {
	return mcpserver.ValidateToolStructuredOutput(toolName, structured)
}
