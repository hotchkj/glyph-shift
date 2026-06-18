package cmd

import (
	"context"
	"encoding/json"
	"io"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

func transformRunErrorExit(runErr error, filePath, workspaceRoot string, stderr io.Writer) int {
	return exitCodeForPipelineErr(runErr, filePath, workspaceRoot, stderr)
}

func exitForTransformSkipped(
	src, workspaceRoot string,
	stderr io.Writer,
	res *fileops.TransformFileResult,
) (code int, skipped bool) {
	if res == nil || !res.Skipped {
		return 0, false
	}

	switch res.SkipReason {
	case "binary":
		return exitCodeForPipelineErr(pipeline.ErrBinarySource, src, workspaceRoot, stderr), true
	case "no transform":
		return exitCodeForPipelineErr(pipeline.ErrNoTransformSpecified, src, workspaceRoot, stderr), true
	default:
		return exitCodeForPipelineErr(pipeline.ErrTransformSkippedUnknown, src, workspaceRoot, stderr), true
	}
}

func runTransformExecute(
	flags *transformFlagValues,
	dir string,
	stdout, stderr io.Writer,
	runner pipeline.Runner,
) int {
	srcPrepared, prepErr := pipeline.PreparePath(flags.source, dir)
	if prepErr != nil {
		wrapped := pipeline.WithPathRole(pipeline.PathRoleSrc, flags.source, prepErr)
		return exitCodeForPipelineErr(wrapped, flags.source, dir, stderr)
	}

	flags.source = srcPrepared

	opts, err := buildTransformOpts(flags)
	if err != nil {
		return exitCodeForPipelineErr(err, "", dir, stderr)
	}

	params := pipeline.TransformParams{
		FilePath: flags.source,
		Root:     dir,
		Opts:     opts,
		Yes:      !flags.preview,
	}

	out, runErr := runner.RunTransform(context.Background(), params)
	if runErr != nil {
		return transformRunErrorExit(runErr, flags.source, dir, stderr)
	}

	if skipCode, ok := exitForTransformSkipped(flags.source, dir, stderr, &out.Result); ok {
		return skipCode
	}

	var result any

	if flags.preview {
		result = buildTransformPreviewOutput(flags, out.Result)
	} else {
		result = buildTransformApplyOutput(flags, out.Result)
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")

	if encErr := enc.Encode(result); encErr != nil {
		writeFailedJSON(stderr, dir, encErr)

		return 1
	}

	return 0
}

//nolint:gocritic // hugeParam: grouped config mirrors preview/apply symmetry; pointer would not clarify ownership
func buildTransformApplyOutput(flags *transformFlagValues, result fileops.TransformFileResult) transformApplyOutput {
	applyOut := transformApplyOutput{Changed: result.WouldChange}

	if flags.lineEndings != "" {
		v := result.EndingsChanged
		applyOut.EndingsChanged = &v
		lfF := result.LFFound
		applyOut.LFFound = &lfF
		lfC := result.LFConverted
		applyOut.LFConverted = &lfC
		crF := result.CRFound
		applyOut.CRFound = &crF
		crC := result.CRConverted
		applyOut.CRConverted = &crC
		crlfF := result.CRLFFound
		applyOut.CRLFFound = &crlfF
		crlfC := result.CRLFConverted
		applyOut.CRLFConverted = &crlfC
	}

	if flags.trimTrailing {
		v := result.TrailingTrimmed
		applyOut.TrailingTrimmed = &v
	}

	if flags.finalNewline {
		v := result.FinalNewlineAdded
		applyOut.FinalNewlineAdded = &v
	}

	return applyOut
}

//nolint:gocritic // hugeParam: grouped config mirrors apply output; pointer would not clarify ownership
func buildTransformPreviewOutput(
	flags *transformFlagValues,
	result fileops.TransformFileResult,
) transformPreviewOutput {
	previewOut := transformPreviewOutput{WouldChange: result.WouldChange}

	if flags.lineEndings != "" {
		v := result.EndingsChanged
		previewOut.EndingsChanged = &v
		lfF := result.LFFound
		previewOut.LFFound = &lfF
		lfC := result.LFConverted
		previewOut.LFConverted = &lfC
		crF := result.CRFound
		previewOut.CRFound = &crF
		crC := result.CRConverted
		previewOut.CRConverted = &crC
		crlfF := result.CRLFFound
		previewOut.CRLFFound = &crlfF
		crlfC := result.CRLFConverted
		previewOut.CRLFConverted = &crlfC
	}

	if flags.trimTrailing {
		v := result.TrailingTrimmed
		previewOut.TrailingTrimmed = &v
	}

	if flags.finalNewline {
		v := result.FinalNewlineAdded
		previewOut.FinalNewlineNeeded = &v
	}

	return previewOut
}
