package testutil

import "errors"

// Sentinel errors for extract perf helpers; callers should wrap with fmt.Errorf("%w ...", ...)
// when adding contextual detail (err113 / static-error discipline).
var (
	errExtractUnknownLineTerminator = errors.New("testutil perf extract: unknown ExtractLineTerminator")

	errExtractLineCountBelowMin = errors.New("testutil perf extract: LineCount must be at least 1")

	errExtractLineLengthNegative = errors.New("testutil perf extract: LineLength must be non-negative")

	errExtractGoldenTerminatorMismatch = errors.New(
		"testutil perf extract: golden output terminator mismatches fixture terminator")

	errNilCountingWriter = errors.New("testutil perf extract: nil CountingWriter Write")

	errMeasureCountingSrcNilResolver = errors.New(
		"testutil MeasurePipelineExtractCountingSrcMem: resolver must not be nil")

	errMeasureCountingSrcNilDeps = errors.New(
		"testutil MeasurePipelineExtractCountingSrcMem: source opener and memory FS must not be nil")

	errMeasureCountingSrcReadAppendDestSnapshot = errors.New(
		"testutil MeasurePipelineExtractCountingSrcMem: read append destination snapshot for heap restore")

	errMeasureCountingSrcRestoreDestination = errors.New(
		"testutil MeasurePipelineExtractCountingSrcMem: restore destination after heap measure")

	errMeasureCountingSrcRemoveDestination = errors.New(
		"testutil MeasurePipelineExtractCountingSrcMem: remove destination before heap measure")

	errMeasurePipelineNilResolver = errors.New("testutil MeasurePipelineExtract: resolver must not be nil")

	errMeasurePipelineNilOpeners = errors.New(
		"testutil MeasurePipelineExtract: CountingSourceOpener and CountingOutputOpener must not be nil")
)
