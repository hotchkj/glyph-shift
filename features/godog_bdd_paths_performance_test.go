//go:build performance && !bdd_strict_timing

package features_test

func bddGodogPaths() []string {
	return performanceFeaturePaths
}

func bddGodogTags() string {
	return performanceFeatureTags
}
