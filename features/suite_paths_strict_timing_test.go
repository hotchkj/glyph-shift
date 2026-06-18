//go:build bdd_strict_timing

package features_test

import "testing"

// Strict timing runs extract performance scenarios tagged @timing_strict in isolation.
// These scenarios assert tight wall-clock bounds and are intended for CI/perf pipelines,
// not the default precommit gate.
// Primary: `mage stricttiming`. Diagnostic: `go test -tags bdd_strict_timing ./features`.
var strictTimingFeaturePaths = []string{
	"bdd/performance/extract_performance_contract.feature",
	"bdd/performance/split_blocks_performance_contract.feature",
	"bdd/performance/transform_performance_contract.feature",
}

var strictTimingFeatureTags = "@timing_strict"

func TestStrictTimingFeaturePathsMatchPerformanceInventory(t *testing.T) {
	t.Parallel()

	if slicesEqual(strictTimingFeaturePaths, expectedBddPerformanceFeatureRelPaths) {
		return
	}
	t.Fatalf(
		"bdd_strict_timing strictTimingFeaturePaths %v want exact %v",
		strictTimingFeaturePaths,
		expectedBddPerformanceFeatureRelPaths,
	)
}
