// User vision: MCP tool calls should be validated through the registered server path, matching the
// protocol surface used by stdin transports and in-memory clients.
package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// InvokeRegisteredTool calls a tool by name through tools/call against [GlyphShiftServer.NewMCPServer]'s
// registrations, using an in-memory MCP transport.
func (s *GlyphShiftServer) InvokeRegisteredTool(
	ctx context.Context,
	toolName string,
	arguments json.RawMessage,
) (*mcp.CallToolResult, error) {
	srv := s.NewMCPServer()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ss, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ss.Close() }()

	client := mcp.NewClient(&mcp.Implementation{Name: "glyph-shift", Version: s.Version}, nil)

	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cs.Close() }()

	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}

	return cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	})
}
