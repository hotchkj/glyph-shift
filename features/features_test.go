package features_test

import (
	"context"
	"testing"

	"github.com/cucumber/godog"

	"github.com/hotchkj/glyph-shift/features/steps"
)

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			tc := steps.NewTestContext()
			sc.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
				tc.Cleanup()
				return ctx, nil
			})
			steps.RegisterCommon(sc, tc)
			steps.RegisterOperations(sc, tc)
			steps.RegisterGolden(sc, tc)
			steps.RegisterCLISurface(sc, tc)
			steps.RegisterMCPParity(sc, tc)
			steps.RegisterExtract(sc, tc)
			steps.RegisterSplit(sc, tc)
			steps.RegisterBlocks(sc, tc)
			steps.RegisterTransform(sc, tc)
			steps.RegisterResult(sc, tc)
			steps.RegisterStdoutJSON(sc, tc)
			steps.RegisterContractPathAssertions(sc, tc)
			steps.RegisterLayer2(sc, tc)
			steps.RegisterFixtures(sc, tc)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    bddGodogPaths(),
			Tags:     bddGodogTags(),
			Strict:   true,
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status from godog suite")
	}
}

func TestFeaturePathsAreExact(t *testing.T) {
	t.Parallel()

	// Four suites: correctness (default), performance, strict timing, integration.
	// Integration tests live in integrations/ and do not use BDD feature paths.
	// Performance and strict timing use the same feature paths; strict timing filters to @timing_strict scenarios.
	validExactSets := [][]string{
		// Correctness (default).
		{
			"bdd/core/extract_operations.feature", "bdd/core/extract_contract.feature",
			"bdd/core/split_operations.feature", "bdd/core/split_contract.feature",
			"bdd/core/blocks_operations.feature", "bdd/core/blocks_contract.feature",
			"bdd/core/transform_operations.feature", "bdd/core/transform_contract.feature",
			"bdd/core/cli_surface.feature",
		},
		// Performance and strict timing (same paths, different tag filters).
		{
			"bdd/performance/extract_performance_contract.feature",
			"bdd/performance/split_blocks_performance_contract.feature",
			"bdd/performance/transform_performance_contract.feature",
		},
	}

	got := bddGodogPaths()
	for _, want := range validExactSets {
		if slicesEqual(got, want) {
			return
		}
	}
	t.Fatalf("bddGodogPaths() %v does not match any valid exact set %v", got, validExactSets)
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
