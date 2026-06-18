package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

// TransformInput is the MCP tool input for transform.
type TransformInput struct {
	Source       string `json:"source" jsonschema:"Source file path"`
	LineEndings  string `json:"line_endings,omitempty" jsonschema:"Target line endings: lf, crlf, or cr"`
	TrimTrailing bool   `json:"trim_trailing,omitempty" jsonschema:"Trim trailing spaces and tabs"`
	FinalNewline bool   `json:"final_newline,omitempty" jsonschema:"Ensure file ends with newline"`
	Preview      *bool  `json:"preview,omitempty" jsonschema:"Preview without writes; executes when omitted or false"`
}

// TransformOutput is the MCP tool output for transform.
// Pointers encode false and 0 when those values are required by the JSON contract.
type TransformOutput struct {
	Changed            *bool `json:"changed,omitempty"`
	WouldChange        *bool `json:"would_change,omitempty"`
	EndingsChanged     *int  `json:"endings_changed,omitempty"`
	LFFound            *int  `json:"lf_found,omitempty"`
	LFConverted        *int  `json:"lf_converted,omitempty"`
	CRFound            *int  `json:"cr_found,omitempty"`
	CRConverted        *int  `json:"cr_converted,omitempty"`
	CRLFFound          *int  `json:"crlf_found,omitempty"`
	CRLFConverted      *int  `json:"crlf_converted,omitempty"`
	TrailingTrimmed    *int  `json:"trailing_trimmed,omitempty"`
	FinalNewlineAdded  *bool `json:"final_newline_added,omitempty"`
	FinalNewlineNeeded *bool `json:"final_newline_needed,omitempty"`
}

func parseLineEndingTarget(raw string) (*fileops.LineEndingTarget, error) {
	switch raw {
	case "":
		return nil, nil
	case "lf":
		t := fileops.TargetLF

		return &t, nil
	case "crlf":
		t := fileops.TargetCRLF

		return &t, nil
	case "cr":
		t := fileops.TargetCR

		return &t, nil
	default:
		return nil, fmt.Errorf("%w: line-endings must be lf, crlf, or cr", pipeline.ErrInvalidLineEndings)
	}
}

func buildTransformOpts(input TransformInput) (fileops.TransformOptions, error) {
	le, err := parseLineEndingTarget(input.LineEndings)
	if err != nil {
		return fileops.TransformOptions{}, err
	}

	return fileops.TransformOptions{
		LineEndings:  le,
		TrimTrailing: input.TrimTrailing,
		FinalNewline: input.FinalNewline,
	}, nil
}

func transformOptsSpecified(input TransformInput) bool {
	return input.LineEndings != "" || input.TrimTrailing || input.FinalNewline
}

func transformApplyMode(input TransformInput) bool {
	if input.Preview != nil {
		return !*input.Preview
	}
	return true
}

func (s *GlyphShiftServer) validateTransformInput(
	ctx context.Context,
	input TransformInput,
) (string, fileops.TransformOptions, error) {
	if err := ctx.Err(); err != nil {
		return "", fileops.TransformOptions{}, err
	}

	opts, err := buildTransformOpts(input)
	if err != nil {
		return "", fileops.TransformOptions{}, err
	}

	if !transformOptsSpecified(input) {
		return "", fileops.TransformOptions{}, fmt.Errorf("%w", pipeline.ErrNoTransformSpecified)
	}

	srcPath, err := s.resolveToolPath(input.Source)
	if err != nil {
		return "", fileops.TransformOptions{}, err
	}

	return srcPath, opts, nil
}

func (s *GlyphShiftServer) handleTransform(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input TransformInput,
) (*mcp.CallToolResult, TransformOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	srcPath, opts, err := s.validateTransformInput(ctx, input)
	if err != nil {
		return nil, TransformOutput{}, sanitizeError(err, s.WorkspaceRoot)
	}

	if err := ctx.Err(); err != nil {
		return nil, TransformOutput{}, sanitizeError(err, s.WorkspaceRoot)
	}

	apply := transformApplyMode(input)

	params := pipeline.TransformParams{
		FilePath: srcPath,
		Root:     s.WorkspaceRoot,
		Opts:     opts,
		Yes:      apply,
	}

	result, runErr := s.runner.RunTransform(ctx, params)
	if runErr != nil {
		return nil, TransformOutput{}, sanitizeError(runErr, s.WorkspaceRoot)
	}
	if skipErr := transformSkippedError(&result.Result); skipErr != nil {
		return nil, TransformOutput{}, sanitizeError(skipErr, s.WorkspaceRoot)
	}

	out := transformOutputForInput(&input, apply, &result.Result)

	return nil, out, nil
}

func transformSkippedError(result *fileops.TransformFileResult) error {
	if !result.Skipped {
		return nil
	}

	switch result.SkipReason {
	case "binary":
		return fmt.Errorf("%w", pipeline.ErrBinarySource)
	case "no transform":
		return fmt.Errorf("%w", pipeline.ErrNoTransformSpecified)
	default:
		return pipeline.ErrTransformSkippedUnknown
	}
}

func transformOutputForInput(input *TransformInput, apply bool, result *fileops.TransformFileResult) TransformOutput {
	out := TransformOutput{}

	if apply {
		out.Changed = boolPtr(result.WouldChange)

		if input.FinalNewline {
			out.FinalNewlineAdded = boolPtr(result.FinalNewlineAdded)
		}
	} else {
		out.WouldChange = boolPtr(result.WouldChange)

		if input.FinalNewline {
			out.FinalNewlineNeeded = boolPtr(result.FinalNewlineAdded)
		}
	}

	if input.LineEndings != "" {
		out.EndingsChanged = intPtr(result.EndingsChanged)
		out.LFFound = intPtr(result.LFFound)
		out.LFConverted = intPtr(result.LFConverted)
		out.CRFound = intPtr(result.CRFound)
		out.CRConverted = intPtr(result.CRConverted)
		out.CRLFFound = intPtr(result.CRLFFound)
		out.CRLFConverted = intPtr(result.CRLFConverted)
	}

	if input.TrimTrailing {
		out.TrailingTrimmed = intPtr(result.TrailingTrimmed)
	}

	return out
}
