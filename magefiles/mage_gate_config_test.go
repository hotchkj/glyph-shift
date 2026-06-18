//go:build mage
// +build mage

package main

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

const minimalGatePolicy = `
[thresholds]
coverage_min = 45.5
crap_max = 200.0
mutation_sites_max = 100
mutation_coverage_min = 0
mutation_kills_min_rate = 0

[lint]
config = ".golangci.yml"
tool_spec = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4"

[quality_scope]
packages = "./..."
tags = ["mage"]
exclude = ["features", "integrations", "internal/testutil", "internal/goldenreader", "testdata"]
test_file_patterns = ["*_test.go"]

[deadcode]
args = ["-test"]
tool_spec = "golang.org/x/tools/cmd/deadcode@v0.44.0"

[crap]
tool_spec = "github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"

[gremlins]
tool_spec = "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"

[mutation_kills]
args = ["--workers=1", "--test-cpu=1", "--timeout-coefficient=1"]

[compile]
tags = ["integration", "performance", "bdd_strict_timing"]

[unittests]
duration_max = 300.0
shuffle = true

[integrationtests]
tags = "integration"
shuffle = true

[performance]
packages = "./features"
tags = "performance"
shuffle = true

[stricttiming]
packages = "./features"
tags = "bdd_strict_timing"
shuffle = true
`

const allGoPackagesPattern = "./..."

var errReadConfigDeviceNotReady = errors.New("device not ready")

func TestParseConfigMutationKillsPolicy(t *testing.T) {
	t.Parallel()

	cfg, err := parseConfig([]byte(minimalGatePolicy))
	if err != nil {
		t.Fatal(err)
	}

	packages := cfg.packages()
	if packages != allGoPackagesPattern {
		t.Fatalf("mutation kill packages = %q, want %q", packages, allGoPackagesPattern)
	}
	if len(cfg.MutationKills.Args) != 3 {
		t.Fatalf("mutation kill args length = %d, want 3", len(cfg.MutationKills.Args))
	}
}

func TestParseConfigAllowsZeroMutationCoverageMin(t *testing.T) {
	t.Parallel()

	policy := []byte(minimalGatePolicy)
	if _, err := parseConfig(policy); err != nil {
		t.Fatalf("parseConfig error = %v, want nil", err)
	}
}

func TestParseConfigRequiresMutationCoverageMin(t *testing.T) {
	t.Parallel()

	policy := []byte(strings.ReplaceAll(minimalGatePolicy, "mutation_coverage_min = 0\n", ""))
	if _, err := parseConfig(policy); !errors.Is(err, errMutationCoverageMinRequired) {
		t.Fatalf("parseConfig error = %v, want %v", err, errMutationCoverageMinRequired)
	}
}

func TestParseConfigRejectsOutOfRangeMutationCoverageMin(t *testing.T) {
	t.Parallel()

	lines := make([]string, 0, 32)
	for _, line := range strings.Split(minimalGatePolicy, "\n") {
		if strings.TrimSpace(line) == "mutation_coverage_min = 0" {
			line = "mutation_coverage_min = 101"
		}
		lines = append(lines, line)
	}
	policy := []byte(strings.Join(lines, "\n"))
	_, err := parseConfig(policy)
	if !errors.Is(err, errMutationCoverageMinOutOfRange) {
		t.Fatalf("parseConfig error = %v, want %v", err, errMutationCoverageMinOutOfRange)
	}
}

func TestParseConfigRequiresMutationKillsMinRate(t *testing.T) {
	t.Parallel()

	policy := []byte(strings.ReplaceAll(minimalGatePolicy, "mutation_kills_min_rate = 0\n", ""))
	if _, err := parseConfig(policy); !errors.Is(err, errMutationKillsMinRateRequired) {
		t.Fatalf("parseConfig error = %v, want %v", err, errMutationKillsMinRateRequired)
	}
}

func TestParseConfigRequiresQualityScopePackages(t *testing.T) {
	t.Parallel()

	lines := make([]string, 0, 32)
	for _, line := range strings.Split(minimalGatePolicy, "\n") {
		if strings.TrimSpace(line) == `packages = "./..."` {
			continue
		}
		lines = append(lines, line)
	}
	policy := []byte(strings.Join(lines, "\n"))
	if _, err := parseConfig(policy); !errors.Is(err, errQualityScopePackagesRequired) {
		t.Fatalf("parseConfig error = %v, want %v", err, errQualityScopePackagesRequired)
	}
}

func TestParseConfigRequiresCompileTags(t *testing.T) {
	t.Parallel()

	policy := []byte(strings.ReplaceAll(
		minimalGatePolicy,
		`tags = ["integration", "performance", "bdd_strict_timing"]`,
		"",
	))
	if _, err := parseConfig(policy); !errors.Is(err, errCompileTagsRequired) {
		t.Fatalf("parseConfig error = %v, want %v", err, errCompileTagsRequired)
	}
}

func TestParseConfigRequiresPerformancePackages(t *testing.T) {
	t.Parallel()

	policy := []byte(strings.Replace(minimalGatePolicy, "packages = \"./features\"\n", "", 1))
	if _, err := parseConfig(policy); !errors.Is(err, errPerformancePackagesRequired) {
		t.Fatalf("parseConfig error = %v, want %v", err, errPerformancePackagesRequired)
	}
}

func TestParseConfigRequiresPerformanceTags(t *testing.T) {
	t.Parallel()

	policy := []byte(strings.ReplaceAll(minimalGatePolicy, "tags = \"performance\"\n", ""))
	if _, err := parseConfig(policy); !errors.Is(err, errPerformanceTagsRequired) {
		t.Fatalf("parseConfig error = %v, want %v", err, errPerformanceTagsRequired)
	}
}

func TestParseConfigRequiresStrictTimingPackages(t *testing.T) {
	t.Parallel()

	policy := []byte(strings.Replace(
		minimalGatePolicy,
		"[stricttiming]\npackages = \"./features\"\n",
		"[stricttiming]\n",
		1,
	))
	if _, err := parseConfig(policy); !errors.Is(err, errStrictTimingPackagesRequired) {
		t.Fatalf("parseConfig error = %v, want %v", err, errStrictTimingPackagesRequired)
	}
}

func TestParseConfigRequiresStrictTimingTags(t *testing.T) {
	t.Parallel()

	policy := []byte(strings.ReplaceAll(minimalGatePolicy, "tags = \"bdd_strict_timing\"\n", ""))
	if _, err := parseConfig(policy); !errors.Is(err, errStrictTimingTagsRequired) {
		t.Fatalf("parseConfig error = %v, want %v", err, errStrictTimingTagsRequired)
	}
}

func TestParseConfigRequiresUnittestsDurationMax(t *testing.T) {
	t.Parallel()

	policy := []byte(strings.ReplaceAll(minimalGatePolicy, "duration_max = 300.0\n", ""))
	if _, err := parseConfig(policy); !errors.Is(err, errUnittestsDurationMaxRequired) {
		t.Fatalf("parseConfig error = %v, want %v", err, errUnittestsDurationMaxRequired)
	}
}

func TestParseConfigRejectsNonPositiveUnittestsDurationMax(t *testing.T) {
	t.Parallel()

	policy := []byte(strings.ReplaceAll(minimalGatePolicy, "duration_max = 300.0", "duration_max = 0"))
	if _, err := parseConfig(policy); !errors.Is(err, errUnittestsDurationMaxOutOfRange) {
		t.Fatalf("parseConfig error = %v, want %v", err, errUnittestsDurationMaxOutOfRange)
	}
}

func TestMutationKillsAndIntegrationShareQualityScopePackages(t *testing.T) {
	t.Parallel()

	cfg, err := parseConfig([]byte(minimalGatePolicy))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.packages(); got != allGoPackagesPattern {
		t.Fatalf("mutation kill packages = %q, want %q", got, allGoPackagesPattern)
	}
	if cfg.packages() != allGoPackagesPattern {
		t.Fatalf("quality scope packages = %q, want %q", cfg.packages(), allGoPackagesPattern)
	}
}

func TestLoadConfig_GateTomlMissing(t *testing.T) {
	t.Parallel()

	_, err := loadConfig("/nonexistent/gate.toml", func(string) ([]byte, error) {
		return nil, fs.ErrNotExist
	})
	if !errors.Is(err, errGateTomlMissing) {
		t.Fatalf("loadConfig error = %v, want %v", err, errGateTomlMissing)
	}
}

func TestLoadConfig_ReadErrorWraps(t *testing.T) {
	t.Parallel()

	_, err := loadConfig("gate.toml", func(string) ([]byte, error) {
		return nil, errReadConfigDeviceNotReady
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errReadConfigDeviceNotReady) {
		t.Fatalf("errors.Is(err, readErr) = false, err = %v", err)
	}
}

func TestParseConfig_InvalidToml(t *testing.T) {
	t.Parallel()

	_, err := parseConfig([]byte(`[[[invalid`))
	if err == nil {
		t.Fatal("expected parse error")
	}
	var parseErr toml.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("parseConfig error = %v, want toml.ParseError", err)
	}
}

func TestParseConfig_UnknownKeys(t *testing.T) {
	t.Parallel()

	policy := minimalGatePolicy + "\n[orphan]\nunexpected_key = true\n"
	_, err := parseConfig([]byte(policy))
	if !errors.Is(err, errUnknownConfigKeys) {
		t.Fatalf("parseConfig error = %v, want %v", err, errUnknownConfigKeys)
	}
}

func TestParseConfig_RequiresCoverageMin(t *testing.T) {
	t.Parallel()

	policy := strings.ReplaceAll(minimalGatePolicy, "coverage_min = 45.5\n", "")
	_, err := parseConfig([]byte(policy))
	if !errors.Is(err, errCoverageMinRequired) {
		t.Fatalf("parseConfig error = %v, want %v", err, errCoverageMinRequired)
	}
}

func TestParseConfig_RejectsOutOfRangeCoverageMin(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		value string
	}{
		{name: "negative", value: "-0.1"},
		{name: "above one hundred", value: "100.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			policy := strings.ReplaceAll(minimalGatePolicy, "coverage_min = 45.5", "coverage_min = "+tc.value)
			_, err := parseConfig([]byte(policy))
			if !errors.Is(err, errCoverageMinOutOfRange) {
				t.Fatalf("parseConfig error = %v, want %v", err, errCoverageMinOutOfRange)
			}
		})
	}
}

func TestParseConfig_RequiresCrapMax(t *testing.T) {
	t.Parallel()

	policy := strings.ReplaceAll(minimalGatePolicy, "crap_max = 200.0\n", "")
	_, err := parseConfig([]byte(policy))
	if !errors.Is(err, errCrapMaxRequired) {
		t.Fatalf("parseConfig error = %v, want %v", err, errCrapMaxRequired)
	}
}

func TestParseConfig_RejectsNonPositiveCrapMax(t *testing.T) {
	t.Parallel()

	policy := strings.ReplaceAll(minimalGatePolicy, "crap_max = 200.0", "crap_max = 0")
	_, err := parseConfig([]byte(policy))
	if !errors.Is(err, errCrapMaxOutOfRange) {
		t.Fatalf("parseConfig error = %v, want %v", err, errCrapMaxOutOfRange)
	}
}

func TestParseConfig_RequiresMutationSitesMax(t *testing.T) {
	t.Parallel()

	policy := strings.ReplaceAll(minimalGatePolicy, "mutation_sites_max = 100\n", "")
	_, err := parseConfig([]byte(policy))
	if !errors.Is(err, errMutationSitesMaxRequired) {
		t.Fatalf("parseConfig error = %v, want %v", err, errMutationSitesMaxRequired)
	}
}

func TestParseConfig_MutationCoverageMinNegative(t *testing.T) {
	t.Parallel()

	policy := strings.ReplaceAll(minimalGatePolicy, "mutation_coverage_min = 0", "mutation_coverage_min = -1")
	_, err := parseConfig([]byte(policy))
	if !errors.Is(err, errMutationCoverageMinOutOfRange) {
		t.Fatalf("parseConfig error = %v, want %v", err, errMutationCoverageMinOutOfRange)
	}
}

func TestParseConfig_MutationKillsMinRateNegative(t *testing.T) {
	t.Parallel()

	policy := strings.ReplaceAll(minimalGatePolicy, "mutation_kills_min_rate = 0", "mutation_kills_min_rate = -3")
	_, err := parseConfig([]byte(policy))
	if !errors.Is(err, errMutationKillsMinRateOutOfRange) {
		t.Fatalf("parseConfig error = %v, want %v", err, errMutationKillsMinRateOutOfRange)
	}
}

func TestParseConfig_QualityScopePackagesWhitespaceOnly(t *testing.T) {
	t.Parallel()

	lines := make([]string, 0, 48)
	for _, line := range strings.Split(minimalGatePolicy, "\n") {
		if strings.TrimSpace(line) == `packages = "./..."` {
			line = `packages = "   "`
		}
		lines = append(lines, line)
	}
	policy := strings.Join(lines, "\n")
	_, err := parseConfig([]byte(policy))
	if !errors.Is(err, errQualityScopePackagesRequired) {
		t.Fatalf("parseConfig error = %v, want %v", err, errQualityScopePackagesRequired)
	}
}
