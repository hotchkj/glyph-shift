package features_test

import (
	"embed"
	"io/fs"
	"slices"
	"strings"
	"testing"
)

//go:embed bdd
var bddEmbeddedFS embed.FS

// expectedBddCoreFeatureRelPaths is the sorted inventory of every .feature file under bdd/core.
// Adding a file under bdd/core without updating this list (and suite path assignments in
// suite_paths_*_test.go, plus validExactSets in features_test.go when applicable) must fail tests.
var expectedBddCoreFeatureRelPaths = []string{
	"bdd/core/blocks_contract.feature",
	"bdd/core/blocks_operations.feature",
	"bdd/core/cli_surface.feature",
	"bdd/core/extract_contract.feature",
	"bdd/core/extract_operations.feature",
	"bdd/core/split_contract.feature",
	"bdd/core/split_operations.feature",
	"bdd/core/transform_contract.feature",
	"bdd/core/transform_operations.feature",
}

// expectedBddPerformanceFeatureRelPaths is the sorted inventory under bdd/performance.
var expectedBddPerformanceFeatureRelPaths = []string{
	"bdd/performance/extract_performance_contract.feature",
	"bdd/performance/split_blocks_performance_contract.feature",
	"bdd/performance/transform_performance_contract.feature",
}

func collectSortedBddFeatureRelPaths(fsys fs.FS, root string) ([]string, error) {
	var out []string
	err := fs.WalkDir(fsys, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".feature") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(out)
	return out, nil
}

func TestBddFeatureInventory_CoreMatchesEmbed(t *testing.T) {
	t.Parallel()

	got, err := collectSortedBddFeatureRelPaths(bddEmbeddedFS, "bdd/core")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, expectedBddCoreFeatureRelPaths) {
		t.Fatalf("bdd/core inventory mismatch:\n got: %#v\nwant: %#v", got, expectedBddCoreFeatureRelPaths)
	}
}

func TestBddFeatureInventory_PerformanceMatchesEmbed(t *testing.T) {
	t.Parallel()

	got, err := collectSortedBddFeatureRelPaths(bddEmbeddedFS, "bdd/performance")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, expectedBddPerformanceFeatureRelPaths) {
		t.Fatalf("bdd/performance inventory mismatch:\n got: %#v\nwant: %#v", got, expectedBddPerformanceFeatureRelPaths)
	}
}

func TestBddFeatureInventory_AllFeaturesUnderBddAccountedFor(t *testing.T) {
	t.Parallel()

	got, err := collectSortedBddFeatureRelPaths(bddEmbeddedFS, "bdd")
	if err != nil {
		t.Fatal(err)
	}
	for _, featurePath := range got {
		if !strings.HasPrefix(featurePath, "bdd/core/") && !strings.HasPrefix(featurePath, "bdd/performance/") {
			t.Errorf("feature file must live under bdd/core or bdd/performance: %s", featurePath)
		}
	}
	want := append(append([]string{}, expectedBddCoreFeatureRelPaths...), expectedBddPerformanceFeatureRelPaths...)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("full bdd .feature inventory mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}
