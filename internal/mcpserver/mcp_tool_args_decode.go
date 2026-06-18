package mcpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

const (
	toolExtract   = "extract"
	toolSplit     = "split"
	toolBlocks    = "blocks"
	toolTransform = "transform"
)

var (
	extractToolArgFields = jsonFieldSet("source", "lines", "destination", "force", "append", "mkdir", "preview")
	splitToolArgFields   = jsonFieldSet(
		"source", "delimiter", "output_dir", "extension", "strip_delimiter",
		"force", "mkdir", "names", "max_files", "preview",
	)
	blocksToolArgFields = jsonFieldSet(
		"source", "start_line", "end_line", "output_dir", "extension",
		"include_delimiters", "force", "mkdir", "names", "max_files", "preview",
	)
	transformToolArgFields = jsonFieldSet("source", "line_endings", "trim_trailing", "final_newline", "preview")
)

type unexpectedToolArgFieldError struct {
	field string
}

func (e *unexpectedToolArgFieldError) Error() string {
	return "unexpected MCP tool argument field: " + e.field
}

func (e *unexpectedToolArgFieldError) Field() string {
	return e.field
}

func decodeExtractToolArgs(argsJSON []byte) (ExtractInput, error) {
	var v ExtractInput
	err := decodeToolArgs(argsJSON, &v, extractToolArgFields)
	return v, err
}

func decodeSplitToolArgs(argsJSON []byte) (SplitInput, error) {
	var v SplitInput
	err := decodeToolArgs(argsJSON, &v, splitToolArgFields)
	return v, err
}

func decodeBlocksToolArgs(argsJSON []byte) (BlocksInput, error) {
	var v BlocksInput
	err := decodeToolArgs(argsJSON, &v, blocksToolArgFields)
	return v, err
}

func decodeTransformToolArgs(argsJSON []byte) (TransformInput, error) {
	var v TransformInput
	err := decodeToolArgs(argsJSON, &v, transformToolArgFields)
	return v, err
}

func jsonFieldSet(fields ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		out[field] = struct{}{}
	}

	return out
}

func rejectUnknownToolArgFields(argsJSON []byte, allowed map[string]struct{}) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(argsJSON, &raw); err != nil {
		return err
	}

	var unknown []string
	for field := range raw {
		if _, ok := allowed[field]; !ok {
			unknown = append(unknown, field)
		}
	}
	if len(unknown) == 0 {
		return nil
	}

	sort.Strings(unknown)

	return &unexpectedToolArgFieldError{field: unknown[0]}
}

func decodeToolArgs(argsJSON []byte, dest any, allowed map[string]struct{}) error {
	if err := rejectUnknownToolArgFields(argsJSON, allowed); err != nil {
		return err
	}

	dec := json.NewDecoder(bytes.NewReader(argsJSON))

	return dec.Decode(dest)
}

func acceptedFieldsHintForTool(toolName string) string {
	switch toolName {
	case toolExtract:
		return "accepted fields: source, lines, destination, preview, force, append, mkdir (see tool input schema)"
	case toolSplit:
		return "accepted fields: source, delimiter, output_dir, preview, extension, strip_delimiter, force, mkdir, names, " +
			"max_files (see tool input schema)"
	case toolBlocks:
		return "accepted fields: source, start_line, end_line, output_dir, preview, extension, include_delimiters, " +
			"force, mkdir, names, max_files (see tool input schema)"
	case toolTransform:
		return "accepted fields: source, line_endings, trim_trailing, final_newline, preview (see tool input schema)"
	default:
		return "see tool input schema"
	}
}

func toolDecodeErrorFieldName(err error) string {
	if err == nil {
		return ""
	}

	var fieldErr *unexpectedToolArgFieldError
	if errors.As(err, &fieldErr) {
		return fieldErr.Field()
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return typeErr.Field
	}

	return ""
}

func unexpectedArgumentToolResult(workspaceRoot, toolName string, decodeErr error) (*mcp.CallToolResult, error) {
	res := toolDecodeErrorFieldName(decodeErr)
	hint := acceptedFieldsHintForTool(toolName)

	fields := map[string]string{}
	if res != "" {
		fields["field"] = res
	} else {
		// No parseable unknown field name (e.g. unmarshal type error without a concrete member path).
		// docs/glyph-shift-json-contract.md lists logical input names for `field`; this sentinel marks
		// whole-document / shape failures so clients still get a non-empty `field` for the
		// unexpected_argument + field variant (mutually exclusive with CLI `argument`).
		fields["field"] = "json"
	}

	outcome := pipeline.ErrorOutcome{
		Error:        unexpectedArgumentSentinel,
		Hint:         hint,
		ExitCode:     pipeline.ExitValidation,
		StringFields: fields,
	}
	payload := pipeline.OperationErrorPayload(workspaceRoot, &outcome)

	result, _, err := errorToolResult(payload)

	return result, err
}

func normalizedToolArgumentsJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}

	return raw
}
