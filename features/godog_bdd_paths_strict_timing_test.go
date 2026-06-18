//go:build bdd_strict_timing

package features_test

func bddGodogPaths() []string {
	return strictTimingFeaturePaths
}

func bddGodogTags() string {
	return strictTimingFeatureTags
}
