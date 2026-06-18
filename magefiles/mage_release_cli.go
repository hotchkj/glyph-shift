//go:build mage
// +build mage

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	goreleaserArtifactBinary = "Binary"
	releaseCLIBinaryName     = "glyph-shift"
	releaseCLIGoosWindows    = "windows"
)

var (
	errReleaseCLIArtifactsList       = errors.New("read or parse dist/artifacts.json")
	errReleaseCLIBinaryNotFound      = errors.New("release CLI binary not found in dist/artifacts.json for host platform")
	errReleaseCLIBinaryAmbiguous     = errors.New("multiple release CLI binaries in dist/artifacts.json for host platform")
	errReleaseCLIUnsupportedPlatform = errors.New("unsupported platform for release CLI staging")
)

type goreleaserArtifactRecord struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Goos   string `json:"goos"`
	Goarch string `json:"goarch"`
	Type   string `json:"type"`
}

// stageHostReleaseCLI copies the GoReleaser-built CLI binary for this host OS/arch from dist/
// (see dist/artifacts.json, type Binary) into bin/glyph-shift[.exe] for integration subprocess tests.
// This is the same on-disk binary that is later archived — not a separate dev build and not re-extracted from zip.
func stageHostReleaseCLI() error {
	goos, goarch, err := hostGoreleaserPlatform()
	if err != nil {
		return err
	}
	binaryPath, err := hostReleaseCLIBinaryPath(goos, goarch)
	if err != nil {
		return err
	}
	data, err := fileOps.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("read release CLI binary %s: %w", binaryPath, err)
	}
	outRel := cliBinaryOutputRel()
	outPath, err := absInRoot(outRel)
	if err != nil {
		return err
	}
	if mkdirErr := fileOps.MkdirAll(filepath.Dir(outPath), releaseManifestDistDirMode); mkdirErr != nil {
		return fmt.Errorf("mkdir bin: %w", mkdirErr)
	}
	if writeErr := fileOps.WriteFile(outPath, data, releaseScriptFileMode); writeErr != nil {
		return fmt.Errorf("write staged CLI %s: %w", outRel, writeErr)
	}
	return nil
}

func hostGoreleaserPlatform() (goos, goarch string, err error) {
	goos = runtime.GOOS
	switch goos {
	case "darwin", "linux", releaseCLIGoosWindows:
	default:
		return "", "", fmt.Errorf("%w: %s", errReleaseCLIUnsupportedPlatform, goos)
	}
	goarch = runtime.GOARCH
	switch goarch {
	case "amd64", "arm64":
		return goos, goarch, nil
	default:
		return "", "", fmt.Errorf("%w: %s", errReleaseCLIUnsupportedPlatform, goarch)
	}
}

func hostReleaseCLIBinaryPath(goos, goarch string) (string, error) {
	artifactsPath, err := absInRoot(filepath.Join("dist", "artifacts.json"))
	if err != nil {
		return "", err
	}
	raw, err := fileOps.ReadFile(artifactsPath)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errReleaseCLIArtifactsList, err)
	}
	var records []goreleaserArtifactRecord
	if unmarshalErr := json.Unmarshal(raw, &records); unmarshalErr != nil {
		return "", fmt.Errorf("%w: %w", errReleaseCLIArtifactsList, unmarshalErr)
	}
	match, err := selectHostReleaseCLIBinaryPath(records, goos, goarch)
	if err != nil {
		return "", err
	}
	return absInRoot(normalizeDistArtifactPath(match))
}

func selectHostReleaseCLIBinaryPath(records []goreleaserArtifactRecord, goos, goarch string) (string, error) {
	var match string
	for _, rec := range records {
		if rec.Type != goreleaserArtifactBinary || rec.Goos != goos || rec.Goarch != goarch {
			continue
		}
		if match != "" {
			return "", fmt.Errorf("%w: %s/%s", errReleaseCLIBinaryAmbiguous, goos, goarch)
		}
		match = rec.Path
	}
	if match == "" {
		return "", fmt.Errorf("%w: %s/%s", errReleaseCLIBinaryNotFound, goos, goarch)
	}
	return match, nil
}

// normalizeDistArtifactPath maps artifacts.json paths (dist/...) to a root-relative path for absInRoot.
func normalizeDistArtifactPath(path string) string {
	clean := filepath.FromSlash(path)
	if strings.HasPrefix(clean, "dist"+string(filepath.Separator)) || clean == "dist" {
		return clean
	}
	return filepath.Join("dist", clean)
}

func cliBinaryOutputRel() string {
	name := releaseCLIBinaryName
	if runtime.GOOS == releaseCLIGoosWindows {
		name += ".exe"
	}
	return filepath.Join("bin", name)
}
