//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/hotchkj/mage-gate/gate"
)

type gateRuntime struct {
	ctx          context.Context
	cfg          config
	qualityScope gate.QualityScope
	pkgScope     gate.PackageScope
	runner       gate.CommandRunner
	resolver     gate.ToolResolver
	store        *gate.ArtifactStore
	fileOps      gate.FileOps
	root         string

	inventory        *gate.QualityScopeInventoryOutput
	primaryCovered   *gate.CoveredTestOutput
	coverage         *gate.CoverageOutput
	mutationScanOut  *gate.MutationScanOutput
	mutationKillsOut *gate.MutationKillsOutput
}

var gateRuntimeState struct {
	mu      sync.Mutex
	current *gateRuntime
	err     error
}

func currentGateRuntime() (*gateRuntime, error) {
	gateRuntimeState.mu.Lock()
	defer gateRuntimeState.mu.Unlock()

	if gateRuntimeState.current != nil || gateRuntimeState.err != nil {
		return gateRuntimeState.current, gateRuntimeState.err
	}

	gateRuntimeState.current, gateRuntimeState.err = newGateRuntime()
	return gateRuntimeState.current, gateRuntimeState.err
}

func newGateRuntime() (*gateRuntime, error) {
	cfg, qualityScope, pkgScope, err := loadConfigAndScope()
	if err != nil {
		return nil, err
	}
	runner, err := newRunner()
	if err != nil {
		return nil, err
	}
	return &gateRuntime{
		ctx:          context.Background(),
		cfg:          cfg,
		qualityScope: qualityScope,
		pkgScope:     pkgScope,
		runner:       runner,
		resolver:     newResolver(),
		store:        store,
		fileOps:      fileOps,
		root:         rootDir,
	}, nil
}

func withGateRuntime(fn func(*gateRuntime) error) error {
	rt, err := currentGateRuntime()
	if err != nil {
		return err
	}
	return fn(rt)
}

func (rt *gateRuntime) compile() error {
	return gate.Compile(
		rt.ctx, rt.runner, rt.fileOps, rt.root, rt.pkgScope,
		compileOptions(&rt.cfg)...,
	)
}

func (rt *gateRuntime) format() error {
	lt, err := lintToolchain(&rt.cfg)
	if err != nil {
		return err
	}
	return gate.Format(
		rt.ctx, rt.runner, rt.resolver, rt.fileOps, rt.root, rt.pkgScope, lt,
	)
}

func (rt *gateRuntime) lint() error {
	lt, err := lintToolchain(&rt.cfg)
	if err != nil {
		return err
	}
	return gate.Lint(
		rt.ctx, rt.runner, rt.resolver, rt.fileOps, rt.root, rt.pkgScope, lt,
	)
}

func (rt *gateRuntime) markdownLint() error {
	if !markdownlintEnabled(&rt.cfg) {
		return nil
	}
	return gate.MarkdownLint(
		rt.ctx, rt.runner, rt.resolver, rt.fileOps, rt.root,
		markdownlintToolSpec(&rt.cfg), markdownlintOpts(&rt.cfg)...,
	)
}

func (rt *gateRuntime) qualityScopeInventory() (*gate.QualityScopeInventoryOutput, error) {
	if rt.inventory != nil {
		return rt.inventory, nil
	}
	inv, err := gate.QualityScopeInventory(
		rt.ctx, rt.runner, rt.store, rt.fileOps, rt.root, rt.qualityScope,
	)
	if err != nil {
		return nil, err
	}
	rt.inventory = &inv
	return rt.inventory, nil
}

func (rt *gateRuntime) vet() error {
	return gate.Vet(rt.ctx, rt.runner, rt.fileOps, rt.root, rt.pkgScope)
}

func (rt *gateRuntime) deadcode() error {
	if strings.TrimSpace(rt.cfg.Deadcode.ToolSpec) == "" {
		return errDeadcodeToolSpecEmpty
	}
	return gate.Deadcode(
		rt.ctx, rt.runner, rt.resolver, rt.fileOps, rt.root, rt.pkgScope,
		deadcodeToolSpec(&rt.cfg), deadcodeOptions(&rt.cfg)...,
	)
}

func (rt *gateRuntime) primaryCoveredTest() (*gate.CoveredTestOutput, error) {
	if rt.primaryCovered != nil {
		return rt.primaryCovered, nil
	}
	inventory, err := rt.qualityScopeInventory()
	if err != nil {
		return nil, err
	}
	coveredOut, err := gate.CoveredTest(
		rt.ctx, rt.runner, rt.store, rt.fileOps, rt.root, rt.pkgScope, rt.qualityScope, *inventory,
		primaryPassOpts(&rt.cfg)...,
	)
	if err != nil {
		return nil, err
	}
	rt.primaryCovered = &coveredOut
	return rt.primaryCovered, nil
}

func (rt *gateRuntime) test() error {
	_, err := rt.primaryCoveredTest()
	return err
}

func (rt *gateRuntime) coverageChecks() error {
	if rt.coverage != nil {
		return nil
	}
	coveredOut, err := rt.primaryCoveredTest()
	if err != nil {
		return err
	}
	covOut, err := rt.runCoverageQualityChecks(coveredOut)
	if err != nil {
		return err
	}
	rt.coverage = &covOut
	return nil
}

func (rt *gateRuntime) runCoverageQualityChecks(coveredOut *gate.CoveredTestOutput) (gate.CoverageOutput, error) {
	testOut, err := coveredOut.TestRun()
	if err != nil {
		return gate.CoverageOutput{}, err
	}
	if durationErr := gate.Duration(
		rt.ctx, rt.runner, rt.store, rt.fileOps, rt.root, testOut, unitTestDurationMax(&rt.cfg),
	); durationErr != nil {
		return gate.CoverageOutput{}, durationErr
	}
	covOut, err := gate.Coverage(rt.ctx, rt.runner, rt.store, rt.fileOps, rt.root, *coveredOut, coverageMin(&rt.cfg))
	if err != nil {
		return gate.CoverageOutput{}, err
	}
	inventory, err := rt.qualityScopeInventory()
	if err != nil {
		return gate.CoverageOutput{}, err
	}
	if err := gate.Crap(
		rt.ctx, rt.runner, rt.resolver, rt.store, rt.fileOps, rt.root, covOut, *inventory,
		crapMax(&rt.cfg), gocycloToolSpec(&rt.cfg), crapOptions(&rt.cfg)...,
	); err != nil {
		return gate.CoverageOutput{}, err
	}

	return covOut, nil
}

func (rt *gateRuntime) mutationScan() error {
	_, err := rt.mutationScanToken()
	return err
}

func (rt *gateRuntime) mutationScanToken() (*gate.MutationScanOutput, error) {
	if rt.mutationScanOut != nil {
		return rt.mutationScanOut, nil
	}
	inventory, err := rt.qualityScopeInventory()
	if err != nil {
		return nil, err
	}
	mr, err := gate.NewMutationRunner(rt.runner, rt.resolver, rt.store, rt.fileOps)
	if err != nil {
		return nil, err
	}
	scanOut, err := mr.Scan(
		rt.ctx, rt.root, rt.qualityScope, *inventory, gremlinsToolSpec(&rt.cfg), gremlinsMutationOptions(&rt.cfg)...,
	)
	if err != nil {
		return nil, err
	}
	rt.mutationScanOut = &scanOut
	return rt.mutationScanOut, nil
}

func (rt *gateRuntime) mutationCoverage() error {
	if rt.mutationKillsOut != nil {
		return gate.MutationCoverageFromKills(*rt.mutationKillsOut, mutationCoverageMin(&rt.cfg))
	}
	scanOut, err := rt.mutationScanToken()
	if err != nil {
		return err
	}
	return gate.MutationCoverage(*scanOut, mutationCoverageMin(&rt.cfg))
}

func (rt *gateRuntime) mutationSites() error {
	if rt.mutationKillsOut != nil {
		return gate.MutationSitesFromKills(*rt.mutationKillsOut, mutationSitesMax(&rt.cfg))
	}
	scanOut, err := rt.mutationScanToken()
	if err != nil {
		return err
	}
	return gate.MutationSites(*scanOut, mutationSitesMax(&rt.cfg))
}

func (rt *gateRuntime) mutationKills() error {
	if rt.mutationKillsOut != nil {
		return nil
	}
	inventory, err := rt.qualityScopeInventory()
	if err != nil {
		return err
	}
	out, err := gate.MutationKills(
		rt.ctx,
		rt.runner,
		rt.resolver,
		rt.store,
		rt.fileOps,
		rt.root,
		rt.qualityScope,
		*inventory,
		mutationKillsMinRate(&rt.cfg),
		gremlinsToolSpec(&rt.cfg),
		mutationKillsOptions(&rt.cfg)...,
	)
	if err == nil {
		rt.mutationKillsOut = &out
	}
	return err
}

func (rt *gateRuntime) integration() error {
	_, err := gate.Test(
		rt.ctx, rt.runner, rt.store, rt.fileOps, rt.root, rt.pkgScope,
		integrationPassOpts(&rt.cfg)...,
	)
	return err
}

func (rt *gateRuntime) taggedGateTest(cfg taggedTestConfig, name string) error {
	pkgScope, err := gate.NewPackageScope(cfg.Packages)
	if err != nil {
		return err
	}
	_, err = gate.Test(rt.ctx, rt.runner, rt.store, rt.fileOps, rt.root, pkgScope, taggedTestOptions(cfg)...)
	if err != nil {
		return fmt.Errorf("mage-gate tagged test %s %s: %w", name, cfg.Packages, err)
	}
	return nil
}

func (rt *gateRuntime) performance() error {
	return rt.taggedGateTest(rt.cfg.Performance, "performance")
}

func (rt *gateRuntime) strictTiming() error {
	return rt.taggedGateTest(rt.cfg.StrictTiming, "stricttiming")
}
