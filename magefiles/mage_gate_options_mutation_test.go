//go:build mage
// +build mage

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/gate"
	"github.com/hotchkj/mage-gate/gatetest"
)

func TestGremlinsMutationOptions_Args(t *testing.T) {
	t.Parallel()

	policy := strings.ReplaceAll(minimalGatePolicy,
		`tool_spec = "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"`,
		`tool_spec = "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"
args = ["--dry-run-timeout=30s"]`,
	)
	cfg, err := parseConfig([]byte(policy))
	if err != nil {
		t.Fatal(err)
	}

	mem := rootedOptionsMemoryFileOps(t)
	inner := newOptionsFakeRunner(mem, testGremlinsScanReport)
	runner := mustNewDisplayRunner(t, inner)
	resolver := gatetest.NewFakeToolResolver()
	artifactStore := newArtifactStore()
	scope, err := gate.NewQualityScope(testPkgPattern)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	inv := mustQualityScopeInventory(t, ctx, runner, artifactStore, mem, testFakeModuleRoot, scope)
	mr, err := gate.NewMutationRunner(runner, resolver, artifactStore, mem)
	if err != nil {
		t.Fatal(err)
	}
	_, err = mr.Scan(ctx, testFakeModuleRoot, scope, inv, gremlinsToolSpec(&cfg), gremlinsMutationOptions(&cfg)...)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !goRunArgsContain(inner.Calls(), "--dry-run-timeout=30s") {
		t.Fatalf("expected gremlins extra args, calls=%v", inner.Calls())
	}
}

func TestMutationKillsOptions_ComposesGremlinsAndKillArgs(t *testing.T) {
	t.Parallel()

	cfg, err := parseConfig([]byte(minimalGatePolicy))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Gremlins.Args = []string{"--gremlins-only=1"}
	cfg.MutationKills.Args = []string{"--kills-only=2"}
	cfgPtr := &cfg

	mem := rootedOptionsMemoryFileOps(t)
	inner := newOptionsFakeRunner(mem, testGremlinsKillReport)
	runner := mustNewDisplayRunner(t, inner)
	resolver := gatetest.NewFakeToolResolver()
	artifactStore := newArtifactStore()
	scope, err := gate.NewQualityScope(testPkgPattern)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	inv := mustQualityScopeInventory(t, ctx, runner, artifactStore, mem, testFakeModuleRoot, scope)
	mr, err := gate.NewMutationRunner(runner, resolver, artifactStore, mem)
	if err != nil {
		t.Fatal(err)
	}
	_, err = mr.Kill(
		ctx,
		testFakeModuleRoot,
		scope,
		inv,
		gremlinsToolSpec(cfgPtr),
		mutationKillsOptions(cfgPtr)...,
	)
	if err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if !goRunArgsContain(inner.Calls(), "--gremlins-only=1") || !goRunArgsContain(inner.Calls(), "--kills-only=2") {
		t.Fatalf("expected composed gremlins argv, calls=%v", inner.Calls())
	}
}

func TestThresholdAdapters(t *testing.T) {
	t.Parallel()

	cfg, err := parseConfig([]byte(minimalGatePolicy))
	if err != nil {
		t.Fatal(err)
	}
	if coverageMin(&cfg) != gate.MinPercent(45.5) {
		t.Fatalf("coverageMin = %#v", coverageMin(&cfg))
	}
	if crapMax(&cfg) != gate.MaxScore(200.0) {
		t.Fatalf("crapMax = %#v", crapMax(&cfg))
	}
	if mutationSitesMax(&cfg) != gate.MaxSites(100) {
		t.Fatalf("mutationSitesMax = %#v", mutationSitesMax(&cfg))
	}
	if unitTestDurationMax(&cfg) != gate.MaxSeconds(300.0) {
		t.Fatalf("unitTestDurationMax = %#v", unitTestDurationMax(&cfg))
	}
	if mutationCoverageMin(&cfg) != gate.MinMutationCoverage(0) {
		t.Fatalf("mutationCoverageMin = %#v", mutationCoverageMin(&cfg))
	}
	if mutationKillsMinRate(&cfg) != gate.MinKillRate(0) {
		t.Fatalf("mutationKillsMinRate = %#v", mutationKillsMinRate(&cfg))
	}
}
