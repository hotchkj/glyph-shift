//go:build mage
// +build mage

package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
)

// withReleaseCLITestSeams pins rootDir and fileOps for release CLI staging tests.
func withReleaseCLITestSeams(t *testing.T, mem *gatetest.MemoryFileOps) {
	t.Helper()

	oldRootDir := rootDir
	oldFileOps := fileOps
	t.Cleanup(func() {
		rootDir = oldRootDir
		fileOps = oldFileOps
	})

	rootDir = testFakeModuleRoot
	if err := mem.Root(rootDir); err != nil {
		t.Fatalf("mem.Root: %v", err)
	}
	fileOps = mem
}

func writeDistArtifactsJSON(t *testing.T, mem *gatetest.MemoryFileOps, records []goreleaserArtifactRecord) {
	t.Helper()

	distDir, err := absInRoot("dist")
	if err != nil {
		t.Fatalf("absInRoot dist: %v", err)
	}
	raw, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("json.Marshal artifacts: %v", err)
	}
	artifactsPath := filepath.Join(distDir, "artifacts.json")
	if writeErr := mem.WriteFile(artifactsPath, raw, 0o644); writeErr != nil {
		t.Fatalf("WriteFile artifacts.json: %v", writeErr)
	}
}

func writeHostReleaseCLIBinaryDistFixture(t *testing.T, mem *gatetest.MemoryFileOps, content []byte) {
	t.Helper()

	goos, goarch, err := hostGoreleaserPlatform()
	if err != nil {
		t.Fatalf("hostGoreleaserPlatform: %v", err)
	}
	binaryRel := releaseCLIBinaryFixtureRel()
	binaryPath, absErr := absInRoot(binaryRel)
	if absErr != nil {
		t.Fatalf("absInRoot binary: %v", absErr)
	}
	if mkdirErr := mem.MkdirAll(filepath.Dir(binaryPath), releaseScriptFileMode); mkdirErr != nil {
		t.Fatalf("MkdirAll binary dir: %v", mkdirErr)
	}
	if writeErr := mem.WriteFile(binaryPath, content, 0o644); writeErr != nil {
		t.Fatalf("WriteFile binary: %v", writeErr)
	}
	writeDistArtifactsJSON(t, mem, []goreleaserArtifactRecord{{
		Name:   filepath.Base(binaryRel),
		Path:   filepath.ToSlash(binaryRel),
		Goos:   goos,
		Goarch: goarch,
		Type:   goreleaserArtifactBinary,
	}})
}

func releaseCLIBinaryFixtureRel() string {
	name := releaseCLIBinaryName
	if runtime.GOOS == releaseCLIGoosWindows {
		name += ".exe"
	}
	return filepath.Join("dist", "release-cli-fixture", name)
}

func TestStageHostReleaseCLIStagesBinaryFromDistNotArchive(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	withReleaseCLITestSeams(t, mem)
	writeHostReleaseCLIBinaryDistFixture(t, mem, []byte("release-cli-bytes"))

	if err := stageHostReleaseCLI(); err != nil {
		t.Fatalf("stageHostReleaseCLI: %v", err)
	}
	outPath, err := absInRoot(cliBinaryOutputRel())
	if err != nil {
		t.Fatalf("absInRoot: %v", err)
	}
	data, err := fileOps.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile staged CLI: %v", err)
	}
	if string(data) != "release-cli-bytes" {
		t.Fatalf("staged CLI = %q, want release-cli-bytes", string(data))
	}
}

func TestHostReleaseCLIBinaryPathRejectsArchiveEntries(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	withReleaseCLITestSeams(t, mem)

	goos, goarch, err := hostGoreleaserPlatform()
	if err != nil {
		t.Fatalf("hostGoreleaserPlatform: %v", err)
	}
	archiveRel := filepath.Join("dist", "glyph-shift_dev_"+goos+"_"+goarch+".zip")
	writeDistArtifactsJSON(t, mem, []goreleaserArtifactRecord{{
		Name:   filepath.Base(archiveRel),
		Path:   filepath.ToSlash(archiveRel),
		Goos:   goos,
		Goarch: goarch,
		Type:   "Archive",
	}})

	if _, err := hostReleaseCLIBinaryPath(goos, goarch); !errors.Is(err, errReleaseCLIBinaryNotFound) {
		t.Fatalf("hostReleaseCLIBinaryPath = %v, want %v", err, errReleaseCLIBinaryNotFound)
	}
}

func TestHostReleaseCLIBinaryPathRejectsAmbiguousBinaryEntries(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	withReleaseCLITestSeams(t, mem)

	goos, goarch, err := hostGoreleaserPlatform()
	if err != nil {
		t.Fatalf("hostGoreleaserPlatform: %v", err)
	}
	binaryRel := releaseCLIBinaryFixtureRel()
	writeDistArtifactsJSON(t, mem, []goreleaserArtifactRecord{
		{
			Name:   filepath.Base(binaryRel),
			Path:   filepath.ToSlash(binaryRel),
			Goos:   goos,
			Goarch: goarch,
			Type:   goreleaserArtifactBinary,
		},
		{
			Name:   filepath.Base(binaryRel) + ".dup",
			Path:   filepath.ToSlash(binaryRel + ".dup"),
			Goos:   goos,
			Goarch: goarch,
			Type:   goreleaserArtifactBinary,
		},
	})

	if _, err := hostReleaseCLIBinaryPath(goos, goarch); !errors.Is(err, errReleaseCLIBinaryAmbiguous) {
		t.Fatalf("hostReleaseCLIBinaryPath = %v, want %v", err, errReleaseCLIBinaryAmbiguous)
	}
}

func TestHostReleaseCLIBinaryPathErrorsWhenArtifactsMissing(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	withReleaseCLITestSeams(t, mem)

	goos, goarch, err := hostGoreleaserPlatform()
	if err != nil {
		t.Fatalf("hostGoreleaserPlatform: %v", err)
	}
	if _, err := hostReleaseCLIBinaryPath(goos, goarch); !errors.Is(err, errReleaseCLIArtifactsList) {
		t.Fatalf("hostReleaseCLIBinaryPath = %v, want %v", err, errReleaseCLIArtifactsList)
	}
}

func TestStageHostReleaseCLIErrorsWhenBinaryFileMissing(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	withReleaseCLITestSeams(t, mem)

	goos, goarch, err := hostGoreleaserPlatform()
	if err != nil {
		t.Fatalf("hostGoreleaserPlatform: %v", err)
	}
	binaryRel := releaseCLIBinaryFixtureRel()
	writeDistArtifactsJSON(t, mem, []goreleaserArtifactRecord{{
		Name:   filepath.Base(binaryRel),
		Path:   filepath.ToSlash(binaryRel),
		Goos:   goos,
		Goarch: goarch,
		Type:   goreleaserArtifactBinary,
	}})

	if err := stageHostReleaseCLI(); err == nil {
		t.Fatal("stageHostReleaseCLI: expected error for missing binary file")
	}
}

func TestStageHostReleaseCLIErrorsWhenBinaryAmbiguous(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	withReleaseCLITestSeams(t, mem)

	goos, goarch, err := hostGoreleaserPlatform()
	if err != nil {
		t.Fatalf("hostGoreleaserPlatform: %v", err)
	}
	binaryRel := releaseCLIBinaryFixtureRel()
	writeDistArtifactsJSON(t, mem, []goreleaserArtifactRecord{
		{
			Name:   filepath.Base(binaryRel),
			Path:   filepath.ToSlash(binaryRel),
			Goos:   goos,
			Goarch: goarch,
			Type:   goreleaserArtifactBinary,
		},
		{
			Name:   filepath.Base(binaryRel) + ".dup",
			Path:   filepath.ToSlash(binaryRel + ".dup"),
			Goos:   goos,
			Goarch: goarch,
			Type:   goreleaserArtifactBinary,
		},
	})

	if err := stageHostReleaseCLI(); !errors.Is(err, errReleaseCLIBinaryAmbiguous) {
		t.Fatalf("stageHostReleaseCLI = %v, want %v", err, errReleaseCLIBinaryAmbiguous)
	}
}
