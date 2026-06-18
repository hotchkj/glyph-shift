//go:build integration

package cli_test

import (
	"testing"

	"github.com/hotchkj/glyph-shift/features/harness"
)

var blocksContractCasesList = []cliCase{
	{
		name: "Blocks_apply_stdout_JSON_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "two-go-blocks.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/contract-apply.golden",
	},
	{
		name: "Blocks_apply_with_empty_blocks_present_stdout_JSON_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "doc.md", []byte("```go\n```\n```go\na\n```\n```go\nb\n```\n"))
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/contract-apply-empty-blocks.golden",
	},
	{
		name: "Blocks_preview_stdout_JSON_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "two-go-blocks.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out", "--preview"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/contract-preview.golden",
	},
	{
		name: "Blocks_contract_stderr_pattern_validation/invalid_pattern_start_line",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "single-go-block.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", "[invalid", blocksGoEnd, "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/contract-invalid-pattern-start.golden",
	},
	{
		name: "Blocks_contract_stderr_pattern_validation/invalid_pattern_end_line",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "single-go-block.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, "[invalid", "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/contract-invalid-pattern-end.golden",
	},
	{
		name: "Blocks_contract_stderr_pattern_validation/pattern_too_long_start_line",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "single-go-block.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", harness.RegexPatternLongerThanMaximum(), blocksGoEnd, "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/contract-pattern-too-long-start.golden",
	},
	{
		name: "Blocks_contract_stderr_pattern_validation/pattern_too_long_end_line",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "single-go-block.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, harness.RegexPatternLongerThanMaximum(), "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/contract-pattern-too-long-end.golden",
	},
	{
		name: "Blocks_contract_stderr_pattern_validation/control_chars_in_input_start_line",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "single-go-block.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", harness.RegexPatternWithControlCharacter(), blocksGoEnd, "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/contract-control-chars-start.golden",
	},
	{
		name: "Blocks_contract_stderr_pattern_validation/control_chars_in_input_end_line",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "single-go-block.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, harness.RegexPatternWithControlCharacter(), "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/contract-control-chars-end.golden",
	},
	{
		name: "Blocks_contract_stderr_max_files_exceeded",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "doc.md", harness.FencedBlocksContent(51))
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", "^```", blocksGoEnd, "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/contract-max-files.golden",
	},
	{
		name: "Blocks_contract_stderr_names_count_mismatch",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "three-go-blocks.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out", "--names", "first,second"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/contract-names-mismatch.golden",
	},
	{
		name: "Blocks_contract_stderr_unclosed_block",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "unclosed-gherkin-block.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", "^```gherkin", blocksGoEnd, "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/contract-unclosed-block.golden",
	},
	{
		name: "Blocks_contract_stderr_no_blocks_found",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "doc.md", []byte("plain\n"))
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/contract-no-blocks-found.golden",
	},
	{
		name: "Blocks_contract_stderr_source_not_found",
		setup: func(t *testing.T, ws string) {
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out"),
		wantExit:     cliExitSourceNotFound(),
		stderrGolden: "blocks/stderr/contract-source-not-found.golden",
	},
	{
		name: "Blocks_contract_stderr_binary_source",
		setup: func(t *testing.T, ws string) {
			cliWriteBinarySource(t, ws, "doc.md", harness.BinarySourceCLIFixture())
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out"),
		wantExit:     cliExitBinarySource(),
		stderrGolden: "blocks/stderr/contract-binary-source.golden",
	},
	{
		name: "Blocks_contract_stderr_destination_exists",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "two-go-blocks.md")
			cliSeedOutDirConflict(t, ws, []byte("exists\n"))
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out"),
		wantExit:     cliExitDestExists(),
		stderrGolden: "blocks/stderr/contract-destination-exists.golden",
	},
	{
		name: "CLI-only_stderr_JSON_shape_for_no_blocks_found",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "doc.md", []byte("plain\n"))
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/cli-only-no-blocks-found.golden",
	},
	{
		name: "Blocks_preview_fails_with_destination_exists_and_does_not_write_planned_output_files",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "two-go-blocks.md")
			cliSeedOutDirConflict(t, ws, []byte("exists\n"))
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out", "--preview"),
		wantExit:     cliExitDestExists(),
		stderrGolden: "blocks/stderr/preview-destination-exists.golden",
		missing:      []string{"out/002.md"},
	},
}

func blocksContractCases() []cliCase {
	return blocksContractCasesList
}
