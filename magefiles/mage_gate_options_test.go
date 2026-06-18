//go:build mage
// +build mage

package main

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gate"
	"github.com/hotchkj/mage-gate/gatetest"
)

const (
	testMageGateModulePath = "github.com/hotchkj/mage-gate"
	testPkgPattern         = "./gate/..."
	testMageGateImport     = "github.com/hotchkj/mage-gate/gate"
	testMageGateHarness    = "github.com/hotchkj/mage-gate/internal/harness"
	testFakeModuleRoot     = "/fake-root"

	testGremlinsToolSpec = "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"
	testGocycloToolSpec  = "github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"

	testGolangCILintSpec = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4"

	// golangciLintCustomToolSpec is the module path for the replace-style lint binary.
	golangciLintCustomToolSpec = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"
)

var (
	testGremlinsScanReport = []byte(`{"files":[{"file_name":"pkg/foo.go","mutations":[]}]}`)
	testGremlinsKillReport = []byte(`{"files":[{"file_name":"pkg/m.go","mutations":[{"status":"KILLED"}]}]}`)
)

// customLintQuietRunner treats the harness-built custom-gcl binary path as success so tests can exercise
// CustomGCL wiring without a real compiled linter binary.
type customLintQuietRunner struct {
	inner *cmdtest.FakeRunner
}

func (w *customLintQuietRunner) Run(
	ctx context.Context, dir string, stdout, stderr io.Writer, name string, args ...string,
) error {
	err := w.inner.Run(ctx, dir, stdout, stderr, name, args...)
	if err != nil && errors.Is(err, cmdtest.ErrUnhandledCommand) && isCustomGCLBinary(name) {
		return nil
	}
	return err
}

func isCustomGCLBinary(name string) bool {
	base := strings.ToLower(filepath.Base(name))

	return base == "custom-gcl" || base == "custom-gcl.exe" || strings.HasPrefix(base, "custom-gcl-")
}

func mustNewDisplayRunner(tb testing.TB, inner gate.CommandRunner) gate.CommandRunner {
	tb.Helper()
	r, err := gate.NewDisplayRunner(inner, gate.OutputModeAgent, io.Discard, io.Discard)
	if err != nil {
		tb.Fatalf("NewDisplayRunner: %v", err)
	}
	return r
}

func mustQualityScopeInventory(
	tb testing.TB,
	ctx context.Context,
	runner gate.CommandRunner,
	store *gate.ArtifactStore,
	mem *gatetest.MemoryFileOps,
	root string,
	scope gate.QualityScope,
) gate.QualityScopeInventoryOutput {
	tb.Helper()
	inv, err := gate.QualityScopeInventory(ctx, runner, store, mem, root, scope)
	if err != nil {
		tb.Fatalf("QualityScopeInventory: %v", err)
	}
	return inv
}

func mustCoveredTestOutputGate(tb testing.TB, out *gate.CoveredTestOutput) gate.CoveredTestOutput {
	tb.Helper()
	if out == nil {
		tb.Fatal("mustCoveredTestOutputGate: result is nil")
		return gate.CoveredTestOutput{}
	}
	if _, err := out.TestRun(); err != nil {
		tb.Fatalf("mustCoveredTestOutputGate: %v", err)
	}
	return *out
}

func newOptionsFakeRunner(
	mem *gatetest.MemoryFileOps,
	gremlinsReport []byte,
) *cmdtest.FakeRunner {
	root := testFakeModuleRoot
	base := []cmdtest.RunnerOption{
		cmdtest.On("go test", gatetest.GoTestPassWithCoverage(mem, testMageGateImport, 100)),
		cmdtest.On("go build", gatetest.NoopCommand),
		cmdtest.On("go vet", gatetest.NoopCommand),
		cmdtest.On("go tool cover", gatetest.GoToolCoverFunc(
			map[string]float64{"github.com/hotchkj/mage-gate/file.go:10:\tValidate": 100.0},
			100.0,
		)),
		cmdtest.On("go list", gatetest.GoList(testMageGateModulePath, root, map[string]gatetest.PackageListInfo{
			testMageGateImport:  gatetest.DirOnly(filepath.Join(root, "gate")),
			testMageGateHarness: gatetest.DirOnly(filepath.Join(root, "internal", "harness")),
		})),
		cmdtest.On("go run "+testGremlinsToolSpec, gatetest.Gremlins(mem, root, gremlinsReport)),
		cmdtest.On("go run "+testGocycloToolSpec, gatetest.NoopCommand),
		cmdtest.On("go run "+golangciLintCustomToolSpec, gatetest.NoopCommand),
		cmdtest.On("gocyclo", gatetest.NoopCommand),
		cmdtest.On("golangci-lint", gatetest.NoopCommand),
		cmdtest.On("deadcode", gatetest.NoopCommand),
	}
	return cmdtest.NewFakeRunner(base...)
}

func toolArgsContain(calls []cmdrunner.Command, toolName, substr string) bool {
	for _, c := range calls {
		if c.Name() != toolName {
			continue
		}
		if slices.Contains(c.Args(), substr) {
			return true
		}
	}
	return false
}

func goRunArgsContain(calls []cmdrunner.Command, substr string) bool {
	for _, c := range calls {
		if c.Name() != "go" || c.Arg(0) != "run" {
			continue
		}
		if slices.Contains(c.Args(), substr) {
			return true
		}
	}
	return false
}

func goTestArgsContain(calls []cmdrunner.Command, arg string) bool {
	for _, c := range calls {
		if c.Name() != "go" || c.Arg(0) != "test" {
			continue
		}
		for _, a := range c.Args() {
			if a == arg {
				return true
			}
		}
	}
	return false
}

func callsBuildCustomGCL(calls []cmdrunner.Command) bool {
	for _, c := range calls {
		if c.Name() == "go" && slices.Contains(c.Args(), golangciLintCustomToolSpec) {
			return true
		}
	}
	return false
}

func lintInvocationArgs(calls []cmdrunner.Command) []string {
	for _, c := range calls {
		if c.Name() == "golangci-lint" || isCustomGCLBinary(c.Name()) {
			return c.Args()
		}
	}
	return nil
}

func deadcodeArgs(calls []cmdrunner.Command) []string {
	for _, c := range calls {
		if c.Name() == "deadcode" {
			return c.Args()
		}
	}
	return nil
}

func TestQualityScopeOptions_IncludesTagsExcludePatternsWhenPresent(t *testing.T) {
	t.Parallel()

	policy := strings.ReplaceAll(minimalGatePolicy,
		`test_file_patterns = ["*_test.go"]`,
		`test_file_patterns = ["*_test.go", "custom_test.go"]`,
	)
	cfg, err := parseConfig([]byte(policy))
	if err != nil {
		t.Fatal(err)
	}
	scope, err := gate.NewQualityScope(cfg.packages(), qualityScopeOptions(&cfg)...)
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	tags := scope.Tags()
	if len(tags) != 1 || tags[0] != "mage" {
		t.Fatalf("Tags = %v, want [mage]", tags)
	}
	excl := scope.ExcludeSegments()
	if len(excl) != 5 {
		t.Fatalf("ExcludeSegments len = %d, want 5", len(excl))
	}
	pats := scope.TestFilePatterns()
	if len(pats) != 2 || pats[0] != "*_test.go" || pats[1] != "custom_test.go" {
		t.Fatalf("TestFilePatterns = %v", pats)
	}
}

func TestLintOptions_CustomGCLAndArgs(t *testing.T) {
	t.Parallel()

	policy := strings.ReplaceAll(minimalGatePolicy,
		`config = ".golangci.yml"`,
		`config = ".golangci.yml"
custom_gcl = "./custom/gcl"
custom_lint_tool_spec = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"
args = ["--verbose", "--max-issues-per-linter=10"]`,
	)
	cfg, err := parseConfig([]byte(policy))
	if err != nil {
		t.Fatal(err)
	}

	mem := rootedOptionsMemoryFileOps(t)
	inner := newOptionsFakeRunner(mem, testGremlinsScanReport)
	runner := mustNewDisplayRunner(t, &customLintQuietRunner{inner: inner})
	resolver := gatetest.NewFakeToolResolver()
	resolver.SetLocalMatch("golangci-lint", testGolangCILintSpec, true)

	pkgScope, err := gate.NewPackageScope(testPkgPattern)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	lt, err := lintToolchain(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	err = gate.Lint(
		ctx,
		runner,
		resolver,
		mem,
		testFakeModuleRoot,
		pkgScope,
		lt,
	)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	lintArgs := lintInvocationArgs(inner.Calls())
	if !slices.Contains(lintArgs, "--verbose") || !slices.Contains(lintArgs, "--max-issues-per-linter=10") {
		t.Fatalf("lint argv = %v, want custom args present", lintArgs)
	}
	if !callsBuildCustomGCL(inner.Calls()) {
		t.Fatalf("expected custom golangci-lint binary build, calls=%v", inner.Calls())
	}
}

func TestCrapOptions_Args(t *testing.T) {
	t.Parallel()

	policy := strings.ReplaceAll(minimalGatePolicy,
		`[crap]
tool_spec = "github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"`,
		`[crap]
tool_spec = "github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"
args = ["-over=12"]`,
	)
	cfg, err := parseConfig([]byte(policy))
	if err != nil {
		t.Fatal(err)
	}

	mem := rootedOptionsMemoryFileOps(t)
	inner := newOptionsFakeRunner(mem, testGremlinsScanReport)
	runner := mustNewDisplayRunner(t, inner)
	resolver := gatetest.NewFakeToolResolver()
	resolver.SetLocalMatch("gocyclo", testGocycloToolSpec, true)

	artifactStore := newArtifactStore()
	ctx := context.Background()
	scope, err := gate.NewQualityScope(testPkgPattern)
	if err != nil {
		t.Fatal(err)
	}
	pkgScope, err := gate.NewPackageScope(testPkgPattern)
	if err != nil {
		t.Fatal(err)
	}
	inv := mustQualityScopeInventory(t, ctx, runner, artifactStore, mem, testFakeModuleRoot, scope)
	out, err := gate.CoveredTest(
		ctx, runner, artifactStore, mem, testFakeModuleRoot, pkgScope, scope, inv, primaryPassOpts(&cfg)...,
	)
	if err != nil {
		t.Fatalf("CoveredTest: %v", err)
	}
	covTok := mustCoveredTestOutputGate(t, &out)
	covOut, err := gate.Coverage(ctx, runner, artifactStore, mem, testFakeModuleRoot, covTok, coverageMin(&cfg))
	if err != nil {
		t.Fatalf("Coverage: %v", err)
	}
	err = gate.Crap(
		ctx, runner, resolver, artifactStore, mem, testFakeModuleRoot, covOut, inv,
		crapMax(&cfg), gocycloToolSpec(&cfg), crapOptions(&cfg)...,
	)
	if err != nil {
		t.Fatalf("Crap: %v", err)
	}
	if !toolArgsContain(inner.Calls(), "gocyclo", "-over=12") {
		t.Fatalf("expected gocyclo invocation with -over=12, calls=%v", inner.Calls())
	}
}

func TestDeadcodeOptions_Args(t *testing.T) {
	t.Parallel()

	cfg, err := parseConfig([]byte(minimalGatePolicy))
	if err != nil {
		t.Fatal(err)
	}

	mem := rootedOptionsMemoryFileOps(t)
	inner := newOptionsFakeRunner(mem, testGremlinsScanReport)
	runner := mustNewDisplayRunner(t, inner)
	resolver := gatetest.NewFakeToolResolver()
	resolver.SetLocalMatch("deadcode", `golang.org/x/tools/cmd/deadcode@v0.44.0`, true)

	pkgScope, err := gate.NewPackageScope(testPkgPattern)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	err = gate.Deadcode(
		ctx, runner, resolver, mem, testFakeModuleRoot, pkgScope,
		deadcodeToolSpec(&cfg), deadcodeOptions(&cfg)...,
	)
	if err != nil {
		t.Fatalf("Deadcode: %v", err)
	}
	args := deadcodeArgs(inner.Calls())
	if !slices.Contains(args, "-test") {
		t.Fatalf("deadcode args = %v, want -test from config", args)
	}
}

func TestCompileOptions_TagsAndArgs(t *testing.T) {
	t.Parallel()

	policy := strings.ReplaceAll(minimalGatePolicy,
		`[compile]
tags = ["integration", "performance", "bdd_strict_timing"]`,
		`[compile]
tags = ["integration", "performance", "bdd_strict_timing"]
args = ["-trimpath"]`,
	)
	cfg, err := parseConfig([]byte(policy))
	if err != nil {
		t.Fatal(err)
	}

	mem := rootedOptionsMemoryFileOps(t)
	inner := newOptionsFakeRunner(mem, testGremlinsScanReport)
	runner := mustNewDisplayRunner(t, inner)
	pkgScope, err := gate.NewPackageScope(testPkgPattern)
	if err != nil {
		t.Fatal(err)
	}
	err = gate.Compile(context.Background(), runner, mem, testFakeModuleRoot, pkgScope, compileOptions(&cfg)...)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !toolArgsContain(inner.Calls(), "go", "-tags=integration,performance,bdd_strict_timing") {
		t.Fatalf("expected compile build tags, calls %#v", inner.Calls())
	}
	if !toolArgsContain(inner.Calls(), "go", "-trimpath") {
		t.Fatalf("expected compile extra args, calls %#v", inner.Calls())
	}
}

func TestPrimaryPassOpts_ShuffleAndExtraArgs(t *testing.T) {
	t.Parallel()

	policy := strings.ReplaceAll(minimalGatePolicy,
		`[unittests]
duration_max = 300.0
shuffle = true`,
		`[unittests]
duration_max = 300.0
shuffle = true
args = ["-count=1", "-run=TestPrimary"]`,
	)
	cfg, err := parseConfig([]byte(policy))
	if err != nil {
		t.Fatal(err)
	}

	mem := rootedOptionsMemoryFileOps(t)
	inner := newOptionsFakeRunner(mem, testGremlinsScanReport)
	runner := mustNewDisplayRunner(t, inner)
	artifactStore := newArtifactStore()
	pkgScope, err := gate.NewPackageScope(testPkgPattern)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, err = gate.Test(ctx, runner, artifactStore, mem, testFakeModuleRoot, pkgScope, primaryPassOpts(&cfg)...)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if !goTestArgsContain(inner.Calls(), "-shuffle=on") {
		t.Fatalf("expected -shuffle=on in go test calls, got %#v", inner.Calls())
	}
	if !goTestArgsContain(inner.Calls(), "-count=1") || !goTestArgsContain(inner.Calls(), "-run=TestPrimary") {
		t.Fatalf("expected extra unittest args, got %#v", inner.Calls())
	}
}

func TestIntegrationPassOpts_TagsShuffleArgs(t *testing.T) {
	t.Parallel()

	base := strings.ReplaceAll(minimalGatePolicy,
		`[integrationtests]
tags = "integration"
shuffle = true`,
		`[integrationtests]
tags = "integration,foo"
shuffle = true
args = ["-parallel=2"]`,
	)
	cfg, err := parseConfig([]byte(base))
	if err != nil {
		t.Fatal(err)
	}

	mem := rootedOptionsMemoryFileOps(t)
	inner := newOptionsFakeRunner(mem, testGremlinsScanReport)
	runner := mustNewDisplayRunner(t, inner)
	artifactStore := newArtifactStore()
	pkgScope, err := gate.NewPackageScope(testPkgPattern)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, err = gate.Test(ctx, runner, artifactStore, mem, testFakeModuleRoot, pkgScope, integrationPassOpts(&cfg)...)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if !goTestArgsContain(inner.Calls(), "-tags=integration,foo") {
		t.Fatalf("expected integration tags on go test, calls %#v", inner.Calls())
	}
	if !goTestArgsContain(inner.Calls(), "-shuffle=on") || !goTestArgsContain(inner.Calls(), "-parallel=2") {
		t.Fatalf("expected shuffle and extra integration args, calls %#v", inner.Calls())
	}
}
