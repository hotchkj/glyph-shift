//go:build mage
// +build mage

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/gate"
	"github.com/magefile/mage/mg"
)

const (
	goreleaserBuildStepID = "goreleaser-build"
	// goreleaserCLI pins GoReleaser v2 CLI for CrossCompile(); bump when adopting a newer toolchain.
	goreleaserCLI = "github.com/goreleaser/goreleaser/v2@v2.14.3"
)

var goreleaserBaseDistArtifacts = []string{"artifacts.json", "metadata.json", "checksums.txt"}

// Release groups release-only helper targets used by GoReleaser hooks.
type Release mg.Namespace

var errHooksDirNotDirectory = errors.New("hooks directory path is not a directory")

var errGoreleaserDistArtifactMissing = errors.New("goreleaser dist artifact missing after release --snapshot")

// pathInRoot maps a root-relative path for fileOps. Production resolves against the host cwd;
// tests override this with [absInRootLogical] when using gatetest.MemoryFileOps (see memmapfs-paths skill).
var pathInRoot = absInRootDisk

func absInRoot(rel string) (string, error) {
	return pathInRoot(rel)
}

func absInRootDisk(rel string) (string, error) {
	base, err := filepath.Abs(rootDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, rel), nil
}

func absInRootLogical(rel string) (string, error) {
	return filepath.ToSlash(rel), nil
}

func storeWriteOnce(stepID, name string, data []byte, prov gate.Provenance) error {
	err := store.Write(stepID, name, data, prov)
	if err != nil && errors.Is(err, gate.ErrArtifactSealed) {
		return nil
	}
	return err
}

type goreleaserDistArtifact struct {
	name string
	path string
}

func requireGoreleaserDistArtifacts(distDir string) error {
	for _, name := range goreleaserBaseDistArtifacts {
		path := filepath.Join(distDir, name)
		if _, err := fileOps.ReadFile(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("%w: %q", errGoreleaserDistArtifactMissing, name)
			}
			return fmt.Errorf("read dist output %s: %w", name, err)
		}
	}
	return nil
}

func collectGoreleaserDistArtifacts(distDir string) []goreleaserDistArtifact {
	paths := make(map[string]string, len(goreleaserBaseDistArtifacts))
	for _, name := range goreleaserBaseDistArtifacts {
		paths[name] = filepath.Join(distDir, name)
	}

	names := make([]string, 0, len(paths))
	for name := range paths {
		names = append(names, name)
	}
	sort.Strings(names)

	artifacts := make([]goreleaserDistArtifact, 0, len(names))
	for _, name := range names {
		artifacts = append(artifacts, goreleaserDistArtifact{name: name, path: paths[name]})
	}

	return artifacts
}

func recordGoreleaserDistArtifacts() error {
	distDir, err := absInRoot("dist")
	if err != nil {
		return err
	}
	artifacts := collectGoreleaserDistArtifacts(distDir)

	prov := gate.Provenance{
		StepID:   goreleaserBuildStepID,
		Tool:     "goreleaser",
		Packages: ".",
	}
	for _, artifact := range artifacts {
		data, err := fileOps.ReadFile(artifact.path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("read dist output %s: %w", artifact.name, err)
		}
		if err := storeWriteOnce(goreleaserBuildStepID, artifact.name, data, prov); err != nil {
			return err
		}
	}
	return nil
}

// CrossCompile runs a GoReleaser matrix smoke test (release --snapshot -> dist/ archives + checksums);
// publish to GitHub is release.yml only.
// Global before.hooks in .goreleaser.yml runs release:windowsresources once.
// Transient root syso files are removed after the run.
func CrossCompile() error {
	defer func() { _ = cleanupWindowsSysoInputs() }()

	ctx := context.Background()
	args := []string{
		"run",
		goreleaserCLI,
		"release",
		"--snapshot",
		"--clean",
	}
	runner, err := newRunner()
	if err != nil {
		return err
	}
	emitGateStepStart(runner, "Cross-compile")
	result, err := cmdrunner.Capture(ctx, runner, rootDir, "go", args...)
	if err != nil {
		toolOutput := result.Stdout + result.Stderr
		return wrapExternalStepError(runner, err, toolOutput)
	}
	distDir, err := absInRoot("dist")
	if err != nil {
		return err
	}
	if err := requireGoreleaserDistArtifacts(distDir); err != nil {
		return fmt.Errorf("goreleaser dist artifacts: %w", err)
	}
	if err := recordGoreleaserDistArtifacts(); err != nil {
		return fmt.Errorf("record goreleaser dist artifacts: %w", err)
	}
	return nil
}

const (
	releaseChecksumFileMode    = 0o600
	releaseManifestDistDirMode = 0o750
	releaseScriptFileMode      = 0o755
	splitFieldsParts           = 2
)

func releaseVersionFromEnv() string {
	version := strings.TrimPrefix(os.Getenv("GORELEASER_CURRENT_TAG"), "v")
	if version != "" {
		return version
	}
	version = strings.TrimPrefix(os.Getenv("VERSION"), "v")
	if version != "" {
		return version
	}
	return "dev"
}

// Hooks registers this repository's custom git hooks directory (.githooks) for the local clone
// via git config (not committed).
func Hooks() error {
	ctx := context.Background()

	hooksDir, err := absInRoot(".githooks")
	if err != nil {
		return err
	}

	if err := validateHooksDir(hooksDir); err != nil {
		return err
	}

	if err := productionRunner.Run(
		ctx, rootDir, os.Stdout, os.Stderr,
		"git", "config", "core.hooksPath", ".githooks",
	); err != nil {
		return fmt.Errorf("git config core.hooksPath: %w", err)
	}

	_, _ = fmt.Fprintln(os.Stdout, "Registered git hooks path: .githooks")
	return nil
}

func validateHooksDir(hooksDir string) error {
	st, err := os.Stat(hooksDir)
	if err != nil {
		return fmt.Errorf("stat hooks dir: %w", err)
	}

	if !st.IsDir() {
		_, _ = fmt.Fprintf(os.Stderr, "hooks directory is not a directory: %s\n", hooksDir)
		return errHooksDirNotDirectory
	}

	return nil
}
