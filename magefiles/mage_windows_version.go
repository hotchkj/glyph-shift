//go:build mage
// +build mage

package main

import (
	"strings"

	"github.com/josephspurrier/goversioninfo"
)

const maxU16 = 65535

func clampIntU16(value int) int {
	switch {
	case value < 0:
		return 0
	case value > maxU16:
		return maxU16
	default:
		return value
	}
}

func clampFileVersion(f goversioninfo.FileVersion) goversioninfo.FileVersion {
	return goversioninfo.FileVersion{
		Major: clampIntU16(f.Major),
		Minor: clampIntU16(f.Minor),
		Patch: clampIntU16(f.Patch),
		Build: clampIntU16(f.Build),
	}
}

// semverNumericCore returns X.Y.Z or X.Y.Z.W suitable for goversioninfo.NewFileVersion (no pre-release / metadata).
func semverNumericCore(label string) string {
	trimmed := strings.TrimSpace(label)
	if trimmed != "" && (trimmed[0] == 'v' || trimmed[0] == 'V') {
		trimmed = trimmed[1:]
	}
	if i := strings.IndexByte(trimmed, '+'); i >= 0 {
		trimmed = trimmed[:i]
	}
	if i := strings.IndexByte(trimmed, '-'); i >= 0 {
		trimmed = trimmed[:i]
	}
	return strings.TrimSpace(trimmed)
}

func fileVersionFromReleaseLabel(label string) goversioninfo.FileVersion {
	core := semverNumericCore(label)
	if core == "" {
		return goversioninfo.FileVersion{}
	}
	fv, err := goversioninfo.NewFileVersion(core)
	if err != nil {
		return goversioninfo.FileVersion{}
	}
	return clampFileVersion(fv)
}

func isHexRune(ch rune) bool {
	switch {
	case ch >= '0' && ch <= '9':
		return true
	case ch >= 'a' && ch <= 'f':
		return true
	case ch >= 'A' && ch <= 'F':
		return true
	default:
		return false
	}
}

func isHexString(hex string) bool {
	if hex == "" {
		return false
	}
	for _, ch := range hex {
		if !isHexRune(ch) {
			return false
		}
	}
	return true
}

func abbreviateGitCommit(hash string) string {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return ""
	}
	if len(hash) >= 40 && isHexString(hash[:40]) {
		return hash[:7]
	}
	if len(hash) > 7 && isHexString(hash) {
		return hash[:7]
	}
	return hash
}

// displayVersionLabel preserves the release label and appends commit metadata (semver build style).
func displayVersionLabel(version, commit string) string {
	versionTrim := strings.TrimSpace(version)
	commitTrim := strings.TrimSpace(commit)
	if commitTrim != "" && strings.EqualFold(commitTrim, "none") {
		commitTrim = ""
	}
	if commitTrim == "" {
		return versionTrim
	}
	short := abbreviateGitCommit(commitTrim)
	if short == "" {
		return versionTrim
	}
	if strings.Contains(versionTrim, "+") {
		return versionTrim + "." + short
	}
	return versionTrim + "+" + short
}
