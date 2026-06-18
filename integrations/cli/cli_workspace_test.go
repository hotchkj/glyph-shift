//go:build integration

package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/features/harness"
	"github.com/hotchkj/glyph-shift/internal/goldenreader"
)

const cliOutDir = harness.OutDir

func cliWriteFile(t *testing.T, ws, rel string, data []byte) {
	t.Helper()

	path := filepath.Join(ws, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), cliFixtureDirPerm); err != nil {
		t.Fatalf("mkdir parents for %q: %v", rel, err)
	}
	if err := os.WriteFile(path, data, cliFixtureFilePerm); err != nil {
		t.Fatalf("write %q: %v", rel, err)
	}
}

func cliWriteNumberedLines(t *testing.T, ws, name string, lineCount int, terminator string) {
	t.Helper()

	data, err := harness.NumberedLineContent(lineCount, terminator)
	if err != nil {
		t.Fatalf("numbered lines: %v", err)
	}

	cliWriteFile(t, ws, name, data)
}

func cliWriteFeaturesInput(t *testing.T, ws, wsName, inputFile string, escaped bool) {
	t.Helper()

	data, err := goldenreader.ReadGolden(filepath.Join("testdata", "inputs", filepath.ToSlash(inputFile)))
	if err != nil {
		t.Fatalf("ReadGolden input %q: %v", inputFile, err)
	}

	if escaped {
		data, err = harness.DecodeEscapedFixture(data)
		if err != nil {
			t.Fatalf("decode escaped input %q: %v", inputFile, err)
		}
	}

	cliWriteFile(t, ws, wsName, data)
}

func cliWriteFromFeaturesInput(t *testing.T, ws, wsName, inputFile string) {
	t.Helper()
	cliWriteFeaturesInput(t, ws, wsName, inputFile, false)
}

func cliWriteFromFeaturesEscapedInput(t *testing.T, ws, wsName, inputFile string) {
	t.Helper()
	cliWriteFeaturesInput(t, ws, wsName, inputFile, true)
}

func cliMkdir(t *testing.T, ws, rel string) {
	t.Helper()

	path := filepath.Join(ws, filepath.FromSlash(rel))
	if err := os.MkdirAll(path, cliFixtureDirPerm); err != nil {
		t.Fatalf("mkdir %q: %v", rel, err)
	}
}

func cliWriteBinarySource(t *testing.T, ws, name string, data []byte) {
	t.Helper()
	cliWriteFile(t, ws, name, data)
}

func cliExpectedDirOutputs(t *testing.T, outDir, featuresExpectedDir string) []cliOutputExpect {
	t.Helper()

	entries, err := goldenreader.ReadGoldenDir(filepath.Join(
		"testdata", "expected", filepath.ToSlash(featuresExpectedDir),
	))
	if err != nil {
		t.Fatalf("ReadGoldenDir %q: %v", featuresExpectedDir, err)
	}

	var out []cliOutputExpect
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}

		wsName := strings.TrimSuffix(ent.Name(), ".bytes")

		out = append(out, cliOutputExpect{
			wsRel:            filepath.ToSlash(filepath.Join(outDir, wsName)),
			featuresExpected: filepath.Join("testdata", "expected", featuresExpectedDir, ent.Name()),
		})
	}

	return out
}

func cliAssertOutDirMatchesExpected(t *testing.T, ws, featuresExpectedDir string) {
	t.Helper()

	assertCLIOutputs(t, ws, cliExpectedDirOutputs(t, cliOutDir, featuresExpectedDir))
}

// cliSeedOutDirConflict pre-creates out/001.md for destination-exists scenarios.
func cliSeedOutDirConflict(t *testing.T, ws string, data []byte) {
	t.Helper()

	cliMkdir(t, ws, cliOutDir)
	cliWriteFile(t, ws, filepath.Join(cliOutDir, "001.md"), data)
}

func cliReadWorkspaceFile(t *testing.T, ws, rel string) []byte {
	t.Helper()

	data, err := os.ReadFile( //nolint:gosec // G304: path under t.TempDir() workspace
		filepath.Join(ws, filepath.FromSlash(rel)),
	)
	if err != nil {
		t.Fatalf("read %q: %v", rel, err)
	}

	return data
}

func cliAssertAllFilesHaveExtension(t *testing.T, ws, dir, ext string) {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(ws, filepath.FromSlash(dir)))
	if err != nil {
		t.Fatalf("read dir %q: %v", dir, err)
	}

	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		if !strings.HasSuffix(ent.Name(), ext) {
			t.Fatalf("file %q in %q: want extension %q", ent.Name(), dir, ext)
		}
	}
}

func cliAssertOutDirEveryFileTerminator(t *testing.T, ws, terminator string) {
	t.Helper()

	root := filepath.Join(ws, cliOutDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read dir %q: %v", cliOutDir, err)
	}

	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}

		rel := filepath.ToSlash(filepath.Join(cliOutDir, ent.Name()))
		cliAssertEveryLineTerminator(t, ws, rel, terminator)
	}
}

func cliAssertEveryLineTerminator(t *testing.T, ws, rel, terminator string) {
	t.Helper()

	data := cliReadWorkspaceFile(t, ws, rel)
	lines, err := harness.ReadLinesFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse %q: %v", rel, err)
	}

	want, err := harness.LineTerminator(terminator)
	if err != nil {
		t.Fatalf("terminator %q: %v", terminator, err)
	}

	for i, ln := range lines {
		if !bytes.Equal(ln.Terminator, want) {
			t.Fatalf("%s line %d: want %s terminator got %q", rel, i+1, terminator, ln.Terminator)
		}
	}
}

func cliAssertNoTrailingWhitespace(t *testing.T, ws, rel string) {
	t.Helper()

	data := cliReadWorkspaceFile(t, ws, rel)
	lines, err := harness.ReadLinesFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse %q: %v", rel, err)
	}

	for i, ln := range lines {
		content := string(ln.Content)
		if strings.TrimRight(content, " \t") != content {
			t.Fatalf("%s line %d has trailing whitespace", rel, i+1)
		}
	}
}

// cliAssertTestTxtFinalNewline checks test.txt (all transform afterRun cases use that path).
// finalNewline: "with", "exactly-one", or "without" (matches BDD transform Then steps).
func cliAssertTestTxtFinalNewline(t *testing.T, ws, finalNewline string) {
	t.Helper()

	const rel = "test.txt"
	data := cliReadWorkspaceFile(t, ws, rel)
	switch finalNewline {
	case "with":
		cliAssertFileEndsWithNewline(t, rel, data)
	case "exactly-one":
		cliAssertFileEndsWithExactlyOneNewline(t, rel, data)
	case "without":
		cliAssertFileEndsWithoutNewline(t, rel, data)
	default:
		t.Fatalf("unknown final newline mode %q", finalNewline)
	}
}

func cliAssertFileEndsWithNewline(t *testing.T, rel string, data []byte) {
	t.Helper()
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("%q does not end with newline", rel)
	}
}

func cliAssertFileEndsWithExactlyOneNewline(t *testing.T, rel string, data []byte) {
	t.Helper()
	cliAssertFileEndsWithNewline(t, rel, data)
	if len(data) >= 2 && data[len(data)-2] == '\n' {
		t.Fatalf("%q ends with more than one newline", rel)
	}
}

func cliAssertFileEndsWithoutNewline(t *testing.T, rel string, data []byte) {
	t.Helper()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		t.Fatalf("%q ends with newline unexpectedly", rel)
	}
}

func cliAssertStillHasTrailingWhitespace(t *testing.T, ws, rel string) {
	t.Helper()

	data := cliReadWorkspaceFile(t, ws, rel)
	lines, err := harness.ReadLinesFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse %q: %v", rel, err)
	}

	for _, ln := range lines {
		content := string(ln.Content)
		if strings.TrimRight(content, " \t") != content {
			return
		}
	}

	t.Fatalf("%q has no trailing whitespace", rel)
}

func cliAssertOutDirFileCount(t *testing.T, ws string, want int) {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(ws, cliOutDir))
	if err != nil {
		t.Fatalf("read dir %q: %v", cliOutDir, err)
	}

	count := 0
	for _, ent := range entries {
		if !ent.IsDir() {
			count++
		}
	}

	if count != want {
		t.Fatalf("dir %q: want %d files got %d", cliOutDir, want, count)
	}
}
