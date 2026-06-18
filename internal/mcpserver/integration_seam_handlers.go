//go:build integration

package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// IntegrationValidateToolPath resolves rawPath under the workspace using production handler logic.
// For integrations/mcpserver tests only; not compiled into default builds.
func IntegrationValidateToolPath(s *GlyphShiftServer, rawPath string) (string, error) {
	return s.validateToolPath(rawPath)
}

// IntegrationHandleExtract forwards to the production extract handler (integration tests only).
func IntegrationHandleExtract(
	ctx context.Context,
	s *GlyphShiftServer,
	req *mcp.CallToolRequest,
	input ExtractInput,
) (*mcp.CallToolResult, ExtractOutput, error) {
	return s.handleExtract(ctx, req, input)
}

// IntegrationHandleSplit forwards to the production split handler (integration tests only).
func IntegrationHandleSplit(
	ctx context.Context,
	s *GlyphShiftServer,
	req *mcp.CallToolRequest,
	input *SplitInput,
) (SplitOutput, error) {
	return s.handleSplit(ctx, req, *input)
}

// IntegrationHandleBlocks forwards to the production blocks handler (integration tests only).
func IntegrationHandleBlocks(
	ctx context.Context,
	s *GlyphShiftServer,
	req *mcp.CallToolRequest,
	input *BlocksInput,
) (BlocksOutput, error) {
	return s.handleBlocks(ctx, req, *input)
}

// IntegrationHandleTransform forwards to the production transform handler (integration tests only).
func IntegrationHandleTransform(
	ctx context.Context,
	s *GlyphShiftServer,
	req *mcp.CallToolRequest,
	input TransformInput,
) (*mcp.CallToolResult, TransformOutput, error) {
	return s.handleTransform(ctx, req, input)
}
