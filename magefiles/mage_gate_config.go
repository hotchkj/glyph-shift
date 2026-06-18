//go:build mage
// +build mage

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/BurntSushi/toml"
)

const policyPath = "gate.toml"

const percentThresholdMax = 100

// errGateTomlMissing is returned when gate.toml is not present at the configured path.
var errGateTomlMissing = errors.New("gate.toml not found; create gate.toml at repository root")

// errUnittestsDurationMaxRequired is returned when [unittests].duration_max is omitted.
var errUnittestsDurationMaxRequired = errors.New("gate.toml [unittests]: duration_max is required")

var errUnittestsDurationMaxOutOfRange = errors.New(
	`gate.toml [unittests]: "duration_max" must be greater than zero`,
)

// errCoverageMinRequired is returned when [thresholds].coverage_min is omitted.
var errCoverageMinRequired = errors.New("gate.toml [thresholds]: coverage_min is required")

var errCoverageMinOutOfRange = errors.New(
	`gate.toml [thresholds]: "coverage_min" must be a number from 0 to 100`,
)

// errCrapMaxRequired is returned when [thresholds].crap_max is omitted.
var errCrapMaxRequired = errors.New("gate.toml [thresholds]: crap_max is required")

var errCrapMaxOutOfRange = errors.New(
	`gate.toml [thresholds]: "crap_max" must be greater than zero`,
)

// errMutationSitesMaxRequired is returned when [thresholds].mutation_sites_max is omitted.
var errMutationSitesMaxRequired = errors.New("gate.toml [thresholds]: mutation_sites_max is required")

// errUnknownConfigKeys is returned when gate.toml contains keys not recognised by the current config schema.
var errUnknownConfigKeys = errors.New("gate.toml contains unrecognised keys (removed or misspelled)")

var errCompileTagsRequired = errors.New(`gate.toml [compile]: "tags" must include at least one build tag`)

var errPerformancePackagesRequired = errors.New(`gate.toml [performance]: "packages" is required`)

var errPerformanceTagsRequired = errors.New(`gate.toml [performance]: "tags" is required`)

var errStrictTimingPackagesRequired = errors.New(`gate.toml [stricttiming]: "packages" is required`)

var errStrictTimingTagsRequired = errors.New(`gate.toml [stricttiming]: "tags" is required`)

// errMutationCoverageMinOutOfRange is returned when [thresholds].mutation_coverage_min is present
// but not an integer in [0, 100] (same range contract as gate.MinMutationCoverage, where 0 is valid).
var errMutationCoverageMinOutOfRange = errors.New(
	`gate.toml [thresholds]: "mutation_coverage_min" must be an integer from 0 to 100`,
)

// errMutationKillsMinRateOutOfRange is returned when [thresholds].mutation_kills_min_rate is present
// but not an integer in [0, 100] (gate.MinKillRate, where 0 disables the threshold check).
var errMutationKillsMinRateOutOfRange = errors.New(
	`gate.toml [thresholds]: "mutation_kills_min_rate" must be an integer from 0 to 100`,
)

var errQualityScopePackagesRequired = errors.New(
	`gate.toml [quality_scope]: "packages" is required`,
)

var errMutationCoverageMinRequired = errors.New(
	`gate.toml [thresholds]: "mutation_coverage_min" is required`,
)

var errMutationKillsMinRateRequired = errors.New(
	`gate.toml [thresholds]: "mutation_kills_min_rate" is required`,
)

type config struct {
	Thresholds       thresholdConfig        `toml:"thresholds"`
	Lint             lintConfig             `toml:"lint"`
	QualityScope     qualityScopeConfig     `toml:"quality_scope"`
	Deadcode         deadcodeConfig         `toml:"deadcode"`
	Markdownlint     markdownlintConfig     `toml:"markdownlint"`
	Crap             crapConfig             `toml:"crap"`
	Gremlins         gremlinsConfig         `toml:"gremlins"`
	MutationKills    mutationKillsConfig    `toml:"mutation_kills"`
	Compile          compileConfig          `toml:"compile"`
	Unittests        unittestsConfig        `toml:"unittests"`
	Integrationtests integrationtestsConfig `toml:"integrationtests"`
	Performance      taggedTestConfig       `toml:"performance"`
	StrictTiming     taggedTestConfig       `toml:"stricttiming"`
}

type thresholdConfig struct {
	CoverageMin          *float64 `toml:"coverage_min"`
	CrapMax              *float64 `toml:"crap_max"`
	MutationSitesMax     *int     `toml:"mutation_sites_max"`
	MutationKillsMinRate *int     `toml:"mutation_kills_min_rate"`

	// MutationCoverageMin is explicit policy; 0 is valid and disables the coverage check.
	MutationCoverageMin *int `toml:"mutation_coverage_min"`
}

type lintConfig struct {
	Config             string   `toml:"config"`
	CustomGCL          string   `toml:"custom_gcl"`
	CustomLintToolSpec string   `toml:"custom_lint_tool_spec"`
	ToolSpec           string   `toml:"tool_spec"`
	Args               []string `toml:"args"`
}

type qualityScopeConfig struct {
	Packages         string   `toml:"packages"`
	Tags             []string `toml:"tags"`
	Exclude          []string `toml:"exclude"`
	TestFilePatterns []string `toml:"test_file_patterns"`
}

type deadcodeConfig struct {
	Args     []string `toml:"args"`
	ToolSpec string   `toml:"tool_spec"`
}

type markdownlintConfig struct {
	ToolSpec string   `toml:"tool_spec"`
	Args     []string `toml:"args"`
}

type crapConfig struct {
	ToolSpec string   `toml:"tool_spec"`
	Args     []string `toml:"args"`
}

// gremlinsConfig holds the shared gremlins module pin for MutationSites and MutationKills.
type gremlinsConfig struct {
	ToolSpec string   `toml:"tool_spec"`
	Args     []string `toml:"args"`
}

// mutationKillsConfig shares package scope from [quality_scope].
type mutationKillsConfig struct {
	Args []string `toml:"args"`
}

type compileConfig struct {
	Tags []string `toml:"tags"`
	Args []string `toml:"args"`
}

type unittestsConfig struct {
	DurationMax *float64 `toml:"duration_max"`
	Shuffle     bool     `toml:"shuffle"`
	Args        []string `toml:"args"`
}

// integrationtestsConfig is an optional second `go test` in the same gate (see magefiles).
type integrationtestsConfig struct {
	Tags    string   `toml:"tags"`
	Shuffle bool     `toml:"shuffle"`
	Args    []string `toml:"args"`
}

type taggedTestConfig struct {
	Packages string   `toml:"packages"`
	Tags     string   `toml:"tags"`
	Shuffle  bool     `toml:"shuffle"`
	Args     []string `toml:"args"`
}

// configReader reads file contents for loadConfig; production uses os.ReadFile.
type configReader func(string) ([]byte, error)

func loadConfig(path string, read configReader) (config, error) {
	// #nosec G304 -- path is supplied by the caller; production uses constant policyPath via os.ReadFile.
	data, err := read(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return config{}, errGateTomlMissing
		}
		return config{}, fmt.Errorf("read config: %w", err)
	}
	return parseConfig(data)
}

func parseConfig(data []byte) (config, error) {
	var parsed config
	md, err := toml.Decode(string(data), &parsed)
	if err != nil {
		return config{}, fmt.Errorf("parse config: %w", err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return config{}, fmt.Errorf("%w: %s", errUnknownConfigKeys, strings.Join(keys, ", "))
	}
	return validateParsedConfig(&md, &parsed)
}

func validateParsedConfig(md *toml.MetaData, parsed *config) (config, error) {
	validators := []func(*toml.MetaData, *config) error{
		validateMandatoryThresholds,
		validateUnittests,
		validateQualityScope,
		validateCompile,
		validatePerformance,
		validateStrictTiming,
		validateMutationCoverageMin,
		validateMutationKillsMinRate,
	}
	for _, validate := range validators {
		if err := validate(md, parsed); err != nil {
			return config{}, err
		}
	}
	return *parsed, nil
}

func requireDefinedFloat64Threshold(
	md *toml.MetaData,
	field string,
	value *float64,
	errMissing error,
) error {
	if !md.IsDefined("thresholds", field) {
		return errMissing
	}
	if value == nil {
		return errMissing
	}
	return nil
}

func requireFloat64Range(value, minValue, maxValue float64, errOutOfRange error) error {
	if value < minValue || value > maxValue {
		return fmt.Errorf("%w: got %.2f", errOutOfRange, value)
	}

	return nil
}

func requireDefinedIntThreshold(
	md *toml.MetaData,
	field string,
	value *int,
	errMissing error,
) error {
	if !md.IsDefined("thresholds", field) {
		return errMissing
	}
	if value == nil {
		return errMissing
	}
	return nil
}

// validateMandatoryThresholds enforces explicit policy in gate.toml (no implicit zero thresholds).
// The [mutation_kills] args table is optional because kill mode is on-demand, but
// thresholds.mutation_kills_min_rate is still required so kill-rate policy is explicit.
func validateMandatoryThresholds(md *toml.MetaData, cfg *config) error {
	if err := requireDefinedFloat64Threshold(
		md, "coverage_min", cfg.Thresholds.CoverageMin, errCoverageMinRequired,
	); err != nil {
		return err
	}
	if err := requireFloat64Range(
		*cfg.Thresholds.CoverageMin, 0, percentThresholdMax, errCoverageMinOutOfRange,
	); err != nil {
		return err
	}
	if err := requireDefinedFloat64Threshold(md, "crap_max", cfg.Thresholds.CrapMax, errCrapMaxRequired); err != nil {
		return err
	}
	if *cfg.Thresholds.CrapMax <= 0 {
		return fmt.Errorf("%w: got %.2f", errCrapMaxOutOfRange, *cfg.Thresholds.CrapMax)
	}
	if err := requireDefinedIntThreshold(
		md, "mutation_sites_max", cfg.Thresholds.MutationSitesMax, errMutationSitesMaxRequired,
	); err != nil {
		return err
	}
	return nil
}

func validateUnittests(md *toml.MetaData, cfg *config) error {
	if !md.IsDefined("unittests", "duration_max") {
		return errUnittestsDurationMaxRequired
	}
	if cfg.Unittests.DurationMax == nil {
		return errUnittestsDurationMaxRequired
	}
	if *cfg.Unittests.DurationMax <= 0 {
		return errUnittestsDurationMaxOutOfRange
	}
	return nil
}

func validateQualityScope(md *toml.MetaData, cfg *config) error {
	if !md.IsDefined("quality_scope", "packages") {
		return errQualityScopePackagesRequired
	}
	if strings.TrimSpace(cfg.QualityScope.Packages) == "" {
		return errQualityScopePackagesRequired
	}
	return nil
}

func validateCompile(md *toml.MetaData, cfg *config) error {
	if !md.IsDefined("compile", "tags") {
		return errCompileTagsRequired
	}
	for _, tag := range cfg.Compile.Tags {
		if strings.TrimSpace(tag) != "" {
			return nil
		}
	}
	return errCompileTagsRequired
}

func validatePerformance(md *toml.MetaData, cfg *config) error {
	return validateTaggedTest(
		md, "performance", cfg.Performance, errPerformancePackagesRequired, errPerformanceTagsRequired,
	)
}

func validateStrictTiming(md *toml.MetaData, cfg *config) error {
	return validateTaggedTest(
		md, "stricttiming", cfg.StrictTiming, errStrictTimingPackagesRequired, errStrictTimingTagsRequired,
	)
}

func validateTaggedTest(
	md *toml.MetaData,
	table string,
	cfg taggedTestConfig,
	errPackages error,
	errTags error,
) error {
	if !md.IsDefined(table, "packages") || strings.TrimSpace(cfg.Packages) == "" {
		return errPackages
	}
	if !md.IsDefined(table, "tags") || strings.TrimSpace(cfg.Tags) == "" {
		return errTags
	}
	return nil
}

// validateMutationCoverageMin enforces explicit 0-100 mutation_coverage_min policy.
func validateMutationCoverageMin(md *toml.MetaData, cfg *config) error {
	if !md.IsDefined("thresholds", "mutation_coverage_min") {
		return errMutationCoverageMinRequired
	}
	if cfg.Thresholds.MutationCoverageMin == nil {
		return errMutationCoverageMinOutOfRange
	}
	v := *cfg.Thresholds.MutationCoverageMin
	if v < 0 || v > 100 {
		return fmt.Errorf("%w: got %d", errMutationCoverageMinOutOfRange, v)
	}
	return nil
}

func validateMutationKillsMinRate(md *toml.MetaData, cfg *config) error {
	if !md.IsDefined("thresholds", "mutation_kills_min_rate") {
		return errMutationKillsMinRateRequired
	}
	if cfg.Thresholds.MutationKillsMinRate == nil {
		return errMutationKillsMinRateOutOfRange
	}
	v := *cfg.Thresholds.MutationKillsMinRate
	if v < 0 || v > 100 {
		return fmt.Errorf("%w: got %d", errMutationKillsMinRateOutOfRange, v)
	}
	return nil
}

func (cfg *config) packages() string {
	return cfg.QualityScope.Packages
}
