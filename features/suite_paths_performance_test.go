//go:build performance

package features_test

import "testing"

// Performance suite runs portable performance contracts excluding @timing_strict wall-clock
// scenarios and @memstats_residency MemStats retained-heap gates (issue #2).
// Diagnostic: `go test -tags performance ./features`.
var performanceFeaturePaths = []string{
	"bdd/performance/extract_performance_contract.feature",
	"bdd/performance/split_blocks_performance_contract.feature",
	"bdd/performance/transform_performance_contract.feature",
}

//nolint:unused // referenced from godog_bdd_paths_performance_test.go when built with tag performance.
var performanceFeatureTags = "~@timing_strict && ~@memstats_residency"

func TestPerformanceFeaturePathsExactHardGuard(t *testing.T) {
	t.Parallel()

	want := expectedBddPerformanceFeatureRelPaths
	if slicesEqual(performanceFeaturePaths, want) {
		return
	}
	t.Fatalf("performance performanceFeaturePaths %v want exact %v", performanceFeaturePaths, want)
}
