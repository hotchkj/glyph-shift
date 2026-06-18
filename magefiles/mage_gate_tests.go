//go:build mage
// +build mage

package main

import "github.com/magefile/mage/mg"

// Test runs the primary correctness pass: one gate.CoveredTest over [quality_scope] packages.
// That pass includes unit tests plus the non-performance feature suite selected by the normal
// ./... package scope. Quality scope exclusions filter scoring surfaces, not the package pattern
// under test. Coverage percent, CRAP, and duration gates are applied by Coverage/Validate.
func Test() error {
	return withGateRuntime((*gateRuntime).test)
}

// Integration runs integration-tagged tests for [integrationtests] in gate.toml.
// It depends on CrossCompile (GoReleaser snapshot) and stages the host release CLI into bin/
// before tests. Use mage integration or validate, not bare go test -tags integration.
func Integration() error {
	mg.Deps(CrossCompile)
	if err := stageHostReleaseCLI(); err != nil {
		return err
	}
	return withGateRuntime((*gateRuntime).integration)
}

// Performance runs the portable BDD performance suite configured by [performance] in gate.toml.
// Validate includes this target as a non-coverage quality phase.
func Performance() error {
	return withGateRuntime((*gateRuntime).performance)
}

// StrictTiming runs the wall-clock BDD suite configured by [stricttiming] in gate.toml.
func StrictTiming() error {
	return withGateRuntime((*gateRuntime).strictTiming)
}
