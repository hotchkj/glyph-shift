//go:build mage
// +build mage

package main

import (
	"context"
	"errors"
	"io"
	iofs "io/fs"
	"path/filepath"
	"testing"

	"github.com/hotchkj/mage-gate/gate"
	"github.com/hotchkj/mage-gate/gatetest"
)

var errReleaseRunnerFailed = errors.New("release runner failed")

type releaseFakeRunner struct {
	err   error
	calls int
	name  string
	args  []string
}

func (r *releaseFakeRunner) Run(
	_ context.Context,
	_ string,
	_ io.Writer,
	_ io.Writer,
	name string,
	args ...string,
) error {
	r.calls++
	r.name = name
	r.args = append([]string(nil), args...)

	return r.err
}

func withReleaseArtifacts(t *testing.T) *gatetest.MemoryFileOps {
	t.Helper()

	oldRootDir := rootDir
	oldFileOps := fileOps
	oldStore := store
	oldProductionRunner := productionRunner
	oldPathInRoot := pathInRoot
	t.Cleanup(func() {
		rootDir = oldRootDir
		fileOps = oldFileOps
		store = oldStore
		productionRunner = oldProductionRunner
		pathInRoot = oldPathInRoot
	})

	rootDir = "."
	pathInRoot = absInRootLogical
	mem := gatetest.NewMemoryFileOps()
	if err := mem.Root("."); err != nil {
		t.Fatalf("mem.Root: %v", err)
	}
	fileOps = mem
	store = newArtifactStore()

	return mem
}

func TestStoreWriteOnceIgnoresSealedArtifact(t *testing.T) {
	withReleaseArtifacts(t)

	prov := gate.Provenance{StepID: goreleaserBuildStepID, Tool: "test", Packages: "."}
	if err := storeWriteOnce(goreleaserBuildStepID, "artifact.json", []byte("first"), prov); err != nil {
		t.Fatalf("first storeWriteOnce: %v", err)
	}
	if err := storeWriteOnce(goreleaserBuildStepID, "artifact.json", []byte("second"), prov); err != nil {
		t.Fatalf("second storeWriteOnce sealed artifact: %v", err)
	}
	got, err := store.Read(goreleaserBuildStepID, "artifact.json")
	if err != nil {
		t.Fatalf("Read artifact: %v", err)
	}
	if string(got) != "first" {
		t.Fatalf("artifact content = %q, want first", got)
	}
}

// stubCrossCompileRunner fakes a successful goreleaser run for tests that invoke CrossCompile.
// mem must already contain dist/artifacts.json; this writes the other required dist outputs.
func stubCrossCompileRunner(t *testing.T, mem *gatetest.MemoryFileOps) *releaseFakeRunner {
	t.Helper()

	runner := &releaseFakeRunner{}
	oldNewRunner := newRunner
	oldStore := store
	newRunner = func() (gate.CommandRunner, error) {
		return mustNewDisplayRunner(t, runner), nil
	}
	store = newArtifactStore()
	t.Cleanup(func() {
		newRunner = oldNewRunner
		store = oldStore
	})

	distDir, err := absInRoot("dist")
	if err != nil {
		t.Fatalf("absInRoot dist: %v", err)
	}
	writeDistFixture(t, mem, distDir, "metadata.json", "metadata.json")
	writeDistFixture(t, mem, distDir, "checksums.txt", "checksums.txt")

	return runner
}

func writeDistFixture(t *testing.T, mem *gatetest.MemoryFileOps, distDir, name, data string) {
	t.Helper()

	if writeErr := mem.WriteFile(filepath.Join(distDir, name), []byte(data), 0o644); writeErr != nil {
		t.Fatalf("WriteFile %s: %v", name, writeErr)
	}
}

func writeReleaseFixture(t *testing.T, mem *gatetest.MemoryFileOps, relPath, data string) string {
	return writeReleaseFixtureMode(t, mem, relPath, data, 0o644)
}

func writeReleaseFixtureMode(
	t *testing.T, mem *gatetest.MemoryFileOps, relPath, data string, mode iofs.FileMode,
) string {
	t.Helper()

	absPath, absErr := absInRoot(relPath)
	if absErr != nil {
		t.Fatalf("absInRoot %s: %v", relPath, absErr)
	}
	if err := mem.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(absPath), err)
	}
	if err := mem.WriteFile(absPath, []byte(data), mode); err != nil {
		t.Fatalf("WriteFile %s: %v", absPath, err)
	}

	return absPath
}

func assertReleaseFixtureMissing(t *testing.T, mem *gatetest.MemoryFileOps, path string) {
	t.Helper()

	if _, err := mem.ReadFile(path); !errors.Is(err, iofs.ErrNotExist) {
		t.Fatalf("read %s error = %v want not exist", path, err)
	}
}

func assertReleaseFixtureContent(t *testing.T, mem *gatetest.MemoryFileOps, path, want string) {
	t.Helper()

	got, err := mem.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("ReadFile %s = %q want %q", path, got, want)
	}
}

func assertStoredArtifactBytes(t *testing.T, name, want string) {
	t.Helper()

	if !store.Has(goreleaserBuildStepID, name) {
		t.Fatalf("%s was not recorded", name)
	}
	got, err := store.Read(goreleaserBuildStepID, name)
	if err != nil {
		t.Fatalf("Read %s: %v", name, err)
	}
	if string(got) != want {
		t.Fatalf("stored %s = %q, want source bytes", name, got)
	}
}

func TestRequireGoreleaserDistArtifactsFailsWhenMissing(t *testing.T) {
	mem := withReleaseArtifacts(t)

	distDir, absErr := absInRoot("dist")
	if absErr != nil {
		t.Fatalf("absInRoot: %v", absErr)
	}
	writeDistFixture(t, mem, distDir, "artifacts.json", `{}`)

	err := requireGoreleaserDistArtifacts(distDir)
	if err == nil {
		t.Fatal("requireGoreleaserDistArtifacts: expected error for missing metadata.json and checksums.txt")
	}
}

func TestRecordGoreleaserDistArtifactsStoresPresentFilesAndSkipsMissing(t *testing.T) {
	mem := withReleaseArtifacts(t)

	distDir, absErr := absInRoot("dist")
	if absErr != nil {
		t.Fatalf("absInRoot: %v", absErr)
	}
	writeDistFixture(t, mem, distDir, "artifacts.json", `{"builds":[]}`)
	writeDistFixture(t, mem, distDir, "checksums.txt", "checksums")

	if recordErr := recordGoreleaserDistArtifacts(); recordErr != nil {
		t.Fatalf("recordGoreleaserDistArtifacts: %v", recordErr)
	}

	if store.Has(goreleaserBuildStepID, "metadata.json") {
		t.Fatal("missing metadata.json should not be recorded")
	}
	assertStoredArtifactBytes(t, "artifacts.json", `{"builds":[]}`)
	assertStoredArtifactBytes(t, "checksums.txt", "checksums")
}

func assertReleaseFakeRunnerGoreleaserSnapshot(t *testing.T, runner *releaseFakeRunner) {
	t.Helper()

	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if runner.name != "go" {
		t.Fatalf("runner name = %q, want go", runner.name)
	}
	wantArgs := []string{"run", goreleaserCLI, "release", "--snapshot", "--clean"}
	if len(runner.args) != len(wantArgs) {
		t.Fatalf("runner args = %v, want %v", runner.args, wantArgs)
	}
	for i, want := range wantArgs {
		if runner.args[i] != want {
			t.Fatalf("runner.args[%d] = %q, want %q", i, runner.args[i], want)
		}
	}
}

func TestCrossCompileRunsGoreleaserAndRecordsArtifacts(t *testing.T) {
	mem := withReleaseArtifacts(t)
	distDir, absErr := absInRoot("dist")
	if absErr != nil {
		t.Fatalf("absInRoot: %v", absErr)
	}
	writeDistFixture(t, mem, distDir, "artifacts.json", "artifacts.json")
	runner := stubCrossCompileRunner(t, mem)

	if err := CrossCompile(); err != nil {
		t.Fatalf("CrossCompile: %v", err)
	}
	assertReleaseFakeRunnerGoreleaserSnapshot(t, runner)
	for _, name := range goreleaserBaseDistArtifacts {
		if !store.Has(goreleaserBuildStepID, name) {
			t.Fatalf("%s was not recorded", name)
		}
	}
}

func TestCleanupWindowsSysoInputsRemovesRootMatchesOnly(t *testing.T) {
	mem := withReleaseArtifacts(t)

	rootSyso := writeReleaseFixture(t, mem, "resource_windows_amd64.syso", "root")
	keepSyso := writeReleaseFixture(t, mem, "resource_linux_amd64.syso", "keep")
	nestedSyso := writeReleaseFixture(t, mem, filepath.Join("nested", "resource_windows_arm64.syso"), "nested")

	if err := cleanupWindowsSysoInputs(); err != nil {
		t.Fatalf("cleanupWindowsSysoInputs: %v", err)
	}

	assertReleaseFixtureMissing(t, mem, rootSyso)
	assertReleaseFixtureContent(t, mem, keepSyso, "keep")
	assertReleaseFixtureContent(t, mem, nestedSyso, "nested")
}

func TestHooksSurfacesStatErrorForInvalidRoot(t *testing.T) {
	withReleaseArtifacts(t)
	rootDir = string([]byte{0})

	if err := Hooks(); err == nil {
		t.Fatal("Hooks invalid root: expected stat error")
	}
}

func TestValidateHooksDirSurfacesStatError(t *testing.T) {
	t.Parallel()

	if err := validateHooksDir(string([]byte{0})); err == nil {
		t.Fatal("validateHooksDir invalid path: expected error")
	}
}

func TestReleaseVersionFromEnvPrefersGoreleaserTag(t *testing.T) {
	t.Setenv("GORELEASER_CURRENT_TAG", "v2.0.0")
	t.Setenv("VERSION", "9.9.9")
	if got := releaseVersionFromEnv(); got != "2.0.0" {
		t.Fatalf("releaseVersionFromEnv() = %q, want 2.0.0", got)
	}
}
