//go:build mage
// +build mage

package main

import (
	"os"
	"strings"

	"github.com/hotchkj/mage-gate/gate"
)

const (
	goBuildTagsArg = "-tags="
	goShuffleOnArg = "-shuffle=on"
)

func qualityScopeOptions(cfg *config) []gate.QualityScopeOption {
	var opts []gate.QualityScopeOption
	if len(cfg.QualityScope.Tags) > 0 {
		opts = append(opts, gate.Tags(cfg.QualityScope.Tags...))
	}
	if len(cfg.QualityScope.Exclude) > 0 {
		opts = append(opts, gate.Exclude(cfg.QualityScope.Exclude...))
	}
	if len(cfg.QualityScope.TestFilePatterns) > 0 {
		opts = append(opts, gate.TestFilePatterns(cfg.QualityScope.TestFilePatterns...))
	}
	return opts
}

func lintOptions(cfg *config) []gate.LintOption {
	var opts []gate.LintOption
	if cfg.Lint.CustomGCL != "" {
		opts = append(opts, gate.CustomGCL(cfg.Lint.CustomGCL))
	}
	if cfg.Lint.CustomLintToolSpec != "" {
		opts = append(opts, gate.CustomLintToolSpec(cfg.Lint.CustomLintToolSpec))
	}
	if len(cfg.Lint.Args) > 0 {
		opts = append(opts, gate.LintArgs(cfg.Lint.Args...))
	}
	return opts
}

func crapOptions(cfg *config) []gate.CrapOption {
	var opts []gate.CrapOption
	if len(cfg.Crap.Args) > 0 {
		opts = append(opts, gate.CrapArgs(cfg.Crap.Args...))
	}
	return opts
}

func deadcodeOptions(cfg *config) []gate.DeadcodeOption {
	var opts []gate.DeadcodeOption
	if len(cfg.Deadcode.Args) > 0 {
		opts = append(opts, gate.DeadcodeArgs(cfg.Deadcode.Args...))
	}
	return opts
}

func compileOptions(cfg *config) []gate.CompileOption {
	var opts []gate.CompileOption
	if tags := joinBuildTags(cfg.Compile.Tags); tags != "" {
		opts = append(opts, gate.CompileArgs(goBuildTagsArg+tags))
	}
	if len(cfg.Compile.Args) > 0 {
		opts = append(opts, gate.CompileArgs(cfg.Compile.Args...))
	}
	return opts
}

func joinBuildTags(tags []string) string {
	var out []string
	for _, tag := range tags {
		if t := strings.TrimSpace(tag); t != "" {
			out = append(out, t)
		}
	}
	return strings.Join(out, ",")
}

// primaryPassOpts maps [unittests] to gate.CoveredTest options for the coverage-bearing pass.
func primaryPassOpts(cfg *config) []gate.TestOption {
	var opts []gate.TestOption
	if cfg.Unittests.Shuffle {
		opts = append(opts, gate.TestArgs(goShuffleOnArg))
	}
	if len(cfg.Unittests.Args) > 0 {
		opts = append(opts, gate.TestArgs(cfg.Unittests.Args...))
	}
	return opts
}

// integrationPassOpts maps [integrationtests] to gate.Test options (integration uses gate.Test only).
func integrationPassOpts(cfg *config) []gate.TestOption {
	var opts []gate.TestOption
	if t := strings.TrimSpace(cfg.Integrationtests.Tags); t != "" {
		opts = append(opts, gate.TestArgs(goBuildTagsArg+t))
	}
	if cfg.Integrationtests.Shuffle {
		opts = append(opts, gate.TestArgs(goShuffleOnArg))
	}
	if len(cfg.Integrationtests.Args) > 0 {
		opts = append(opts, gate.TestArgs(cfg.Integrationtests.Args...))
	}
	return opts
}

func taggedTestOptions(cfg taggedTestConfig) []gate.TestOption {
	var opts []gate.TestOption
	if t := strings.TrimSpace(cfg.Tags); t != "" {
		opts = append(opts, gate.TestArgs(goBuildTagsArg+t))
	}
	if cfg.Shuffle {
		opts = append(opts, gate.TestArgs(goShuffleOnArg))
	}
	if len(cfg.Args) > 0 {
		opts = append(opts, gate.TestArgs(cfg.Args...))
	}
	return opts
}

// gremlinsMutationOptions maps [gremlins] args to gate.MutationOption for scan and kill targets.
func gremlinsMutationOptions(cfg *config) []gate.MutationOption {
	var opts []gate.MutationOption
	if len(cfg.Gremlins.Args) > 0 {
		opts = append(opts, gate.MutationArgs(cfg.Gremlins.Args...))
	}
	return opts
}

func mutationKillsOptions(cfg *config) []gate.MutationOption {
	opts := gremlinsMutationOptions(cfg)
	if len(cfg.MutationKills.Args) > 0 {
		opts = append(opts, gate.MutationArgs(cfg.MutationKills.Args...))
	}
	return opts
}

// isCI reports whether the process runs under a CI environment (GitHub Actions and most CI set CI).
func isCI() bool {
	return os.Getenv("CI") != ""
}

// gateOutputMode selects agent-first diagnostics locally and full subprocess logs in CI.
func gateOutputMode() gate.OutputMode {
	if isCI() {
		return gate.OutputModeVerbose
	}
	return gate.OutputModeAgent
}

var (
	readPolicyFile = os.ReadFile
	newResolver    = gate.NewProductionToolResolver
)

var newRunner = func() (gate.CommandRunner, error) {
	return gate.NewDisplayRunner(gate.NewProductionRunner(), gateOutputMode(), os.Stdout, os.Stderr)
}

func lintConfigPath(cfg *config) gate.LintConfigValue {
	return gate.LintConfig(cfg.Lint.Config)
}

func lintToolSpec(cfg *config) gate.LintToolValue {
	return gate.LintToolSpec(cfg.Lint.ToolSpec)
}

func lintToolchain(cfg *config) (gate.LintToolchain, error) {
	return gate.NewLintToolchain(
		lintConfigPath(cfg),
		lintToolSpec(cfg),
		lintOptions(cfg)...,
	)
}

func markdownlintEnabled(cfg *config) bool {
	return strings.TrimSpace(cfg.Markdownlint.ToolSpec) != ""
}

func markdownlintOpts(cfg *config) []gate.MarkdownLintOption {
	var opts []gate.MarkdownLintOption
	if len(cfg.Markdownlint.Args) > 0 {
		opts = append(opts, gate.MarkdownLintArgs(cfg.Markdownlint.Args...))
	}
	return opts
}

func markdownlintToolSpec(cfg *config) gate.MarkdownLintToolValue {
	return gate.MarkdownLintToolSpec(cfg.Markdownlint.ToolSpec)
}

func deadcodeToolSpec(cfg *config) gate.DeadcodeToolValue {
	return gate.DeadcodeToolSpec(cfg.Deadcode.ToolSpec)
}

func gocycloToolSpec(cfg *config) gate.GocycloToolValue {
	return gate.GocycloToolSpec(cfg.Crap.ToolSpec)
}

func gremlinsToolSpec(cfg *config) gate.GremlinsToolValue {
	return gate.GremlinsToolSpec(cfg.Gremlins.ToolSpec)
}

func coverageMin(cfg *config) gate.CoverageThreshold {
	return gate.MinPercent(*cfg.Thresholds.CoverageMin)
}

func crapMax(cfg *config) gate.CrapThreshold {
	return gate.MaxScore(*cfg.Thresholds.CrapMax)
}

func mutationSitesMax(cfg *config) gate.MutationSitesThreshold {
	// Invariant: parseConfig requires [thresholds].mutation_sites_max.
	return gate.MaxSites(*cfg.Thresholds.MutationSitesMax)
}

func unitTestDurationMax(cfg *config) gate.DurationThreshold {
	// Invariant: parseConfig requires [unittests].duration_max.
	return gate.MaxSeconds(*cfg.Unittests.DurationMax)
}

func mutationCoverageMin(cfg *config) gate.MutationCoverageThreshold {
	// Invariant: parseConfig requires [thresholds].mutation_coverage_min; 0 is valid.
	return gate.MinMutationCoverage(*cfg.Thresholds.MutationCoverageMin)
}

func mutationKillsMinRate(cfg *config) gate.MinKillRateThreshold {
	// Invariant: parseConfig requires [thresholds].mutation_kills_min_rate; 0 is valid.
	return gate.MinKillRate(*cfg.Thresholds.MutationKillsMinRate)
}
