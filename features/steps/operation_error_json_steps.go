package steps

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cucumber/godog"

	"github.com/hotchkj/glyph-shift/internal/fsnorm"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

func operationErrorMapFromLayer1(tc *TestContext) (map[string]interface{}, error) {
	if tc.LastOperationError == nil {
		return nil, errOperationJSONSourceNeedsLayer1Error
	}

	out := pipeline.ClassifyOperationError(tc.LastOperationError, tc.LastOperationErrorFallbackPath)

	payload, err := pipeline.FormatOperationErrorJSON(tc.Ws.Root(), out)
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{}, len(payload))
	for fieldName, fieldValue := range payload {
		result[fieldName] = fieldValue
	}

	return result, nil
}

// establishedOperationErrorJSONSources reports which error-JSON channels in tc currently hold
// usable assertion data (validated operation-error JSON shape, same bar as stderr parsing).
// MCP structuredContent counts only when tc.MCPError is set (structured payload classified by
// operationErrorFieldsFromMap in captureLastMCPToolResult); MCP text content counts only when
// operationErrorFieldsFromMap succeeds. Success-shaped MCP payloads must not compete with
// operation or stderr for omitted-source resolution. Order is stable for error messages.
func establishedOperationErrorJSONSources(tc *TestContext) []string {
	var out []string

	if tc.LastOperationError != nil {
		out = append(out, "operation")
	}

	if _, err := parseStrictGlyphShiftStderrJSON(tc.Stderr); err == nil {
		out = append(out, "CLI")
	}

	if tc.MCPError != nil {
		out = append(out, "MCP structuredContent")
	}

	if tc.MCPContentJSON != nil {
		if _, err := operationErrorFieldsFromMap(tc.MCPContentJSON); err == nil {
			out = append(out, "MCP content")
		}
	}

	return out
}

func resolveOmittedOperationErrorJSON(tc *TestContext) (map[string]interface{}, error) {
	sources := establishedOperationErrorJSONSources(tc)
	switch len(sources) {
	case 0:
		return parseStrictGlyphShiftStderrJSON(tc.Stderr)
	case 1:
		switch sources[0] {
		case "operation":
			return operationErrorMapFromLayer1(tc)
		case "CLI":
			return parseStrictGlyphShiftStderrJSON(tc.Stderr)
		case "MCP structuredContent":
			if tc.MCPStructuredContent == nil {
				return nil, errMCPStructuredContentNil
			}

			return tc.MCPStructuredContent, nil
		case "MCP content":
			if tc.MCPContentJSON == nil {
				return nil, errMCPContentJSONNil
			}

			return tc.MCPContentJSON, nil
		default:
			return nil, fmt.Errorf("%w: internal: unknown established source %q",
				errOperationErrorJSONAssert, sources[0])
		}
	default:
		return nil, fmt.Errorf(
			"%w: multiple sources available (%s); specify operation, CLI, stderr, MCP "+
				"structuredContent, or MCP content",
			errOperationErrorJSONSourceAmbiguous, strings.Join(sources, ", "))
	}
}

func resolveOperationErrorJSON(tc *TestContext, source string) (map[string]interface{}, error) {
	sourceName := strings.TrimSpace(source)
	switch sourceName {
	case "":
		return resolveOmittedOperationErrorJSON(tc)
	case "operation":
		return operationErrorMapFromLayer1(tc)
	case "CLI", "stderr":
		return parseStrictGlyphShiftStderrJSON(tc.Stderr)
	case "MCP structuredContent":
		if tc.MCPStructuredContent == nil {
			return nil, errMCPStructuredContentNil
		}

		return tc.MCPStructuredContent, nil
	case "MCP content":
		if tc.MCPContentJSON == nil {
			return nil, errMCPContentJSONNil
		}

		return tc.MCPContentJSON, nil
	default:
		return nil, fmt.Errorf("%w: %q", errUnknownOperationErrorJSONSource, sourceName)
	}
}

func parseCommaSeparatedFieldNames(csv string) ([]string, error) {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			return nil, fmt.Errorf("%w in %q", errEmptyCommaSeparatedPathSegment, csv)
		}

		out = append(out, s)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("%w: %q", errEmptyFieldNameList, csv)
	}

	return out, nil
}

func jsonInt64MatchingExactInt(v interface{}, want int64) bool {
	switch value := v.(type) {
	case json.Number:
		integer, err := value.Int64()
		if err != nil {
			return false
		}

		return integer == want
	case float64:
		return int64(value) == want && value == float64(int64(value))
	case int:
		return int64(value) == want
	case int64:
		return value == want
	default:
		return false
	}
}

func assertOpErrJSONFieldIsExact(field, expected string, raw interface{}) error {
	switch value := raw.(type) {
	case string:
		if value != expected {
			return fmt.Errorf("%w: field %q want %q got %q", errOperationErrorJSONAssert, field, expected, value)
		}
	case json.Number:
		if value.String() != expected {
			return fmt.Errorf("%w: field %q want %q got %v", errOperationErrorJSONAssert, field, expected, raw)
		}
	case float64:
		if wantedFloat, err := strconv.ParseFloat(expected, 64); err == nil && value == wantedFloat {
			return nil
		}

		return fmt.Errorf("%w: field %q want %q got %v", errOperationErrorJSONAssert, field, expected, raw)
	default:
		if fmt.Sprint(raw) != expected {
			return fmt.Errorf("%w: field %q want %q got %v", errOperationErrorJSONAssert, field, expected, raw)
		}
	}

	return nil
}

func assertOpErrJSONFieldIsInteger(field, expected string, raw interface{}) error {
	want, perr := strconv.ParseInt(expected, 10, 64)
	if perr != nil {
		return fmt.Errorf("parse expected integer: %w", perr)
	}

	if !jsonInt64MatchingExactInt(raw, want) {
		return fmt.Errorf("%w: field %q want integer %d got %v (%T)",
			errOperationErrorJSONAssert, field, want, raw, raw)
	}

	return nil
}

func assertOpErrJSONFieldIsStringArray(field, expected string, raw interface{}) error {
	arr, ok := raw.([]interface{})
	if !ok {
		return fmt.Errorf("%w: field %q want JSON array got %T", errOperationErrorJSONAssert, field, raw)
	}

	trimmed := strings.TrimSpace(expected)
	if trimmed == "" {
		if len(arr) == 0 {
			return nil
		}

		return fmt.Errorf("%w: field %q want empty array got %v", errOperationErrorJSONAssert, field, arr)
	}

	parts := strings.Split(expected, ",")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
		if parts[index] == "" {
			return fmt.Errorf("%w: empty segment in expected list %q", errOperationErrorJSONAssert, expected)
		}
	}

	if len(parts) != len(arr) {
		return fmt.Errorf("%w: field %q want %d elements got %d in %v",
			errOperationErrorJSONAssert, field, len(parts), len(arr), arr)
	}

	for index := range parts {
		elem, elemOK := arr[index].(string)
		if !elemOK {
			return fmt.Errorf("%w: field %q index %d not string", errOperationErrorJSONAssert, field, index)
		}

		if elem != parts[index] {
			return fmt.Errorf("%w: field %q index %d want %q got %q",
				errOperationErrorJSONAssert, field, index, parts[index], elem)
		}
	}

	return nil
}

func assertOpErrJSONFieldIsWorkspacePath(tc *TestContext, field, expected string, raw interface{}) error {
	gotStr, ok := raw.(string)
	if !ok {
		return fmt.Errorf("%w: field %q want string path got %T", errOperationErrorJSONAssert, field, raw)
	}

	wantAbs := tc.Ws.Join(fsnorm.Canonical(strings.TrimSpace(expected)))
	if filepath.Clean(gotStr) != filepath.Clean(wantAbs) {
		return fmt.Errorf("%w: field %q want absolute path %q got %q",
			errOperationErrorJSONAssert, field, wantAbs, gotStr)
	}

	if !filepath.IsAbs(gotStr) {
		return fmt.Errorf("%w: field %q path not absolute: %q", errOperationErrorJSONAssert, field, gotStr)
	}

	return nil
}

func assertOperationErrorJSONField(
	tc *TestContext,
	source, field, assertion, expected string,
) error {
	obj, err := resolveOperationErrorJSON(tc, source)
	if err != nil {
		return err
	}

	raw, ok := obj[field]
	if !ok {
		return fmt.Errorf("%w: missing field %q in %#v", errOperationErrorJSONAssert, field, obj)
	}

	switch assertion {
	case "is":
		return assertOpErrJSONFieldIsExact(field, expected, raw)
	case "is integer":
		return assertOpErrJSONFieldIsInteger(field, expected, raw)
	case "is string array":
		return assertOpErrJSONFieldIsStringArray(field, expected, raw)
	case "is workspace path":
		return assertOpErrJSONFieldIsWorkspacePath(tc, field, expected, raw)
	default:
		return fmt.Errorf("%w: unknown assertion %q", errOperationErrorJSONAssert, assertion)
	}
}

func assertOperationErrorJSONFieldsExactly(tc *TestContext, source, csv string) error {
	obj, err := resolveOperationErrorJSON(tc, source)
	if err != nil {
		return err
	}

	wantFields, ferr := parseCommaSeparatedFieldNames(csv)
	if ferr != nil {
		return ferr
	}

	wantSorted := append([]string(nil), wantFields...)
	sort.Strings(wantSorted)

	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	if len(keys) != len(wantSorted) {
		return fmt.Errorf("%w: key count got %v want %v (full object %#v)",
			errOperationErrorJSONAssert, keys, wantSorted, obj)
	}

	for i := range keys {
		if keys[i] != wantSorted[i] {
			return fmt.Errorf("%w: keys got %v want %v (full object %#v)",
				errOperationErrorJSONAssert, keys, wantSorted, obj)
		}
	}

	return nil
}

func registerOperationErrorJSONSteps(sc *godog.ScenarioContext, tc *TestContext) {
	//nolint:lll // Godog regex length for optional source and multi-word assertions.
	sc.Then(
		`^the (?:(operation|CLI|stderr|MCP structuredContent|MCP content) )?`+
			`error JSON field "([^"]*)" (is string array|is workspace path|is integer|is) "([^"]*)"$`,
		func(source, field, assertion, expected string) error {
			return assertOperationErrorJSONField(tc, source, field, assertion, expected)
		},
	)

	sc.Then(
		`^the (?:(operation|CLI|stderr|MCP structuredContent|MCP content) )?`+
			`error JSON field "([^"]*)" is exactly:$`,
		func(source, field string, doc *godog.DocString) error {
			if doc == nil {
				return fmt.Errorf("%w: missing DocString for field %q exact assertion", errOperationErrorJSONAssert, field)
			}
			return assertOperationErrorJSONField(tc, source, field, "is", strings.TrimSpace(doc.Content))
		},
	)

	sc.Then(
		`^the (?:(operation|CLI|stderr|MCP structuredContent|MCP content) )?`+
			`error JSON fields are exactly "([^"]*)"$`,
		func(source, csv string) error {
			return assertOperationErrorJSONFieldsExactly(tc, source, csv)
		},
	)
}
