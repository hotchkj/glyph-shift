//go:build mage
// +build mage

// Mage targets for github.com/hotchkj/glyph-shift (mage-gate v0.7.0, policy in gate.toml).
package main

import (
	"fmt"
	"os"

	"github.com/hotchkj/mage-gate/gate"
)

// init panics on chdir or FileOps root failure: mage cannot run from the wrong directory.
func init() {
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		if chErr := os.Chdir(".."); chErr != nil {
			panic(fmt.Sprintf("mage init: chdir to repo root: %v", chErr))
		}
	}
	if rootErr := fileOps.Root(rootDir); rootErr != nil {
		panic(fmt.Sprintf("mage init: fileOps.Root: %v", rootErr))
	}
}

var (
	// productionRunner is used for subprocess-style targets that do not use gate display wrapping.
	// The quality gate uses newRunner(), which selects OutputModeVerbose when CI is set (see gateOutputMode).
	productionRunner gate.CommandRunner  = gate.NewProductionRunner()
	fileOps          gate.FileOps        = gate.NewProductionFileOps()
	newArtifactStore                     = gate.NewArtifactStore
	store            *gate.ArtifactStore = newArtifactStore()
	rootDir                              = "."
)
