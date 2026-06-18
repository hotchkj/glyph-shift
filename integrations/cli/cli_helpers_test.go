//go:build integration

package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hotchkj/glyph-shift/features/harness"
	"github.com/hotchkj/glyph-shift/internal/goldenreader"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/gate"
)

const (
	cliGoldenRootRel       = "integrations/cli/testdata/golden"
	cliGoldenFamilyWindows = "windows"
	cliGoldenFamilyUnix    = "unix"
	cliFixtureDirPerm      = 0o750
	cliFixtureFilePerm     = 0o600
	cliSubprocessTimeout   = 60 * time.Second
)

// cliCase is one BDD-aligned subprocess scenario.
type cliCase struct {
	name         string
	setup        func(t *testing.T, ws string)
	argv         []string
	wantExit     int
	stdoutGolden string // under cliGoldenRootRel; empty means stdout must be empty
	stderrGolden string
	outputs      []cliOutputExpect
	lineRanges   []cliLineRangeExpect
	appends      []cliAppendExpect
	missing      []string                      // workspace-relative paths that must not exist after run
	unchanged    []string                      // workspace-relative paths that must match bytes captured after setup
	afterRun     func(t *testing.T, ws string) // optional file/content checks beyond golden I/O
}

type cliOutputExpect struct {
	wsRel            string
	featuresExpected string // path relative to features/, e.g. testdata/expected/...
}

type cliLineRangeExpect struct {
	outRel    string
	srcRel    string
	startLine int
	endLine   int
}

type cliAppendExpect struct {
	outRel              string
	prefixFeaturesInput string
	suffixSrcRel        string
	suffixStart         int
	suffixEnd           int
}

func cliGoldenPath(rel string) string {
	root, err := goldenreader.RepoRoot()
	if err != nil {
		panic(err)
	}

	return filepath.Join(root, cliGoldenRootRel, filepath.FromSlash(rel))
}

// cliGoldenOSFamily returns the golden filename suffix family for the host OS.
func cliGoldenOSFamily() string {
	if runtime.GOOS == cliGoldenFamilyWindows {
		return cliGoldenFamilyWindows
	}

	return cliGoldenFamilyUnix
}

// resolveCLIGoldenRel maps a logical golden rel (e.g. extract/stdout/preview-45-55.golden)
// to an OS-specific variant when present (preview-45-55.windows.golden / .unix.golden).
func resolveCLIGoldenRel(rel string) string {
	if rel == "" || !strings.HasSuffix(rel, ".golden") {
		return rel
	}

	stem := strings.TrimSuffix(rel, ".golden")
	osRel := stem + "." + cliGoldenOSFamily() + ".golden"
	if _, err := os.Stat(cliGoldenPath(osRel)); err == nil {
		return osRel
	}

	return rel
}

func resolveBinary(t *testing.T) string {
	t.Helper()

	root, err := goldenreader.RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}

	name := "glyph-shift"
	if runtime.GOOS == cliGoldenFamilyWindows {
		name += ".exe"
	}

	path := filepath.Join(root, "bin", name)
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("CLI binary missing at %s (run mage integration): %v", path, statErr)
	}

	return path
}

func runCLI(t *testing.T, workspace string, argv []string) cmdrunner.CommandResult {
	t.Helper()

	runner := gate.NewProductionRunner()
	binary := resolveBinary(t)
	cmd := cmdrunner.NewCommand(workspace, binary, argv...)

	ctx, cancel := context.WithTimeout(context.Background(), cliSubprocessTimeout)
	defer cancel()

	result, capErr := cmdrunner.Capture(ctx, runner, cmd.Dir(), cmd.Name(), cmd.Args()...)
	// Non-zero exits return an error from Capture but still populate stdout/stderr/ExitCode.
	if capErr != nil && result.ExitCode == 0 {
		t.Fatalf("Capture %v: %v (stdout=%q stderr=%q)", argv, capErr, result.Stdout, result.Stderr)
	}

	return result
}

func readCLIGolden(t *testing.T, rel string) []byte {
	t.Helper()
	if rel == "" {
		return nil
	}

	data, err := os.ReadFile(cliGoldenPath(resolveCLIGoldenRel(rel)))
	if err != nil {
		t.Fatalf("read golden %q: %v", rel, err)
	}

	return data
}

// normalizeCLIJSON rewrites ephemeral absolute paths in live subprocess JSON output
// so comparisons use committed WORKSPACE placeholders. Golden bytes are not passed through this.
func normalizeCLIJSON(workspace string, raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}

	ws := filepath.Clean(workspace)
	repl := []string{
		ws,
		filepath.ToSlash(ws),
		strings.ReplaceAll(ws, `\`, `\\`),
	}
	out := raw
	for _, r := range repl {
		if r == "" {
			continue
		}
		out = bytes.ReplaceAll(out, []byte(r), []byte("WORKSPACE"))
	}

	tempRoot := filepath.Clean(os.TempDir())
	for _, pair := range []struct{ abs, placeholder string }{
		{filepath.Join(tempRoot, "etc", "passwd"), "WORKSPACE/etc/passwd"},
		{filepath.Join(tempRoot, "tmp", "evil"), "WORKSPACE/tmp/evil"},
		{filepath.Join(tempRoot, "tmp", "evil.txt"), "WORKSPACE/tmp/evil.txt"},
	} {
		for _, form := range []string{pair.abs, filepath.ToSlash(pair.abs), strings.ReplaceAll(pair.abs, `\`, `\\`)} {
			if form == "" {
				continue
			}
			out = bytes.ReplaceAll(out, []byte(form), []byte(pair.placeholder))
		}
	}

	return out
}

func assertCLIStream(
	t *testing.T,
	workspace string,
	got string,
	goldenRel string,
	wantEmpty bool,
	stream string,
) {
	t.Helper()

	if wantEmpty {
		if got != "" {
			t.Fatalf("%s: want empty, got %q", stream, got)
		}

		return
	}

	want := readCLIGolden(t, goldenRel)
	gotNorm := normalizeCLIJSON(workspace, []byte(got))
	resolvedGolden := resolveCLIGoldenRel(goldenRel)
	if !bytes.Equal(gotNorm, want) {
		t.Fatalf("%s mismatch (golden %s)\nwant:\n%s\ngot:\n%s",
			stream, resolvedGolden, string(want), string(gotNorm))
	}
}

func readFeaturesExpectedBytes(t *testing.T, featuresRel string) []byte {
	t.Helper()

	data, err := goldenreader.ReadGolden(featuresRel)
	if err != nil {
		t.Fatalf("ReadGolden %q: %v", featuresRel, err)
	}

	if strings.HasSuffix(featuresRel, ".bytes") {
		decoded, decErr := harness.DecodeEscapedFixture(data)
		if decErr != nil {
			t.Fatalf("decode expected %q: %v", featuresRel, decErr)
		}

		return decoded
	}

	return data
}

func expectedLineRangeBytes(t *testing.T, workspace, srcRel string, start, end int) []byte {
	t.Helper()

	srcPath := filepath.Join(workspace, filepath.FromSlash(srcRel))
	srcData, err := os.ReadFile(srcPath) //nolint:gosec // G304: path under t.TempDir() workspace
	if err != nil {
		t.Fatalf("read source %q: %v", srcRel, err)
	}

	out, err := harness.ExpectedLineRangeBytes(srcData, start, end)
	if err != nil {
		t.Fatalf("line range %d-%d for %q: %v", start, end, srcRel, err)
	}

	return out
}

func assertCLIAppendOutputs(t *testing.T, workspace string, expects []cliAppendExpect) {
	t.Helper()

	for _, exp := range expects {
		prefix, err := goldenreader.ReadGolden(filepath.Join("testdata", "inputs", exp.prefixFeaturesInput))
		if err != nil {
			t.Fatalf("ReadGolden prefix %q: %v", exp.prefixFeaturesInput, err)
		}

		suffix := expectedLineRangeBytes(t, workspace, exp.suffixSrcRel, exp.suffixStart, exp.suffixEnd)
		want := append(append([]byte(nil), prefix...), suffix...)
		outPath := filepath.Join(workspace, filepath.FromSlash(exp.outRel))
		got, err := os.ReadFile(outPath) //nolint:gosec // G304: path under t.TempDir() workspace
		if err != nil {
			t.Fatalf("read append output %q: %v", exp.outRel, err)
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("append output %q mismatch", exp.outRel)
		}
	}
}

func assertCLILineRangeOutputs(t *testing.T, workspace string, expects []cliLineRangeExpect) {
	t.Helper()

	for _, exp := range expects {
		want := expectedLineRangeBytes(t, workspace, exp.srcRel, exp.startLine, exp.endLine)
		outPath := filepath.Join(workspace, filepath.FromSlash(exp.outRel))
		got, err := os.ReadFile(outPath) //nolint:gosec // G304: path under t.TempDir() workspace
		if err != nil {
			t.Fatalf("read output %q: %v", exp.outRel, err)
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("line range %d-%d from %q vs %q:\nwant %q\ngot %q",
				exp.startLine, exp.endLine, exp.srcRel, exp.outRel, string(want), string(got))
		}
	}
}

func assertCLIOutputs(t *testing.T, workspace string, expects []cliOutputExpect) {
	t.Helper()

	for _, exp := range expects {
		want := readFeaturesExpectedBytes(t, exp.featuresExpected)

		gotPath := filepath.Join(workspace, filepath.FromSlash(exp.wsRel))
		got, err := os.ReadFile(gotPath) //nolint:gosec // G304: path under t.TempDir() workspace
		if err != nil {
			t.Fatalf("read output %q: %v", exp.wsRel, err)
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("output %q bytes mismatch vs %q", exp.wsRel, exp.featuresExpected)
		}
	}
}

func assertCLIMissing(t *testing.T, workspace string, paths []string) {
	t.Helper()

	for _, rel := range paths {
		p := filepath.Join(workspace, filepath.FromSlash(rel))
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("path %q should not exist", rel)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %q: %v", rel, err)
		}
	}
}

func assertCLIUnchanged(t *testing.T, workspace string, paths []string, before map[string][]byte) {
	t.Helper()

	for _, rel := range paths {
		p := filepath.Join(workspace, filepath.FromSlash(rel))
		got, err := os.ReadFile(p) //nolint:gosec // G304: path under t.TempDir() workspace
		if err != nil {
			t.Fatalf("read unchanged %q: %v", rel, err)
		}

		want, ok := before[rel]
		if !ok {
			t.Fatalf("no before snapshot for %q", rel)
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("path %q changed unexpectedly", rel)
		}
	}
}

func snapshotWorkspaceFiles(t *testing.T, workspace string, paths []string) map[string][]byte {
	t.Helper()

	out := make(map[string][]byte, len(paths))
	for _, rel := range paths {
		p := filepath.Join(workspace, filepath.FromSlash(rel))
		data, err := os.ReadFile(p) //nolint:gosec // G304: path under t.TempDir() workspace
		if err != nil {
			t.Fatalf("snapshot %q: %v", rel, err)
		}

		out[rel] = append([]byte(nil), data...)
	}

	return out
}

func runCLICase(t *testing.T, row *cliCase) {
	t.Helper()

	ws := t.TempDir()
	if row.setup != nil {
		row.setup(t, ws)
	}

	var before map[string][]byte
	if len(row.unchanged) > 0 {
		before = snapshotWorkspaceFiles(t, ws, row.unchanged)
	}

	result := runCLI(t, ws, row.argv)

	if result.ExitCode != row.wantExit {
		t.Fatalf("exit: got %d want %d\nstderr=%q\nstdout=%q",
			result.ExitCode, row.wantExit, result.Stderr, result.Stdout)
	}

	wantStdoutEmpty := row.stdoutGolden == "" && row.wantExit != 0
	wantStderrEmpty := row.stderrGolden == "" && row.wantExit == 0

	assertCLIStream(t, ws, result.Stdout, row.stdoutGolden, wantStdoutEmpty, "stdout")
	assertCLIStream(t, ws, result.Stderr, row.stderrGolden, wantStderrEmpty, "stderr")
	assertCLIOutputs(t, ws, row.outputs)
	assertCLILineRangeOutputs(t, ws, row.lineRanges)
	assertCLIAppendOutputs(t, ws, row.appends)
	assertCLIMissing(t, ws, row.missing)
	assertCLIUnchanged(t, ws, row.unchanged, before)
	if row.afterRun != nil {
		row.afterRun(t, ws)
	}
}

func extractArgv(lines, src, dst string, flags ...string) []string {
	args := []string{
		"extract",
		"--source", src,
		"--lines", lines,
		"--destination", dst,
	}
	return append(args, flags...)
}

func splitArgv(src, delimiter, outDir string, flags ...string) []string {
	args := []string{
		"split",
		"--source", src,
		"--delimiter", delimiter,
		"--output-dir", outDir,
	}
	return append(args, flags...)
}

func blocksArgv(src, start, end, outDir string, flags ...string) []string {
	args := []string{
		"blocks",
		"--source", src,
		"--start-line", start,
		"--end-line", end,
		"--output-dir", outDir,
	}
	return append(args, flags...)
}

func transformArgv(src string, flags ...string) []string {
	args := []string{"transform", "--source", src}
	return append(args, flags...)
}

func cliExitSuccess() int { return 0 }

func cliExitValidation() int { return pipeline.ExitValidation }

func cliExitDestExists() int { return pipeline.ExitDestExists }

func cliExitSourceNotFound() int { return pipeline.ExitSourceNotFound }

func cliExitBinarySource() int { return pipeline.ExitBinarySource }

func cliExitNotRegularFile() int { return pipeline.ExitNotRegularFile }
