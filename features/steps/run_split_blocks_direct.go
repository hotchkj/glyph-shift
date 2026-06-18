package steps

import (
	"context"
	"errors"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/fsnorm"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

func wrapPatternField(field string, err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, validate.ErrInvalidPattern) ||
		errors.Is(err, validate.ErrPatternTooLong) ||
		errors.Is(err, validate.ErrControlChar) {
		return &pipeline.PatternFieldError{Field: field, Cause: err}
	}

	return err
}

// splitDirectInput mirrors cmd split flags relevant to Layer 1 scenarios.
type splitDirectInput struct {
	Src               string
	Delimiter         string
	OutDir            string
	Preview           bool
	StripDelimiter    bool
	Force             bool
	Mkdir             bool
	MaxFilesSpecified bool
	MaxFiles          int
	NamesRaw          string
}

//nolint:gocritic // hugeParam: measurement bundle mirrors SplitPipelinePerfMeasurement grouping in testutil helpers.
func recordSplitMeasuredRun(
	tc *TestContext,
	logicalSrc string,
	srcPath string,
	meas testutil.SplitPipelinePerfMeasurement,
	res pipeline.SplitPipelineResult,
	runErr error,
) {
	tc.PerfSplitPipelineBySource[logicalSrc] = meas
	tc.LastPerfSplitPipeline = meas

	if runErr != nil {
		tc.LastSplitResult = nil
		tc.LastOperationError = runErr
		tc.LastOperationErrorFallbackPath = srcPath

		return
	}

	copyRes := res
	tc.LastSplitResult = &copyRes
	tc.LastOperationError = nil
	tc.LastOperationErrorFallbackPath = ""
}

//nolint:gocritic // hugeParam: measurement + result structs mirror BlocksPipeline grouping in harness.
func recordBlocksMeasuredRun(
	tc *TestContext,
	logicalSrc string,
	srcPath string,
	meas testutil.BlocksPipelinePerfMeasurement,
	res pipeline.BlocksPipelineResult,
	runErr error,
) {
	tc.PerfBlocksPipelineBySource[logicalSrc] = meas
	tc.LastPerfBlocksPipeline = meas

	if runErr != nil {
		tc.LastBlocksResult = nil
		tc.LastOperationError = runErr
		tc.LastOperationErrorFallbackPath = srcPath

		return
	}

	copyRes := res
	tc.LastBlocksResult = &copyRes
	tc.LastOperationError = nil
	tc.LastOperationErrorFallbackPath = ""
}

// runSplitWithInjectedPipelineRunner runs split through the injected operation runner harness.
func runSplitWithInjectedPipelineRunner(tc *TestContext, srcPath string, params *pipeline.SplitParams) error {
	runner, ctorErr := newFeatureOperationRunner(tc)
	if ctorErr != nil {
		return ctorErr
	}

	res, runErr := runner.RunSplit(context.Background(), *params)
	if runErr != nil {
		tc.LastOperationError = runErr
		tc.LastOperationErrorFallbackPath = srcPath

		return nil
	}

	copyRes := res
	tc.LastSplitResult = &copyRes
	tc.LastOperationError = nil
	tc.LastOperationErrorFallbackPath = ""

	return nil
}

// runSplitDirect runs the split pipeline like cmd.runSplitExecute without populating
// tc.Stdout, tc.Stderr, or tc.ExitCode.
//
// Inline sources written via harness Given steps populate tc.SourceFiles; when present for the
// split source basename, deterministic measurement instrumentation runs (matching extract perf path).
//
//nolint:gocritic // hugeParam: grouped input mirrors cmd splitFlagValues / SplitParams assembly.
func runSplitDirect(tc *TestContext, in splitDirectInput) error {
	tc.resetOperationResult()

	dir := fsnorm.DirNative(tc.Ws.Root())
	srcPath := fsnorm.ResolveUnderWorkspace(fsnorm.Canonical(in.Src), dir)
	outDirPath := fsnorm.ResolveUnderWorkspace(fsnorm.Canonical(in.OutDir), dir)

	maxFiles := pipeline.DefaultMaxFiles
	if in.MaxFilesSpecified {
		maxFiles = in.MaxFiles
	}

	if maxFiles < 1 {
		tc.LastOperationError = pipeline.ErrMaxFilesAtLeastOne
		tc.LastOperationErrorFallbackPath = ""

		return nil
	}

	re, patErr := validate.ValidatePattern(in.Delimiter)
	if patErr != nil {
		tc.LastOperationError = wrapPatternField("delimiter", patErr)
		tc.LastOperationErrorFallbackPath = srcPath

		return nil
	}

	namesList, nerr := pipeline.ParseCommaSeparatedNames(in.NamesRaw)
	if nerr != nil {
		tc.LastOperationError = nerr
		tc.LastOperationErrorFallbackPath = ""

		return nil
	}

	params := pipeline.SplitParams{
		SrcPath:        srcPath,
		OutDir:         outDirPath,
		Root:           dir,
		Delimiter:      re,
		Naming:         fileops.Sequential,
		StripDelimiter: in.StripDelimiter,
		Extension:      "",
		Force:          in.Force,
		Mkdir:          in.Mkdir,
		Preview:        in.Preview,
		MaxFiles:       maxFiles,
		Names:          namesList,
	}

	inlineBytes, haveInline := tc.SourceFiles[in.Src]
	if haveInline {
		srcOp := &testutil.CountingSourceOpener{Immutable: inlineBytes, AllowedPath: srcPath}
		resolver := tc.symlinkAwareResolver(testutil.NewMemPathResolverWithFS(tc.Ws.FS()))

		meas, res, runErr := testutil.MeasurePipelineSplitCountingSrcMem(
			context.Background(), srcOp, tc.Ws.FS(), resolver, params,
		)

		recordSplitMeasuredRun(tc, in.Src, srcPath, meas, res, runErr)

		return nil
	}

	return runSplitWithInjectedPipelineRunner(tc, srcPath, &params)
}

// blocksDirectInput mirrors cmd blocks flags relevant to Layer 1 scenarios.
type blocksDirectInput struct {
	Src               string
	Start             string
	End               string
	OutDir            string
	Preview           bool
	IncludeDelimiters bool
	Force             bool
	Mkdir             bool
	MaxFilesSpecified bool
	MaxFiles          int
	NamesRaw          string
}

// prepareBlocksDirect validates flags and builds blocks params.
// ok is false when runBlocksDirect should return immediately (tc already populated).
func prepareBlocksDirect(tc *TestContext, in *blocksDirectInput) (
	srcPath string,
	outDirPath string,
	params pipeline.BlocksParams,
	ok bool,
) {
	tc.resetOperationResult()

	dir := fsnorm.DirNative(tc.Ws.Root())
	srcPath = fsnorm.ResolveUnderWorkspace(fsnorm.Canonical(in.Src), dir)
	outDirPath = fsnorm.ResolveUnderWorkspace(fsnorm.Canonical(in.OutDir), dir)

	maxFiles := pipeline.DefaultMaxFiles
	if in.MaxFilesSpecified {
		maxFiles = in.MaxFiles
	}

	if maxFiles < 1 {
		tc.LastOperationError = pipeline.ErrMaxFilesAtLeastOne
		tc.LastOperationErrorFallbackPath = ""

		return "", "", pipeline.BlocksParams{}, false
	}

	startRE, patErr := validate.ValidatePattern(in.Start)
	if patErr != nil {
		tc.LastOperationError = wrapPatternField("start_line", patErr)
		tc.LastOperationErrorFallbackPath = srcPath

		return "", "", pipeline.BlocksParams{}, false
	}

	endRE, patErr := validate.ValidatePattern(in.End)
	if patErr != nil {
		tc.LastOperationError = wrapPatternField("end_line", patErr)
		tc.LastOperationErrorFallbackPath = srcPath

		return "", "", pipeline.BlocksParams{}, false
	}

	namesList, nerr := pipeline.ParseCommaSeparatedNames(in.NamesRaw)
	if nerr != nil {
		tc.LastOperationError = nerr
		tc.LastOperationErrorFallbackPath = ""

		return "", "", pipeline.BlocksParams{}, false
	}

	params = pipeline.BlocksParams{
		SrcPath:           srcPath,
		OutDir:            outDirPath,
		Root:              dir,
		StartDelimiter:    startRE,
		EndDelimiter:      endRE,
		Naming:            fileops.Sequential,
		IncludeDelimiters: in.IncludeDelimiters,
		Extension:         "",
		Force:             in.Force,
		Mkdir:             in.Mkdir,
		Preview:           in.Preview,
		MaxFiles:          maxFiles,
		Names:             namesList,
	}

	return srcPath, outDirPath, params, true
}

// runBlocksDirect runs the blocks pipeline like cmd.runBlocksExecute without populating
// tc.Stdout, tc.Stderr, or tc.ExitCode.
//
// Inline tc.SourceFiles[in.Src] measurement follows the same policy as runSplitDirect.
//
//nolint:gocritic // hugeParam: grouped input mirrors cmd blocksFlagValues / BlocksParams assembly.
func runBlocksDirect(tc *TestContext, in blocksDirectInput) error {
	srcPath, _, params, ready := prepareBlocksDirect(tc, &in)
	if !ready {
		return nil
	}

	inlineBytes, haveInline := tc.SourceFiles[in.Src]
	if haveInline {
		srcOp := &testutil.CountingSourceOpener{Immutable: inlineBytes, AllowedPath: srcPath}
		resolver := tc.symlinkAwareResolver(testutil.NewMemPathResolverWithFS(tc.Ws.FS()))

		meas, res, runErr := testutil.MeasurePipelineBlocksCountingSrcMem(
			context.Background(), srcOp, tc.Ws.FS(), resolver, params,
		)

		recordBlocksMeasuredRun(tc, in.Src, srcPath, meas, res, runErr)

		return nil
	}

	runner, ctorErr := newFeatureOperationRunner(tc)
	if ctorErr != nil {
		return ctorErr
	}

	res, runErr := runner.RunBlocks(context.Background(), params)
	if runErr != nil {
		tc.LastOperationError = runErr
		tc.LastOperationErrorFallbackPath = srcPath

		return nil
	}

	copyRes := res
	tc.LastBlocksResult = &copyRes
	tc.LastOperationError = nil
	tc.LastOperationErrorFallbackPath = ""

	return nil
}
