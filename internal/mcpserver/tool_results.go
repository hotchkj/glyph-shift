package mcpserver

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

func toolResultFromStructured(value any, isError bool) (*mcp.CallToolResult, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
		StructuredContent: json.RawMessage(encoded),
		IsError:           isError,
	}, nil
}

func successToolResult(value any) (*mcp.CallToolResult, any, error) {
	result, err := toolResultFromStructured(value, false)
	if err != nil {
		return nil, nil, err
	}

	return result, nil, nil
}

func errorToolResult(payload any) (*mcp.CallToolResult, any, error) {
	result, err := toolResultFromStructured(payload, true)
	if err != nil {
		return nil, nil, err
	}

	return result, nil, nil
}

func (s *GlyphShiftServer) handleExtractTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ExtractInput,
) (*mcp.CallToolResult, any, error) {
	_, out, err := s.handleExtract(ctx, req, input)
	if err != nil {
		return errorToolResult(s.operationErrorMap(err, input.Source))
	}

	return successToolResult(out)
}

//nolint:gocritic // MCP AddTool invokes handlers with value inputs.
func (s *GlyphShiftServer) handleSplitTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input SplitInput,
) (*mcp.CallToolResult, any, error) {
	out, err := s.handleSplit(ctx, req, input)
	if err != nil {
		return errorToolResult(s.operationErrorMap(err, input.Source))
	}

	return successToolResult(out)
}

//nolint:gocritic // MCP AddTool invokes handlers with value inputs.
func (s *GlyphShiftServer) handleBlocksTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input BlocksInput,
) (*mcp.CallToolResult, any, error) {
	out, err := s.handleBlocks(ctx, req, input)
	if err != nil {
		if errors.Is(err, errBlocksOutputInvariantBroken) {
			outcome := pipeline.ErrorOutcome{
				Error:    "invalid_input",
				Hint:     errBlocksOutputInvariantBroken.Error(),
				ExitCode: pipeline.ExitValidation,
			}
			payload := pipeline.OperationErrorPayload(s.WorkspaceRoot, &outcome)

			return errorToolResult(payload)
		}

		return errorToolResult(s.operationErrorMap(err, input.Source))
	}

	return successToolResult(out)
}

func (s *GlyphShiftServer) handleTransformTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input TransformInput,
) (*mcp.CallToolResult, any, error) {
	_, out, err := s.handleTransform(ctx, req, input)
	if err != nil {
		return errorToolResult(s.operationErrorMap(err, input.Source))
	}

	return successToolResult(out)
}

func (s *GlyphShiftServer) operationErrorMap(err error, fallbackPath string) map[string]any {
	err = sanitizeError(err, s.WorkspaceRoot)

	preparedPrimary := ""
	var lexicalPrimary string

	var fallbackPrepErr error

	if fallbackPath != "" {
		preparedPath, prepareErr := pipeline.PreparePath(fallbackPath, s.WorkspaceRoot)

		switch {
		case prepareErr != nil:
			lexicalPrimary = fallbackPath
			fallbackPrepErr = prepareErr
		default:
			preparedPrimary = preparedPath
		}
	}

	outcome := pipeline.ClassifyOperationError(err, preparedPrimary)

	switch {
	case lexicalPrimary != "" && fallbackPrepErr != nil &&
		!pipeline.IsOperationOutcomeRenderableAtJSONEdge(s.WorkspaceRoot, &outcome):
		outcome = pipeline.ReconcileOperationOutcomeWithMCPToolPrimaryPathPrepFailure(
			&outcome,
			lexicalPrimary,
			fallbackPrepErr,
		)
	default:
	}

	return pipeline.OperationErrorPayload(s.WorkspaceRoot, &outcome)
}
