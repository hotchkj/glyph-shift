//go:build !performance && !bdd_strict_timing

package features_test

func bddGodogPaths() []string {
	return featurePaths
}

func bddGodogTags() string {
	return featureTags
}
