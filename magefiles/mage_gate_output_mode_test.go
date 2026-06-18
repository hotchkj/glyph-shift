//go:build mage
// +build mage

package main

import (
	"testing"

	"github.com/hotchkj/mage-gate/gate"
)

func TestIsCIReflectsEnvironment(t *testing.T) {
	t.Setenv("CI", "")
	if isCI() {
		t.Fatal("isCI = true, want false when CI is empty")
	}
	t.Setenv("CI", "true")
	if !isCI() {
		t.Fatal("isCI = false, want true when CI is set")
	}
}

func TestGateOutputModeSelectsVerboseInCI(t *testing.T) {
	t.Setenv("CI", "")
	if got := gateOutputMode(); got != gate.OutputModeAgent {
		t.Fatalf("gateOutputMode = %q, want agent when CI unset", got)
	}
	t.Setenv("CI", "true")
	if got := gateOutputMode(); got != gate.OutputModeVerbose {
		t.Fatalf("gateOutputMode = %q, want verbose when CI is set", got)
	}
}

func TestNewRunnerWiresGateOutputMode(t *testing.T) {
	t.Setenv("CI", "true")
	runner, err := newRunner()
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}
	if got := gate.RunnerOutputMode(runner); got != gate.OutputModeVerbose {
		t.Fatalf("RunnerOutputMode = %q, want verbose in CI", got)
	}
	t.Setenv("CI", "")
	runner, err = newRunner()
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}
	if got := gate.RunnerOutputMode(runner); got != gate.OutputModeAgent {
		t.Fatalf("RunnerOutputMode = %q, want agent outside CI", got)
	}
}
