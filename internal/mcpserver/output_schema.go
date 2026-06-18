package mcpserver

func stringSchema() map[string]any {
	return map[string]any{"type": "string"}
}

// contractStringSchema matches docs/glyph-shift-json-contract.md:
// emitted operation error strings are non-empty at the JSON edge.
func contractStringSchema() map[string]any {
	return map[string]any{
		"type":      "string",
		"minLength": 1,
	}
}

func integerSchema() map[string]any {
	return map[string]any{"type": "integer"}
}

func booleanSchema() map[string]any {
	return map[string]any{"type": "boolean"}
}

func stringArraySchema() map[string]any {
	return map[string]any{
		"type":  "array",
		"items": stringSchema(),
	}
}

func jsonConst(val any) map[string]any {
	return map[string]any{"const": val}
}

func enumStringSchema(values ...string) map[string]any {
	items := make([]any, len(values))
	for i, v := range values {
		items[i] = v
	}

	return map[string]any{
		"type": "string",
		"enum": items,
	}
}

func objectSchema(required []string, properties map[string]any) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             required,
		"properties":           properties,
	}
}

func nestedObjectOneOf(variants ...map[string]any) map[string]any {
	return map[string]any{
		"type":  "object",
		"oneOf": variants,
	}
}

// objectAnyOf matches if any variant subschema matches (each variant is typically a full object schema).
func objectAnyOf(variants ...map[string]any) map[string]any {
	branches := make([]any, len(variants))
	for i, v := range variants {
		branches[i] = v
	}

	return map[string]any{"anyOf": branches}
}

func extractOutputSchema() map[string]any {
	return toolOutputSchemaWithOperationError(toolExtract,
		objectSchema(
			[]string{"lines_extracted"},
			map[string]any{"lines_extracted": integerSchema()},
		),
		objectSchema(
			[]string{"would_extract_lines", "would_create"},
			map[string]any{
				"would_extract_lines": integerSchema(),
				"would_create":        stringSchema(),
			},
		),
	)
}

func splitOutputSchema() map[string]any {
	return toolOutputSchemaWithOperationError(toolSplit,
		objectSchema(
			[]string{"files_created"},
			map[string]any{"files_created": stringArraySchema()},
		),
		objectSchema(
			[]string{"would_create"},
			map[string]any{"would_create": stringArraySchema()},
		),
	)
}

func blocksOutputSchema() map[string]any {
	blockCountProperties := map[string]any{
		"content_blocks_found": integerSchema(),
		"empty_blocks_found":   integerSchema(),
	}

	applyProperties := map[string]any{
		"content_blocks_found": blockCountProperties["content_blocks_found"],
		"empty_blocks_found":   blockCountProperties["empty_blocks_found"],
		"files_created":        stringArraySchema(),
	}

	previewProperties := map[string]any{
		"content_blocks_found": blockCountProperties["content_blocks_found"],
		"empty_blocks_found":   blockCountProperties["empty_blocks_found"],
		"would_create":         stringArraySchema(),
	}

	return toolOutputSchemaWithOperationError(toolBlocks,
		objectSchema([]string{"content_blocks_found", "files_created"}, applyProperties),
		objectSchema([]string{"content_blocks_found", "would_create"}, previewProperties),
	)
}

func transformOutputSchema() map[string]any {
	lineEndingCountProps := map[string]any{
		"endings_changed": integerSchema(),
		"lf_found":        integerSchema(),
		"lf_converted":    integerSchema(),
		"cr_found":        integerSchema(),
		"cr_converted":    integerSchema(),
		"crlf_found":      integerSchema(),
		"crlf_converted":  integerSchema(),
	}

	lineEndingCountRequired := []string{
		"endings_changed",
		"lf_found",
		"lf_converted",
		"cr_found",
		"cr_converted",
		"crlf_found",
		"crlf_converted",
	}

	applyMinimalProps := map[string]any{
		"changed":             booleanSchema(),
		"final_newline_added": booleanSchema(),
		"trailing_trimmed":    integerSchema(),
	}

	previewMinimalProps := map[string]any{
		"would_change":         booleanSchema(),
		"final_newline_needed": booleanSchema(),
		"trailing_trimmed":     integerSchema(),
	}

	applyBundleProps := map[string]any{
		"changed":             booleanSchema(),
		"final_newline_added": booleanSchema(),
		"trailing_trimmed":    integerSchema(),
	}
	for k, schema := range lineEndingCountProps {
		applyBundleProps[k] = schema
	}

	previewBundleProps := map[string]any{
		"would_change":         booleanSchema(),
		"final_newline_needed": booleanSchema(),
		"trailing_trimmed":     integerSchema(),
	}
	for k, schema := range lineEndingCountProps {
		previewBundleProps[k] = schema
	}

	applyBundleRequired := append([]string{"changed"}, lineEndingCountRequired...)
	previewBundleRequired := append([]string{"would_change"}, lineEndingCountRequired...)

	// Non-line-ending success shapes always pair `changed`/`would_change` with the requested
	// transform facet (final-newline booleans or trim count); `changed`/`would_change` alone
	// are not valid per docs/glyph-shift-json-contract.md.
	applyMinimalUnion := objectAnyOf(
		objectSchema([]string{"changed", "final_newline_added"}, applyMinimalProps),
		objectSchema([]string{"changed", "trailing_trimmed"}, applyMinimalProps),
	)
	previewMinimalUnion := objectAnyOf(
		objectSchema([]string{"would_change", "final_newline_needed"}, previewMinimalProps),
		objectSchema([]string{"would_change", "trailing_trimmed"}, previewMinimalProps),
	)

	applyShapes := nestedObjectOneOf(
		applyMinimalUnion,
		objectSchema(applyBundleRequired, applyBundleProps),
	)

	previewShapes := nestedObjectOneOf(
		previewMinimalUnion,
		objectSchema(previewBundleRequired, previewBundleProps),
	)

	return toolOutputSchemaWithOperationError(toolTransform, applyShapes, previewShapes)
}
