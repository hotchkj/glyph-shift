//go:build integration

package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/features/harness"
)

const splitDelimHeading = "^## "

var splitOperationCasesList = []cliCase{
	{
		name: "Preamble_and_sections_are_split_into_separate_files",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "multi-section.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "split/stdout/preamble-sections.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "multi-section-split")
		},
	},
	{
		name: "Empty_preamble_before_first_delimiter_is_skipped",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "split/stdout/heading-start.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "heading-start-split")
		},
	},
	{
		name: "Delimiter_lines_are_excluded_when_strip_is_requested",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out", "--strip-delimiter"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "split/stdout/heading-start-stripped.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "heading-start-split-stripped")
		},
	},
	{
		name: "Existing_output_files_are_not_overwritten_without_force",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
			cliSeedOutDirConflict(t, ws, []byte("placeholder\n"))
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out"),
		wantExit:     cliExitDestExists(),
		stderrGolden: "split/stderr/destination-exists.golden",
	},
	{
		name: "Existing_output_files_are_overwritten_with_force",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
			cliSeedOutDirConflict(t, ws, []byte("placeholder\n"))
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out", "--force"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "split/stdout/heading-start-force.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "heading-start-split")
		},
	},
	{
		name: "Line_endings_are_preserved_across_split",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start-crlf.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "split/stdout/heading-start-crlf.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirEveryFileTerminator(t, ws, "CRLF")
		},
	},
	{
		name: "Mixed_line_endings_are_preserved_across_split",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesEscapedInput(t, ws, "doc.md", "split-mixed-endings.bytes")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "split/stdout/mixed-endings.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "split-mixed-endings-split")
		},
	},
	{
		name: "UTF-8_BOM_is_preserved_across_split",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesEscapedInput(t, ws, "doc.md", "bom-split.bytes")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "split/stdout/bom-split.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "bom-split")
		},
	},
	{
		name: "Invalid_regex_pattern_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "doc.md", []byte("text\n"))
		},
		argv:         splitArgv("doc.md", "[invalid", "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/invalid-pattern.golden",
		missing:      []string{"out"},
	},
	{
		name: "Regex_pattern_length_is_bounded",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "doc.md", []byte("text\n"))
		},
		argv:         splitArgv("doc.md", harness.RegexPatternLongerThanMaximum(), "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/pattern-too-long.golden",
	},
	{
		name: "Regex_pattern_control_characters_are_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "doc.md", []byte("text\n"))
		},
		argv:         splitArgv("doc.md", harness.RegexPatternWithControlCharacter(), "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/control-chars-delimiter.golden",
	},
	{
		name: "Pattern_that_matches_nothing_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
		},
		argv:         splitArgv("doc.md", "^ZZZNOMATCH", "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/no-delimiter-match.golden",
		missing:      []string{"out"},
	},
	{
		name: "Output_directory_path_traversal_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "../../tmp/evil"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/invalid-input-out-dir.golden",
	},
	{
		name:         "Source_path_outside_workspace_is_rejected",
		argv:         splitArgv("../../etc/passwd", splitDelimHeading, "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/invalid-input-src-outside.golden",
	},
	{
		name:         "Source_file_not_found_is_rejected",
		argv:         splitArgv("nonexistent.md", splitDelimHeading, "out"),
		wantExit:     cliExitSourceNotFound(),
		stderrGolden: "split/stderr/source-not-found.golden",
	},
	{
		name: "Binary_source_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteBinarySource(t, ws, "data.bin", harness.BinarySourceCLIFixture())
		},
		argv:         splitArgv("data.bin", splitDelimHeading, "out"),
		wantExit:     cliExitBinarySource(),
		stderrGolden: "split/stderr/binary-source.golden",
	},
	{
		name: "Output_directory_is_created_on_demand",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out", "--mkdir"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "split/stdout/mkdir-out.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 2)
		},
	},
	{
		name: "Output_file_count_exceeding_default_limit_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "many-sections.md", harness.DelimitedSectionsContent(51))
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("many-sections.md", "^---$", "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/max-files-default-50.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 0)
		},
	},
	{
		name: "Explicit_max-files_limit_is_enforced",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out", "--max-files", "1"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/max-files-explicit-1.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 0)
		},
	},
	{
		name: "Invalid_max-files_value_is_rejected_consistently",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out", "--max-files", "0"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/invalid-max-files.golden",
	},
	{
		name: "Output_files_are_named_from_a_provided_list",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out", "--names", "first,second"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "split/stdout/named-first-second.golden",
		afterRun: func(t *testing.T, ws string) {
			if _, err := os.Stat(filepath.Join(ws, "out", "first.md")); err != nil {
				t.Fatalf("out/first.md: %v", err)
			}
			if _, err := os.Stat(filepath.Join(ws, "out", "second.md")); err != nil {
				t.Fatalf("out/second.md: %v", err)
			}
		},
	},
	{
		name: "Name_count_mismatch_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "multi-section.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out", "--names", "only-one"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/names-count-mismatch.golden",
	},
	{
		name: "Unsafe_explicit_output_names_are_rejected/dup_dup",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out", "--names", "dup,dup"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/unsafe-names-dup.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 0)
		},
	},
	{
		name: "Unsafe_explicit_output_names_are_rejected/parent_evil",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out", "--names", "../evil,ok"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/unsafe-names-parent.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 0)
		},
	},
	{
		name: "Unsafe_explicit_output_names_are_rejected/CON",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out", "--names", "CON,ok"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/unsafe-names-con.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 0)
		},
	},
	{
		name: "Output_extension_defaults_to_source_file_extension",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "split/stdout/extension-default-md.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertAllFilesHaveExtension(t, ws, "out", ".md")
		},
	},
	{
		name: "Preview_reports_split_metadata_without_creating_files",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "heading-start.md")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out", "--preview"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "split/stdout/preview-heading-start.golden",
		missing:      []string{"out"},
	},
}

func splitOperationCases() []cliCase {
	return splitOperationCasesList
}

var splitContractCasesList = []cliCase{
	{
		name: "Split_apply_stdout_JSON_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "multi-section.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "split/stdout/contract-apply.golden",
	},
	{
		name: "Split_preview_stdout_JSON_matches_contract",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "multi-section.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out", "--preview"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "split/stdout/contract-preview.golden",
	},
	{
		name: "Split_contract_stderr_pattern_validation/invalid_pattern",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "multi-section.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", "[invalid", "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/contract-invalid-pattern.golden",
	},
	{
		name: "Split_contract_stderr_pattern_validation/pattern_too_long",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "multi-section.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", harness.RegexPatternLongerThanMaximum(), "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/contract-pattern-too-long.golden",
	},
	{
		name: "Split_contract_stderr_pattern_validation/control_chars_in_input",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "multi-section.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", harness.RegexPatternWithControlCharacter(), "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/contract-control-chars.golden",
	},
	{
		name: "Split_contract_stderr_max_files_exceeded",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "doc.md", harness.DelimitedSectionsContent(11))
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", "^---$", "out", "--max-files", "10"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/contract-max-files-10.golden",
	},
	{
		name: "Split_contract_stderr_names_count_mismatch",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "multi-section.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out", "--names", "one,two"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/contract-names-mismatch.golden",
	},
	{
		name: "Split_contract_stderr_no_delimiter_match",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "multi-section.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", "^NOMATCH", "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/contract-no-delimiter-match.golden",
	},
	{
		name: "Split_contract_stderr_source_not_found",
		setup: func(t *testing.T, ws string) {
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out"),
		wantExit:     cliExitSourceNotFound(),
		stderrGolden: "split/stderr/contract-source-not-found.golden",
	},
	{
		name: "Split_contract_stderr_binary_source",
		setup: func(t *testing.T, ws string) {
			cliWriteBinarySource(t, ws, "doc.md", harness.BinarySourceCLIFixture())
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out"),
		wantExit:     cliExitBinarySource(),
		stderrGolden: "split/stderr/contract-binary-source.golden",
	},
	{
		name: "Split_contract_stderr_destination_exists",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "multi-section.md")
			cliSeedOutDirConflict(t, ws, []byte("exists\n"))
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out"),
		wantExit:     cliExitDestExists(),
		stderrGolden: "split/stderr/contract-destination-exists.golden",
	},
	{
		name: "CLI-only_stderr_JSON_shape_for_no_delimiter_match",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "multi-section.md")
			cliMkdir(t, ws, "out")
		},
		argv:         splitArgv("doc.md", "^NOMATCH", "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "split/stderr/cli-only-no-delimiter-match.golden",
	},
	{
		name: "Split_preview_fails_with_destination_exists_and_does_not_write_planned_output_files",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "multi-section.md")
			cliSeedOutDirConflict(t, ws, []byte("exists\n"))
		},
		argv:         splitArgv("doc.md", splitDelimHeading, "out", "--preview"),
		wantExit:     cliExitDestExists(),
		stderrGolden: "split/stderr/preview-destination-exists.golden",
		missing:      []string{"out/002.md", "out/003.md", "out/004.md"},
	},
}

func splitContractCases() []cliCase {
	return splitContractCasesList
}
