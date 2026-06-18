package steps

import (
	"context"
	"fmt"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/fsnorm"
	"github.com/hotchkj/glyph-shift/internal/linparse"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
)

// runExtractDirect executes the extract pipeline like cmd.runExtractExecute without
// going through CLI dispatch (DispatchCLI or the RunProductionCLI/DispatchProductionCLI chain).
// It does not write tc.Stdout, tc.Stderr, or tc.ExitCode.
//
// options may include stepFlagPreview (--preview) for preview mode and/or at most one
// Gherkin extract option phrase ("overwrite", "append", "create directories").
//
// Returns an error only when the step cannot run (invalid option combination). Pipeline
// failures and line-range parse errors are stored on tc.LastOperationError; success
// stores a copy of the result in tc.LastExtractResult.
func runExtractDirect(tc *TestContext, lines, src, dst string, options ...string) error {
	tc.resetOperationResult()

	preview, withPhrase, optErr := extractDirectParsedOptions(options)
	if optErr != nil {
		return optErr
	}

	force, appendMode, mkdir, phraseErr := extractDirectPhraseFlags(withPhrase)
	if phraseErr != nil {
		return phraseErr
	}

	start, end, parseErr := linparse.ParseCLIRange(lines)
	if parseErr != nil {
		tc.LastOperationError = linparse.NewLineRangeParseError(parseErr)
		tc.LastOperationErrorFallbackPath = ""

		return nil
	}

	dir := fsnorm.DirNative(tc.Ws.Root())

	srcPath := fsnorm.ResolveUnderWorkspace(fsnorm.Canonical(src), dir)
	destCanonical := fsnorm.Canonical(dst)
	destPath := fsnorm.ResolveUnderWorkspace(destCanonical, dir)

	params := pipeline.ExtractParams{
		SrcPath:  srcPath,
		DestPath: destPath,
		Root:     dir,
		Lines:    fileops.LineRange{Start: start, End: end},
		Force:    force,
		Append:   appendMode,
		Mkdir:    mkdir,
		Preview:  preview,
	}

	sourceBytes, haveSource := tc.SourceFiles[src]
	if !haveSource {
		return runExtractDirectWithoutInlineSource(tc, params, srcPath, destCanonical, preview)
	}

	return runExtractDirectWithInlineSource(
		tc, sourceBytes, params, src, srcPath, destCanonical, preview,
	)
}

func extractDirectParsedOptions(options []string) (preview bool, withPhrase string, err error) {
	for _, option := range options {
		switch option {
		case stepFlagPreview:
			preview = true
		default:
			if withPhrase != "" {
				return false, "", fmt.Errorf("%w", errTooManyExtractOptions)
			}

			withPhrase = option
		}
	}

	return preview, withPhrase, nil
}

func extractDirectPhraseFlags(withPhrase string) (force, appendMode, mkdir bool, err error) {
	if withPhrase == "" {
		return false, false, false, nil
	}

	return extractDirectOptionFlags(withPhrase)
}

//nolint:gocritic // hugeParam: Gherkin direct runner forwards CLI-shaped ExtractParams.
func runExtractDirectWithoutInlineSource(
	tc *TestContext,
	params pipeline.ExtractParams,
	srcPath, destCanonical string,
	preview bool,
) error {
	runner, ctorErr := newFeatureOperationRunner(tc)
	if ctorErr != nil {
		return ctorErr
	}

	res, runErr := runner.RunExtract(context.Background(), params)
	if runErr != nil {
		tc.LastOperationError = runErr
		tc.LastOperationErrorFallbackPath = srcPath

		return nil
	}

	applyExtractDirectSuccess(tc, res, preview, destCanonical, "", nil)

	return nil
}

//nolint:gocritic // hugeParam: Gherkin direct runner forwards CLI-shaped ExtractParams.
func runExtractDirectWithInlineSource(
	tc *TestContext,
	sourceBytes []byte,
	params pipeline.ExtractParams,
	logicalSrc, srcPath, destCanonical string,
	preview bool,
) error {
	srcOp := &testutil.CountingSourceOpener{Immutable: sourceBytes, AllowedPath: srcPath}
	resolver := tc.symlinkAwareResolver(testutil.NewMemPathResolverWithFS(tc.Ws.FS()))

	meas, res, runErr := testutil.MeasurePipelineExtractCountingSrcMem(
		context.Background(), srcOp, tc.Ws.FS(), resolver, params,
	)
	if runErr != nil {
		tc.LastOperationError = runErr
		tc.LastOperationErrorFallbackPath = srcPath

		return nil
	}

	applyExtractDirectSuccess(tc, res, preview, destCanonical, logicalSrc, &meas)

	return nil
}

func applyExtractDirectSuccess(
	tc *TestContext,
	res fileops.ExtractResult,
	preview bool,
	destCanonical, perfLogicalSrc string,
	meas *testutil.ExtractMeasurement,
) {
	copyRes := res
	tc.LastExtractResult = &copyRes
	tc.LastOperationError = nil
	tc.LastOperationErrorFallbackPath = ""

	if preview {
		tc.LastPreviewDestPath = destCanonical
	}

	if meas != nil && perfLogicalSrc != "" {
		tc.PerfExtractBySource[perfLogicalSrc] = *meas
		tc.LastPerfExtract = *meas
	}
}

func extractDirectOptionFlags(option string) (force, appendMode, mkdir bool, err error) {
	switch option {
	case stepOptOverwrite:
		return true, false, false, nil
	case "append":
		return false, true, false, nil
	case stepOptCreateDirectories:
		return false, false, true, nil
	default:
		return false, false, false, fmt.Errorf("%w: %q", errUnknownExtractOption, option)
	}
}
