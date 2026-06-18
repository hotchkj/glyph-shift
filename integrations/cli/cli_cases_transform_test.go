//go:build integration

package cli_test

import (
	"testing"

	"github.com/hotchkj/glyph-shift/features/harness"
)

var transformOperationCasesList = []cliCase{
	{
		name: "CRLF_endings_are_converted_to_LF",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "test.txt", 5, "CRLF")
		},
		argv:         transformArgv("test.txt", "--line-endings", "lf"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/crlf-to-lf.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertEveryLineTerminator(t, ws, "test.txt", "LF")
		},
	},
	{
		name: "LF_endings_are_converted_to_CRLF",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "test.txt", 5, "LF")
		},
		argv:         transformArgv("test.txt", "--line-endings", "crlf"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/lf-to-crlf.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertEveryLineTerminator(t, ws, "test.txt", "CRLF")
		},
	},
	{
		name: "Mixed_endings_are_normalized",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "mixed.txt", harness.MixedLineEndingsBytes)
		},
		argv:         transformArgv("mixed.txt", "--line-endings", "lf"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/mixed-to-lf.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertEveryLineTerminator(t, ws, "mixed.txt", "LF")
		},
	},
	{
		name: "Transform_on_already-normalized_file_makes_no_changes",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "test.txt", 5, "LF")
		},
		argv:         transformArgv("test.txt", "--line-endings", "lf"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/already-lf.golden",
		unchanged:    []string{"test.txt"},
	},
	{
		name: "Trailing_whitespace_is_trimmed",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "test.txt", harness.TrailingWhitespaceBytes)
		},
		argv:         transformArgv("test.txt", "--trim-trailing"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/trim-trailing.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertNoTrailingWhitespace(t, ws, "test.txt")
		},
	},
	{
		name: "Final_newline_is_ensured",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "test.txt", harness.SoloLineNoFinalNewline)
		},
		argv:         transformArgv("test.txt", "--final-newline"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/final-newline-added.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertTestTxtFinalNewline(t, ws, "with")
		},
	},
	{
		name: "Final_newline_is_not_duplicated",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "test.txt", "already-has-final-newline.txt")
		},
		argv:         transformArgv("test.txt", "--final-newline"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/final-newline-unchanged.golden",
		unchanged:    []string{"test.txt"},
		afterRun: func(t *testing.T, ws string) {
			cliAssertTestTxtFinalNewline(t, ws, "exactly-one")
		},
	},
	{
		name: "Multiple_transforms_are_applied_in_one_invocation",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "test.txt", harness.CRLFTrailingNoFinalNewline)
		},
		argv: transformArgv(
			"test.txt",
			"--line-endings", "lf",
			"--trim-trailing",
			"--final-newline",
		),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/combined-lf-trim-final.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertNoTrailingWhitespace(t, ws, "test.txt")
			cliAssertTestTxtFinalNewline(t, ws, "with")
		},
	},
	{
		name: "No_transform_specified_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "test.txt", "no-trailing-newline.txt")
		},
		argv:         transformArgv("test.txt"),
		wantExit:     cliExitValidation(),
		stderrGolden: "transform/stderr/no-transform-specified.golden",
	},
	{
		name: "Invalid_line_ending_target_is_rejected_consistently",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "test.txt", 5, "LF")
		},
		argv:         transformArgv("test.txt", "--line-endings", "bogus"),
		wantExit:     cliExitValidation(),
		stderrGolden: "transform/stderr/invalid-line-endings.golden",
	},
	{
		name: "Directory_path_is_rejected_as_source",
		setup: func(t *testing.T, ws string) {
			cliMkdir(t, ws, "src")
		},
		argv:         transformArgv("src", "--line-endings", "lf"),
		wantExit:     cliExitNotRegularFile(),
		stderrGolden: "transform/stderr/directory-not-file.golden",
	},
	{
		name: "Source_path_is_treated_literally_without_glob_expansion",
		setup: func(t *testing.T, ws string) {
			cliMkdir(t, ws, "src")
			cliWriteFile(t, ws, "src/a.txt", harness.ThreeLinesSharedContent)
			cliWriteFile(t, ws, "src/b.txt", harness.ThreeLinesSharedContent)
		},
		argv:         transformArgv("src/*.txt", "--line-endings", "lf"),
		wantExit:     cliExitSourceNotFound(),
		stderrGolden: "transform/stderr/source-not-found-glob.golden",
	},
	{
		name:         "Missing_source_is_rejected",
		argv:         transformArgv("nonexistent.txt", "--line-endings", "lf"),
		wantExit:     cliExitSourceNotFound(),
		stderrGolden: "transform/stderr/source-not-found.golden",
	},
	{
		name: "Binary_source_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteBinarySource(t, ws, "image.png", harness.ImageBinaryBytes)
		},
		argv:         transformArgv("image.png", "--line-endings", "lf"),
		wantExit:     cliExitBinarySource(),
		stderrGolden: "transform/stderr/binary-source.golden",
		unchanged:    []string{"image.png"},
	},
	{
		name: "Preview_reports_what_would_change_without_modifying",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "test.txt", 5, "CRLF")
		},
		argv:         transformArgv("test.txt", "--line-endings", "lf", "--preview"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/preview-crlf-to-lf.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertEveryLineTerminator(t, ws, "test.txt", "CRLF")
		},
	},
	{
		name: "Preview_on_already-normalized_file_reports_no_changes",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "test.txt", 5, "LF")
		},
		argv:         transformArgv("test.txt", "--line-endings", "lf", "--preview"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/preview-already-lf.golden",
		unchanged:    []string{"test.txt"},
	},
	{
		name: "Preview_reports_trailing_whitespace_without_modifying",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "test.txt", harness.TrailingWhitespaceBytes)
		},
		argv:         transformArgv("test.txt", "--trim-trailing", "--preview"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/preview-trim-trailing.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertStillHasTrailingWhitespace(t, ws, "test.txt")
		},
	},
	{
		name: "Preview_reports_missing_final_newline_without_modifying",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "test.txt", harness.SoloLineNoFinalNewline)
		},
		argv:         transformArgv("test.txt", "--final-newline", "--preview"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/preview-final-newline.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertTestTxtFinalNewline(t, ws, "without")
		},
	},
	{
		name: "CR_line_ending_conversion",
		setup: func(t *testing.T, ws string) {
			cliWriteNumberedLines(t, ws, "test.txt", 5, "LF")
		},
		argv:         transformArgv("test.txt", "--line-endings", "cr"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/lf-to-cr.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertEveryLineTerminator(t, ws, "test.txt", "CR")
		},
	},
	{
		name: "Result_distinguishes_line_ending_types",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "mixed.txt", harness.MixedEndingStatsContent(3, 2, 1))
		},
		argv:         transformArgv("mixed.txt", "--line-endings", "lf"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/mixed-ending-stats.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertEveryLineTerminator(t, ws, "mixed.txt", "LF")
		},
	},
}

func transformOperationCases() []cliCase {
	return transformOperationCasesList
}

var transformContractCasesList = []cliCase{
	{
		name: "Transform_apply_stdout_JSON_with_line-ending_stats_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "code.go", []byte("line1\nline2\n"))
		},
		argv:         transformArgv("code.go", "--line-endings", "crlf"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/contract-apply-line-endings.golden",
	},
	{
		name: "Transform_contract_stderr_invalid_transform_specification/no_transform_specified",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "code.go", []byte("line1\nline2\n"))
		},
		argv:         transformArgv("code.go"),
		wantExit:     cliExitValidation(),
		stderrGolden: "transform/stderr/contract-no-transform-specified.golden",
	},
	{
		name: "Transform_contract_stderr_invalid_transform_specification/invalid_line_endings",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "code.go", []byte("line1\nline2\n"))
		},
		argv:         transformArgv("code.go", "--line-endings", "bogus"),
		wantExit:     cliExitValidation(),
		stderrGolden: "transform/stderr/contract-invalid-line-endings.golden",
	},
	{
		name:         "Transform_contract_stderr_path-scoped_failures/source_not_found",
		setup:        func(t *testing.T, ws string) {},
		argv:         transformArgv("code.go", "--line-endings", "lf"),
		wantExit:     cliExitSourceNotFound(),
		stderrGolden: "transform/stderr/contract-source-not-found.golden",
	},
	{
		name: "Transform_contract_stderr_path-scoped_failures/binary_source",
		setup: func(t *testing.T, ws string) {
			cliWriteBinarySource(t, ws, "code.go", harness.BinarySourceCLIFixture())
		},
		argv:         transformArgv("code.go", "--line-endings", "lf"),
		wantExit:     cliExitBinarySource(),
		stderrGolden: "transform/stderr/contract-binary-source.golden",
	},
	{
		name: "Transform_contract_stderr_path-scoped_failures/directory_not_file",
		setup: func(t *testing.T, ws string) {
			cliMkdir(t, ws, "code.go")
		},
		argv:         transformArgv("code.go", "--line-endings", "lf"),
		wantExit:     cliExitNotRegularFile(),
		stderrGolden: "transform/stderr/contract-directory-not-file.golden",
	},
	{
		name: "Transform_preview_stdout_JSON_with_line-ending_stats_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "code.go", []byte("line1\nline2\n"))
		},
		argv:         transformArgv("code.go", "--line-endings", "crlf", "--preview"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/contract-preview-line-endings.golden",
	},
	{
		name: "Transform_apply_stdout_JSON_with_trim-trailing_stats_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "code.go", []byte("line1  \nline2\n"))
		},
		argv:         transformArgv("code.go", "--trim-trailing"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/contract-apply-trim-trailing.golden",
	},
	{
		name: "Transform_apply_stdout_JSON_with_final-newline_stats_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "code.go", []byte("line1\nline2"))
		},
		argv:         transformArgv("code.go", "--final-newline"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/contract-apply-final-newline-added.golden",
	},
	{
		name: "Transform_apply_stdout_JSON_when_no_change_is_needed_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "code.go", []byte("line1\nline2\n"))
		},
		argv:         transformArgv("code.go", "--final-newline"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/contract-apply-final-newline-unchanged.golden",
	},
	{
		name: "CLI-only_stderr_JSON_shape_for_no_transform_specified",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "code.go", []byte("line1\nline2\n"))
		},
		argv:         transformArgv("code.go"),
		wantExit:     cliExitValidation(),
		stderrGolden: "transform/stderr/cli-only-no-transform-specified.golden",
	},
	{
		name: "Transform_preview_stdout_JSON_with_trim-trailing_stats_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "code.go", []byte("line1  \nline2\n"))
		},
		argv:         transformArgv("code.go", "--trim-trailing", "--preview"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/contract-preview-trim-trailing.golden",
	},
	{
		name: "Transform_preview_stdout_JSON_with_final-newline_needed_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "code.go", []byte("line1\nline2"))
		},
		argv:         transformArgv("code.go", "--final-newline", "--preview"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/contract-preview-final-newline-needed.golden",
	},
	{
		name: "Transform_preview_stdout_JSON_when_no_final-newline_change_needed_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "code.go", []byte("line1\nline2\n"))
		},
		argv:         transformArgv("code.go", "--final-newline", "--preview"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "transform/stdout/contract-preview-final-newline-not-needed.golden",
	},
}

func transformContractCases() []cliCase {
	return transformContractCasesList
}
