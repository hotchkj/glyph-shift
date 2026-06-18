package steps

import (
	"context"
	"errors"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

// errStubTransformPerfMeasureFailure is returned by the transform perf measure stub on its first call.
var errStubTransformPerfMeasureFailure = errors.New("stub transform perf measure failure")

func installTransformPerfMeasureCountingStub(t *testing.T) (cleanup func(), calls *int) {
	t.Helper()

	oldMeas := transformPerfMeasure
	var measureCallCount int
	transformPerfMeasure = func(
		ctx context.Context,
		st pipeline.FileStater,
		resolver validate.PathResolver,
		session fileops.FileSession,
		params pipeline.TransformParams,
		measureTempCreates, preview bool,
	) (testutil.TransformPipelinePerfMeasurement, pipeline.TransformPipelineResult, error) {
		measureCallCount++
		if measureCallCount == 1 {
			return testutil.TransformPipelinePerfMeasurement{}, pipeline.TransformPipelineResult{},
				errStubTransformPerfMeasureFailure
		}

		return testutil.TransformPipelinePerfMeasurement{}, pipeline.TransformPipelineResult{}, nil
	}

	cleanup = func() {
		transformPerfMeasure = oldMeas
	}

	return cleanup, &measureCallCount
}

func assertFirstTransformPerfRunFailsWithStub(t *testing.T, tc *TestContext, opts fileops.TransformOptions) {
	t.Helper()

	errFirst := transformPerfRun(tc, "small", false, opts, false)
	if errFirst == nil {
		t.Fatal(
			"expected first transformPerfRun to return measurement error, got nil " +
				"(would allow later When to mask failure)",
		)
	}
	if !errors.Is(errFirst, errStubTransformPerfMeasureFailure) {
		t.Fatalf("first run: expected %v, got %v", errStubTransformPerfMeasureFailure, errFirst)
	}
	if tc.LastOperationError == nil || !errors.Is(tc.LastOperationError, errStubTransformPerfMeasureFailure) {
		t.Fatalf("first run: LastOperationError should hold stub error, got %v", tc.LastOperationError)
	}
	if _, ok := tc.PerfTransformBySource["small"]; ok {
		t.Fatal("failed run should not record measurement for logical source")
	}
}

func assertSecondTransformPerfRunSucceedsAndClearsError(t *testing.T, tc *TestContext, opts fileops.TransformOptions) {
	t.Helper()

	errSecond := transformPerfRun(tc, "large", false, opts, false)
	if errSecond != nil {
		t.Fatalf("second transformPerfRun: %v", errSecond)
	}
	if tc.LastOperationError != nil {
		t.Fatalf("after successful second run, LastOperationError should be cleared, got %v", tc.LastOperationError)
	}
	if _, ok := tc.PerfTransformBySource["large"]; !ok {
		t.Fatal("successful second run should record measurement for logical source")
	}
}

// Proves a failed first transformPerfRun is reported to Godog (non-nil return) so a second
// successful When cannot clear LastOperationError before a generic "operation succeeds" Then.
func TestTransformPerfRun_FirstMeasureErrorReturnedNotMaskedByLaterSuccess(t *testing.T) {
	t.Parallel()

	cleanup, calls := installTransformPerfMeasureCountingStub(t)
	t.Cleanup(cleanup)

	tc := NewTestContext()
	t.Cleanup(func() { tc.Cleanup() })

	lf := fileops.TargetLF
	opts := fileops.TransformOptions{LineEndings: &lf}

	assertFirstTransformPerfRunFailsWithStub(t, tc, opts)
	assertSecondTransformPerfRunSucceedsAndClearsError(t, tc, opts)

	if *calls != 2 {
		t.Fatalf("expected measure stub to run twice, got %d", *calls)
	}
}
