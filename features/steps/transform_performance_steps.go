package steps

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cucumber/godog"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/fsnorm"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

var (
	errTransformPerfMissingPair        = errors.New("transform perf: missing measurement for one or both sources in pair")
	errTransformPerfMissingMeasurement = errors.New("transform perf: missing measurement for named source")
	errTransformRetainedHeapOverBudget = errors.New("transform retained heap exceeds paired small/large budget")
	errTransformPreviewTempCreates     = errors.New("transform preview recorded non-zero temp creates")
)

// transformPerfMeasure runs transform perf measurement; swapped in tests (no real pipeline / FS required when stubbed).
var transformPerfMeasure = defaultTransformPerfMeasure

func defaultTransformPerfMeasure(
	ctx context.Context,
	st pipeline.FileStater,
	resolver validate.PathResolver,
	session fileops.FileSession,
	params pipeline.TransformParams,
	measureTempCreates, preview bool,
) (testutil.TransformPipelinePerfMeasurement, pipeline.TransformPipelineResult, error) {
	if measureTempCreates && preview {
		return testutil.MeasurePipelineTransformRecordsPreviewWithoutWrites(ctx, st, resolver, session, params)
	}

	return testutil.MeasurePipelineTransformPeakHeap(ctx, st, resolver, session, params)
}

func transformPerfCRLFNumberedLines(lineCount int) []byte {
	var sb strings.Builder
	for i := 1; i <= lineCount; i++ {
		_, _ = fmt.Fprintf(&sb, "line %d\r\n", i)
	}

	return []byte(sb.String())
}

func transformPerfCRLFTrailingSpacesLines(lineCount int) []byte {
	var sb strings.Builder
	for i := 1; i <= lineCount; i++ {
		_, _ = fmt.Fprintf(&sb, "line %d  \r\n", i)
	}

	return []byte(sb.String())
}

func transformPerfCRLFNoFinalNewline(lineCount int) []byte {
	var sb strings.Builder
	for i := 1; i < lineCount; i++ {
		_, _ = fmt.Fprintf(&sb, "line %d\r\n", i)
	}

	_, _ = fmt.Fprintf(&sb, "line %d", lineCount)

	return []byte(sb.String())
}

func transformPerfPair(
	tc *TestContext,
	largeName, smallName string,
) (lg, sm testutil.TransformPipelinePerfMeasurement, err error) {
	lg, okL := tc.PerfTransformBySource[largeName]
	sm, okS := tc.PerfTransformBySource[smallName]
	if !okL || !okS {
		return lg, sm, fmt.Errorf("%w (large=%q ok=%v small=%q ok=%v)",
			errTransformPerfMissingPair, largeName, okL, smallName, okS)
	}

	return lg, sm, nil
}

func transformPerfRun(
	tc *TestContext,
	logicalName string,
	preview bool,
	opts fileops.TransformOptions,
	measureTempCreates bool,
) error {
	tc.resetOperationResult()

	dir := fsnorm.DirNative(tc.Ws.Root())
	filePath := fsnorm.ResolveUnderWorkspace(fsnorm.Canonical(logicalName), dir)

	mem := testutil.NewMemFileSession()
	mem.SetFs(tc.Ws.FS())

	st := testutil.NewMemFileStaterWithFS(mem.Fs)
	resolver := testutil.NewMemPathResolverWithFS(tc.Ws.FS())

	params := pipeline.TransformParams{
		FilePath: filePath,
		Root:     dir,
		Opts:     opts,
		Yes:      !preview,
	}

	meas, res, err := transformPerfMeasure(
		context.Background(), st, resolver, mem, params, measureTempCreates, preview,
	)
	if err != nil {
		tc.LastOperationError = err

		return err
	}

	tc.LastOperationError = nil

	copyRes := res
	tc.LastTransformResult = &copyRes

	tc.PerfTransformBySource[logicalName] = meas
	tc.LastPerfTransform = meas

	return nil
}

// RegisterTransformPerformance registers transform performance contract steps (suite: bdd_transform_performance).
func RegisterTransformPerformance(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(
		//nolint:lll // Godog scenario vocabulary
		`^transform performance CRLF sources "([^"]*)" \((\d+) lines\) and "([^"]*)" \((\d+) lines\)$`,
		func(smallName string, smallLines int, largeName string, largeLines int) error {
			if err := writeSourceFile(tc, smallName, transformPerfCRLFNumberedLines(smallLines)); err != nil {
				return err
			}

			return writeSourceFile(tc, largeName, transformPerfCRLFNumberedLines(largeLines))
		},
	)

	sc.Given(
		//nolint:lll // Godog scenario vocabulary
		`^transform performance CRLF sources with trailing spaces "([^"]*)" \((\d+) lines\) and "([^"]*)" \((\d+) lines\)$`,
		func(smallName string, smallLines int, largeName string, largeLines int) error {
			if err := writeSourceFile(tc, smallName, transformPerfCRLFTrailingSpacesLines(smallLines)); err != nil {
				return err
			}

			return writeSourceFile(tc, largeName, transformPerfCRLFTrailingSpacesLines(largeLines))
		},
	)

	sc.Given(
		//nolint:lll // Godog scenario vocabulary
		`^transform performance CRLF sources without final newline "([^"]*)" \((\d+) lines\) and "([^"]*)" \((\d+) lines\)$`,
		func(smallName string, smallLines int, largeName string, largeLines int) error {
			if err := writeSourceFile(tc, smallName, transformPerfCRLFNoFinalNewline(smallLines)); err != nil {
				return err
			}

			return writeSourceFile(tc, largeName, transformPerfCRLFNoFinalNewline(largeLines))
		},
	)

	lf := fileops.TargetLF

	sc.When(
		`^the caller previews transform with line-endings lf for performance source "([^"]*)"$`,
		func(name string) error {
			return transformPerfRun(tc, name, true, fileops.TransformOptions{LineEndings: &lf}, false)
		},
	)

	sc.When(
		`^the caller applies transform with line-endings lf for performance source "([^"]*)"$`,
		func(name string) error {
			return transformPerfRun(tc, name, false, fileops.TransformOptions{LineEndings: &lf}, false)
		},
	)

	sc.When(
		//nolint:lll // Godog scenario vocabulary
		`^the caller applies transform with line-endings lf and trim-trailing for performance source "([^"]*)"$`,
		func(name string) error {
			return transformPerfRun(tc, name, false, fileops.TransformOptions{LineEndings: &lf, TrimTrailing: true}, false)
		},
	)

	sc.When(
		//nolint:lll // Godog scenario vocabulary
		`^the caller applies transform with line-endings lf and final-newline for performance source "([^"]*)"$`,
		func(name string) error {
			return transformPerfRun(tc, name, false, fileops.TransformOptions{LineEndings: &lf, FinalNewline: true}, false)
		},
	)

	sc.When(
		//nolint:lll // Godog scenario vocabulary
		`^the caller previews transform with line-endings lf measuring publish temps for performance source "([^"]*)"$`,
		func(name string) error {
			return transformPerfRun(tc, name, true, fileops.TransformOptions{LineEndings: &lf}, true)
		},
	)

	registerTransformPerfThenRetainedHeap(sc, tc)
	registerTransformPerfThenPreviewTemp(sc, tc)
}

func registerTransformPerfThenRetainedHeap(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(
		`^transform retained heap for "([^"]*)" is no more than 2 times "([^"]*)"$`,
		func(largeName, smallName string) error {
			lg, sm, errPair := transformPerfPair(tc, largeName, smallName)
			if errPair != nil {
				return errPair
			}

			budget := testutil.StreamingBodyResidencyRetainedHeapBudget(sm.RetainedHeapAllocDelta)
			if lg.RetainedHeapAllocDelta <= budget {
				return nil
			}

			return fmt.Errorf(
				"%w: large=%q delta=%d budget=%d small=%q delta=%d ratio_cap=%d noise=%d",
				errTransformRetainedHeapOverBudget,
				largeName,
				lg.RetainedHeapAllocDelta,
				budget,
				smallName,
				sm.RetainedHeapAllocDelta,
				testutil.StreamingBodyRetainedHeapMaxLargeToSmallRatio,
				testutil.StreamingBodyResidencyRetainedHeapNoiseAllowance,
			)
		},
	)
}

func registerTransformPerfThenPreviewTemp(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(
		`^transform preview records zero temp creates for "([^"]*)"$`,
		func(name string) error {
			meas, ok := tc.PerfTransformBySource[name]
			if !ok {
				return fmt.Errorf("%w: %q", errTransformPerfMissingMeasurement, name)
			}

			if meas.PreviewTempCreates != 0 {
				return fmt.Errorf("%w: %q got %d", errTransformPreviewTempCreates, name, meas.PreviewTempCreates)
			}

			return nil
		},
	)
}
