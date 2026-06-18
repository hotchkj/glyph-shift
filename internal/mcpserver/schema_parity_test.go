package mcpserver

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

// mcpToCLIFlag maps MCP JSON field names to their expected CLI flag counterparts.
// MCP field names use underscores; CLI flags use hyphens.
var mcpToCLIFlag = map[string]string{
	"source":             "source",
	"lines":              "lines",
	"destination":        "destination",
	"force":              "force",
	"append":             "append",
	"mkdir":              "mkdir",
	"preview":            "preview",
	"delimiter":          "delimiter",
	"output_dir":         "output-dir",
	"extension":          "extension",
	"strip_delimiter":    "strip-delimiter",
	"names":              "names",
	"max_files":          "max-files",
	"start_line":         "start-line",
	"end_line":           "end-line",
	"include_delimiters": "include-delimiters",
	"line_endings":       "line-endings",
	"trim_trailing":      "trim-trailing",
	"final_newline":      "final-newline",
}

func jsonFieldNames(t *testing.T, v interface{}) []string {
	t.Helper()

	rt := reflect.TypeOf(v)
	var names []string

	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}

		name := strings.SplitN(tag, ",", 2)[0]
		names = append(names, name)
	}

	return names
}

func cliParamNamesByTool(t *testing.T) map[string]map[string]struct{} {
	t.Helper()

	out := make(map[string]map[string]struct{})
	for _, schema := range fileops.BuiltinOperationSchemas() {
		names := make(map[string]struct{}, len(schema.Parameters))
		for _, p := range schema.Parameters {
			names[p.Name] = struct{}{}
		}

		out[schema.Name] = names
	}

	return out
}

func TestSchemaParity_AllMCPFieldsHaveCLIMappings(t *testing.T) {
	t.Parallel()

	inputs := map[string]interface{}{
		toolExtract:   ExtractInput{},
		toolSplit:     SplitInput{},
		toolBlocks:    BlocksInput{},
		toolTransform: TransformInput{},
	}

	cliByTool := cliParamNamesByTool(t)

	for tool, input := range inputs {
		cliParams := cliByTool[tool]
		fields := jsonFieldNames(t, input)
		for _, jsonField := range fields {
			wantCLI, ok := mcpToCLIFlag[jsonField]
			if !ok {
				t.Errorf("%s: MCP field %q has no CLI flag mapping in mcpToCLIFlag", tool, jsonField)
				continue
			}

			if _, inSchema := cliParams[wantCLI]; !inSchema {
				t.Errorf("%s: MCP field %q maps to CLI %q, not listed in fileops schema parameters",
					tool, jsonField, wantCLI)
			}
		}
	}
}

func TestSchemaParity_NoDuplicateFields(t *testing.T) {
	t.Parallel()

	inputs := map[string]interface{}{
		toolExtract:   ExtractInput{},
		toolSplit:     SplitInput{},
		toolBlocks:    BlocksInput{},
		toolTransform: TransformInput{},
	}

	for tool, input := range inputs {
		fields := jsonFieldNames(t, input)
		seen := make(map[string]bool, len(fields))

		for _, f := range fields {
			if seen[f] {
				t.Errorf("%s: duplicate JSON field name %q", tool, f)
			}

			seen[f] = true
		}
	}
}

func TestSchemaParity_FieldCountsMatchExpected(t *testing.T) {
	t.Parallel()

	expected := map[string]int{
		toolExtract:   7,
		toolSplit:     10,
		toolBlocks:    11,
		toolTransform: 5,
	}

	inputs := map[string]interface{}{
		toolExtract:   ExtractInput{},
		toolSplit:     SplitInput{},
		toolBlocks:    BlocksInput{},
		toolTransform: TransformInput{},
	}

	for tool, input := range inputs {
		fields := jsonFieldNames(t, input)
		want := expected[tool]

		if len(fields) != want {
			t.Errorf("%s: expected %d fields, got %d (%v) -- update expected count if field was intentionally added or removed",
				tool, want, len(fields), fields)
		}
	}
}

func stringSet(names []string) map[string]struct{} {
	s := make(map[string]struct{}, len(names))
	for _, name := range names {
		s[name] = struct{}{}
	}

	return s
}

func assertFileopsAndJSONFieldSetsMatch(
	t *testing.T, tool string, fileopsNames, jsonNames map[string]struct{},
) {
	t.Helper()

	for name := range fileopsNames {
		if _, ok := jsonNames[name]; !ok {
			t.Errorf("%s: fileops schema lists output field %q but MCP output struct has no matching json tag",
				tool, name)
		}
	}

	for name := range jsonNames {
		if _, ok := fileopsNames[name]; !ok {
			t.Errorf("%s: MCP output struct has json field %q but fileops schema OutputFields does not list it",
				tool, name)
		}
	}
}

// TestSchemaParity_FileopsOutputFieldsMatchMCPOutputJSONTags ensures
// BuiltinOperationSchemas output field names match the JSON tags on MCP success
// output structs (ExtractOutput, SplitOutput, BlocksOutput, TransformOutput).
func TestSchemaParity_FileopsOutputFieldsMatchMCPOutputJSONTags(t *testing.T) {
	t.Parallel()

	mcpOutputs := map[string]interface{}{
		toolExtract:   ExtractOutput{},
		toolSplit:     SplitOutput{},
		toolBlocks:    BlocksOutput{},
		toolTransform: TransformOutput{},
	}

	for _, schema := range fileops.BuiltinOperationSchemas() {
		tool := schema.Name
		outStruct, ok := mcpOutputs[tool]
		if !ok {
			t.Fatalf("no MCP output struct registered for operation %q", tool)
		}

		want := make(map[string]struct{}, len(schema.OutputFields))
		for _, of := range schema.OutputFields {
			want[of.Name] = struct{}{}
		}

		got := stringSet(jsonFieldNames(t, outStruct))
		assertFileopsAndJSONFieldSetsMatch(t, tool, want, got)
	}
}

func TestSchemaParity_ToolOutputSchemaSplitBlocksMatchesDeclaredHelpers(t *testing.T) {
	t.Parallel()

	gotSplit, err := ToolOutputSchema(toolSplit)
	if err != nil {
		t.Fatalf("ToolOutputSchema split: %v", err)
	}

	if !reflect.DeepEqual(gotSplit, splitOutputSchema()) {
		t.Fatalf("ToolOutputSchema(\"split\") diverged from splitOutputSchema() helper")
	}

	gotBlocks, err := ToolOutputSchema(toolBlocks)
	if err != nil {
		t.Fatalf("ToolOutputSchema blocks: %v", err)
	}

	if !reflect.DeepEqual(gotBlocks, blocksOutputSchema()) {
		t.Fatalf("ToolOutputSchema(\"blocks\") diverged from blocksOutputSchema() helper")
	}
}

type schemaParityRepresentativeCase struct {
	name       string
	schemaTool string
	raw        string
}

func representativeContractSchemaParityCases() []schemaParityRepresentativeCase {
	// Doc-example parity: rooted absolute native paths and error payloads use a fixed
	// `/workspace/repo` root so fixtures align with Unix examples in the contract doc.
	const repo = "/workspace/repo"

	cases := make([]schemaParityRepresentativeCase, 0, 16)
	cases = append(cases, representativeContractSchemaParityCasesPaths(repo)...)
	cases = append(cases, representativeContractSchemaParityCasesErrors(repo)...)
	cases = append(cases, representativeContractSchemaParityCasesBlocks(repo)...)
	cases = append(cases, representativeContractSchemaParityCasesTransform()...)

	return cases
}

func representativeContractSchemaParityCasesPaths(repo string) []schemaParityRepresentativeCase {
	return []schemaParityRepresentativeCase{
		{
			name:       "extract_preview_would_create_absolute",
			schemaTool: toolExtract,
			raw:        `{"would_extract_lines":76,"would_create":"` + repo + `/out/fragment.go"}`,
		},
		{
			name:       "split_apply_files_created_absolute_native",
			schemaTool: toolSplit,
			raw: `{"files_created":["` + repo + `/out/001.md","` + repo + `/out/002.md","` + repo + `/out/003.md","` + repo +
				`/out/004.md"]}`,
		},
		{
			name:       "split_preview_would_create_absolute_native",
			schemaTool: toolSplit,
			raw: `{"would_create":["` + repo + `/out/001.md","` + repo + `/out/002.md","` + repo + `/out/003.md","` + repo +
				`/out/004.md"]}`,
		},
	}
}

func representativeContractSchemaParityCasesErrors(repo string) []schemaParityRepresentativeCase {
	return []schemaParityRepresentativeCase{
		{
			name:       "split_error_no_delimiter_match",
			schemaTool: toolSplit,
			raw: `{"error":"no_delimiter_match","src":"` + repo + `/doc.md","hint":"` +
				`The delimiter pattern did not match any source lines."}`,
		},
		{
			name:       "split_preview_collision_destination_exists_absolute_dest_path",
			schemaTool: toolSplit,
			raw: `{"error":"destination_exists","dest":"` + repo + `/out/001.md","hint":"` +
				`Use --force on the CLI or force: true in MCP JSON to overwrite, ` +
				`or append when the operation supports append mode."}`,
		},
		{
			name:       "extract_preview_collision_destination_exists_absolute_dest_path",
			schemaTool: toolExtract,
			raw: `{"error":"destination_exists","dest":"` + repo + `/out.txt","hint":"` +
				`Use --force on the CLI or force: true in MCP JSON to overwrite, ` +
				`or append when the operation supports append mode."}`,
		},
		{
			name:       "blocks_preview_collision_destination_exists_absolute_dest_path",
			schemaTool: toolBlocks,
			raw: `{"error":"destination_exists","dest":"` + repo + `/out/001.md","hint":"` +
				`Use --force on the CLI or force: true in MCP JSON to overwrite, ` +
				`or append when the operation supports append mode."}`,
		},
		{
			name:       "split_error_max_files_exceeded",
			schemaTool: toolSplit,
			raw: `{"error":"max_files_exceeded","hint":"split: maximum output file count exceeded",` +
				`"max_files":10,"would_create_count":11}`,
		},
	}
}

func representativeContractSchemaParityCasesBlocks(repo string) []schemaParityRepresentativeCase {
	return []schemaParityRepresentativeCase{
		{
			name:       "blocks_apply_with_empty_blocks_found_absolute_paths",
			schemaTool: toolBlocks,
			raw: `{"content_blocks_found":2,"empty_blocks_found":1,"files_created":["` + repo + `/out/001.md","` + repo +
				`/out/002.md"]}`,
		},
		{
			name:       "blocks_apply_without_empty_blocks_absolute_paths",
			schemaTool: toolBlocks,
			raw:        `{"content_blocks_found":2,"files_created":["` + repo + `/out/001.md","` + repo + `/out/002.md"]}`,
		},
		{
			name:       "blocks_preview_with_empty_blocks_absolute_paths",
			schemaTool: toolBlocks,
			raw: `{"content_blocks_found":2,"empty_blocks_found":1,"would_create":["` + repo + `/out/001.md","` + repo +
				`/out/002.md"]}`,
		},
		{
			name:       "blocks_error_no_blocks_found",
			schemaTool: toolBlocks,
			raw:        `{"error":"no_blocks_found","src":"` + repo + `/doc.md","hint":"hint text"}`,
		},
	}
}

func representativeContractSchemaParityCasesTransform() []schemaParityRepresentativeCase {
	return []schemaParityRepresentativeCase{
		{
			name:       "transform_apply_line_endings_trim_trailing_doc_fragment",
			schemaTool: toolTransform,
			raw: `{"changed":true,"endings_changed":87,"lf_found":12,"lf_converted":12,"cr_found":0,"cr_converted":0,` +
				`"crlf_found":75,"crlf_converted":75,"trailing_trimmed":3}`,
		},
		{
			name:       "transform_preview_final_newline_only_doc_fragment",
			schemaTool: toolTransform,
			raw:        `{"would_change":false,"final_newline_needed":false}`,
		},
		{
			name:       "transform_apply_final_newline_only_doc_fragment",
			schemaTool: toolTransform,
			raw:        `{"changed":true,"final_newline_added":true}`,
		},
	}
}

// TestSchemaParity_RepresentativeContractJSONPassesDeclaredOutputSchemas checks representative
// success and error payloads from docs/glyph-shift-json-contract.md satisfy the MCP outputSchema unions.
func TestSchemaParity_RepresentativeContractJSONPassesDeclaredOutputSchemas(t *testing.T) {
	t.Parallel()

	for _, tt := range representativeContractSchemaParityCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			schema, schemaErr := ToolOutputSchema(tt.schemaTool)
			if schemaErr != nil {
				t.Fatalf("ToolOutputSchema: %v", schemaErr)
			}

			var structured any
			if decodeErr := json.Unmarshal([]byte(tt.raw), &structured); decodeErr != nil {
				t.Fatalf("fixture JSON: %v", decodeErr)
			}

			asMap, ok := structured.(map[string]any)
			if !ok {
				t.Fatalf("fixture must decode to a JSON object for parity checks, got %T", structured)
			}
			assertDecodedJSONObjectIsFlatForTOONTransport(t, asMap, tt.name)

			if validateErr := validateStructuredContentAgainstOutputSchema(schema, structured); validateErr != nil {
				t.Fatalf("%v\npayload=%s", validateErr, tt.raw)
			}
		})
	}
}
