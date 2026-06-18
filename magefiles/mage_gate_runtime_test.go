//go:build mage
// +build mage

package main

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/gate"
	"github.com/hotchkj/mage-gate/gatetest"
)

func newRuntimeTestScope(t *testing.T, cfg *config) (gate.QualityScope, gate.PackageScope) {
	t.Helper()

	qualityScope, err := gate.NewQualityScope(cfg.packages(), qualityScopeOptions(cfg)...)
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	pkgScope, err := gate.NewPackageScope(qualityScope.Packages())
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}

	return qualityScope, pkgScope
}

func newRuntimeTestTarget(t *testing.T, cfg *config, mem *gatetest.MemoryFileOps) *gateRuntime {
	t.Helper()
	if err := mem.Root(testFakeModuleRoot); err != nil {
		t.Fatalf("mem.Root: %v", err)
	}

	runner := mustNewDisplayRunner(t, newOptionsFakeRunner(mem, testGremlinsScanReport))
	resolver := gatetest.NewFakeToolResolver()
	resolver.SetLocalMatch("golangci-lint", testGolangCILintSpec, true)
	resolver.SetLocalMatch("gocyclo", testGocycloToolSpec, true)
	resolver.SetLocalMatch("deadcode", "golang.org/x/tools/cmd/deadcode@v0.44.0", true)
	qualityScope, pkgScope := newRuntimeTestScope(t, cfg)
	artifactStore := newArtifactStore()

	return &gateRuntime{
		ctx:          context.Background(),
		cfg:          *cfg,
		qualityScope: qualityScope,
		pkgScope:     pkgScope,
		runner:       runner,
		resolver:     resolver,
		store:        artifactStore,
		fileOps:      mem,
		root:         testFakeModuleRoot,
	}
}

func countGoRunWithArg(calls []cmdrunner.Command, want string) int {
	count := 0
	for _, call := range calls {
		if call.Name() != "go" || call.Arg(0) != "run" {
			continue
		}
		for _, arg := range call.Args() {
			if arg == want {
				count++
				break
			}
		}
	}

	return count
}

func TestLoadConfigAndScopeUsesInjectedPolicyReader(t *testing.T) {
	oldReadPolicyFile := readPolicyFile
	t.Cleanup(func() { readPolicyFile = oldReadPolicyFile })

	readPolicyFile = func(path string) ([]byte, error) {
		if path != policyPath {
			t.Fatalf("read policy path = %q, want %q", path, policyPath)
		}

		return []byte(minimalGatePolicy), nil
	}

	cfg, qualityScope, pkgScope, err := loadConfigAndScope()
	if err != nil {
		t.Fatalf("loadConfigAndScope: %v", err)
	}
	if cfg.packages() != allGoPackagesPattern {
		t.Fatalf("packages = %q, want %q", cfg.packages(), allGoPackagesPattern)
	}
	if qualityScope.Packages() != allGoPackagesPattern {
		t.Fatalf("quality scope packages = %q, want %q", qualityScope.Packages(), allGoPackagesPattern)
	}
	if pkgScope.Packages() != allGoPackagesPattern {
		t.Fatalf("package scope packages = %q, want %q", pkgScope.Packages(), allGoPackagesPattern)
	}
}

func installCurrentRuntime(t *testing.T, rt *gateRuntime) {
	t.Helper()

	gateRuntimeState.mu.Lock()
	oldCurrent := gateRuntimeState.current
	oldErr := gateRuntimeState.err
	gateRuntimeState.current = rt
	gateRuntimeState.err = nil
	gateRuntimeState.mu.Unlock()

	t.Cleanup(func() {
		gateRuntimeState.mu.Lock()
		gateRuntimeState.current = oldCurrent
		gateRuntimeState.err = oldErr
		gateRuntimeState.mu.Unlock()
	})
}

func TestGateRuntimeTargetsDispatchThroughSharedRuntime(t *testing.T) {
	cfg, err := parseConfig([]byte(minimalGatePolicy))
	if err != nil {
		t.Fatal(err)
	}
	mem := gatetest.NewMemoryFileOps()
	withReleaseCLITestSeams(t, mem)
	writeHostReleaseCLIBinaryDistFixture(t, mem, []byte("dispatch-release-cli"))
	stubCrossCompileRunner(t, mem)
	inner := newOptionsFakeRunner(mem, testGremlinsKillReport)
	rt := newRuntimeTestTarget(t, &cfg, mem)
	rt.runner = mustNewDisplayRunner(t, inner)
	installCurrentRuntime(t, rt)

	targets := []struct {
		name string
		run  func() error
	}{
		{name: "Compile", run: Compile},
		{name: "Lint", run: Lint},
		{name: "Vet", run: Vet},
		{name: "Deadcode", run: Deadcode},
		{name: "Test", run: Test},
		{name: "Coverage", run: Coverage},
		{name: "MutationScan", run: MutationScan},
		{name: "MutationCoverage", run: MutationCoverage},
		{name: "MutationSites", run: MutationSites},
		{name: "MutationKills", run: MutationKills},
		{name: "Integration", run: Integration},
		{name: "Performance", run: Performance},
		{name: "StrictTiming", run: StrictTiming},
	}
	for _, target := range targets {
		if err := target.run(); err != nil {
			t.Fatalf("%s: %v", target.name, err)
		}
	}

	wantOut, absErr := absInRoot(cliBinaryOutputRel())
	if absErr != nil {
		t.Fatalf("absInRoot output: %v", absErr)
	}
	staged, readErr := fileOps.ReadFile(wantOut)
	if readErr != nil {
		t.Fatalf("ReadFile staged CLI %q: %v", wantOut, readErr)
	}
	if string(staged) != "dispatch-release-cli" {
		t.Fatalf("staged CLI = %q, want dispatch-release-cli", string(staged))
	}
	integrationTag := goBuildTagsArg + strings.TrimSpace(cfg.Integrationtests.Tags)
	if !sawGoTestArg(inner.Calls(), integrationTag) {
		t.Fatalf("integration pass did not run with tag %q", cfg.Integrationtests.Tags)
	}
}

func TestCurrentGateRuntimeBuildsAndCachesRuntime(t *testing.T) {
	oldReadPolicyFile := readPolicyFile
	oldNewRunner := newRunner
	oldNewResolver := newResolver
	oldStore := store
	oldFileOps := fileOps
	oldRootDir := rootDir
	gateRuntimeState.mu.Lock()
	oldCurrent := gateRuntimeState.current
	oldErr := gateRuntimeState.err
	gateRuntimeState.current = nil
	gateRuntimeState.err = nil
	gateRuntimeState.mu.Unlock()
	t.Cleanup(func() {
		readPolicyFile = oldReadPolicyFile
		newRunner = oldNewRunner
		newResolver = oldNewResolver
		store = oldStore
		fileOps = oldFileOps
		rootDir = oldRootDir
		gateRuntimeState.mu.Lock()
		gateRuntimeState.current = oldCurrent
		gateRuntimeState.err = oldErr
		gateRuntimeState.mu.Unlock()
	})

	mem := gatetest.NewMemoryFileOps()
	fileOps = mem
	store = newArtifactStore()
	rootDir = testFakeModuleRoot
	readPolicyFile = func(string) ([]byte, error) {
		return []byte(minimalGatePolicy), nil
	}
	inner := newOptionsFakeRunner(mem, testGremlinsScanReport)
	newRunner = func() (gate.CommandRunner, error) {
		return mustNewDisplayRunner(t, inner), nil
	}
	newResolver = func() gate.ToolResolver {
		resolver := gatetest.NewFakeToolResolver()
		resolver.SetLocalMatch("golangci-lint", testGolangCILintSpec, true)
		return resolver
	}

	first, err := currentGateRuntime()
	if err != nil {
		t.Fatalf("first currentGateRuntime: %v", err)
	}
	second, err := currentGateRuntime()
	if err != nil {
		t.Fatalf("second currentGateRuntime: %v", err)
	}
	if first != second {
		t.Fatalf("currentGateRuntime did not cache runtime: first=%p second=%p", first, second)
	}
}

func TestLoadConfigAndScopeReadErrorWraps(t *testing.T) {
	oldReadPolicyFile := readPolicyFile
	t.Cleanup(func() { readPolicyFile = oldReadPolicyFile })

	readPolicyFile = func(string) ([]byte, error) {
		return nil, errReadConfigDeviceNotReady
	}

	_, _, _, err := loadConfigAndScope()
	if !errors.Is(err, errReadConfigDeviceNotReady) {
		t.Fatalf("loadConfigAndScope error = %v, want %v", err, errReadConfigDeviceNotReady)
	}
}

func TestGateRuntimeDeadcodeEmptyToolSpec(t *testing.T) {
	t.Parallel()

	cfg, err := parseConfig([]byte(minimalGatePolicy))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Deadcode.ToolSpec = " \t "
	rt := newRuntimeTestTarget(t, &cfg, gatetest.NewMemoryFileOps())

	if err := rt.deadcode(); !errors.Is(err, errDeadcodeToolSpecEmpty) {
		t.Fatalf("deadcode error = %v, want %v", err, errDeadcodeToolSpecEmpty)
	}
}

func TestGateRuntimeMutationScanTokenCachesOutput(t *testing.T) {
	t.Parallel()

	cfg, err := parseConfig([]byte(minimalGatePolicy))
	if err != nil {
		t.Fatal(err)
	}
	mem := gatetest.NewMemoryFileOps()
	inner := newOptionsFakeRunner(mem, testGremlinsScanReport)
	rt := newRuntimeTestTarget(t, &cfg, mem)
	rt.runner = mustNewDisplayRunner(t, inner)

	if _, err := rt.mutationScanToken(); err != nil {
		t.Fatalf("first mutationScanToken: %v", err)
	}
	if _, err := rt.mutationScanToken(); err != nil {
		t.Fatalf("second mutationScanToken: %v", err)
	}

	if got := countGoRunWithArg(inner.Calls(), testGremlinsToolSpec); got != 1 {
		t.Fatalf("gremlins scan calls = %d, want 1", got)
	}
}

func TestGateRuntimeMutationCoverageAndSitesPreferKills(t *testing.T) {
	t.Parallel()

	cfg, err := parseConfig([]byte(minimalGatePolicy))
	if err != nil {
		t.Fatal(err)
	}
	mem := gatetest.NewMemoryFileOps()
	inner := newOptionsFakeRunner(mem, testGremlinsKillReport)
	runner := mustNewDisplayRunner(t, inner)
	resolver := gatetest.NewFakeToolResolver()
	artifactStore := newArtifactStore()
	qualityScope, _ := newRuntimeTestScope(t, &cfg)
	inv, err := gate.QualityScopeInventory(
		context.Background(), runner, artifactStore, mem, testFakeModuleRoot, qualityScope,
	)
	if err != nil {
		t.Fatalf("QualityScopeInventory: %v", err)
	}
	mutationRunner, err := gate.NewMutationRunner(runner, resolver, artifactStore, mem)
	if err != nil {
		t.Fatal(err)
	}
	killsOut, err := mutationRunner.Kill(
		context.Background(),
		testFakeModuleRoot,
		qualityScope,
		inv,
		gremlinsToolSpec(&cfg),
		mutationKillsOptions(&cfg)...,
	)
	if err != nil {
		t.Fatalf("Kill: %v", err)
	}

	rt := newRuntimeTestTarget(t, &cfg, mem)
	rt.mutationKillsOut = &killsOut
	rt.runner = mustNewDisplayRunner(t, newOptionsFakeRunner(mem, []byte("not-json")))

	if err := rt.mutationCoverage(); err != nil {
		t.Fatalf("mutationCoverage from kills: %v", err)
	}
	if err := rt.mutationSites(); err != nil {
		t.Fatalf("mutationSites from kills: %v", err)
	}
}

func TestGateRuntimeCoverageChecksCachesOutput(t *testing.T) {
	t.Parallel()

	cfg, err := parseConfig([]byte(minimalGatePolicy))
	if err != nil {
		t.Fatal(err)
	}
	mem := gatetest.NewMemoryFileOps()
	inner := newOptionsFakeRunner(mem, testGremlinsScanReport)
	rt := newRuntimeTestTarget(t, &cfg, mem)
	rt.runner = mustNewDisplayRunner(t, inner)

	if err := rt.coverageChecks(); err != nil {
		t.Fatalf("first coverageChecks: %v", err)
	}
	firstCalls := len(inner.Calls())
	if err := rt.coverageChecks(); err != nil {
		t.Fatalf("second coverageChecks: %v", err)
	}
	if got := len(inner.Calls()); got != firstCalls {
		t.Fatalf("cached coverageChecks added calls: got %d want %d", got, firstCalls)
	}
}

func TestGateRuntimeTaggedGateTestsUseExpectedTags(t *testing.T) {
	t.Parallel()

	cfg, err := parseConfig([]byte(minimalGatePolicy))
	if err != nil {
		t.Fatal(err)
	}
	mem := gatetest.NewMemoryFileOps()
	inner := newOptionsFakeRunner(mem, testGremlinsScanReport)
	rt := newRuntimeTestTarget(t, &cfg, mem)
	rt.runner = mustNewDisplayRunner(t, inner)

	if err := rt.performance(); err != nil {
		t.Fatalf("performance: %v", err)
	}
	if err := rt.strictTiming(); err != nil {
		t.Fatalf("strictTiming: %v", err)
	}

	sawPerformance := sawGoTestArg(inner.Calls(), "-tags=performance")
	sawStrict := sawGoTestArg(inner.Calls(), "-tags=bdd_strict_timing")
	if !sawPerformance || !sawStrict {
		t.Fatalf("tagged tests: performance=%v strict=%v calls=%v", sawPerformance, sawStrict, inner.Calls())
	}
}

func sawGoTestArg(calls []cmdrunner.Command, want string) bool {
	for _, call := range calls {
		if call.Name() == "go" && call.Arg(0) == "test" && slices.Contains(call.Args(), want) {
			return true
		}
	}

	return false
}
