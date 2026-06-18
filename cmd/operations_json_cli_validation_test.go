package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

// Two non-empty written blocks (dual inner): matches pipeline run_blocks_test fixture.
const phase5BlocksTwoNonEmptyBlocksSource = "header\n<<BEGIN>>\ninner1\n<<END>>\n<<BEGIN>>\ninner2\n<<END>>\n"

// phase5SplitTwoSectionSource has one delimiter line so the scan yields two output sections.
const phase5SplitTwoSectionSource = "intro\n---\nrest\n"

type phase5PrepFn func(t *testing.T, root string, src *testutil.MemSourceOpener, out *testutil.MemOutputOpener)

type phase5CLIMatrixCase struct {
	name      string
	cmd       string
	args      []string
	wantError string
	prep      phase5PrepFn
}

func phase5CLIMatrixCases() []phase5CLIMatrixCase {
	splitCases := phase5SplitCLIMatrixCases()
	blocksCases := phase5BlocksCLIMatrixCases()
	out := make([]phase5CLIMatrixCase, 0, len(splitCases)+len(blocksCases))

	out = append(out, splitCases...)
	out = append(out, blocksCases...)

	return out
}

func phase5SplitCLIMatrixCases() []phase5CLIMatrixCase {
	return []phase5CLIMatrixCase{
		{
			name:      "split_delimiter_invalid_regex",
			cmd:       subcmdSplit,
			args:      []string{"--source", "doc.md", "--delimiter", "[invalid", "--output-dir", "out"},
			wantError: "invalid_pattern",
		},
		{
			name:      "split_names_parse_empty_segment",
			cmd:       subcmdSplit,
			args:      []string{"--source", "doc.md", "--delimiter", "^---$", "--output-dir", "out", "--names", "a,,b"},
			wantError: "invalid_input",
		},
		{
			name:      "split_names_count_mismatch_two_sections",
			cmd:       subcmdSplit,
			args:      []string{"--source", "doc.md", "--delimiter", "^---$", "--output-dir", "out", "--names", "one"},
			wantError: "names_count_mismatch",
			prep: func(t *testing.T, root string, src *testutil.MemSourceOpener, out *testutil.MemOutputOpener) {
				t.Helper()
				srcPath := filepath.Join(root, "doc.md")
				if err := afero.WriteFile(src.Fs, srcPath, []byte(phase5SplitTwoSectionSource), 0o600); err != nil {
					t.Fatalf("write source: %v", err)
				}
				if err := out.Fs.MkdirAll(filepath.Join(root, "out"), 0o700); err != nil {
					t.Fatalf("mkdir out: %v", err)
				}
			},
		},
		{
			name:      "split_max_files_zero",
			cmd:       subcmdSplit,
			args:      []string{"--source", "doc.md", "--delimiter", "^---$", "--output-dir", "out", "--max-files", "0"},
			wantError: "invalid_input",
		},
		{
			name:      "split_missing_delimiter_flag",
			cmd:       subcmdSplit,
			args:      []string{"--source", "doc.md", "--output-dir", "out"},
			wantError: "missing_required_flag",
		},
		{
			name:      "split_empty_delimiter_flag_value",
			cmd:       subcmdSplit,
			args:      []string{"--source", "doc.md", "--delimiter", "", "--output-dir", "out"},
			wantError: "invalid_pattern",
		},
	}
}

func phase5BlocksCLIMatrixCases() []phase5CLIMatrixCase {
	return []phase5CLIMatrixCase{
		{
			name:      "blocks_start_invalid_regex",
			cmd:       subcmdBlocks,
			args:      []string{"--source", "doc.md", "--start-line", "[invalid", "--end-line", "^```$", "--output-dir", "out"},
			wantError: "invalid_pattern",
		},
		{
			name: "blocks_end_invalid_regex",
			cmd:  subcmdBlocks,
			args: []string{
				"--source", "doc.md", "--start-line", "^```go$",
				"--end-line", "[invalid", "--output-dir", "out",
			},
			wantError: "invalid_pattern",
		},
		{
			name: "blocks_names_parse_empty_segment",
			cmd:  subcmdBlocks,
			args: []string{
				"--source", "doc.md", "--start-line", "^<<BEGIN>>$", "--end-line", "^<<END>>$",
				"--output-dir", "out", "--names", "a,,b",
			},
			wantError: "invalid_input",
		},
		{
			name: "blocks_names_count_mismatch_two_blocks",
			cmd:  subcmdBlocks,
			args: []string{
				"--source", "doc.md",
				"--start-line", "^<<BEGIN>>$", "--end-line", "^<<END>>$",
				"--output-dir", "out", "--names", "one",
			},
			wantError: "names_count_mismatch",
			prep: func(t *testing.T, root string, src *testutil.MemSourceOpener, out *testutil.MemOutputOpener) {
				t.Helper()
				srcPath := filepath.Join(root, "doc.md")
				if err := afero.WriteFile(src.Fs, srcPath, []byte(phase5BlocksTwoNonEmptyBlocksSource), 0o600); err != nil {
					t.Fatalf("write source: %v", err)
				}
				if err := out.Fs.MkdirAll(filepath.Join(root, "out"), 0o700); err != nil {
					t.Fatalf("mkdir out: %v", err)
				}
			},
		},
		{
			name: "blocks_max_files_zero",
			cmd:  subcmdBlocks,
			args: []string{
				"--source", "doc.md", "--start-line", "^<<BEGIN>>$", "--end-line", "^<<END>>$",
				"--output-dir", "out", "--max-files", "0",
			},
			wantError: "invalid_input",
		},
		{
			name:      "blocks_missing_start_flag",
			cmd:       subcmdBlocks,
			args:      []string{"--source", "doc.md", "--end-line", "^<<END>>$", "--output-dir", "out"},
			wantError: "missing_required_flag",
		},
		{
			name:      "blocks_missing_end_flag",
			cmd:       subcmdBlocks,
			args:      []string{"--source", "doc.md", "--start-line", "^<<BEGIN>>$", "--output-dir", "out"},
			wantError: "missing_required_flag",
		},
		{
			name:      "blocks_empty_start_flag_value",
			cmd:       subcmdBlocks,
			args:      []string{"--source", "doc.md", "--start-line", "", "--end-line", "^<<END>>$", "--output-dir", "out"},
			wantError: "invalid_pattern",
		},
		{
			name:      "blocks_empty_end_flag_value",
			cmd:       subcmdBlocks,
			args:      []string{"--source", "doc.md", "--start-line", "^<<BEGIN>>$", "--end-line", "", "--output-dir", "out"},
			wantError: "invalid_pattern",
		},
	}
}

func TestPhase5PassC_CLI_RoutingMatrixSplitBlocks(t *testing.T) {
	t.Parallel()

	for _, tc := range phase5CLIMatrixCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := testCmdWorkspaceRoot()
			srcMem := testutil.NewMemSourceOpener()
			outMem := testutil.NewMemOutputOpener()
			if tc.prep != nil {
				tc.prep(t, root, srcMem, outMem)
			}

			var stdout, stderr bytes.Buffer

			var code int

			switch tc.cmd {
			case subcmdSplit:
				code = runSplit(tc.args, root, &stdout, &stderr, newTestOperationRunner(t, srcMem, outMem))
			case subcmdBlocks:
				code = runBlocks(tc.args, root, &stdout, &stderr, newTestOperationRunner(t, srcMem, outMem))
			default:
				t.Fatalf("unknown cmd %q", tc.cmd)
			}

			if code != exitValidation {
				t.Fatalf("exit code: got %d want exitValidation (%d)", code, exitValidation)
			}

			if stdout.String() != "" {
				t.Fatalf("stdout: want empty, got %q", stdout.String())
			}

			assertExactlyOneConsumerCommandErrorJSON(t, &stderr, tc.wantError)
		})
	}
}

func assertExactlyOneConsumerCommandErrorJSON(t *testing.T, stderr *bytes.Buffer, wantError string) {
	t.Helper()

	text := stderr.String()
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")

	var consumer []errorJSONOutput

	for _, line := range lines {
		if payload, ok := tryParseConsumerCommandErrorLine(line); ok {
			consumer = append(consumer, payload)
		}
	}

	if len(consumer) != 1 {
		t.Fatalf("want exactly 1 consumer command-error JSON object (non-empty error field), got %d: stderr=%q",
			len(consumer), text)
	}

	if consumer[0].Error != wantError {
		t.Fatalf("error: got %q want %s", consumer[0].Error, wantError)
	}
}

func tryParseConsumerCommandErrorLine(line string) (errorJSONOutput, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return errorJSONOutput{}, false
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return errorJSONOutput{}, false
	}

	if isSlogStructuredLine(raw) {
		return errorJSONOutput{}, false
	}

	_, errOK := raw["error"].(string)
	if !errOK {
		return errorJSONOutput{}, false
	}

	var payload errorJSONOutput
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return errorJSONOutput{}, false
	}

	if payload.Error == "" {
		return errorJSONOutput{}, false
	}

	return payload, true
}

func isSlogStructuredLine(raw map[string]any) bool {
	if raw == nil {
		return false
	}

	_, hasTime := raw["time"]
	_, hasLevel := raw["level"]
	_, hasMsg := raw["msg"]

	return hasTime && hasLevel && hasMsg
}

// assertOneStderrJSONError is an alias kept for tests that assert a single command error without mixed stderr.
func assertOneStderrJSONError(t *testing.T, stderr *bytes.Buffer, wantError string) {
	t.Helper()
	assertExactlyOneConsumerCommandErrorJSON(t, stderr, wantError)
}

func TestDispatchCLI_explicitEmptyRequiredValuesAreClassified(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		command   string
		args      []string
		wantError string
	}{
		{
			name:      "extract_src",
			command:   subcmdExtract,
			args:      []string{"--source", "", "--lines", "1-1", "--destination", "out.txt"},
			wantError: "invalid_input",
		},
		{
			name:      "extract_lines",
			command:   subcmdExtract,
			args:      []string{"--source", "doc.md", "--lines", "", "--destination", "out.txt"},
			wantError: "invalid_input",
		},
		{
			name:      "extract_dest",
			command:   subcmdExtract,
			args:      []string{"--source", "doc.md", "--lines", "1-1", "--destination", ""},
			wantError: "invalid_input",
		},
		{
			name:      "split_delimiter",
			command:   "split",
			args:      []string{"--source", "doc.md", "--delimiter", "", "--output-dir", "out"},
			wantError: "invalid_pattern",
		},
		{
			name:      "split_out_dir",
			command:   "split",
			args:      []string{"--source", "doc.md", "--delimiter", "^---$", "--output-dir", ""},
			wantError: "invalid_input",
		},
		{
			name:      "blocks_start",
			command:   "blocks",
			args:      []string{"--source", "doc.md", "--start-line", "", "--end-line", "^```$", "--output-dir", "out"},
			wantError: "invalid_pattern",
		},
		{
			name:      "blocks_end",
			command:   "blocks",
			args:      []string{"--source", "doc.md", "--start-line", "^```go$", "--end-line", "", "--output-dir", "out"},
			wantError: "invalid_pattern",
		},
		{
			name:      "blocks_out_dir",
			command:   "blocks",
			args:      []string{"--source", "doc.md", "--start-line", "^```go$", "--end-line", "^```$", "--output-dir", ""},
			wantError: "invalid_input",
		},
		{
			name:      "transform_src",
			command:   subcmdTransform,
			args:      []string{"--source", "", "--line-endings", "lf"},
			wantError: "invalid_input",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			exitCode, stdoutText, stderrBuf := runOperationWithRawBuffers(t, tc.command, tc.args, testCmdWorkspaceRoot())
			if exitCode != exitValidation {
				t.Fatalf("exit code: got %d want %d", exitCode, exitValidation)
			}
			if stdoutText != "" {
				t.Fatalf("stdout: want empty, got %q", stdoutText)
			}
			assertOneStderrJSONError(t, stderrBuf, tc.wantError)
		})
	}
}

func runOperationWithRawBuffers(t *testing.T, command string, args []string, root string) (
	exitCode int,
	stdoutText string,
	stderrBuf *bytes.Buffer,
) {
	t.Helper()

	srcMem := testutil.NewMemSourceOpener()
	outMem := testutil.NewMemOutputOpener()
	var stdout, stderr bytes.Buffer

	switch command {
	case subcmdExtract:
		exitCode = runExtract(args, root, &stdout, &stderr, newTestOperationRunner(t, srcMem, outMem))
	case subcmdSplit:
		exitCode = runSplit(args, root, &stdout, &stderr, newTestOperationRunner(t, srcMem, outMem))
	case subcmdBlocks:
		exitCode = runBlocks(args, root, &stdout, &stderr, newTestOperationRunner(t, srcMem, outMem))
	case subcmdTransform:
		exitCode = runTransform(args, root, &stdout, &stderr, newTestOperationRunner(t, srcMem, outMem))
	default:
		t.Fatalf("unsupported command %q", command)
	}

	return exitCode, stdout.String(), &stderr
}
