// Error JSON field merging keeps operation-error payload variants flat and schema-specific.
package pipeline

import (
	"fmt"
	"strings"
)

//nolint:gocritic // hugeParam: merge stage uses a copy so the source ErrorOutcome is not aliased into the map build
func mergeFormattedStringAndArrayFields(outcome ErrorOutcome, payload map[string]any) error {
	if err := mergeAllowedStringFieldsIntoMap(outcome, payload); err != nil {
		return err
	}

	if outcome.Error == opErrClassUnexpectedArgument && payload["argument"] != nil && payload["field"] != nil {
		return fmt.Errorf("%w", errOperationUnexpectedArgumentArgumentAndField)
	}

	return mergeStringArrayFieldsForErrorClass(outcome, payload)
}

//nolint:gocritic // hugeParam: same as mergeFormattedStringAndArrayFields
func mergeAllowedStringFieldsIntoMap(outcome ErrorOutcome, payload map[string]any) error {
	allowed := stringFieldKeysForError(outcome.Error)
	if outcome.StringFields == nil {
		return nil
	}

	for fieldKey, val := range outcome.StringFields {
		if strings.TrimSpace(val) == "" {
			continue
		}

		if !allowed[fieldKey] {
			return fmt.Errorf("%w: class %q field %q", errOperationUnexpectedStringField, outcome.Error, fieldKey)
		}

		payload[fieldKey] = val
	}

	return nil
}

//nolint:gocritic // hugeParam: same as mergeFormattedStringAndArrayFields
func mergeStringArrayFieldsForErrorClass(outcome ErrorOutcome, payload map[string]any) error {
	switch outcome.Error {
	case opErrClassMissingRequiredFlag:
		return mergeMissingRequiredFlagMissingFlags(outcome, payload)
	default:
		if len(outcome.StringArrayFields) > 0 {
			return fmt.Errorf("%w: class %q", errOperationUnexpectedStringArrayContext, outcome.Error)
		}

		return nil
	}
}

//nolint:gocritic // hugeParam: same as mergeFormattedStringAndArrayFields
func mergeMissingRequiredFlagMissingFlags(outcome ErrorOutcome, payload map[string]any) error {
	mf := outcome.StringArrayFields["missing_flags"]
	out := make([]string, 0, len(mf))
	for _, s := range mf {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}

	if len(out) == 0 {
		return fmt.Errorf("%w", errOperationMissingRequiredFlagMissingFlags)
	}

	payload["missing_flags"] = out

	return nil
}

func stringFieldKeysForError(errClass string) map[string]bool {
	switch errClass {
	case opErrClassUnknownCommand:
		return map[string]bool{"command": true}
	case opErrClassUnexpectedArgument:
		// CLI (argument) and MCP decode (field) use mutually exclusive outcome fields.
		return map[string]bool{"argument": true, "field": true}
	case opErrClassInvalidFlag:
		return map[string]bool{"flag": true}
	case opErrClassInvalidPattern, opErrClassPatternTooLong, opErrClassControlCharsInInput:
		return map[string]bool{"field": true}
	default:
		return map[string]bool{}
	}
}

//nolint:gocritic // hugeParam: same as mergeFormattedStringAndArrayFields
func mergeFormattedIntFields(outcome ErrorOutcome, payload map[string]any) error {
	if len(outcome.IntFields) == 0 {
		switch outcome.Error {
		case opErrClassEmptyRange, opErrClassRangeExceedsFile, opErrClassUnclosedBlock,
			opErrClassMaxFilesExceeded, opErrClassNamesCountMismatch:
			return fmt.Errorf("%w: class %q", errOperationMissingRequiredIntegerContext, outcome.Error)
		default:
			return nil
		}
	}

	allowed := intFieldKeysForError(outcome.Error)
	for key, val := range outcome.IntFields {
		if !allowed[key] {
			return fmt.Errorf("%w: class %q field %q", errOperationUnexpectedIntegerField, outcome.Error, key)
		}

		payload[key] = val
	}

	for key := range allowed {
		if _, ok := payload[key]; !ok {
			return fmt.Errorf("%w: class %q field %q", errOperationMissingRequiredIntegerField, outcome.Error, key)
		}
	}

	return nil
}

func intFieldKeysForError(errClass string) map[string]bool {
	switch errClass {
	case opErrClassEmptyRange:
		return map[string]bool{"range_start": true, "range_end": true}
	case opErrClassRangeExceedsFile:
		return map[string]bool{"file_lines": true, "range_start": true, "range_end": true}
	case opErrClassUnclosedBlock:
		return map[string]bool{"start_line": true}
	case opErrClassMaxFilesExceeded:
		return map[string]bool{"max_files": true, "would_create_count": true}
	case opErrClassNamesCountMismatch:
		return map[string]bool{"names_count": true, "output_count": true}
	default:
		return map[string]bool{}
	}
}
