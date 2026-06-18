package mcpserver

import (
	"slices"
	"testing"
)

func operationErrorVariantIDsForTool(toolName string) []string {
	ids := make([]string, 0, len(operationErrorRegistry))
	for _, entry := range operationErrorRegistry {
		if _, ok := entry.tools[toolName]; !ok {
			continue
		}

		ids = append(ids, entry.id)
	}

	return ids
}

func variantIDSet(ids []string) map[string]struct{} {
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}

	return set
}

func diffVariantIDs(got, want map[string]struct{}) (missing, extra []string) {
	for id := range want {
		if _, ok := got[id]; !ok {
			missing = append(missing, id)
		}
	}

	for id := range got {
		if _, ok := want[id]; !ok {
			extra = append(extra, id)
		}
	}

	slices.Sort(missing)
	slices.Sort(extra)

	return missing, extra
}

// expectedOperationErrorVariantsByTool mirrors runtime classification and the JSON contract matrix.
// Each tool must advertise exactly these registry variant IDs in outputSchema OperationError branches.
var expectedOperationErrorVariantsByTool = map[string][]string{
	toolExtract: {
		"source",
		"empty_range",
		"range_exceeds_file",
		"destination_exists",
		"source_fingerprint_mismatch",
		"path_invalid_src",
		"path_invalid_dest",
		"invalid_input",
		"write_failed",
		"internal_error",
		"internal_error_src",
		"internal_error_output_path",
	},
	toolSplit: {
		"source",
		"destination_exists",
		"source_fingerprint_mismatch",
		"pattern_field",
		"path_invalid_src",
		"path_invalid_out_dir",
		"path_invalid_output_path",
		"invalid_input",
		"max_files_exceeded",
		"names_count_mismatch",
		"write_failed",
		"internal_error",
		"internal_error_src",
		"internal_error_output_path",
	},
	toolBlocks: {
		"source",
		"unclosed_block",
		"destination_exists",
		"source_fingerprint_mismatch",
		"pattern_field",
		"path_invalid_src",
		"path_invalid_out_dir",
		"path_invalid_output_path",
		"invalid_input",
		"max_files_exceeded",
		"names_count_mismatch",
		"write_failed",
		"internal_error",
		"internal_error_src",
		"internal_error_output_path",
	},
	toolTransform: {
		"source",
		"path_invalid_src",
		"invalid_input",
		"no_transform_specified",
		"invalid_line_endings",
		"write_failed",
		"internal_error",
		"internal_error_src",
	},
}

func TestOperationErrorRegistry_ParityWithExpectedMatrix(t *testing.T) {
	t.Parallel()

	canonicalTools := []string{toolExtract, toolSplit, toolBlocks, toolTransform}
	if len(expectedOperationErrorVariantsByTool) != len(canonicalTools) {
		t.Fatalf("expected matrix covers %d tools, want %d canonical MCP tools",
			len(expectedOperationErrorVariantsByTool), len(canonicalTools))
	}

	for _, toolName := range canonicalTools {
		t.Run(toolName, func(t *testing.T) {
			t.Parallel()

			wantIDs, ok := expectedOperationErrorVariantsByTool[toolName]
			if !ok {
				t.Fatalf("expected matrix missing tool %q", toolName)
			}

			gotIDs := operationErrorVariantIDsForTool(toolName)
			missing, extra := diffVariantIDs(variantIDSet(gotIDs), variantIDSet(wantIDs))

			if len(missing) > 0 || len(extra) > 0 {
				t.Fatalf("registry parity failed for %s: missing=%v extra=%v got=%v want=%v",
					toolName, missing, extra, gotIDs, wantIDs)
			}
		})
	}
}

func TestValidateToolStructuredOutput_RejectsToolInapplicableSentinels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tool    string
		payload map[string]any
	}{
		{
			tool: toolExtract,
			payload: map[string]any{
				"error": "no_delimiter_match",
				"hint":  "delimiter mismatch",
				"src":   "/workspace/a.txt",
			},
		},
		{
			tool: toolSplit,
			payload: map[string]any{
				"error": "no_blocks_found",
				"hint":  "no blocks",
				"src":   "/workspace/a.txt",
			},
		},
		{
			tool: toolTransform,
			payload: map[string]any{
				"error": "destination_exists",
				"hint":  "dest exists",
				"dest":  "/workspace/out.txt",
			},
		},
		{
			tool: toolBlocks,
			payload: map[string]any{
				"error": "no_transform_specified",
				"hint":  "transform not applicable",
				"src":   "/workspace/a.txt",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			t.Parallel()

			if err := ValidateToolStructuredOutput(tc.tool, tc.payload); err == nil {
				t.Fatalf("expected outputSchema rejection for inapplicable sentinel on %s", tc.tool)
			}
		})
	}
}
