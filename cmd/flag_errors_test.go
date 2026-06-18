package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/testutil"
)

// testWorkspaceLexicalDir is passed to DispatchCLI and RunMCPServer for tests whose code paths never
// open workspace files (unknown_command, invalid_flag, version/help); forbidigo forbids t.TempDir in unit tests.
const testWorkspaceLexicalDir = "glyph-shift-flag-errors-test-workspace"

// firstStdoutLineNormalizes trims a trailing carriage return before comparing, for CRLF-safe output reads.
func firstStdoutLine(stdout string) string {
	line, _, _ := strings.Cut(stdout, "\n")

	return strings.TrimSuffix(line, "\r")
}

func TestDispatchCLI_unknownCommand_JSON_stderr_exit6_emptyStdout(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := DispatchCLI([]string{"bogus"}, "1.0.0", testWorkspaceLexicalDir, &stdout, &stderr, errorContractRunner{})
	if code != exitValidation {
		t.Fatalf("exit code: got %d want %d (exitValidation)", code, exitValidation)
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout: want empty, got %q", stdout.String())
	}

	var got errorJSONOutput
	if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
		t.Fatalf("decode stderr JSON: %v\nstderr=%q", err, stderr.String())
	}

	if got.Error != "unknown_command" {
		t.Fatalf("error: got %q want unknown_command", got.Error)
	}

	if got.Command != "bogus" {
		t.Fatalf("command: got %q want bogus", got.Command)
	}

	wantHint := "Run glyph-shift --help to see available commands."
	if got.Hint != wantHint {
		t.Fatalf("hint: got %q want %q", got.Hint, wantHint)
	}
}

func assertInvalidFlagStderr(t *testing.T, stdout, stderr *bytes.Buffer) {
	t.Helper()

	if stdout.Len() != 0 {
		t.Fatalf("stdout: want empty, got %q", stdout.String())
	}

	var got errorJSONOutput
	if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
		t.Fatalf("decode stderr JSON: %v\nstderr=%q", err, stderr.String())
	}

	if got.Error != "invalid_flag" {
		t.Fatalf("error: got %q want invalid_flag", got.Error)
	}

	if got.Src != "" || got.Dest != "" || got.OutputPath != "" {
		t.Fatalf("path context: want empty, got src=%q dest=%q output_path=%q", got.Src, got.Dest, got.OutputPath)
	}

	wantHint := "flag provided but not defined: -bogus"
	if got.Hint != wantHint {
		t.Fatalf("hint: got %q want %q", got.Hint, wantHint)
	}
}

func TestDispatchCLI_invalidFlag_JSON_stderr_exit6_emptyStdout(t *testing.T) {
	t.Parallel()

	cases := []string{subcmdExtract, subcmdSplit, subcmdBlocks, subcmdTransform, "version"}
	for _, sub := range cases {
		t.Run(sub, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			args := []string{sub, "--bogus"}
			code := DispatchCLI(args, "1.0.0", testWorkspaceLexicalDir, &stdout, &stderr, errorContractRunner{})
			if code != exitValidation {
				t.Fatalf("exit code: got %d want %d", code, exitValidation)
			}

			assertInvalidFlagStderr(t, &stdout, &stderr)
		})
	}
}

func TestDispatchCLI_version_printsRelease_exit0_emptyStderr(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ver := "9.8.7-test"
	code := DispatchCLI([]string{"version"}, ver, testWorkspaceLexicalDir, &stdout, &stderr, errorContractRunner{})
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr: want empty, got %q", stderr.String())
	}

	want := "glyph-shift " + ver + "\n"
	if got := stdout.String(); got != want {
		t.Fatalf("stdout: got %q want %q", got, want)
	}
}

const wantVersionUsageFirstLine = `Usage: glyph-shift version`

func TestDispatchCLI_version_help_exit0_usageStdout(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := DispatchCLI(
		[]string{"version", "--help"}, "1.0.0", testWorkspaceLexicalDir,
		&stdout, &stderr, errorContractRunner{},
	)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr: want empty on help, got %q", stderr.String())
	}

	got := firstStdoutLine(stdout.String())
	if got != wantVersionUsageFirstLine {
		t.Fatalf("first stdout line: got %q want %q", got, wantVersionUsageFirstLine)
	}
}

const wantExtractUsageFirstLine = "Usage: glyph-shift extract " +
	"--source <path> --lines <range> --destination <path> [options]"

func TestDispatchCLI_subcommandHelp_exit0_usageStdout(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := DispatchCLI(
		[]string{subcmdExtract, "--help"}, "1.0.0", testWorkspaceLexicalDir,
		&stdout, &stderr, errorContractRunner{},
	)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr: want empty on help, got %q", stderr.String())
	}

	got := firstStdoutLine(stdout.String())
	if got != wantExtractUsageFirstLine {
		t.Fatalf("first stdout line: got %q want %q", got, wantExtractUsageFirstLine)
	}
}

const wantMCPUsageFirstLine = `Usage: glyph-shift mcp [options]`

func TestRunMCPServer_invalidFlag_JSON_stderr_exit6_emptyStdout(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := RunMCPServer(
		[]string{"--bogus"}, "1.0.0", testWorkspaceLexicalDir,
		bytes.NewReader(nil), &stdout, &stderr, MCPServerDeps{
			Runner:   errorContractRunner{},
			Resolver: testutil.NoSymlinkPathResolver{},
		},
	)
	if code != exitValidation {
		t.Fatalf("exit code: got %d want %d", code, exitValidation)
	}

	assertInvalidFlagStderr(t, &stdout, &stderr)
}

func TestRunMCPServer_help_exit0_usageStdout(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := RunMCPServer(
		[]string{"--help"}, "1.0.0", testWorkspaceLexicalDir,
		bytes.NewReader(nil), &stdout, &stderr, MCPServerDeps{
			Runner:   errorContractRunner{},
			Resolver: testutil.NoSymlinkPathResolver{},
		},
	)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr: want empty on help, got %q", stderr.String())
	}

	got := firstStdoutLine(stdout.String())
	if got != wantMCPUsageFirstLine {
		t.Fatalf("first stdout line: got %q want %q", got, wantMCPUsageFirstLine)
	}
}
