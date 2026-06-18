package mcpserver

import (
	"context"
	"errors"
	"regexp"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

// errBlocksOutputInvariantBroken is returned when the pipeline reports more
// concrete output paths than delimiter blocks scanned (parity with CLI invalid_input).
var errBlocksOutputInvariantBroken = errors.New(
	"blocks reported more output files than delimiter blocks found; result is internally inconsistent. " +
		"Check --source, --start-line, and --end-line patterns, or retry after upgrading glyph-shift",
)

// BlocksInput is the MCP tool input for blocks.
type BlocksInput struct {
	Source            string   `json:"source" jsonschema:"Source file path"`
	StartLine         string   `json:"start_line" jsonschema:"Regex for start delimiter"`
	EndLine           string   `json:"end_line" jsonschema:"Regex for end delimiter"`
	OutputDir         string   `json:"output_dir" jsonschema:"Output directory"`
	Extension         string   `json:"extension,omitempty" jsonschema:"Output filename extension"`
	IncludeDelimiters bool     `json:"include_delimiters,omitempty" jsonschema:"Include delimiter lines in output"`
	Force             bool     `json:"force,omitempty" jsonschema:"Overwrite existing output files"`
	Mkdir             bool     `json:"mkdir,omitempty" jsonschema:"Create output directory if missing"`
	Names             []string `json:"names,omitempty" jsonschema:"Basenames for each non-empty written block"`
	MaxFiles          *int     `json:"max_files,omitempty" jsonschema:"Max blocks incl. empties (default 50 if omitted)"`
	Preview           bool     `json:"preview,omitempty" jsonschema:"Preview: counts and names without writing files"`
}

// BlocksOutput is the MCP tool output for blocks.
type BlocksOutput struct {
	ContentBlocksFound int       `json:"content_blocks_found"`
	EmptyBlocksFound   int       `json:"empty_blocks_found,omitempty"`
	FilesCreated       *[]string `json:"files_created,omitempty"`
	WouldCreate        *[]string `json:"would_create,omitempty"`
}

//nolint:gocritic // hugeParam: MCP AddTool uses concrete input struct
func (s *GlyphShiftServer) handleBlocks(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input BlocksInput,
) (BlocksOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return BlocksOutput{}, sanitizeError(err, s.WorkspaceRoot)
	}

	out, err := s.runBlocks(ctx, input)
	return out, err
}

//nolint:gocritic // tooManyResultsChecker: explicit decomposition for MCP tool validation
func (s *GlyphShiftServer) validateBlocksInput(input *BlocksInput) (
	srcPath, outDir string,
	startRE, endRE *regexp.Regexp,
	err error,
) {
	srcPath, err = s.resolveToolPath(input.Source)
	if err != nil {
		return srcPath, outDir,
			startRE, endRE,
			pipeline.WithPathRole(pipeline.PathRoleSrc, input.Source, err)
	}

	outDir, err = s.resolveToolPath(input.OutputDir)
	if err != nil {
		return srcPath, outDir,
			startRE, endRE,
			pipeline.WithPathRole(pipeline.PathRoleOutDir, input.OutputDir, err)
	}

	startRE, err = validate.ValidatePattern(input.StartLine)
	if err != nil {
		return srcPath, outDir, startRE, endRE, wrapValidatePatternAsFieldError("start_line", err)
	}

	endRE, err = validate.ValidatePattern(input.EndLine)
	if err != nil {
		return srcPath, outDir, startRE, endRE, wrapValidatePatternAsFieldError("end_line", err)
	}

	if input.MaxFiles != nil && *input.MaxFiles < 1 {
		return srcPath, outDir,
			startRE, endRE,
			pipeline.ErrMaxFilesAtLeastOne
	}

	return srcPath, outDir, startRE, endRE, nil
}

//nolint:gocritic // hugeParam: same concrete input as MCP AddTool handler
func (s *GlyphShiftServer) runBlocks(ctx context.Context, input BlocksInput) (BlocksOutput, error) {
	srcPath, outDir, startRE, endRE, err := s.validateBlocksInput(&input)
	if err != nil {
		return BlocksOutput{}, sanitizeError(err, s.WorkspaceRoot)
	}

	params := pipeline.BlocksParams{
		SrcPath:           srcPath,
		OutDir:            outDir,
		Root:              s.WorkspaceRoot,
		StartDelimiter:    startRE,
		EndDelimiter:      endRE,
		Naming:            fileops.Sequential,
		IncludeDelimiters: input.IncludeDelimiters,
		Extension:         input.Extension,
		Force:             input.Force,
		Mkdir:             input.Mkdir,
		Preview:           input.Preview,
		Names:             input.Names,
		MaxFiles:          mcpMaxFilesForPipeline(input.MaxFiles),
	}

	pres, err := s.runner.RunBlocks(ctx, params)
	if err != nil {
		return BlocksOutput{}, sanitizeError(err, s.WorkspaceRoot)
	}

	if len(pres.Files) > pres.BlocksFound {
		return BlocksOutput{}, errBlocksOutputInvariantBroken
	}

	contentBlocksFound := len(pres.Files)
	emptyBlocksFound := pres.BlocksFound - contentBlocksFound

	out := BlocksOutput{
		ContentBlocksFound: contentBlocksFound,
		EmptyBlocksFound:   emptyBlocksFound,
	}

	if input.Preview {
		out.WouldCreate = stringSlicePtr(pres.Files)
	} else {
		out.FilesCreated = stringSlicePtr(pres.Files)
	}

	return out, nil
}
