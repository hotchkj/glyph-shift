//go:build integration

package cli_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/goldenreader"
)

// portableCLIGoldenPaths lists unsuffixed goldens shared across OS families because
// the CLI echoes caller path slash style on traversal rejection (not native absolutes).
// Values are the exact WORKSPACE path placeholders committed in each golden.
var portableCLIGoldenPaths = map[string][]string{
	"extract/stderr/invalid-input-src-outside.golden":  {"WORKSPACE/etc/passwd"},
	"extract/stderr/invalid-input-dest-outside.golden": {"WORKSPACE/tmp/evil.txt"},
	"split/stderr/invalid-input-src-outside.golden":    {"WORKSPACE/etc/passwd"},
	"split/stderr/invalid-input-out-dir.golden":        {"WORKSPACE/tmp/evil"},
	"blocks/stderr/invalid-input-src-outside.golden":   {"WORKSPACE/etc/passwd"},
	"blocks/stderr/invalid-input-out-dir.golden":       {"WORKSPACE/tmp/evil"},
}

func TestResolveCLIGoldenRel(t *testing.T) {
	t.Parallel()

	family := cliGoldenOSFamily()
	osRel := "extract/stdout/preview-45-55." + family + ".golden"
	if _, err := os.Stat(cliGoldenPath(osRel)); err != nil {
		t.Fatalf("OS-specific golden missing: %v", err)
	}

	got := resolveCLIGoldenRel("extract/stdout/preview-45-55.golden")
	if got != osRel {
		t.Fatalf("resolveCLIGoldenRel() = %q, want %q", got, osRel)
	}

	portable := "extract/stdout/contract-apply-1-10.golden"
	if resolveCLIGoldenRel(portable) != portable {
		t.Fatalf("path-free golden should not resolve to OS variant")
	}
}

func TestCLIGoldenPortableRegistryFilesExist(t *testing.T) {
	t.Parallel()

	for rel := range portableCLIGoldenPaths {
		if _, err := os.Stat(cliGoldenPath(rel)); err != nil {
			t.Fatalf("portable golden %q: %v", rel, err)
		}
	}
}

func TestCLIGoldenOSFamilyPairsSymmetric(t *testing.T) {
	t.Parallel()

	root, err := goldenreader.RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}

	windowsByStem, unixByStem, walkErr := collectCLIGoldenOSFamilyStems(t, filepath.Join(root, cliGoldenRootRel))
	if walkErr != nil {
		t.Fatalf("walk goldens: %v", walkErr)
	}

	assertCLIGoldenPairCompleteness(t, windowsByStem, unixByStem)

	if _, statErr := os.Stat(cliGoldenPath("extract/stdout/contract-apply-1-10.golden")); statErr != nil {
		t.Fatal("path-free golden should remain unsuffixed")
	}
}

func collectCLIGoldenOSFamilyStems(
	t *testing.T,
	goldenRoot string,
) (windowsByStem, unixByStem map[string]string, err error) {
	t.Helper()

	windowsByStem = make(map[string]string)
	unixByStem = make(map[string]string)

	err = filepath.WalkDir(goldenRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return nil
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".golden") {
			return nil
		}

		rel, relErr := filepath.Rel(goldenRoot, path)
		if relErr != nil {
			return relErr
		}

		relSlash := filepath.ToSlash(rel)
		recordCLIGoldenStem(t, goldenRoot, relSlash, name, windowsByStem, unixByStem)

		return nil
	})

	return windowsByStem, unixByStem, err
}

func recordCLIGoldenStem(
	t *testing.T,
	goldenRoot, relSlash, name string,
	windowsByStem, unixByStem map[string]string,
) {
	t.Helper()

	switch {
	case strings.HasSuffix(name, ".windows.golden"):
		stem := strings.TrimSuffix(relSlash, ".windows.golden")
		windowsByStem[stem] = relSlash
	case strings.HasSuffix(name, ".unix.golden"):
		stem := strings.TrimSuffix(relSlash, ".unix.golden")
		unixByStem[stem] = relSlash
	default:
		assertUnsuffixedCLIGoldenPathPolicy(t, goldenRoot, relSlash)
	}
}

func assertUnsuffixedCLIGoldenPathPolicy(t *testing.T, goldenRoot, relSlash string) {
	t.Helper()

	path := filepath.Join(goldenRoot, filepath.FromSlash(relSlash))
	data, readErr := os.ReadFile(path) //nolint:gosec // G304: path under committed goldenRoot from WalkDir
	if readErr != nil {
		t.Fatalf("read golden %q: %v", relSlash, readErr)
	}

	gotPaths := workspacePathPlaceholdersInGolden(data)
	if wantPaths, portable := portableCLIGoldenPaths[relSlash]; portable {
		assertPortableCLIGoldenPaths(t, relSlash, gotPaths, wantPaths)
		return
	}

	if len(gotPaths) == 0 {
		return
	}

	if cliGoldenHasOSFamilyPair(goldenRoot, relSlash) {
		return
	}

	t.Errorf(
		"unsuffixed golden %q uses WORKSPACE path placeholders %v but has no .windows/.unix pair",
		relSlash, gotPaths,
	)
}

func cliGoldenHasOSFamilyPair(goldenRoot, relSlash string) bool {
	stem := strings.TrimSuffix(relSlash, ".golden")
	winPath := filepath.Join(goldenRoot, filepath.FromSlash(stem+".windows.golden"))
	unixPath := filepath.Join(goldenRoot, filepath.FromSlash(stem+".unix.golden"))

	if _, err := os.Stat(winPath); err != nil {
		return false
	}
	if _, err := os.Stat(unixPath); err != nil {
		return false
	}

	return true
}

func assertPortableCLIGoldenPaths(t *testing.T, relSlash string, got, want []string) {
	t.Helper()

	gotSorted := append([]string(nil), got...)
	wantSorted := append([]string(nil), want...)
	slices.Sort(gotSorted)
	slices.Sort(wantSorted)
	if !slices.Equal(gotSorted, wantSorted) {
		t.Errorf("portable golden %q: WORKSPACE paths = %v, want %v", relSlash, got, want)
	}

	for _, p := range got {
		if strings.Contains(p, `\`) {
			t.Errorf("portable golden %q: path %q must use forward slashes", relSlash, p)
		}
	}
}

func TestCLIGoldenUnixPathsUseForwardSlashes(t *testing.T) {
	t.Parallel()

	root, err := goldenreader.RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}

	goldenRoot := filepath.Join(root, cliGoldenRootRel)
	err = filepath.WalkDir(goldenRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".unix.golden") {
			return nil
		}

		data, readErr := os.ReadFile(path) //nolint:gosec // G304: path under committed goldenRoot from WalkDir
		if readErr != nil {
			return readErr
		}

		if unixGoldenHasBackslashInWorkspacePath(data) {
			rel, relErr := filepath.Rel(goldenRoot, path)
			if relErr != nil {
				return relErr
			}

			t.Errorf(".unix.golden %s has backslash in WORKSPACE path; use forward slashes throughout", filepath.ToSlash(rel))
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk goldens: %v", err)
	}
}

// workspacePathPlaceholdersInGolden returns JSON string values that begin with WORKSPACE.
func workspacePathPlaceholdersInGolden(data []byte) []string {
	var paths []string

	searchFrom := 0
	needle := []byte(`"WORKSPACE`)

	for {
		rel := data[searchFrom:]
		i := bytes.Index(rel, needle)
		if i < 0 {
			break
		}

		valueStart := searchFrom + i + 1
		rest := data[valueStart:]
		endQuote := bytes.IndexByte(rest, '"')
		if endQuote < len("WORKSPACE") {
			searchFrom = valueStart + len("WORKSPACE")
			continue
		}

		paths = append(paths, string(rest[:endQuote]))
		searchFrom = valueStart + endQuote
	}

	return paths
}

func unixGoldenHasBackslashInWorkspacePath(data []byte) bool {
	for _, p := range workspacePathPlaceholdersInGolden(data) {
		if strings.Contains(p, `\`) {
			return true
		}
	}

	return false
}

func assertCLIGoldenPairCompleteness(t *testing.T, windowsByStem, unixByStem map[string]string) {
	t.Helper()

	for stem, winRel := range windowsByStem {
		if _, ok := unixByStem[stem]; !ok {
			t.Errorf("missing .unix.golden pair for %s", winRel)
		}
	}

	for stem, unixRel := range unixByStem {
		if _, ok := windowsByStem[stem]; !ok {
			t.Errorf("missing .windows.golden pair for %s", unixRel)
		}
	}
}
