package pipeline

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ValidateOperationErrorPayload reports whether m matches the docs/glyph-shift-json-contract.md
// operation error oneOf shape (keys, types, and sentinel-specific required sets).
func ValidateOperationErrorPayload(m map[string]any) error {
	return validateFormattedOperationError(m)
}

func validateFormattedOperationError(payload map[string]any) error {
	if payload == nil {
		return fmt.Errorf("%w", errOpErrJSONNilMap)
	}

	for jsonKey, val := range payload {
		if err := valueJSONEdgeSafe(jsonKey, val); err != nil {
			return err
		}
	}

	if err := matchContractErrorVariant(payload); err != nil {
		return err
	}

	return nil
}

func valueJSONEdgeSafe(key string, raw any) error {
	if raw == nil {
		return fmt.Errorf("%w: key %q", errOpErrJSONKeyIsNull, key)
	}

	switch typed := raw.(type) {
	case string:
		return validateJSONEdgeNonEmptyString(key, typed)
	case []string:
		return validateJSONEdgeStringSlice(key, typed)
	case []any:
		return validateJSONEdgeAnyStringSlice(key, typed)
	case int, int64, float64:
		// emitted from int fields; accept JSON number typing from decode too
		return nil
	case json.Number:
		// stdlib JSON decoder with UseNumber for test/transport strict integer decoding
		return nil
	default:
		return fmt.Errorf("%w: key %q type %T", errOpErrJSONUnsupportedValueType, key, raw)
	}
}

func validateJSONEdgeNonEmptyString(key, s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("%w: key %q", errOpErrJSONKeyEmptyString, key)
	}

	return nil
}

func validateJSONEdgeStringSlice(key string, t []string) error {
	if len(t) == 0 {
		return fmt.Errorf("%w: key %q", errOpErrJSONKeyEmptyArray, key)
	}

	for idx, line := range t {
		if strings.TrimSpace(line) == "" {
			return fmt.Errorf("%w: %s[%d]", errOpErrJSONArrayStringElemEmpty, key, idx)
		}
	}

	return nil
}

func validateJSONEdgeAnyStringSlice(key string, t []any) error {
	if len(t) == 0 {
		return fmt.Errorf("%w: key %q", errOpErrJSONKeyEmptyArray, key)
	}

	for idx, el := range t {
		str, ok := el.(string)
		if !ok {
			return fmt.Errorf("%w: %s[%d]", errOpErrJSONArrayElemNotString, key, idx)
		}

		if strings.TrimSpace(str) == "" {
			return fmt.Errorf("%w: %s[%d]", errOpErrJSONArrayStringElemEmpty, key, idx)
		}
	}

	return nil
}

var contractErrorStaticKeySets = map[string][]string{
	opErrClassWriteFailed:         {"error", "hint"},
	opErrClassUnknownCommand:      {"command", "error", "hint"},
	opErrClassInvalidFlag:         {"error", "flag", "hint"},
	opErrClassMissingRequiredFlag: {"error", "hint", "missing_flags"},
	opErrClassDestinationExists:   {"dest", "error", "hint"},
	opErrClassSourceFingerprintMM: {"error", "hint", "output_path"},
	opErrClassMaxFilesExceeded:    {"error", "hint", "max_files", "would_create_count"},
	opErrClassNamesCountMismatch:  {"error", "hint", "names_count", "output_count"},
	opErrClassEmptyRange:          {"error", "hint", "range_end", "range_start", "src"},
	opErrClassRangeExceedsFile:    {"error", "file_lines", "hint", "range_end", "range_start", "src"},
	opErrClassUnclosedBlock:       {"error", "hint", "src", "start_line"},
	opErrClassInvalidPattern:      {"error", "field", "hint"},
	opErrClassPatternTooLong:      {"error", "field", "hint"},
	"source_not_found":            {"error", "hint", "src"},
	"binary_source":               {"error", "hint", "src"},
	"directory_not_file":          {"error", "hint", "src"},
	"not_regular_file":            {"error", "hint", "src"},
	"no_delimiter_match":          {"error", "hint", "src"},
	"no_blocks_found":             {"error", "hint", "src"},
	"no_transform_specified":      {"error", "hint"},
	"invalid_line_endings":        {"error", "hint"},
}

func matchContractErrorVariant(payload map[string]any) error {
	errVal, err := payloadErrorSentinel(payload)
	if err != nil {
		return err
	}

	keys := sortedMapKeys(payload)

	if want, ok := contractErrorStaticKeySets[errVal]; ok {
		return expectExactKeySet(keys, want)
	}

	return matchDynamicContractErrorVariant(errVal, payload, keys)
}

func matchDynamicContractErrorVariant(errVal string, payload map[string]any, keys []string) error {
	switch errVal {
	case opErrClassUnexpectedArgument:
		if hasKey(payload, "argument") {
			return expectExactKeySet(keys, []string{"argument", "error", "hint"})
		}

		return expectExactKeySet(keys, []string{"error", "field", "hint"})
	case opErrClassInvalidInput:
		return matchInvalidInputVariant(payload, keys)
	case opErrClassControlCharsInInput:
		if hasKey(payload, "field") {
			return expectExactKeySet(keys, []string{"error", "field", "hint"})
		}

		return matchPathScopedControlOrInvalid(payload, keys)
	case opErrClassInternalError:
		return matchInternalErrorContractKeys(payload, keys)
	default:
		return fmt.Errorf("%w: %q", errOpErrJSONUnknownErrorSentinel, errVal)
	}
}

func payloadErrorSentinel(payload map[string]any) (string, error) {
	errVal, ok := payload["error"].(string)
	if !ok || strings.TrimSpace(errVal) == "" {
		return "", fmt.Errorf("%w", errOpErrJSONMissingErrorSentinel)
	}

	return errVal, nil
}

func matchInternalErrorContractKeys(payload map[string]any, keys []string) error {
	switch len(keys) {
	case internalErrorJSONKeyCountBase:
		return expectExactKeySet(keys, []string{"error", "hint"})
	case internalErrorJSONKeyCountPaths:
		if hasKey(payload, "src") {
			return expectExactKeySet(keys, []string{"error", "hint", "src"})
		}

		return expectExactKeySet(keys, []string{"error", "hint", "output_path"})
	default:
		return fmt.Errorf("%w: %v", errOpErrJSONInternalErrorKeysInvalid, keys)
	}
}

func matchInvalidInputVariant(payload map[string]any, keys []string) error {
	if len(keys) == internalErrorJSONKeyCountBase {
		return expectExactKeySet(keys, []string{"error", "hint"})
	}

	return matchPathScopedControlOrInvalid(payload, keys)
}

func matchPathScopedControlOrInvalid(payload map[string]any, keys []string) error {
	switch {
	case hasKey(payload, "src"):
		return expectExactKeySet(keys, []string{"error", "hint", "src"})
	case hasKey(payload, "dest"):
		return expectExactKeySet(keys, []string{"dest", "error", "hint"})
	case hasKey(payload, "out_dir"):
		return expectExactKeySet(keys, []string{"error", "hint", "out_dir"})
	case hasKey(payload, "output_path"):
		return expectExactKeySet(keys, []string{"error", "hint", "output_path"})
	default:
		return fmt.Errorf("%w: %v", errOpErrJSONInvalidInputKeys, keys)
	}
}

func expectExactKeySet(gotKeys, want []string) error {
	wantSorted := append([]string(nil), want...)
	sort.Strings(wantSorted)

	if len(gotKeys) != len(wantSorted) {
		return fmt.Errorf("%w: got %v want %v", errOpErrJSONKeyCountMismatch, gotKeys, wantSorted)
	}

	for i := range gotKeys {
		if gotKeys[i] != wantSorted[i] {
			return fmt.Errorf("%w: got %v want %v", errOpErrJSONKeysMismatch, gotKeys, wantSorted)
		}
	}

	return nil
}

func sortedMapKeys(payload map[string]any) []string {
	out := make([]string, 0, len(payload))
	for jsonKey := range payload {
		out = append(out, jsonKey)
	}

	sort.Strings(out)

	return out
}

func hasKey(payload map[string]any, name string) bool {
	_, ok := payload[name]

	return ok
}
