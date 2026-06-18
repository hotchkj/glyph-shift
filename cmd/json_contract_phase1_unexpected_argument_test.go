package cmd

import (
	"bytes"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/testutil"
)

func assertUnexpectedArgumentContract(t *testing.T, got *errorJSONOutput, command string) {
	t.Helper()

	if got.Error != "unexpected_argument" {
		t.Fatalf("error: got %q want unexpected_argument", got.Error)
	}

	if got.Argument == "" {
		t.Fatalf("argument: want non-empty unexpected token or context, got empty")
	}

	wantHint := trailingArgumentHint(command)
	if wantHint == "" {
		t.Fatalf("unsupported command tag %q", command)
	}
	if got.Hint != wantHint {
		t.Fatalf("hint: got %q want %q", got.Hint, wantHint)
	}
}

func phase1CaseExtractTrailingPositional() phase1UnexpectedArgumentCase {
	return phase1UnexpectedArgumentCase{
		name:       "extract_trailing_positional",
		commandTag: subcmdExtract,
		dispatch: func(t *testing.T) (int, *bytes.Buffer, *bytes.Buffer) {
			t.Helper()
			var stdout, stderr bytes.Buffer
			code := DispatchCLI(
				[]string{
					subcmdExtract,
					"--source", "doc.md", "--lines", "1-1", "--destination", "out.txt",
					"surplus-token",
				},
				"1.0.0",
				testWorkspaceLexicalDir,
				&stdout,
				&stderr,
				errorContractRunner{},
			)

			return code, &stdout, &stderr
		},
	}
}

func phase1CaseSplitTrailingPositional() phase1UnexpectedArgumentCase {
	return phase1UnexpectedArgumentCase{
		name:       "split_trailing_positional",
		commandTag: subcmdSplit,
		dispatch: func(t *testing.T) (int, *bytes.Buffer, *bytes.Buffer) {
			t.Helper()
			var stdout, stderr bytes.Buffer
			code := DispatchCLI(
				[]string{
					"split",
					"--source", "doc.md", "--delimiter", phase1SplitDelimiterRegexp, "--output-dir", "out",
					"extra-arg",
				},
				"1.0.0",
				testWorkspaceLexicalDir,
				&stdout,
				&stderr,
				errorContractRunner{},
			)

			return code, &stdout, &stderr
		},
	}
}

func phase1CaseBlocksTrailingPositional() phase1UnexpectedArgumentCase {
	return phase1UnexpectedArgumentCase{
		name:       "blocks_trailing_positional",
		commandTag: subcmdBlocks,
		dispatch: func(t *testing.T) (int, *bytes.Buffer, *bytes.Buffer) {
			t.Helper()
			var stdout, stderr bytes.Buffer
			code := DispatchCLI(
				[]string{
					"blocks",
					"--source", "doc.md",
					"--start-line", "^```go$",
					"--end-line", "^```$",
					"--output-dir", "out",
					"extra-arg",
				},
				"1.0.0",
				testWorkspaceLexicalDir,
				&stdout,
				&stderr,
				errorContractRunner{},
			)

			return code, &stdout, &stderr
		},
	}
}

func phase1CaseTransformTrailingPositional() phase1UnexpectedArgumentCase {
	return phase1UnexpectedArgumentCase{
		name:       "transform_trailing_positional",
		commandTag: subcmdTransform,
		dispatch: func(t *testing.T) (int, *bytes.Buffer, *bytes.Buffer) {
			t.Helper()
			var stdout, stderr bytes.Buffer
			code := DispatchCLI(
				[]string{
					subcmdTransform,
					"--source", "doc.md",
					"--line-endings", "lf",
					"extra-arg",
				},
				"1.0.0",
				testWorkspaceLexicalDir,
				&stdout,
				&stderr,
				errorContractRunner{},
			)

			return code, &stdout, &stderr
		},
	}
}

func phase1CaseVersionTrailingPositional() phase1UnexpectedArgumentCase {
	return phase1UnexpectedArgumentCase{
		name:       "version_trailing_positional",
		commandTag: "version",
		dispatch: func(t *testing.T) (int, *bytes.Buffer, *bytes.Buffer) {
			t.Helper()
			var stdout, stderr bytes.Buffer
			code := DispatchCLI(
				[]string{"version", "surplus"},
				"1.0.0",
				testWorkspaceLexicalDir,
				&stdout,
				&stderr,
				errorContractRunner{},
			)

			return code, &stdout, &stderr
		},
	}
}

func phase1CaseMCPTrailingPositionalAfterFlags() phase1UnexpectedArgumentCase {
	return phase1UnexpectedArgumentCase{
		name:       "mcp_trailing_positional_after_flags",
		commandTag: "mcp",
		dispatch: func(t *testing.T) (int, *bytes.Buffer, *bytes.Buffer) {
			t.Helper()
			var stdout, stderr bytes.Buffer
			code := RunMCPServer(
				[]string{"--workspace-root", testWorkspaceLexicalDir, "surplus-arg"},
				"1.0.0",
				testWorkspaceLexicalDir,
				bytes.NewReader(nil),
				&stdout,
				&stderr,
				MCPServerDeps{
					Runner:   errorContractRunner{},
					Resolver: testutil.NoSymlinkPathResolver{},
				},
			)

			return code, &stdout, &stderr
		},
	}
}

func phase1UnexpectedArgumentTableCases() []phase1UnexpectedArgumentCase {
	return []phase1UnexpectedArgumentCase{
		phase1CaseExtractTrailingPositional(),
		phase1CaseSplitTrailingPositional(),
		phase1CaseBlocksTrailingPositional(),
		phase1CaseTransformTrailingPositional(),
		phase1CaseVersionTrailingPositional(),
		phase1CaseMCPTrailingPositionalAfterFlags(),
	}
}

type phase1UnexpectedArgumentCase struct {
	name       string
	commandTag string
	dispatch   func(t *testing.T) (code int, stdout, stderr *bytes.Buffer)
}

func TestPhase1_unexpectedArgument_exit6_stderrContract_table(t *testing.T) {
	t.Parallel()

	for _, tc := range phase1UnexpectedArgumentTableCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			code, stdout, stderr := tc.dispatch(t)
			if code != exitValidation {
				t.Fatalf("exit code: got %d want exitValidation (%d)", code, exitValidation)
			}

			if stdout.Len() != 0 {
				t.Fatalf("stdout: want empty, got %q", stdout.String())
			}

			got := decodeSingleStderrErrorJSON(t, stderr)
			assertUnexpectedArgumentContract(t, &got, tc.commandTag)
		})
	}
}
