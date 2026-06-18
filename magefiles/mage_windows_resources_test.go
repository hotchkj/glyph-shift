//go:build mage
// +build mage

package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
	"github.com/josephspurrier/goversioninfo"
)

var errWriteWindowsSyso = errors.New("write syso failed")

func TestClampIntU16(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   int
		want int
	}{
		{-1, 0},
		{0, 0},
		{65535, 65535},
		{65536, 65535},
		{1000, 1000},
	}
	for _, tc := range cases {
		if got := clampIntU16(tc.in); got != tc.want {
			t.Fatalf("clampIntU16(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSemverNumericCore(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"v1.2.3", "1.2.3"},
		{"V0.0.4", "0.0.4"},
		{"  2.3.4-rc.1  ", "2.3.4"},
		{"5.6.7+buildmeta", "5.6.7"},
		{"9.8.7-beta+meta", "9.8.7"},
		{"", ""},
		{"   ", ""},
	}
	for _, tc := range cases {
		if got := semverNumericCore(tc.in); got != tc.want {
			t.Fatalf("semverNumericCore(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDisplayVersionLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		version, commit string
		want            string
	}{
		{"1.0.0", "", "1.0.0"},
		{"1.0.0", "  none  ", "1.0.0"},
		{"1.0.0", "abc123def456", "1.0.0+abc123d"},
		{"1.0.0+already", "fedcba987654321098765432109876543210abcd", "1.0.0+already.fedcba9"},
	}
	for _, tc := range cases {
		if got := displayVersionLabel(tc.version, tc.commit); got != tc.want {
			t.Fatalf("displayVersionLabel(%q,%q) = %q, want %q", tc.version, tc.commit, got, tc.want)
		}
	}
}

func TestAbbreviateGitCommit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"  ", ""},
		{"short", "short"},
		{"abcdefg", "abcdefg"},
		{"abcdef0", "abcdef0"},
		{"1234567890123", "1234567"},
		{"fedcba987654321098765432109876543210abcd", "fedcba9"},
	}
	for _, tc := range cases {
		if got := abbreviateGitCommit(tc.in); got != tc.want {
			t.Fatalf("abbreviateGitCommit(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsHexString(t *testing.T) {
	t.Parallel()

	if isHexString("") {
		t.Fatal(`isHexString("") = true, want false`)
	}
	if !isHexString("deadbeef") {
		t.Fatal("isHexString(deadbeef) = false, want true")
	}
	if !isHexString("DEADBEEF") {
		t.Fatal("isHexString(DEADBEEF) = false, want true")
	}
	if isHexString("not-hex") {
		t.Fatal("isHexString(not-hex) = true, want false")
	}
}

func TestFileVersionFromReleaseLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		label string
		want  goversioninfo.FileVersion
	}{
		{"", goversioninfo.FileVersion{}},
		{"not semver at all", goversioninfo.FileVersion{}},
		{"1.2.3.4", goversioninfo.FileVersion{Major: 1, Minor: 2, Patch: 3, Build: 4}},
		{"v2.0.0-ignored", goversioninfo.FileVersion{Major: 2, Minor: 0, Patch: 0, Build: 0}},
	}
	for _, tc := range cases {
		got := fileVersionFromReleaseLabel(tc.label)
		if got != tc.want {
			t.Fatalf("fileVersionFromReleaseLabel(%q) = %+v, want %+v", tc.label, got, tc.want)
		}
	}

	large := fileVersionFromReleaseLabel("999999.1.0.0")
	if large.Major != maxU16 || large.Minor != 1 || large.Patch != 0 || large.Build != 0 {
		t.Fatalf("clamped large major = %+v, want Major=%d Minor=1 Patch=0 Build=0", large, maxU16)
	}
}

const testVersionInfoJSON = `{
	"FixedFileInfo": {
		"FileVersion": {"Major": 0, "Minor": 0, "Patch": 0, "Build": 0},
		"ProductVersion": {"Major": 0, "Minor": 0, "Patch": 0, "Build": 0},
		"FileFlagsMask": "3f",
		"FileFlags": "00",
		"FileOS": "040004",
		"FileType": "01",
		"FileSubType": "00"
	},
	"StringFileInfo": {
		"CompanyName": "hotchkj",
		"FileDescription": "glyph-shift command-line tool",
		"FileVersion": "",
		"InternalName": "glyph-shift",
		"OriginalFilename": "glyph-shift.exe",
		"ProductName": "glyph-shift",
		"ProductVersion": ""
	},
	"VarFileInfo": {"Translation": {"LangID": "0409", "CharsetID": "04B0"}},
	"IconPath": "",
	"ManifestPath": ""
}`

func withWindowsResourceArtifacts(t *testing.T) *gatetest.MemoryFileOps {
	t.Helper()

	oldRootDir := rootDir
	oldFileOps := fileOps
	oldStore := store
	oldPathInRoot := pathInRoot
	t.Cleanup(func() {
		rootDir = oldRootDir
		fileOps = oldFileOps
		store = oldStore
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

func TestGenerateWindowsVersionResourcesWithWriterStoresProvenance(t *testing.T) {
	mem := withWindowsResourceArtifacts(t)

	jsonPath, err := absInRoot(versionInfoJSON)
	if err != nil {
		t.Fatalf("absInRoot: %v", err)
	}
	if writeErr := mem.WriteFile(jsonPath, []byte(testVersionInfoJSON), 0o644); writeErr != nil {
		t.Fatalf("WriteFile versioninfo: %v", writeErr)
	}

	var writes []string
	writer := func(vi *goversioninfo.VersionInfo, outputPath, arch string) error {
		if vi.StringFileInfo.FileVersion != "v1.2.3+abcdef0" {
			t.Fatalf("FileVersion label = %q, want v1.2.3+abcdef0", vi.StringFileInfo.FileVersion)
		}
		writes = append(writes, filepath.Base(outputPath)+":"+arch)

		return nil
	}

	err = generateWindowsVersionResourcesWithWriter("v1.2.3", "abcdef012345", writer)
	if err != nil {
		t.Fatalf("generateWindowsVersionResourcesWithWriter: %v", err)
	}

	assertWindowsSysoWrites(t, writes)
	assertWindowsSysoProvenance(t)
}

func assertWindowsSysoWrites(t *testing.T, writes []string) {
	t.Helper()

	wantWrites := []string{
		"resource_windows_386.syso:386",
		"resource_windows_amd64.syso:amd64",
		"resource_windows_arm.syso:arm",
		"resource_windows_arm64.syso:arm64",
	}
	if len(writes) != len(wantWrites) {
		t.Fatalf("writes len = %d, want %d (%v)", len(writes), len(wantWrites), writes)
	}
	for index, want := range wantWrites {
		if writes[index] != want {
			t.Fatalf("writes[%d] = %q, want %q", index, writes[index], want)
		}
	}
}

func assertWindowsSysoProvenance(t *testing.T) {
	t.Helper()

	recordBytes, err := store.Read(windowsVersionInfoStepID, "resource_windows_amd64.syso.provenance.json")
	if err != nil {
		t.Fatalf("Read provenance: %v", err)
	}
	var record windowsSysoProvenanceRecord
	if err := json.Unmarshal(recordBytes, &record); err != nil {
		t.Fatalf("Unmarshal provenance: %v", err)
	}
	if record.Arch != "amd64" || record.NumericFixedVersion != "1.2.3.0" {
		t.Fatalf("provenance = %+v, want arch amd64 numeric 1.2.3.0", record)
	}
}

func TestGenerateWindowsVersionResourcesWithWriterReadAndParseErrors(t *testing.T) {
	mem := withWindowsResourceArtifacts(t)

	err := generateWindowsVersionResourcesWithWriter("1.0.0", "", func(*goversioninfo.VersionInfo, string, string) error {
		t.Fatal("writer must not be called when versioninfo.json is missing")
		return nil
	})
	if err == nil {
		t.Fatal("missing versioninfo.json error = nil, want error")
	}

	jsonPath, pathErr := absInRoot(versionInfoJSON)
	if pathErr != nil {
		t.Fatalf("absInRoot: %v", pathErr)
	}
	if writeErr := mem.WriteFile(jsonPath, []byte("{not-json"), 0o644); writeErr != nil {
		t.Fatalf("WriteFile invalid versioninfo: %v", writeErr)
	}
	err = generateWindowsVersionResourcesWithWriter("1.0.0", "", func(*goversioninfo.VersionInfo, string, string) error {
		t.Fatal("writer must not be called when versioninfo.json is invalid")
		return nil
	})
	if err == nil {
		t.Fatal("invalid versioninfo.json error = nil, want error")
	}
}

func TestGenerateWindowsVersionResourcesWithWriterSurfacesWriterError(t *testing.T) {
	mem := withWindowsResourceArtifacts(t)

	jsonPath, err := absInRoot(versionInfoJSON)
	if err != nil {
		t.Fatalf("absInRoot: %v", err)
	}
	if writeErr := mem.WriteFile(jsonPath, []byte(testVersionInfoJSON), 0o644); writeErr != nil {
		t.Fatalf("WriteFile versioninfo: %v", writeErr)
	}

	err = generateWindowsVersionResourcesWithWriter("1.0.0", "", func(*goversioninfo.VersionInfo, string, string) error {
		return errWriteWindowsSyso
	})
	if !errors.Is(err, errWriteWindowsSyso) {
		t.Fatalf("writer error = %v, want %v", err, errWriteWindowsSyso)
	}
}

func TestReleaseWindowsresourcesRejectsBlankVersion(t *testing.T) {
	t.Parallel()

	var release Release
	if err := release.Windowsresources(" \t ", nil); !errors.Is(err, errReleaseWindowsresourcesVersion) {
		t.Fatalf("Windowsresources blank version error = %v, want %v", err, errReleaseWindowsresourcesVersion)
	}
}

func TestReleaseWindowsresourcesSurfacesMissingVersionInfo(t *testing.T) {
	withWindowsResourceArtifacts(t)

	var release Release
	if err := release.Windowsresources("1.0.0", nil); err == nil {
		t.Fatal("Windowsresources missing versioninfo: expected error")
	}
}

func TestReleaseWindowsresourcesAcceptsCommitArgumentBeforeVersionInfoRead(t *testing.T) {
	withWindowsResourceArtifacts(t)

	commit := "abcdef0"
	var release Release
	if err := release.Windowsresources("1.0.0", &commit); err == nil {
		t.Fatal("Windowsresources missing versioninfo with commit: expected error")
	}
}

func TestCleanupWindowsSysoInputsNoMatches(t *testing.T) {
	withWindowsResourceArtifacts(t)

	if err := cleanupWindowsSysoInputs(); err != nil {
		t.Fatalf("cleanupWindowsSysoInputs no matches: %v", err)
	}
}
