//go:build !performance && !bdd_strict_timing

package features_test

import (
	"slices"
	"testing"
)

var featurePaths = []string{
	"bdd/core/extract_operations.feature",
	"bdd/core/extract_contract.feature",
	"bdd/core/split_operations.feature",
	"bdd/core/split_contract.feature",
	"bdd/core/blocks_operations.feature",
	"bdd/core/blocks_contract.feature",
	"bdd/core/transform_operations.feature",
	"bdd/core/transform_contract.feature",
	"bdd/core/cli_surface.feature",
}

var featureTags = "~@timing_strict"

func TestDefaultFeaturePathsMatchCoreInventory(t *testing.T) {
	t.Parallel()

	got := append([]string{}, featurePaths...)
	want := append([]string{}, expectedBddCoreFeatureRelPaths...)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("default featurePaths mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}
