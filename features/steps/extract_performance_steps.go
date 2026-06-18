package steps

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/hotchkj/glyph-shift/internal/testutil"
)

// Multiplier applied in BDD performance contracts comparing large vs small extract workloads.
const extractPerfWorkloadRatioMultiplier = 2

// Baseline source for the single-argument "grows with selected output" step; matches feature wording
// that pairs "large.md" with "small.md" in the same scenarios.
const extractPerfSourceWorkGrowthBaselineSource = "small.md"

var (
	errPerfMissingLargeMeasurement = errors.New(
		"perf measurements: missing large source (expect after successful extract When step)",
	)

	errPerfMissingSmallMeasurement = errors.New(
		"perf measurements: missing small source (expect after successful extract When step)",
	)

	errPerfAllocBaselineSmallZeroLargeNonZero = errors.New(
		"extract allocated memory (TotalAllocDelta): baseline small had zero while large non-zero",
	)

	errPerfAllocRatioExceeded = errors.New(
		"extract allocated memory: large TotalAllocDelta exceeds multiple of small",
	)

	errPerfSourceGrowthSourceBytesOrder = errors.New(
		"extract source work growth: large SourceBytesRead must exceed small",
	)

	errPerfSourceGrowthOutputBytesOrder = errors.New(
		"extract source work growth: large output bytes must exceed small",
	)

	errPerfRetainedBaselineSmallZeroLargeNonZero = errors.New(
		"extract retained memory (HeapAllocDelta): baseline small had zero while large non-zero",
	)

	errPerfRetainedRatioExceeded = errors.New(
		"extract retained memory: large heap delta exceeds multiple of small",
	)

	errPerfPreviewMissingMeasurement = errors.New(
		"extract preview dest bytes: missing perf measurement",
	)

	errPerfPreviewDestinationWritesNonZero = errors.New(
		"extract preview: destination writes must be zero",
	)

	errPerfWallClockExceeded = errors.New("wall-clock: operation exceeded time limit")

	errPerfSourceWorkBaselineSmallZeroLargeNonZero = errors.New(
		"extract source work: baseline small had SourceBytesRead=0 while large had work",
	)

	errPerfSourceWorkRatioExceeded = errors.New(
		"extract source work: large SourceBytesRead exceeds multiple of small",
	)
)

// RegisterExtractPerformance registers extract performance contract Then steps.
func RegisterExtractPerformance(sc *godog.ScenarioContext, tc *TestContext) {
	registerExtractPerfSourceWorkRatioAgainstSmall(sc, tc)

	registerExtractPerfAllocatedMemoryAgainstSmall(sc, tc)

	registerExtractPerfSourceWorkGrows(sc, tc)

	registerExtractPerfRetainedMemoryAgainstSmall(sc, tc)

	registerExtractPerfPreviewDestBytesSteps(sc, tc)

	registerExtractPerfWallClockStep(sc, tc)
}

func registerExtractPerfSourceWorkRatioAgainstSmall(sc *godog.ScenarioContext, tc *TestContext) {
	patternThenSourceWorkVsSmall := "^extract source work for \"([^\"]*)\" is no more than 2 times \"([^\"]*)\"$"

	sc.Then(patternThenSourceWorkVsSmall, func(largeName, smallName string) error {
		lg, sm, errPair := perfPair(tc, largeName, smallName)
		if errPair != nil {
			return errPair
		}

		return assertSourceWorkRatio(
			largeName, smallName, &lg, &sm,
			int64(extractPerfWorkloadRatioMultiplier))
	})

	patternThenPreviewVsSmall := "^extract preview source work for \"([^\"]*)\" " +
		"is no more than 2 times \"([^\"]*)\"$"

	sc.Then(patternThenPreviewVsSmall, func(largeName, smallName string) error {
		lg, sm, errPair := perfPair(tc, largeName, smallName)
		if errPair != nil {
			return errPair
		}

		return assertSourceWorkRatio(
			largeName, smallName, &lg, &sm,
			int64(extractPerfWorkloadRatioMultiplier))
	})
}

func registerExtractPerfAllocatedMemoryAgainstSmall(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(
		`^extract allocated memory for "([^"]*)" is no more than 2 times "([^"]*)"$`,
		func(largeName, smallName string) error {
			lg, sm, errPair := perfPair(tc, largeName, smallName)
			if errPair != nil {
				return errPair
			}

			ls, ss := lg.TotalAllocDelta, sm.TotalAllocDelta
			if ss == 0 && ls != 0 {
				return fmt.Errorf(
					"%w: names small=%s large=%s counters=%s total_alloc_large=%d total_alloc_small=%d; %s vs %s",
					errPerfAllocBaselineSmallZeroLargeNonZero,
					smallName, largeName, formatMeasCounters(&lg, &sm),
					ls, ss,
					formatMeasDetail(largeName, &lg),
					formatMeasDetail(smallName, &sm),
				)
			}

			if ls <= uint64(extractPerfWorkloadRatioMultiplier)*ss {
				return nil
			}

			return fmt.Errorf(
				"%w: ratio ~%v large_total_alloc_delta=%d small_total_alloc_delta=%d; %s vs %s",
				errPerfAllocRatioExceeded,
				ratioU64(ls, ss), ls, ss,
				formatMeasDetail(largeName, &lg),
				formatMeasDetail(smallName, &sm),
			)
		},
	)
}

func registerExtractPerfSourceWorkGrows(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(
		`^extract source work for "([^"]*)" grows with selected output$`,
		func(largeName string) error {
			baselineName := extractPerfSourceWorkGrowthBaselineSource
			lg, sm, errPair := perfPair(tc, largeName, baselineName)
			if errPair != nil {
				return fmt.Errorf(
					"grows-with-output step compares %q against baseline %q: %w",
					largeName, baselineName, errPair,
				)
			}

			if lg.SourceBytesRead <= sm.SourceBytesRead {
				return fmt.Errorf(
					"%w: large=%q baseline_small_source=%q source_bytes=%d vs %d output_bytes=%d vs %d; %s vs %s",
					errPerfSourceGrowthSourceBytesOrder,
					largeName, baselineName,
					lg.SourceBytesRead, sm.SourceBytesRead,
					len(lg.OutputBytes), len(sm.OutputBytes),
					formatMeasDetail(largeName, &lg),
					formatMeasDetail(baselineName, &sm),
				)
			}

			if len(lg.OutputBytes) <= len(sm.OutputBytes) {
				return fmt.Errorf(
					"%w: large=%q baseline_small_source=%q output_bytes=%d vs %d source_bytes_read=%d vs %d; %s vs %s",
					errPerfSourceGrowthOutputBytesOrder,
					largeName, baselineName,
					len(lg.OutputBytes), len(sm.OutputBytes),
					lg.SourceBytesRead, sm.SourceBytesRead,
					formatMeasDetail(largeName, &lg),
					formatMeasDetail(baselineName, &sm),
				)
			}

			return nil
		},
	)
}

func registerExtractPerfRetainedMemoryAgainstSmall(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(
		`^extract retained memory for "([^"]*)" is no more than 2 times "([^"]*)"$`,
		func(largeName, smallName string) error {
			lg, sm, errPair := perfPair(tc, largeName, smallName)
			if errPair != nil {
				return errPair
			}

			lh := nonNegativeHeapDelta(lg.HeapAllocDelta)
			sh := nonNegativeHeapDelta(sm.HeapAllocDelta)
			if sh == 0 && lh != 0 {
				return fmt.Errorf(
					"%w: names small=%s large=%s counters=%s; %s vs %s",
					errPerfRetainedBaselineSmallZeroLargeNonZero,
					smallName, largeName,
					formatMeasCounters(&lg, &sm),
					formatMeasDetail(largeName, &lg),
					formatMeasDetail(smallName, &sm),
				)
			}

			if lh <= int64(extractPerfWorkloadRatioMultiplier)*sh {
				return nil
			}

			return fmt.Errorf(
				"%w: large_heap_delta=%d small_heap_delta=%d raw_large=%d raw_small=%d; %s vs %s",
				errPerfRetainedRatioExceeded,
				lh, sh, lg.HeapAllocDelta, sm.HeapAllocDelta,
				formatMeasDetail(largeName, &lg),
				formatMeasDetail(smallName, &sm),
			)
		},
	)
}

func registerExtractPerfPreviewDestBytesSteps(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^extract preview writes no destination bytes for "([^"]*)"$`, func(name string) error {
		meas, ok := tc.PerfExtractBySource[name]
		if !ok {
			return fmt.Errorf("%w for %q", errPerfPreviewMissingMeasurement, name)
		}

		if meas.DestinationBytesWritten != 0 {
			return fmt.Errorf(
				"%w for %q: destination_bytes_written=%d destination_opens=%d mkdir_all_calls=%d; %s",
				errPerfPreviewDestinationWritesNonZero,
				name,
				meas.DestinationBytesWritten, meas.DestinationOpens, meas.DestinationMkdirAllCalls,
				formatMeasDetail(name, &meas),
			)
		}

		return nil
	})
}

func registerExtractPerfWallClockStep(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^the operation completes within (\d+) seconds?$`, func(n int) error {
		limit := time.Duration(n) * time.Second
		if tc.LastPerfExtract.Elapsed <= limit {
			return nil
		}

		measCopy := tc.LastPerfExtract

		return fmt.Errorf(
			"%w: took %s, limit %s; last_measurement=%s",
			errPerfWallClockExceeded,
			tc.LastPerfExtract.Elapsed,
			limit,
			formatMeasDetail("<last>", &measCopy),
		)
	})
}

func perfPair(
	tc *TestContext, largeName, smallName string,
) (lg, sm testutil.ExtractMeasurement, err error) {
	lg, okL := tc.PerfExtractBySource[largeName]
	sm, okS := tc.PerfExtractBySource[smallName]

	if !okL {
		return lg, sm, fmt.Errorf("%w: name %q", errPerfMissingLargeMeasurement, largeName)
	}

	if !okS {
		return lg, sm, fmt.Errorf("%w: name %q", errPerfMissingSmallMeasurement, smallName)
	}

	return lg, sm, nil
}

func sourceWorkBytes(m *testutil.ExtractMeasurement) int64 {
	if m == nil {
		return 0
	}

	return m.SourceBytesRead
}

func assertSourceWorkRatio(
	largeName, smallName string,
	lg, sm *testutil.ExtractMeasurement,
	multiple int64,
) error {
	lWork := sourceWorkBytes(lg)
	sWork := sourceWorkBytes(sm)

	if sWork == 0 && lWork > 0 {
		return fmt.Errorf(
			"%w: names small=%s large=%s large_bytes=%d large_seek=%d large_reads=%d small_seek=%d small_reads=%d "+
				"large_dest_writes=%d small_dest_writes=%d; %s vs %s",
			errPerfSourceWorkBaselineSmallZeroLargeNonZero,
			smallName, largeName, lWork,
			lg.SourceSeekCalls, lg.SourceReadCalls,
			sm.SourceSeekCalls, sm.SourceReadCalls,
			lg.DestinationBytesWritten, sm.DestinationBytesWritten,
			formatMeasDetail(largeName, lg),
			formatMeasDetail(smallName, sm),
		)
	}

	if lWork <= multiple*sWork {
		return nil
	}

	return fmt.Errorf(
		"%w: multiple=%d large_source_bytes=%d small_source_bytes=%d ratio approx %.3f seek_calls %d vs %d "+
			"read_calls %d vs %d total_alloc_delta %d vs %d; %s vs %s",
		errPerfSourceWorkRatioExceeded,
		multiple, lWork, sWork, ratioI64(lWork, sWork),
		lg.SourceSeekCalls, sm.SourceSeekCalls,
		lg.SourceReadCalls, sm.SourceReadCalls,
		lg.TotalAllocDelta, sm.TotalAllocDelta,
		formatMeasDetail(largeName, lg),
		formatMeasDetail(smallName, sm),
	)
}

func nonNegativeHeapDelta(delta int64) int64 {
	if delta < 0 {
		return 0
	}

	return delta
}

func ratioI64(large, small int64) float64 {
	if small == 0 {
		return -1
	}

	return float64(large) / float64(small)
}

func ratioU64(large, small uint64) float64 {
	if small == 0 {
		return -1
	}

	return float64(large) / float64(small)
}

func formatMeasCounters(large, small *testutil.ExtractMeasurement) string {
	if large == nil || small == nil {
		return "<nil measurement pointer>"
	}

	var sb strings.Builder

	sb.WriteString("large{")

	sb.WriteString(formatMeasDetail("L", large))

	sb.WriteString("} small{")

	sb.WriteString(formatMeasDetail("S", small))

	sb.WriteString("}")

	return sb.String()
}

const measDetailFormat = "%s source_bytes=%d reads=%d seeks=%d opens=%d dest_bytes=%d dest_opens=%d mkdir_calls=%d " +
	"out_bytes=%d lines=%d elapsed=%s total_alloc_delta=%d heap_delta=%d"

func formatMeasDetail(label string, meas *testutil.ExtractMeasurement) string {
	if meas == nil {
		return fmt.Sprintf("%s=<nil>", label)
	}

	return fmt.Sprintf(
		measDetailFormat,
		label,
		meas.SourceBytesRead,
		meas.SourceReadCalls,
		meas.SourceSeekCalls,
		meas.SourceOpens,
		meas.DestinationBytesWritten,
		meas.DestinationOpens,
		meas.DestinationMkdirAllCalls,
		len(meas.OutputBytes),
		meas.LinesExtracted,
		meas.Elapsed,
		meas.TotalAllocDelta,
		meas.HeapAllocDelta,
	)
}
