package mcpserver

import (
	"context"
	"regexp"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

// SplitInput is the MCP tool input for split.
type SplitInput struct {
	Source         string   `json:"source" jsonschema:"Source file path"`
	Delimiter      string   `json:"delimiter" jsonschema:"Regex pattern for delimiter lines"`
	OutputDir      string   `json:"output_dir" jsonschema:"Output directory path"`
	Extension      string   `json:"extension,omitempty" jsonschema:"Output filename extension including dot"`
	StripDelimiter bool     `json:"strip_delimiter,omitempty" jsonschema:"Omit delimiter line from each section"`
	Force          bool     `json:"force,omitempty" jsonschema:"Overwrite existing output files"`
	Mkdir          bool     `json:"mkdir,omitempty" jsonschema:"Create output directory if missing"`
	Names          []string `json:"names,omitempty" jsonschema:"Output basenames per section"`
	MaxFiles       *int     `json:"max_files,omitempty" jsonschema:"Maximum sections (default 50 when omitted)"`
	Preview        bool     `json:"preview,omitempty" jsonschema:"Report output basenames without writing"`
}

// SplitOutput is the MCP tool output for split (Pointer slices encode empty [] not omitted).
type SplitOutput struct {
	FilesCreated *[]string `json:"files_created,omitempty"`
	WouldCreate  *[]string `json:"would_create,omitempty"`
}

//nolint:gocritic // tooManyResultsChecker: explicit decomposition for MCP tool validation
func (s *GlyphShiftServer) validateSplitInput(input *SplitInput) (
	srcPath, outDir string,
	re *regexp.Regexp,
	err error,
) {
	srcPath, err = s.resolveToolPath(input.Source)
	if err != nil {
		return srcPath, outDir,
			re,
			pipeline.WithPathRole(pipeline.PathRoleSrc, input.Source, err)
	}

	outDir, err = s.resolveToolPath(input.OutputDir)
	if err != nil {
		return srcPath, outDir,
			re,
			pipeline.WithPathRole(pipeline.PathRoleOutDir, input.OutputDir, err)
	}

	re, err = validate.ValidatePattern(input.Delimiter)
	if err != nil {
		return srcPath, outDir, re, wrapValidatePatternAsFieldError("delimiter", err)
	}

	if input.MaxFiles != nil && *input.MaxFiles < 1 {
		return srcPath, outDir,
			re,
			pipeline.ErrMaxFilesAtLeastOne
	}

	return srcPath, outDir, re, nil
}

//nolint:gocritic // hugeParam: MCP AddTool uses concrete input struct
func (s *GlyphShiftServer) handleSplit(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input SplitInput,
) (SplitOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return SplitOutput{}, sanitizeError(err, s.WorkspaceRoot)
	}

	srcPath, outDir, re, err := s.validateSplitInput(&input)
	if err != nil {
		return SplitOutput{}, sanitizeError(err, s.WorkspaceRoot)
	}

	params := pipeline.SplitParams{
		SrcPath:        srcPath,
		OutDir:         outDir,
		Root:           s.WorkspaceRoot,
		Delimiter:      re,
		Naming:         fileops.Sequential,
		StripDelimiter: input.StripDelimiter,
		Extension:      input.Extension,
		Force:          input.Force,
		Mkdir:          input.Mkdir,
		Preview:        input.Preview,
		Names:          input.Names,
		MaxFiles:       mcpMaxFilesForPipeline(input.MaxFiles),
	}

	pres, err := s.runner.RunSplit(ctx, params)
	if err != nil {
		return SplitOutput{}, sanitizeError(err, s.WorkspaceRoot)
	}

	var out SplitOutput

	if input.Preview {
		out.WouldCreate = stringSlicePtr(pres.Files)
	} else {
		out.FilesCreated = stringSlicePtr(pres.Files)
	}

	return out, nil
}
