//go:build mage
// +build mage

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hotchkj/mage-gate/gate"
	"github.com/josephspurrier/goversioninfo"
)

const (
	windowsVersionInfoStepID = "windows-versioninfo"
	versionInfoJSON          = "versioninfo.json"
)

var errReleaseWindowsresourcesVersion = errors.New(
	"release:windowsresources requires a non-blank version argument (first positional parameter)")

// windowsSysoProvenanceRecord is ArtifactStore metadata for a generated linker .syso (not the object bytes).
type windowsSysoProvenanceRecord struct {
	FileName            string `json:"fileName"`
	Arch                string `json:"arch"`
	VersionLabel        string `json:"versionLabel"`
	NumericFixedVersion string `json:"numericFixedVersion"`
	FileVersionLabel    string `json:"fileVersionLabel"`
	ProductVersionLabel string `json:"productVersionLabel"`
	Tool                string `json:"tool"`
}

type windowsSysoWriter func(vi *goversioninfo.VersionInfo, outputPath, arch string) error

func writeWindowsSyso(vi *goversioninfo.VersionInfo, outputPath, arch string) error {
	return vi.WriteSyso(outputPath, arch)
}

// generateWindowsVersionResources writes resource_windows_<arch>.syso beside main for linking,
// records per-arch provenance in ArtifactStore (not raw .syso bytes).
// Transient .syso files remain on disk until cleanup runs.
func generateWindowsVersionResources(version, commit string) error {
	return generateWindowsVersionResourcesWithWriter(version, commit, writeWindowsSyso)
}

func generateWindowsVersionResourcesWithWriter(version, commit string, writeSyso windowsSysoWriter) error {
	jsonPath, err := absInRoot(versionInfoJSON)
	if err != nil {
		return err
	}
	jsonBytes, err := fileOps.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", versionInfoJSON, err)
	}

	vi := &goversioninfo.VersionInfo{}
	if errParse := vi.ParseJSON(jsonBytes); errParse != nil {
		return fmt.Errorf("parse versioninfo json: %w", errParse)
	}

	fv := fileVersionFromReleaseLabel(version)
	vi.FixedFileInfo.FileVersion = fv
	vi.FixedFileInfo.ProductVersion = fv

	label := displayVersionLabel(version, commit)
	vi.StringFileInfo.FileVersion = label
	vi.StringFileInfo.ProductVersion = label

	vi.Build()
	vi.Walk()

	numericFixed := fmt.Sprintf("%d.%d.%d.%d", fv.Major, fv.Minor, fv.Patch, fv.Build)

	prov := gate.Provenance{
		StepID:   windowsVersionInfoStepID,
		Tool:     "goversioninfo",
		Packages: ".",
	}

	archs := []string{"386", "amd64", "arm", "arm64"}
	for _, arch := range archs {
		if storeErr := writeWindowsVersionResourceArch(vi, arch, label, numericFixed, prov, writeSyso); storeErr != nil {
			return storeErr
		}
	}
	return nil
}

func writeWindowsVersionResourceArch(
	vi *goversioninfo.VersionInfo,
	arch string,
	label string,
	numericFixed string,
	prov gate.Provenance,
	writeSyso windowsSysoWriter,
) error {
	fileName := fmt.Sprintf("resource_windows_%s.syso", arch)
	outAbs, pathErr := absInRoot(fileName)
	if pathErr != nil {
		return pathErr
	}
	if writeErr := writeSyso(vi, outAbs, arch); writeErr != nil {
		return fmt.Errorf("write syso for %s: %w", arch, writeErr)
	}

	return storeWindowsSysoProvenance(fileName, arch, label, numericFixed, vi, prov)
}

func storeWindowsSysoProvenance(
	fileName string,
	arch string,
	label string,
	numericFixed string,
	vi *goversioninfo.VersionInfo,
	prov gate.Provenance,
) error {
	record := windowsSysoProvenanceRecord{
		FileName:            fileName,
		Arch:                arch,
		VersionLabel:        label,
		NumericFixedVersion: numericFixed,
		FileVersionLabel:    vi.StringFileInfo.FileVersion,
		ProductVersionLabel: vi.StringFileInfo.ProductVersion,
		Tool:                "goversioninfo",
	}
	meta, encErr := json.MarshalIndent(record, "", "  ")
	if encErr != nil {
		return fmt.Errorf("encode provenance for %s: %w", fileName, encErr)
	}
	storeName := fileName + ".provenance.json"

	return storeWriteOnce(windowsVersionInfoStepID, storeName, meta, prov)
}

// Windowsresources builds Windows PE version resources for all standard arches. Pass version as the first
// positional argument; optional git commit for string labels via -commit=value (Mage pointer flag).
func (Release) Windowsresources(version string, commit *string) error {
	versionTrim := strings.TrimSpace(version)
	if versionTrim == "" {
		return errReleaseWindowsresourcesVersion
	}
	commitStr := ""
	if commit != nil {
		commitStr = *commit
	}
	return generateWindowsVersionResources(versionTrim, commitStr)
}
