package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/linparse"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

// ExtractInput is the MCP tool input for extract.
type ExtractInput struct {
	Source      string `json:"source" jsonschema:"Source file path relative to workspace root"`
	Lines       string `json:"lines" jsonschema:"Line range e.g. 45-55, 95-, -10"`
	Destination string `json:"destination" jsonschema:"Destination file path relative to workspace root"`
	Force       bool   `json:"force,omitempty" jsonschema:"Overwrite existing destination"`
	Append      bool   `json:"append,omitempty" jsonschema:"Append to existing destination"`
	Mkdir       bool   `json:"mkdir,omitempty" jsonschema:"Create destination parent directories"`
	Preview     bool   `json:"preview,omitempty" jsonschema:"Report line count and destination without writing"`
}

// ExtractOutput is the MCP tool output for extract.
// Pointer fields separate apply vs preview shapes; non-nil pointers encode 0 and "" in JSON.
type ExtractOutput struct {
	LinesExtracted    *int    `json:"lines_extracted,omitempty"`
	WouldExtractLines *int    `json:"would_extract_lines,omitempty"`
	WouldCreate       *string `json:"would_create,omitempty"`
}

func (s *GlyphShiftServer) handleExtract(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input ExtractInput,
) (*mcp.CallToolResult, ExtractOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, ExtractOutput{}, sanitizeError(err, s.WorkspaceRoot)
	}

	srcPath, err := s.resolveToolPath(input.Source)
	if err != nil {
		return nil, ExtractOutput{},
			sanitizeError(pipeline.WithPathRole(pipeline.PathRoleSrc, input.Source, err), s.WorkspaceRoot)
	}

	destPath, err := s.resolveToolPath(input.Destination)
	if err != nil {
		return nil, ExtractOutput{},
			sanitizeError(pipeline.WithPathRole(pipeline.PathRoleDest, input.Destination, err), s.WorkspaceRoot)
	}

	start, end, perr := linparse.ParseCLIRange(input.Lines)
	if perr != nil {
		return nil, ExtractOutput{}, sanitizeError(linparse.NewLineRangeParseError(perr), s.WorkspaceRoot)
	}

	params := pipeline.ExtractParams{
		SrcPath:  srcPath,
		DestPath: destPath,
		Root:     s.WorkspaceRoot,
		Lines:    fileops.LineRange{Start: start, End: end},
		Force:    input.Force,
		Append:   input.Append,
		Mkdir:    input.Mkdir,
		Preview:  input.Preview,
	}

	res, err := s.runner.RunExtract(ctx, params)
	if err != nil {
		return nil, ExtractOutput{}, sanitizeError(err, s.WorkspaceRoot)
	}

	var out ExtractOutput

	if input.Preview {
		out.WouldExtractLines = intPtr(res.LinesExtracted)
		out.WouldCreate = stringPtr(res.WouldCreatePath)
	} else {
		out.LinesExtracted = intPtr(res.LinesExtracted)
	}

	return nil, out, nil
}
