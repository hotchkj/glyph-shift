//go:build integration

package cli_test

import (
	"testing"

	"github.com/hotchkj/glyph-shift/features/harness"
)

const (
	blocksGoStart      = "^```go"
	blocksGoEnd        = "^```$"
	blocksGherkinStart = "^```gherkin"
	blocksCodeStart    = "^```code"
	blocksOuterStart   = "^```outer"
	blocksFenceStart   = "^```"
)

var blocksOperationCasesList = []cliCase{
	{
		name: "Extract_delimited_blocks_from_document",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "nested-fences.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGherkinStart, blocksGoEnd, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/nested-fences.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "nested-fences-blocks")
		},
	},
	{
		name: "Block_content_excludes_delimiters_by_default",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "single-go-block.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/go-no-delimiters.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "go-block-no-delimiters")
		},
	},
	{
		name: "Delimiters_are_included_when_requested",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "single-go-block.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out", "--include-delimiters"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/go-with-delimiters.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "go-block-with-delimiters")
		},
	},
	{
		name: "Empty_block_produces_no_output_file",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "empty-go-block.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/empty-block.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 0)
		},
	},
	{
		name: "Block_content_is_extracted_byte-faithfully",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "two-code-blocks.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksCodeStart, blocksGoEnd, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/code-blocks.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "code-blocks-content")
		},
	},
	{
		name: "Mixed_line_endings_are_preserved_across_block_extraction",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesEscapedInput(t, ws, "doc.md", "blocks-mixed-endings.bytes")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/mixed-endings.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "blocks-mixed-endings")
		},
	},
	{
		name: "UTF-8_BOM_is_preserved_inside_block_content",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesEscapedInput(t, ws, "doc.md", "blocks-bom.bytes")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/bom-blocks.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "blocks-bom")
		},
	},
	{
		name: "First_end_match_closes_the_block_regardless_of_inner_delimiter-like_lines",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "inner-fence.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksOuterStart, blocksGoEnd, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/inner-fence.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "inner-fence-blocks")
		},
	},
	{
		name: "Output_directory_path_traversal_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "doc.md", []byte("text\n"))
		},
		argv:         blocksArgv("doc.md", blocksFenceStart, blocksGoEnd, "../../tmp/evil"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/invalid-input-out-dir.golden",
	},
	{
		name:         "Source_path_outside_workspace_is_rejected",
		argv:         blocksArgv("../../etc/passwd", blocksFenceStart, blocksGoEnd, "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/invalid-input-src-outside.golden",
	},
	{
		name: "Invalid_start_pattern_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "single-go-block.md")
		},
		argv:         blocksArgv("doc.md", "[invalid", blocksGoEnd, "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/invalid-pattern-start.golden",
	},
	{
		name: "Existing_output_files_are_not_overwritten_without_force",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "single-go-block.md")
			cliSeedOutDirConflict(t, ws, []byte("placeholder\n"))
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out"),
		wantExit:     cliExitDestExists(),
		stderrGolden: "blocks/stderr/destination-exists.golden",
	},
	{
		name: "Existing_output_files_are_overwritten_with_force",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "single-go-block.md")
			cliSeedOutDirConflict(t, ws, []byte("placeholder\n"))
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out", "--force"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/force-overwrite.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "go-block-no-delimiters")
		},
	},
	{
		name:         "Source_file_not_found_is_rejected",
		argv:         blocksArgv("nonexistent.md", blocksFenceStart, blocksGoEnd, "out"),
		wantExit:     cliExitSourceNotFound(),
		stderrGolden: "blocks/stderr/source-not-found.golden",
	},
	{
		name: "Binary_source_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteBinarySource(t, ws, "data.bin", harness.BinarySourceCLIFixture())
		},
		argv:         blocksArgv("data.bin", blocksFenceStart, blocksGoEnd, "out"),
		wantExit:     cliExitBinarySource(),
		stderrGolden: "blocks/stderr/binary-source.golden",
	},
	{
		name: "Output_directory_is_created_on_demand",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "single-go-block.md")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out", "--mkdir"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/mkdir-out.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 1)
		},
	},
	{
		name: "Unclosed_block_is_an_error",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "unclosed-gherkin-block.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGherkinStart, blocksGoEnd, "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/unclosed-block.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 0)
		},
	},
	{
		name: "No_matching_blocks_is_an_error",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "doc.md", []byte("plain\ntext\n"))
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", "^```go$", blocksGoEnd, "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/no-blocks-found.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 0)
		},
	},
	{
		name: "Multiple_empty_blocks_are_counted_but_produce_no_files",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "doc.md", harness.EmptyGoFencedBlocksContent(3))
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/empty-blocks-only.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 0)
		},
	},
	{
		name: "Block_count_exceeding_limit_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "many-blocks.md", harness.FencedBlocksContent(51))
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("many-blocks.md", blocksFenceStart, blocksGoEnd, "out"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/max-files-50.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 0)
		},
	},
	{
		name: "Empty_blocks_do_not_consume_the_max-files_limit",
		setup: func(t *testing.T, ws string) {
			cliWriteFile(t, ws, "doc.md", []byte("```go\n```\n```go\nbody\n```\n"))
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out", "--max-files", "1"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/max-files-empty-skip.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 1)
		},
	},
	{
		name: "Invalid_max-files_value_is_rejected_consistently",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "two-go-blocks.md")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out", "--max-files", "0"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/invalid-max-files.golden",
	},
	{
		name: "Output_files_are_named_from_a_provided_list",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "two-go-blocks.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out", "--names", "auth,db"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/named-auth-db.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirMatchesExpected(t, ws, "go-blocks-named")
		},
	},
	{
		name: "Name_count_mismatch_is_rejected",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "three-go-blocks.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out", "--names", "first,second"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/names-count-mismatch.golden",
	},
	{
		name: "Unsafe_explicit_block_output_names_are_rejected/dup_dup",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "two-go-blocks.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out", "--names", "dup,dup"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/unsafe-names-dup.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 0)
		},
	},
	{
		name: "Unsafe_explicit_block_output_names_are_rejected/parent_evil",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "two-go-blocks.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out", "--names", "../evil,ok"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/unsafe-names-parent.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 0)
		},
	},
	{
		name: "Unsafe_explicit_block_output_names_are_rejected/CON",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "two-go-blocks.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out", "--names", "CON,ok"),
		wantExit:     cliExitValidation(),
		stderrGolden: "blocks/stderr/unsafe-names-con.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertOutDirFileCount(t, ws, 0)
		},
	},
	{
		name: "Output_extension_defaults_to_source_file_extension",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "single-go-block.md")
			cliMkdir(t, ws, "out")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/extension-default-md.golden",
		afterRun: func(t *testing.T, ws string) {
			cliAssertAllFilesHaveExtension(t, ws, "out", ".md")
		},
	},
	{
		name: "Preview_reports_block_metadata_without_creating_files",
		setup: func(t *testing.T, ws string) {
			cliWriteFromFeaturesInput(t, ws, "doc.md", "two-go-blocks.md")
		},
		argv:         blocksArgv("doc.md", blocksGoStart, blocksGoEnd, "out", "--preview"),
		wantExit:     cliExitSuccess(),
		stdoutGolden: "blocks/stdout/preview-two-blocks.golden",
		missing:      []string{"out"},
	},
}

func blocksOperationCases() []cliCase {
	return blocksOperationCasesList
}
