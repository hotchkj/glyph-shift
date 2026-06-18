//go:build mage
// +build mage

package main

import (
	"errors"
	"fmt"

	"github.com/hotchkj/mage-gate/gate"
	"github.com/magefile/mage/mg"
)

var errDeadcodeToolSpecEmpty = errors.New("deadcode: gate.toml [deadcode].tool_spec is empty")

func loadConfigAndScope() (config, gate.QualityScope, gate.PackageScope, error) {
	cfg, err := loadConfig(policyPath, readPolicyFile)
	if err != nil {
		return config{}, gate.QualityScope{}, gate.PackageScope{}, fmt.Errorf("load config: %w", err)
	}
	qualityScope, err := gate.NewQualityScope(cfg.packages(), qualityScopeOptions(&cfg)...)
	if err != nil {
		return config{}, gate.QualityScope{}, gate.PackageScope{}, fmt.Errorf("create quality scope: %w", err)
	}
	pkgScope, err := gate.NewPackageScope(qualityScope.Packages())
	if err != nil {
		return config{}, gate.QualityScope{}, gate.PackageScope{}, fmt.Errorf("create package scope: %w", err)
	}
	return cfg, qualityScope, pkgScope, nil
}

// Compile runs compile-only verification (go build) over [gate.toml] package scope.
func Compile() error {
	return withGateRuntime((*gateRuntime).compile)
}

// Format runs golangci-lint fmt (apply) using [lint] from gate.toml.
func Format() error {
	return withGateRuntime((*gateRuntime).format)
}

// Lint runs golangci-lint using [lint] from gate.toml.
func Lint() error {
	return withGateRuntime((*gateRuntime).lint)
}

// MarkdownLint runs gomarklint when [markdownlint].tool_spec is set in gate.toml.
func MarkdownLint() error {
	return withGateRuntime((*gateRuntime).markdownLint)
}

// Vet runs go vet over the configured package scope.
func Vet() error {
	return withGateRuntime((*gateRuntime).vet)
}

// Deadcode runs deadcode when [deadcode].tool_spec is set in gate.toml.
func Deadcode() error {
	return withGateRuntime((*gateRuntime).deadcode)
}

// Coverage runs the coverage-bearing CoveredTest pass on the quality scope, per-package duration
// bound against [unittests].duration_max, then coverage percent and CRAP gates.
func Coverage() error {
	return withGateRuntime((*gateRuntime).coverageChecks)
}

// MutationScan runs the gremlins dry-run scan and stores its output for mutation gates.
func MutationScan() error {
	return withGateRuntime((*gateRuntime).mutationScan)
}

// MutationCoverage enforces mutation test-profile coverage from kill output when available,
// otherwise from the shared dry-run scan output.
func MutationCoverage() error {
	return withGateRuntime((*gateRuntime).mutationCoverage)
}

// MutationSites enforces the per-file mutation site budget from kill output when available,
// otherwise from the shared dry-run scan output.
func MutationSites() error {
	return withGateRuntime((*gateRuntime).mutationSites)
}

// MutationKills runs gremlins mutation testing in kill mode for [quality_scope].packages.
// It is intentionally separate from Validate/pre-commit; use it for slower focused mutation checks.
func MutationKills() error {
	return withGateRuntime((*gateRuntime).mutationKills)
}

// ValidateCore runs the quality gate without integration-tagged tests.
// Kill-mode mutation testing remains separate in MutationKills.
func ValidateCore() {
	mg.SerialDeps(
		Lint, Deadcode, Vet, Compile, MarkdownLint, Test, Coverage, Performance,
		MutationScan, MutationCoverage, MutationSites,
	)
}

// Validate runs ValidateCore then Integration (CrossCompile + staged release CLI).
// Kill-mode mutation testing remains separate in MutationKills.
func Validate() {
	mg.SerialDeps(ValidateCore, Integration)
}
