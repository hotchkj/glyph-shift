//go:build integration

package cli_test

import (
	"testing"

	"github.com/hotchkj/glyph-shift/features/harness"
)

var extractOperationCasesList = []cliCase{
	{
		name: "Extract_a_range_of_lines/45-55",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 100, "LF")
		},
		argv:         extractArgv("45-55", "plan.md", "output.txt"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "extract/stdout/extract-range-45-55.golden",
		lineRanges: []cliLineRangeExpect{{
			outRel: "output.txt", srcRel: "plan.md", startLine: 45, endLine: 55,
		}},
	},
	{
		name: "Extract_a_range_of_lines/95-",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 100, "LF")
		},
		argv:         extractArgv("95-", "plan.md", "output.txt"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "extract/stdout/extract-range-95-end.golden",
		lineRanges: []cliLineRangeExpect{{
			outRel: "output.txt", srcRel: "plan.md", startLine: 95, endLine: 100,
		}},
	},
	{
		name: "Extract_a_range_of_lines/-10",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 100, "LF")
		},
		argv:         extractArgv("-10", "plan.md", "output.txt"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "extract/stdout/extract-range-1-10.golden",
		lineRanges: []cliLineRangeExpect{{
			outRel: "output.txt", srcRel: "plan.md", startLine: 1, endLine: 10,
		}},
	},
	{
		name: "Line_numbers_are_1-based",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "three.txt", "three-lines.txt")
		},
		argv:         extractArgv("2-2", "three.txt", "output.txt"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "extract/stdout/extract-three-line-2.golden",
		lineRanges: []cliLineRangeExpect{{
			outRel: "output.txt", srcRel: "three.txt", startLine: 2, endLine: 2,
		}},
	},
	{
		name: "Empty_range_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 100, "LF")
		},
		argv:         extractArgv("50-49", "plan.md", "output.txt"),
		wantExit:     cliExitValidation(),
		stderrGolden: "extract/stderr/empty-range.golden",
		missing:      []string{"output.txt"},
	},
	{
		name: "Invalid_extract_line_syntax_is_rejected_as_invalid_input",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 100, "LF")
		},
		argv:         extractArgv("not-a-range", "plan.md", "output.txt"),
		wantExit:     cliExitValidation(),
		stderrGolden: "extract/stderr/invalid-input-lines.golden",
		missing:      []string{"output.txt"},
	},
	{
		name: "Existing_destination_is_not_overwritten_without_force",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 100, "LF")
			cliWriteFile(t, ws, "output.txt", []byte{})
		},
		argv:         extractArgv("1-10", "plan.md", "output.txt"),
		wantExit:     cliExitDestExists(),
		stderrGolden: "extract/stderr/destination-exists.golden",
		unchanged:    []string{"output.txt"},
	},
	{
		name: "Existing_destination_is_overwritten_with_force",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 100, "LF")
			cliWriteFile(t, ws, "output.txt", []byte("old"))
		},
		argv:         extractArgv("1-10", "plan.md", "output.txt", "--force"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "extract/stdout/extract-range-1-10.golden",
		lineRanges: []cliLineRangeExpect{{
			outRel: "output.txt", srcRel: "plan.md", startLine: 1, endLine: 10,
		}},
	},
	{
		name: "Content_is_appended_to_existing_destination",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 10, "LF")
			cliWriteFromFeaturesInput(t, ws, "output.txt", "existing-content.txt")
		},
		argv:         extractArgv("1-3", "plan.md", "output.txt", "--append"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "extract/stdout/extract-append-1-3.golden",
		appends: []cliAppendExpect{{
			outRel: "output.txt", prefixFeaturesInput: "existing-content.txt",
			suffixSrcRel: "plan.md", suffixStart: 1, suffixEnd: 3,
		}},
	},
	{
		name: "Destination_directories_are_created_on_demand",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 10, "LF")
		},
		argv:         extractArgv("1-5", "plan.md", "deep/nested/output.txt", "--mkdir"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "extract/stdout/extract-mkdir-1-5.golden",
		lineRanges: []cliLineRangeExpect{{
			outRel: "deep/nested/output.txt", srcRel: "plan.md", startLine: 1, endLine: 5,
		}},
	},
	{
		name: "Mixed_line_endings_are_preserved_across_extraction",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "mixed.txt", "mixed-endings.txt")
		},
		argv:         extractArgv("2-5", "mixed.txt", "output.txt"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "extract/stdout/extract-mixed-2-5.golden",
		outputs: []cliOutputExpect{{
			wsRel:            "output.txt",
			featuresExpected: "testdata/expected/mixed-endings-extract/output.txt",
		}},
	},
	{
		name: "UTF-8_BOM_is_preserved_across_extraction",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesEscapedInput(t, ws, "bom.txt", "bom-lines.bytes")
		},
		argv:         extractArgv("1-2", "bom.txt", "output.txt"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "extract/stdout/extract-bom-1-2.golden",
		outputs: []cliOutputExpect{{
			wsRel:            "output.txt",
			featuresExpected: "testdata/expected/bom-extract/output.txt.bytes",
		}},
	},
	{
		name: "Source_path_outside_workspace_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 10, "LF")
		},
		argv:         extractArgv("1-10", "../../etc/passwd", "output.txt"),
		wantExit:     cliExitValidation(),
		stderrGolden: "extract/stderr/invalid-input-src-outside.golden",
		missing:      []string{"output.txt"},
	},
	{
		name:         "Missing_source_is_rejected",
		argv:         extractArgv("1-10", "nonexistent.md", "output.txt"),
		wantExit:     cliExitSourceNotFound(),
		stderrGolden: "extract/stderr/source-not-found.golden",
		missing:      []string{"output.txt"},
	},
	{
		name: "Destination_path_outside_workspace_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 10, "LF")
		},
		argv:         extractArgv("1-10", "plan.md", "../../tmp/evil.txt"),
		wantExit:     cliExitValidation(),
		stderrGolden: "extract/stderr/invalid-input-dest-outside.golden",
	},
	{
		name: "Destination_that_is_a_directory_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 10, "LF")
			cliMkdir(t, ws, "outdir")
		},
		argv:         extractArgv("1-10", "plan.md", "outdir"),
		wantExit:     cliExitDestExists(),
		stderrGolden: "extract/stderr/destination-is-directory.golden",
	},
	{
		name: "Range_beyond_file_length_is_an_error",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "short.txt", 5, "LF")
		},
		argv:         extractArgv("3-999", "short.txt", "output.txt"),
		wantExit:     cliExitValidation(),
		stderrGolden: "extract/stderr/range-exceeds-file.golden",
		missing:      []string{"output.txt"},
	},
	{
		name: "Binary_source_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteBinarySource(t, ws, "data.bin", harness.BinarySourceCLIFixture())
		},
		argv:         extractArgv("1-5", "data.bin", "output.txt"),
		wantExit:     cliExitBinarySource(),
		stderrGolden: "extract/stderr/binary-source.golden",
		missing:      []string{"output.txt"},
	},
	{
		name: "Preview_reports_what_would_be_extracted_without_writing",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 100, "LF")
		},
		argv:         extractArgv("45-55", "plan.md", "output.txt", "--preview"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "extract/stdout/preview-45-55.golden",
		missing:      []string{"output.txt"},
	},
}

func extractOperationCases() []cliCase {
	return extractOperationCasesList
}

var extractContractCasesList = []cliCase{
	{
		name: "Extract_apply_stdout_JSON_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 10, "LF")
		},
		argv:         extractArgv("1-10", "plan.md", "output.txt"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "extract/stdout/contract-apply-1-10.golden",
	},
	{
		name: "Invalid_extract_line_range_reports_invalid_input_on_CLI",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 10, "LF")
		},
		argv:         extractArgv("not-a-range", "plan.md", "output.txt"),
		wantExit:     cliExitValidation(),
		stderrGolden: "extract/stderr/invalid-input-lines.golden",
	},
	{
		name: "Extract_contract_stderr_empty_range",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 10, "LF")
		},
		argv:         extractArgv("2-1", "plan.md", "output.txt"),
		wantExit:     cliExitValidation(),
		stderrGolden: "extract/stderr/contract-empty-range.golden",
	},
	{
		name: "Extract_contract_stderr_range_exceeds_file",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 10, "LF")
		},
		argv:         extractArgv("1-999", "plan.md", "output.txt"),
		wantExit:     cliExitValidation(),
		stderrGolden: "extract/stderr/range-exceeds-file-10-lines.golden",
	},
	{
		name:         "Extract_contract_stderr_source_not_found",
		argv:         extractArgv("1-2", "plan.md", "output.txt"),
		wantExit:     cliExitSourceNotFound(),
		stderrGolden: "extract/stderr/source-not-found-plan.golden",
	},
	{
		name: "Extract_contract_stderr_binary_source",
		setup: func(t *testing.T, ws string) {
			cliWriteBinarySource(t, ws, "plan.md", harness.BinarySourceCLIFixture())
		},
		argv:         extractArgv("1-2", "plan.md", "output.txt"),
		wantExit:     cliExitBinarySource(),
		stderrGolden: "extract/stderr/binary-source-plan.golden",
	},
	{
		name: "Extract_contract_stderr_destination_exists",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 10, "LF")
			cliWriteFile(t, ws, "output.txt", []byte("exists\n"))
		},
		argv:         extractArgv("1-2", "plan.md", "output.txt"),
		wantExit:     cliExitDestExists(),
		stderrGolden: "extract/stderr/destination-exists.golden",
	},
	{
		name: "Extract_preview_stdout_JSON_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 100, "LF")
		},
		argv:         extractArgv("45-55", "plan.md", "output.txt", "--preview"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "extract/stdout/preview-45-55.golden",
	},
	{
		name: "CLI-only_stderr_JSON_shape_for_range_exceeds_file",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 10, "LF")
		},
		argv:         extractArgv("1-999", "plan.md", "output.txt"),
		wantExit:     cliExitValidation(),
		stderrGolden: "extract/stderr/range-exceeds-file-10-lines.golden",
	},
	{
		name: "CLI_extract_rejects_trailing_positional_token",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "plan.md", 10, "LF")
		},
		argv: []string{
			"extract",
			"--source", "plan.md",
			"--lines", "1-3",
			"--destination", "output.txt",
			"extra-positional-arg",
		},
		wantExit:     cliExitValidation(),
		stderrGolden: "extract/stderr/unexpected-argument.golden",
	},
}

func extractContractCases() []cliCase {
	return extractContractCasesList
}
