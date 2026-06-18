package mcpserver

type operationErrorVariant struct {
	id    string
	tools map[string]struct{}
	build func(toolName string) map[string]any
}

func toolsSet(names ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}

	return set
}

var (
	allMCPTools      = toolsSet(toolExtract, toolSplit, toolBlocks, toolTransform)
	fileOutputTools  = toolsSet(toolExtract, toolSplit, toolBlocks)
	splitBlocksTools = toolsSet(toolSplit, toolBlocks)
	extractOnly      = toolsSet(toolExtract)
	blocksOnly       = toolsSet(toolBlocks)
	transformOnly    = toolsSet(toolTransform)

	patternFieldEnum       = enumStringSchema("invalid_pattern", "pattern_too_long", "control_chars_in_input")
	pathInvalidControlEnum = enumStringSchema("invalid_input", "control_chars_in_input")
)

const (
	oeDefContractString   = "cs"
	oeDefPathInvalidError = "piv"
)

func operationErrorLocalRef(def string) map[string]any {
	return map[string]any{"$ref": "#/$defs/" + def}
}

func operationErrorSharedDefs() map[string]any {
	return map[string]any{
		oeDefContractString:   contractStringSchema(),
		oeDefPathInvalidError: pathInvalidControlEnum,
	}
}

func oeContractString() map[string]any {
	return operationErrorLocalRef(oeDefContractString)
}

func oePathInvalidError() map[string]any {
	return operationErrorLocalRef(oeDefPathInvalidError)
}

func operationErrorBranch(required []string, errorSchema, fields map[string]any) map[string]any {
	properties := map[string]any{
		"error": errorSchema,
		"hint":  oeContractString(),
	}
	for name, schema := range fields {
		properties[name] = schema
	}

	return objectSchema(required, properties)
}

func sourceErrorEnumForTool(toolName string) map[string]any {
	switch toolName {
	case toolSplit:
		return enumStringSchema(
			"source_not_found",
			"binary_source",
			"directory_not_file",
			"not_regular_file",
			"no_delimiter_match",
		)
	case toolBlocks:
		return enumStringSchema(
			"source_not_found",
			"binary_source",
			"directory_not_file",
			"not_regular_file",
			"no_blocks_found",
		)
	default:
		return enumStringSchema(
			"source_not_found",
			"binary_source",
			"directory_not_file",
			"not_regular_file",
		)
	}
}

func sourceOperationErrorSchema(toolName string) map[string]any {
	return operationErrorBranch(
		[]string{"error", "hint", "src"},
		sourceErrorEnumForTool(toolName),
		map[string]any{"src": oeContractString()},
	)
}

func emptyRangeOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch(
		[]string{"error", "hint", "range_end", "range_start", "src"},
		jsonConst("empty_range"),
		map[string]any{"src": oeContractString(), "range_start": integerSchema(), "range_end": integerSchema()},
	)
}

func rangeExceedsFileOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch(
		[]string{"error", "file_lines", "hint", "range_end", "range_start", "src"},
		jsonConst("range_exceeds_file"),
		map[string]any{
			"src":         oeContractString(),
			"file_lines":  integerSchema(),
			"range_start": integerSchema(),
			"range_end":   integerSchema(),
		},
	)
}

func unclosedBlockOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch(
		[]string{"error", "hint", "src", "start_line"},
		jsonConst("unclosed_block"),
		map[string]any{"src": oeContractString(), "start_line": integerSchema()},
	)
}

func destinationExistsOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch(
		[]string{"dest", "error", "hint"},
		jsonConst("destination_exists"),
		map[string]any{"dest": oeContractString()},
	)
}

func sourceFingerprintMismatchOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch(
		[]string{"error", "hint", "output_path"},
		jsonConst("source_fingerprint_mismatch"),
		map[string]any{"output_path": oeContractString()},
	)
}

func patternFieldOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch(
		[]string{"error", "field", "hint"},
		patternFieldEnum,
		map[string]any{"field": oeContractString()},
	)
}

func pathInvalidSrcOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch(
		[]string{"error", "hint", "src"},
		oePathInvalidError(),
		map[string]any{"src": oeContractString()},
	)
}

func pathInvalidDestOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch(
		[]string{"dest", "error", "hint"},
		oePathInvalidError(),
		map[string]any{"dest": oeContractString()},
	)
}

func pathInvalidOutDirOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch(
		[]string{"error", "hint", "out_dir"},
		oePathInvalidError(),
		map[string]any{"out_dir": oeContractString()},
	)
}

func pathInvalidOutputPathOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch(
		[]string{"error", "hint", "output_path"},
		oePathInvalidError(),
		map[string]any{"output_path": oeContractString()},
	)
}

func genericInvalidInputOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch([]string{"error", "hint"}, jsonConst("invalid_input"), nil)
}

func noTransformSpecifiedOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch([]string{"error", "hint"}, jsonConst("no_transform_specified"), nil)
}

func invalidLineEndingsOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch([]string{"error", "hint"}, jsonConst("invalid_line_endings"), nil)
}

func maxFilesExceededOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch(
		[]string{"error", "hint", "max_files", "would_create_count"},
		jsonConst("max_files_exceeded"),
		map[string]any{"max_files": integerSchema(), "would_create_count": integerSchema()},
	)
}

func namesCountMismatchOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch(
		[]string{"error", "hint", "names_count", "output_count"},
		jsonConst("names_count_mismatch"),
		map[string]any{"names_count": integerSchema(), "output_count": integerSchema()},
	)
}

func writeFailedOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch([]string{"error", "hint"}, jsonConst("write_failed"), nil)
}

func internalErrorOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch([]string{"error", "hint"}, jsonConst("internal_error"), nil)
}

func internalErrorSrcOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch(
		[]string{"error", "hint", "src"},
		jsonConst("internal_error"),
		map[string]any{"src": oeContractString()},
	)
}

func internalErrorOutputPathOperationErrorSchema(_ string) map[string]any {
	return operationErrorBranch(
		[]string{"error", "hint", "output_path"},
		jsonConst("internal_error"),
		map[string]any{"output_path": oeContractString()},
	)
}

// operationErrorRegistry binds each OperationError oneOf branch to the MCP tools that may emit it.
// Registry order is the branch order advertised in each tool's outputSchema.
var operationErrorRegistry = []operationErrorVariant{
	{id: "source", tools: allMCPTools, build: sourceOperationErrorSchema},
	{id: "empty_range", tools: extractOnly, build: emptyRangeOperationErrorSchema},
	{id: "range_exceeds_file", tools: extractOnly, build: rangeExceedsFileOperationErrorSchema},
	{id: "unclosed_block", tools: blocksOnly, build: unclosedBlockOperationErrorSchema},
	{id: "destination_exists", tools: fileOutputTools, build: destinationExistsOperationErrorSchema},
	{id: "source_fingerprint_mismatch", tools: fileOutputTools, build: sourceFingerprintMismatchOperationErrorSchema},
	{id: "pattern_field", tools: splitBlocksTools, build: patternFieldOperationErrorSchema},
	{id: "path_invalid_src", tools: allMCPTools, build: pathInvalidSrcOperationErrorSchema},
	{id: "path_invalid_dest", tools: extractOnly, build: pathInvalidDestOperationErrorSchema},
	{id: "path_invalid_out_dir", tools: splitBlocksTools, build: pathInvalidOutDirOperationErrorSchema},
	{id: "path_invalid_output_path", tools: splitBlocksTools, build: pathInvalidOutputPathOperationErrorSchema},
	{id: "invalid_input", tools: allMCPTools, build: genericInvalidInputOperationErrorSchema},
	{id: "no_transform_specified", tools: transformOnly, build: noTransformSpecifiedOperationErrorSchema},
	{id: "invalid_line_endings", tools: transformOnly, build: invalidLineEndingsOperationErrorSchema},
	{id: "max_files_exceeded", tools: splitBlocksTools, build: maxFilesExceededOperationErrorSchema},
	{id: "names_count_mismatch", tools: splitBlocksTools, build: namesCountMismatchOperationErrorSchema},
	{id: "write_failed", tools: allMCPTools, build: writeFailedOperationErrorSchema},
	{id: "internal_error", tools: allMCPTools, build: internalErrorOperationErrorSchema},
	{id: "internal_error_src", tools: allMCPTools, build: internalErrorSrcOperationErrorSchema},
	{id: "internal_error_output_path", tools: fileOutputTools, build: internalErrorOutputPathOperationErrorSchema},
}

// operationErrorBranchesForTool returns the MCP outputSchema error branches each tool may return.
func operationErrorBranchesForTool(toolName string) []map[string]any {
	branches := make([]map[string]any, 0, len(operationErrorRegistry))
	for _, entry := range operationErrorRegistry {
		if _, ok := entry.tools[toolName]; !ok {
			continue
		}

		branches = append(branches, entry.build(toolName))
	}

	return branches
}

// operationErrorSchemaForTool builds the nested oneOf under $defs.OperationError for toolName.
func operationErrorSchemaForTool(toolName string) map[string]any {
	branches := operationErrorBranchesForTool(toolName)
	oneOf := make([]any, len(branches))
	for i, branch := range branches {
		oneOf[i] = branch
	}

	return map[string]any{
		"type":  "object",
		"oneOf": oneOf,
	}
}

// operationErrorRef returns the top-level oneOf branch that points at $defs.OperationError.
func operationErrorRef() map[string]any {
	return map[string]any{"$ref": "#/$defs/OperationError"}
}

// toolOutputSchemaWithOperationError wraps per-tool success variants and a $ref-backed OperationError
// def in a token-efficient outputSchema.
func toolOutputSchemaWithOperationError(toolName string, successVariants ...map[string]any) map[string]any {
	oneOf := make([]any, 0, len(successVariants)+1)
	for _, variant := range successVariants {
		oneOf = append(oneOf, variant)
	}

	oneOf = append(oneOf, operationErrorRef())

	defs := operationErrorSharedDefs()
	defs["OperationError"] = operationErrorSchemaForTool(toolName)

	return map[string]any{
		"type":  "object",
		"$defs": defs,
		"oneOf": oneOf,
	}
}
