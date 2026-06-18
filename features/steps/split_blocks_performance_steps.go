package steps

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/cucumber/godog"

	"github.com/hotchkj/glyph-shift/internal/testutil"
)

const splitBlocksPerfWorkloadRatioMultiplier = 2

// splitBlocksPerfHeavyTailLinePairs repeats filler line pairs appended to overrun fixtures.
const splitBlocksPerfHeavyTailLinePairs = 200_000

const (
	splitBlocksPerfFencedGoStart = "```go\n"
	splitBlocksPerfFencedGoEnd   = "```\n"
)

const (
	errFmtSplitBlocksPerfWrapName = "%w: name %q"
	errFmtSplitBlocksPerfDetailVs = "%w: detail %s vs %s"
)

var (
	errSplitPerfMissingLargeMeas = errors.New(
		"split perf measurements: missing large source (expect preceding split When step with inline fixture)",
	)

	errSplitPerfMissingSmallMeas = errors.New(
		"split perf measurements: missing small source (expect preceding split When step with inline fixture)",
	)

	errBlocksPerfMissingLargeMeas = errors.New(
		"blocks perf measurements: missing large source (expect preceding blocks When step with inline fixture)",
	)

	errBlocksPerfMissingSmallMeas = errors.New(
		"blocks perf measurements: missing small source (expect preceding blocks When step with inline fixture)",
	)

	errSplitPerfAllocRatio = errors.New("split measured total allocations exceed 2× small reference")

	errSplitPerfRetainedRatio = errors.New("split measured retained heap exceeds 2× small reference")

	errBlocksPerfAllocRatio = errors.New("blocks measured total allocations exceed 2× small reference")

	errBlocksPerfRetainedRatio = errors.New("blocks measured retained heap exceeds 2× small reference")

	errSplitPerfSourceWorkBaseline = errors.New(
		"split measured source reads: baseline small SourceBytesRead=0 while large had work",
	)

	errSplitPerfSourceWorkRatio = errors.New("split measured source reads exceed 2× small reference")

	errBlocksPerfSourceWorkBaseline = errors.New(
		"blocks measured source reads: baseline small SourceBytesRead=0 while large had work",
	)

	errBlocksPerfSourceWorkRatio = errors.New("blocks measured source reads exceed 2× small reference")

	errSplitPreviewDestNonZero = errors.New("split preview must write zero destination payload bytes")

	errBlocksPreviewDestNonZero = errors.New("blocks preview must write zero destination payload bytes")

	errSplitPreviewMissingMeas = errors.New(
		"split preview perf: missing measurement (expect preview When inline source first)",
	)

	errBlocksPreviewMissingMeas = errors.New(
		"blocks preview perf: missing measurement (expect preview When inline source first)",
	)

	errPerfMaxFilesEffectiveAtLeastOne = errors.New(
		"max-files effective must be >= 1",
	)

	errPerfLineCountAtLeastOne = errors.New(
		"lineCount must be >= 1",
	)

	errPerfLineCountWidthAtLeastOne = errors.New(
		"lineCount and width must be >= 1",
	)
)

// RegisterSplitBlocksPerformance registers boundedness and preview assertions for portable split/blocks perf contracts.
func RegisterSplitBlocksPerformance(sc *godog.ScenarioContext, tc *TestContext) {
	registerSplitBlocksPerfFixtureGivens(sc, tc)
	registerSplitPipelinePerfSteps(sc, tc)
	registerBlocksPipelinePerfSteps(sc, tc)
}

func registerSplitBlocksPerfFixtureGivens(sc *godog.ScenarioContext, tc *TestContext) {
	registerSplitOverrunDelimiterGiven(sc, tc)
	registerBlocksOverrunFencesGiven(sc, tc)
	registerPreambleNumberedSourceGiven(sc, tc)
	registerFencedGoNumberedSourceGiven(sc, tc)
	registerSplitHeavyBodyGiven(sc, tc)
	registerBlocksHeavyBodyGiven(sc, tc)
}

func registerSplitOverrunDelimiterGiven(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(
		//nolint:lll // Godog regexp + scenario vocabulary
		`^split overrun sources "([^"]*)" \(small\) and "([^"]*)" \(large\) for max-files (\d+) with delimiter line "---"$`,
		func(smallName, largeName string, maxFilesEffective int) error {
			if maxFilesEffective < 1 {
				return fmt.Errorf("%w, got %d", errPerfMaxFilesEffectiveAtLeastOne, maxFilesEffective)
			}

			prefix := testutil.BuildMaxFilesExceededSplitPrefix('@', "---\n", maxFilesEffective)
			largeTail := bytes.Repeat([]byte("~\n"), splitBlocksPerfHeavyTailLinePairs)
			largeSrc := append(append(make([]byte, 0, len(prefix)+len(largeTail)), prefix...), largeTail...)

			if err := writeSourceFile(tc, smallName, prefix); err != nil {
				return err
			}

			return writeSourceFile(tc, largeName, largeSrc)
		},
	)
}

func registerBlocksOverrunFencesGiven(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(
		//nolint:lll // Godog regexp + scenario vocabulary exceeds 120 chars
		`^blocks overrun sources "([^"]*)" \(small\) and "([^"]*)" \(large\) for max-files (\d+) with golang fences$`,
		func(smallName, largeName string, maxFilesEffective int) error {
			if maxFilesEffective < 1 {
				return fmt.Errorf("%w, got %d", errPerfMaxFilesEffectiveAtLeastOne, maxFilesEffective)
			}

			blocksBeyond := maxFilesEffective + 1

			prefix := testutil.BuildMaxFilesExceededBlocksPrefix(
				'@', "",
				splitBlocksPerfFencedGoStart, "b\n", "```\n",
				blocksBeyond,
			)

			largeTail := bytes.Repeat([]byte("~\n"), splitBlocksPerfHeavyTailLinePairs)
			largeSrc := append(append(make([]byte, 0, len(prefix)+len(largeTail)), prefix...), largeTail...)

			if err := writeSourceFile(tc, smallName, prefix); err != nil {
				return err
			}

			return writeSourceFile(tc, largeName, largeSrc)
		},
	)
}

func registerPreambleNumberedSourceGiven(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(
		`^a "---" preamble numbered source file "([^"]*)" with (\d+) body lines$`,
		func(name string, lineCount int) error {
			if lineCount < 1 {
				return errPerfLineCountAtLeastOne
			}

			var sb strings.Builder
			sb.WriteString("---\n")

			for i := 1; i <= lineCount; i++ {
				_, _ = fmt.Fprintf(&sb, "line %d\n", i)
			}

			return writeSourceFile(tc, name, []byte(sb.String()))
		},
	)
}

func registerFencedGoNumberedSourceGiven(sc *godog.ScenarioContext, tc *TestContext) {
	stepFencedGoNumbered := `^a fenced-go numbered source file "([^"]*)" with (\d+) inner body lines$`
	sc.Given(stepFencedGoNumbered, func(name string, lineCount int) error {
		if lineCount < 1 {
			return errPerfLineCountAtLeastOne
		}

		var sb strings.Builder
		sb.WriteString(splitBlocksPerfFencedGoStart)

		for i := 1; i <= lineCount; i++ {
			_, _ = fmt.Fprintf(&sb, "line %d\n", i)
		}

		sb.WriteString(splitBlocksPerfFencedGoEnd)

		return writeSourceFile(tc, name, []byte(sb.String()))
	})
}

func registerSplitHeavyBodyGiven(sc *godog.ScenarioContext, tc *TestContext) {
	stepSplitHeavy := `^split single-section heavy-body source file "([^"]*)" with delimiter line "---" ` +
		`and (\d+) body lines (\d+) bytes wide$`
	sc.Given(
		stepSplitHeavy,
		func(name string, lineCount, width int) error {
			if lineCount < 1 || width < 1 {
				return errPerfLineCountWidthAtLeastOne
			}

			src := testutil.BuildLargeSplitSingleSectionSource(lineCount, width, []byte("---\n"))

			return writeSourceFile(tc, name, src)
		},
	)
}

func registerBlocksHeavyBodyGiven(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(
		`^blocks single fenced heavy-body source file "([^"]*)" with (\d+) inner body lines (\d+) bytes wide$`,
		func(name string, lineCount, width int) error {
			if lineCount < 1 || width < 1 {
				return errPerfLineCountWidthAtLeastOne
			}

			src := testutil.BuildLargeBlocksSingleBodySource(
				nil,
				[]byte(splitBlocksPerfFencedGoStart),
				[]byte(splitBlocksPerfFencedGoEnd),
				lineCount,
				width,
			)

			return writeSourceFile(tc, name, src)
		},
	)
}

func splitPipelineMeasPair(tc *TestContext, largeName, smallName string,
) (lg, sm testutil.SplitPipelinePerfMeasurement, err error) {
	lg, okL := tc.PerfSplitPipelineBySource[largeName]
	sm, okS := tc.PerfSplitPipelineBySource[smallName]

	if !okL {
		return lg, sm, fmt.Errorf(errFmtSplitBlocksPerfWrapName, errSplitPerfMissingLargeMeas, largeName)
	}

	if !okS {
		return lg, sm, fmt.Errorf(errFmtSplitBlocksPerfWrapName, errSplitPerfMissingSmallMeas, smallName)
	}

	return lg, sm, nil
}

func blocksPipelineMeasPair(tc *TestContext, largeName, smallName string,
) (lg, sm testutil.BlocksPipelinePerfMeasurement, err error) {
	lg, okL := tc.PerfBlocksPipelineBySource[largeName]
	sm, okS := tc.PerfBlocksPipelineBySource[smallName]

	if !okL {
		return lg, sm, fmt.Errorf(errFmtSplitBlocksPerfWrapName, errBlocksPerfMissingLargeMeas, largeName)
	}

	if !okS {
		return lg, sm, fmt.Errorf(errFmtSplitBlocksPerfWrapName, errBlocksPerfMissingSmallMeas, smallName)
	}

	return lg, sm, nil
}

func registerSplitPipelinePerfSteps(sc *godog.ScenarioContext, tc *TestContext) {
	registerSplitMeasuredPipelineAssertions(sc, tc)
	registerSplitPreviewAssertion(sc, tc)
}

func registerSplitMeasuredPipelineAssertions(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^split measured source reads for "([^"]*)" are no more than 2 times "([^"]*)"$`,
		func(largeName, smallName string) error { return splitThenMeasuredSourceReads(tc, largeName, smallName) })

	sc.Then(`^split measured total allocations for "([^"]*)" are no more than 2 times "([^"]*)"$`,
		func(largeName, smallName string) error { return splitThenMeasuredTotalAlloc(tc, largeName, smallName) })

	sc.Then(`^split measured retained heap for "([^"]*)" is no more than 2 times "([^"]*)"$`,
		func(largeName, smallName string) error {
			return splitThenMeasuredRetainedHeap(tc, largeName, smallName)
		})
}

func splitThenMeasuredSourceReads(tc *TestContext, largeName, smallName string) error {
	lg, sm, pairErr := splitPipelineMeasPair(tc, largeName, smallName)
	if pairErr != nil {
		return pairErr
	}

	chk := sourceReadRatioCheck[testutil.SplitPipelinePerfMeasurement,
		testutil.SplitPipelinePerfMeasurement]{
		Label:                       "split measured source reads",
		LargeName:                   largeName,
		SmallName:                   smallName,
		LWide:                       splitPerfSourceReads(&lg),
		SWide:                       splitPerfSourceReads(&sm),
		LargeMeas:                   &lg,
		SmallMeas:                   &sm,
		PassWhenBaselineSmallUnread: false,
		PassWhenInvertedOrdering:    false,
		BaselineUnread:              errSplitPerfSourceWorkBaseline,
		RatioExceeded:               errSplitPerfSourceWorkRatio,
		FormatLarge:                 formatSplitMeasBrief,
		FormatSmall:                 formatSplitMeasBrief,
	}

	return assertSourceReadRatioAgainstSmall(chk)
}

func splitThenMeasuredTotalAlloc(tc *TestContext, largeName, smallName string) error {
	lg, sm, pairErr := splitPipelineMeasPair(tc, largeName, smallName)
	if pairErr != nil {
		return pairErr
	}

	ls, ss := lg.TotalAllocDelta, sm.TotalAllocDelta
	if ss == 0 && ls != 0 {
		return fmt.Errorf(
			"%w: small_total_alloc=%d large_total_alloc=%d; detail %s vs %s",
			errSplitPerfAllocRatio, ss, ls,
			formatSplitMeasBrief(largeName, &lg), formatSplitMeasBrief(smallName, &sm),
		)
	}

	if ls <= uint64(splitBlocksPerfWorkloadRatioMultiplier)*ss {
		return nil
	}

	return fmt.Errorf(
		"%w: ratio ~%.3f small=%d large=%d; detail %s vs %s",
		errSplitPerfAllocRatio, ratioU64(ls, ss), ss, ls,
		formatSplitMeasBrief(largeName, &lg),
		formatSplitMeasBrief(smallName, &sm),
	)
}

func splitThenMeasuredRetainedHeap(tc *TestContext, largeName, smallName string) error {
	lg, sm, pairErr := splitPipelineMeasPair(tc, largeName, smallName)
	if pairErr != nil {
		return pairErr
	}

	lh := nonNegativeHeapDelta(lg.HeapAllocDelta)
	sh := nonNegativeHeapDelta(sm.HeapAllocDelta)
	if sh == 0 && lh != 0 {
		return fmt.Errorf(
			errFmtSplitBlocksPerfDetailVs,
			errSplitPerfRetainedRatio,
			formatSplitMeasBrief(largeName, &lg), formatSplitMeasBrief(smallName, &sm),
		)
	}

	if lh <= int64(splitBlocksPerfWorkloadRatioMultiplier)*sh {
		return nil
	}

	return fmt.Errorf(
		"%w: lh=%d sh=%d; detail %s vs %s",
		errSplitPerfRetainedRatio, lh, sh,
		formatSplitMeasBrief(largeName, &lg), formatSplitMeasBrief(smallName, &sm),
	)
}

func registerSplitPreviewAssertion(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^split preview writes no measured destination payload bytes for "([^"]*)"$`, func(name string) error {
		meas, ok := tc.PerfSplitPipelineBySource[name]
		if !ok {
			return fmt.Errorf("%w: %q", errSplitPreviewMissingMeas, name)
		}

		if meas.DestinationBytesWritten != 0 {
			return fmt.Errorf(
				"%w for %q: dest_bytes_written=%d dest_opens=%d mkdir_calls=%d; %s",
				errSplitPreviewDestNonZero, name,
				meas.DestinationBytesWritten, meas.DestinationOpens, meas.DestinationMkdirAllCalls,
				formatSplitMeasBrief(name, &meas),
			)
		}

		return nil
	})
}

func registerBlocksPipelinePerfSteps(sc *godog.ScenarioContext, tc *TestContext) {
	registerBlocksMeasuredPipelineAssertions(sc, tc)
	registerBlocksPreviewAssertion(sc, tc)
}

func registerBlocksMeasuredPipelineAssertions(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^blocks measured source reads for "([^"]*)" are no more than 2 times "([^"]*)"$`,
		func(largeName, smallName string) error {
			return blocksThenMeasuredSourceReads(tc, largeName, smallName)
		})

	sc.Then(`^blocks measured total allocations for "([^"]*)" are no more than 2 times "([^"]*)"$`,
		func(largeName, smallName string) error { return blocksThenMeasuredTotalAlloc(tc, largeName, smallName) })

	sc.Then(`^blocks measured retained heap for "([^"]*)" is no more than 2 times "([^"]*)"$`,
		func(largeName, smallName string) error {
			return blocksThenMeasuredRetainedHeap(tc, largeName, smallName)
		})
}

func blocksThenMeasuredSourceReads(tc *TestContext, largeName, smallName string) error {
	lg, sm, pairErr := blocksPipelineMeasPair(tc, largeName, smallName)
	if pairErr != nil {
		return pairErr
	}

	chk := sourceReadRatioCheck[testutil.BlocksPipelinePerfMeasurement,
		testutil.BlocksPipelinePerfMeasurement]{
		Label:                       "blocks measured source reads",
		LargeName:                   largeName,
		SmallName:                   smallName,
		LWide:                       blocksPerfSourceReads(&lg),
		SWide:                       blocksPerfSourceReads(&sm),
		LargeMeas:                   &lg,
		SmallMeas:                   &sm,
		PassWhenBaselineSmallUnread: false,
		PassWhenInvertedOrdering:    false,
		BaselineUnread:              errBlocksPerfSourceWorkBaseline,
		RatioExceeded:               errBlocksPerfSourceWorkRatio,
		FormatLarge:                 formatBlocksMeasBrief,
		FormatSmall:                 formatBlocksMeasBrief,
	}

	return assertSourceReadRatioAgainstSmall(chk)
}

func blocksThenMeasuredTotalAlloc(tc *TestContext, largeName, smallName string) error {
	lg, sm, pairErr := blocksPipelineMeasPair(tc, largeName, smallName)
	if pairErr != nil {
		return pairErr
	}

	ls, ss := lg.TotalAllocDelta, sm.TotalAllocDelta
	if ss == 0 && ls != 0 {
		return fmt.Errorf(
			errFmtSplitBlocksPerfDetailVs,
			errBlocksPerfAllocRatio,
			formatBlocksMeasBrief(largeName, &lg), formatBlocksMeasBrief(smallName, &sm),
		)
	}

	if ls <= uint64(splitBlocksPerfWorkloadRatioMultiplier)*ss {
		return nil
	}

	return fmt.Errorf(
		"%w: ratio ~%.3f detail %s vs %s",
		errBlocksPerfAllocRatio, ratioU64(ls, ss),
		formatBlocksMeasBrief(largeName, &lg), formatBlocksMeasBrief(smallName, &sm),
	)
}

func blocksThenMeasuredRetainedHeap(tc *TestContext, largeName, smallName string) error {
	lg, sm, pairErr := blocksPipelineMeasPair(tc, largeName, smallName)
	if pairErr != nil {
		return pairErr
	}

	lh := nonNegativeHeapDelta(lg.HeapAllocDelta)
	sh := nonNegativeHeapDelta(sm.HeapAllocDelta)
	if sh == 0 && lh != 0 {
		return fmt.Errorf(
			errFmtSplitBlocksPerfDetailVs,
			errBlocksPerfRetainedRatio,
			formatBlocksMeasBrief(largeName, &lg), formatBlocksMeasBrief(smallName, &sm),
		)
	}

	if lh <= int64(splitBlocksPerfWorkloadRatioMultiplier)*sh {
		return nil
	}

	return fmt.Errorf(
		"%w: lh=%d sh=%d; detail %s vs %s",
		errBlocksPerfRetainedRatio, lh, sh,
		formatBlocksMeasBrief(largeName, &lg), formatBlocksMeasBrief(smallName, &sm),
	)
}

func registerBlocksPreviewAssertion(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^blocks preview writes no measured destination payload bytes for "([^"]*)"$`, func(name string) error {
		meas, ok := tc.PerfBlocksPipelineBySource[name]
		if !ok {
			return fmt.Errorf("%w: %q", errBlocksPreviewMissingMeas, name)
		}

		if meas.DestinationBytesWritten != 0 {
			return fmt.Errorf(
				"%w for %q: dest_bytes_written=%d dest_opens=%d mkdir_calls=%d; %s",
				errBlocksPreviewDestNonZero, name,
				meas.DestinationBytesWritten, meas.DestinationOpens, meas.DestinationMkdirAllCalls,
				formatBlocksMeasBrief(name, &meas),
			)
		}

		return nil
	})
}

func splitPerfSourceReads(m *testutil.SplitPipelinePerfMeasurement) int64 {
	if m == nil {
		return 0
	}

	return m.SourceBytesRead
}

func blocksPerfSourceReads(m *testutil.BlocksPipelinePerfMeasurement) int64 {
	if m == nil {
		return 0
	}

	return m.SourceBytesRead
}

type sourceReadRatioCheck[L any, S any] struct {
	Label                       string
	LargeName, SmallName        string
	LWide, SWide                int64
	LargeMeas                   *L
	SmallMeas                   *S
	PassWhenBaselineSmallUnread bool
	PassWhenInvertedOrdering    bool
	BaselineUnread              error
	RatioExceeded               error
	FormatLarge                 func(string, *L) string
	FormatSmall                 func(string, *S) string
}

func assertSourceReadRatioAgainstSmall[L, S any](chk sourceReadRatioCheck[L, S]) error {
	switch {
	case chk.SWide == 0 && chk.LWide > 0 && chk.PassWhenBaselineSmallUnread:
		return nil
	case chk.SWide == 0 && chk.LWide > 0:
		return fmt.Errorf(
			"%w [%s]: large_reads=%d small_reads=%d; large=%s small=%s",
			chk.BaselineUnread, chk.Label,
			chk.LWide, chk.SWide,
			chk.FormatLarge(chk.LargeName, chk.LargeMeas), chk.FormatSmall(chk.SmallName, chk.SmallMeas),
		)
	default:
		if chk.LWide <= int64(splitBlocksPerfWorkloadRatioMultiplier)*chk.SWide {
			return nil
		}

		if chk.PassWhenInvertedOrdering && chk.LWide < chk.SWide {
			return nil
		}

		return fmt.Errorf(
			"%w [%s]: multiple=%d large_reads=%d small_reads=%d ratio_approx=%.4f detail large=%s small=%s",
			chk.RatioExceeded, chk.Label,
			splitBlocksPerfWorkloadRatioMultiplier, chk.LWide, chk.SWide,
			ratioI64(chk.LWide, chk.SWide),
			chk.FormatLarge(chk.LargeName, chk.LargeMeas), chk.FormatSmall(chk.SmallName, chk.SmallMeas),
		)
	}
}

func formatSplitMeasBrief(label string, meas *testutil.SplitPipelinePerfMeasurement) string {
	if meas == nil {
		return fmt.Sprintf("%s=<nil>", label)
	}

	return fmt.Sprintf(
		"%s src_reads=%d read_calls=%d seek_calls=%d opens=%d dest_bytes=%d dest_opens=%d mkdir_calls=%d "+
			"planned_outputs=%d elapsed=%s total_alloc_delta=%d heap_delta=%d",
		label,
		meas.SourceBytesRead,
		meas.SourceReadCalls,
		meas.SourceSeekCalls,
		meas.SourceOpens,
		meas.DestinationBytesWritten,
		meas.DestinationOpens,
		meas.DestinationMkdirAllCalls,
		meas.PlannedOutputCount,
		meas.Elapsed,
		meas.TotalAllocDelta,
		meas.HeapAllocDelta,
	)
}

func formatBlocksMeasBrief(label string, meas *testutil.BlocksPipelinePerfMeasurement) string {
	if meas == nil {
		return fmt.Sprintf("%s=<nil>", label)
	}

	return fmt.Sprintf(
		"%s src_reads=%d read_calls=%d seek_calls=%d opens=%d dest_bytes=%d dest_opens=%d mkdir_calls=%d "+
			"planned_outputs=%d elapsed=%s total_alloc_delta=%d heap_delta=%d",
		label,
		meas.SourceBytesRead,
		meas.SourceReadCalls,
		meas.SourceSeekCalls,
		meas.SourceOpens,
		meas.DestinationBytesWritten,
		meas.DestinationOpens,
		meas.DestinationMkdirAllCalls,
		meas.PlannedOutputCount,
		meas.Elapsed,
		meas.TotalAllocDelta,
		meas.HeapAllocDelta,
	)
}
