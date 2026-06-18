package pipeline

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
)

func TestFormatOperationErrorJSON_destinationExistsUsesDestPath(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string([]rune{filepath.Separator}), "ws", "repo")
	relDest := filepath.Join("out", "001.md")

	preparedDest, err := PreparePath(relDest, root)
	if err != nil {
		t.Fatalf("PreparePath: %v", err)
	}

	outcome := ClassifyOperationError(ErrDestinationExists, preparedDest)

	payload, ferr := FormatOperationErrorJSON(root, outcome)
	if ferr != nil {
		t.Fatalf("FormatOperationErrorJSON: %v", ferr)
	}

	if payload["error"] != "destination_exists" {
		t.Fatalf("error: %v", payload["error"])
	}

	dest, ok := payload["dest"].(string)
	if !ok || !filepath.IsAbs(dest) {
		t.Fatalf("dest: got %v", payload["dest"])
	}

	if _, has := payload["resource"]; has {
		t.Fatalf("legacy resource key must not be emitted, got %v", payload["resource"])
	}
}

func TestFormatOperationErrorJSON_validation(t *testing.T) {
	t.Parallel()

	t.Run("rejects_empty_hint_when_required", func(t *testing.T) {
		t.Parallel()

		root := "/workspace"
		_, err := FormatOperationErrorJSON(root, ErrorOutcome{
			Error:    "max_files_exceeded",
			Hint:     "",
			ExitCode: ExitValidation,
			IntFields: map[string]int{
				"max_files":          2,
				"would_create_count": 3,
			},
		})
		if err == nil {
			t.Fatal("expected error for empty hint")
		}
	})

	t.Run("rejects_int_variant_without_int_fields", func(t *testing.T) {
		t.Parallel()

		_, err := FormatOperationErrorJSON("/workspace", ErrorOutcome{
			Error:    "names_count_mismatch",
			Hint:     "count mismatch",
			ExitCode: ExitValidation,
		})
		if err == nil {
			t.Fatal("expected error when required-int variant has no IntFields")
		}
	})
}

func TestFormatOperationErrorJSON_writeFailedVariant(t *testing.T) {
	t.Parallel()

	payload, err := FormatOperationErrorJSON("/workspace", ErrorOutcome{
		Error:    opErrClassWriteFailed,
		Hint:     "write failed",
		ExitCode: ExitGeneral,
		Src:      "ignored.txt",
	})
	if err != nil {
		t.Fatal(err)
	}

	if payload["error"] != opErrClassWriteFailed || payload["hint"] != "write failed" {
		t.Fatalf("payload = %#v", payload)
	}
	if _, has := payload["src"]; has {
		t.Fatalf("write_failed must not include path slots: %#v", payload)
	}
}

func TestFormatOperationErrorJSON_invalidInputPathSlots(t *testing.T) {
	t.Parallel()

	payload, err := FormatOperationErrorJSON("/workspace", ErrorOutcome{
		Error:    opErrClassInvalidInput,
		Hint:     "bad path",
		ExitCode: ExitValidation,
		OutDir:   "out",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, has := payload["out_dir"]; !has {
		t.Fatalf("expected out_dir path slot, got %#v", payload)
	}
}

func TestFormatOperationErrorJSON_unknownCommandUsesCommandKey(t *testing.T) {
	t.Parallel()

	payload, err := FormatOperationErrorJSON("/x", ErrorOutcome{
		Error:    "unknown_command",
		Hint:     "try help",
		ExitCode: ExitValidation,
		StringFields: map[string]string{
			"command": "nope",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if payload["command"] != "nope" {
		t.Fatalf("got %#v", payload)
	}

	// unknown_command carries command+hint only: no src/dest slots and no legacy `resource` key.
	for _, key := range []string{"src", "dest", "resource"} {
		if _, has := payload[key]; has {
			t.Fatalf("unexpected key %q", key)
		}
	}
}

func TestFormatOperationErrorJSON_preservesNonEmptyHint(t *testing.T) {
	t.Parallel()

	payload, err := FormatOperationErrorJSON(".", ErrorOutcome{
		Error:    opErrClassInternalError,
		Hint:     "specific failure detail",
		ExitCode: ExitGeneral,
	})
	if err != nil {
		t.Fatal(err)
	}

	if payload["hint"] != "specific failure detail" {
		t.Fatalf("got %#v", payload)
	}
}

func TestFormatOperationErrorJSON_internalErrorPathContextOutputPathOmitSrc(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string([]rune{filepath.Separator}), "workspace")

	wrapped := pathContextError(PathRoleOutputPath, filepath.Join("rel", "out.go"), errClassifyTestUnexpected)
	outcome := ClassifyOperationError(wrapped, "primary-src.go")

	payload, err := FormatOperationErrorJSON(root, outcome)
	if err != nil {
		t.Fatal(err)
	}

	if outcome.Error != opErrClassInternalError {
		t.Fatalf("expected internal_error, got %#v", outcome)
	}

	if _, has := payload["src"]; has {
		t.Fatalf("expected src omitted when output_path context is set, got %#v", payload["src"])
	}

	op, ok := payload["output_path"].(string)
	if !ok || !filepath.IsAbs(op) {
		t.Fatalf("output_path: got %#v", payload["output_path"])
	}
}

func TestFormatOperationErrorJSON_internalErrorCollapsesDestIntoHintOnlyKeys(t *testing.T) {
	t.Parallel()

	payload, err := FormatOperationErrorJSON("/workspace", ErrorOutcome{
		Error:    opErrClassInternalError,
		Hint:     "failed",
		ExitCode: ExitGeneral,
		Dest:     "out.txt",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, has := payload["dest"]; has {
		t.Fatalf("dest must collapse out of internal_error payload: %#v", payload)
	}
	if payload["hint"] != "out.txt: failed" {
		t.Fatalf("hint: got %#v", payload["hint"])
	}
}

func TestFormatOperationErrorJSON_internalErrorCollapsesOutDirIntoHintOnlyKeys(t *testing.T) {
	t.Parallel()

	payload, err := FormatOperationErrorJSON("/workspace", ErrorOutcome{
		Error:    opErrClassInternalError,
		Hint:     "failed",
		ExitCode: ExitGeneral,
		OutDir:   "out",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, has := payload["out_dir"]; has {
		t.Fatalf("out_dir must collapse out of internal_error payload: %#v", payload)
	}
	if payload["hint"] != "out: failed" {
		t.Fatalf("hint: got %#v", payload["hint"])
	}
}

func TestFormatOperationErrorJSON_internalErrorPrefersOutputPathClearsConflictingSlotsBeforeValidate(t *testing.T) {
	t.Parallel()

	payload, err := FormatOperationErrorJSON("/workspace", ErrorOutcome{
		Error:      opErrClassInternalError,
		Hint:       "failed",
		ExitCode:   ExitGeneral,
		Src:        "src.txt",
		Dest:       "dest.txt",
		OutDir:     "out",
		OutputPath: "generated.txt",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"src", "dest", "out_dir"} {
		if _, has := payload[key]; has {
			t.Fatalf("%s must be omitted when output_path is present: %#v", key, payload)
		}
	}
	if _, has := payload["output_path"]; !has {
		t.Fatalf("output_path missing: %#v", payload)
	}
}

func TestOperationErrorPayload_FormatFailureFallsBackToWriteFailed(t *testing.T) {
	t.Parallel()

	formatErrVictim := ErrorOutcome{
		Error:    opErrClassInternalError,
		Hint:     "",
		ExitCode: ExitGeneral,
	}

	_, formatErr := FormatOperationErrorJSON("/workspace", formatErrVictim)
	if formatErr == nil {
		t.Fatal("expected format error")
	}

	want := WriteFailedOutcomeFromFormattingError(formatErr, &formatErrVictim)

	payload := OperationErrorPayload("/workspace", &formatErrVictim)

	if payload["error"] != opErrClassWriteFailed {
		t.Fatalf("error: got %#v", payload)
	}

	gotHint := payload["hint"].(string)

	if gotHint != want.Hint {
		t.Fatalf("hint:\ngot:  %q\nwant: %q", gotHint, want.Hint)
	}

	sentencePairs := quotedPairMapFromSentence(t,
		formatClassificationDiagnosticSentence(TagOperationJSONFormatterSuppressedClassification, nil, &formatErrVictim))

	if sentencePairs["_tag"] != TagOperationJSONFormatterSuppressedClassification {
		t.Fatalf("_tag mismatch: %+v", sentencePairs)
	}

	if got := sentencePairs["error"]; got != formatErrVictim.Error {
		t.Fatalf("suppressed classification error=%q want %q", got, formatErrVictim.Error)
	}

	if err := ValidateOperationErrorPayload(payload); err != nil {
		t.Fatalf("fallback payload must satisfy contract: %v", err)
	}
}

func TestValidateOperationErrorPayloadRejectsJSONEdgeViolations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload map[string]any
		want    error
	}{
		{name: "nil_map", payload: nil, want: errOpErrJSONNilMap},
		{
			name:    "nil_value",
			payload: map[string]any{"error": opErrClassWriteFailed, "hint": nil},
			want:    errOpErrJSONKeyIsNull,
		},
		{
			name:    "empty_string",
			payload: map[string]any{"error": opErrClassWriteFailed, "hint": " "},
			want:    errOpErrJSONKeyEmptyString,
		},
		{
			name: "empty_string_slice",
			payload: map[string]any{
				"error": opErrClassMissingRequiredFlag, "hint": "h", "missing_flags": []string{},
			},
			want: errOpErrJSONKeyEmptyArray,
		},
		{
			name: "blank_string_slice_element",
			payload: map[string]any{
				"error": opErrClassMissingRequiredFlag, "hint": "h", "missing_flags": []string{"src", " "},
			},
			want: errOpErrJSONArrayStringElemEmpty,
		},
		{
			name: "empty_any_slice",
			payload: map[string]any{
				"error": opErrClassMissingRequiredFlag, "hint": "h", "missing_flags": []any{},
			},
			want: errOpErrJSONKeyEmptyArray,
		},
		{
			name: "non_string_any_slice_element",
			payload: map[string]any{
				"error": opErrClassMissingRequiredFlag, "hint": "h", "missing_flags": []any{"src", 1},
			},
			want: errOpErrJSONArrayElemNotString,
		},
		{
			name: "blank_any_slice_element",
			payload: map[string]any{
				"error": opErrClassMissingRequiredFlag, "hint": "h", "missing_flags": []any{"src", ""},
			},
			want: errOpErrJSONArrayStringElemEmpty,
		},
		{
			name:    "unsupported_type",
			payload: map[string]any{"error": opErrClassWriteFailed, "hint": true},
			want:    errOpErrJSONUnsupportedValueType,
		},
		{
			name:    "missing_error",
			payload: map[string]any{"hint": "h"},
			want:    errOpErrJSONMissingErrorSentinel,
		},
		{
			name:    "unknown_error",
			payload: map[string]any{"error": "nope", "hint": "h"},
			want:    errOpErrJSONUnknownErrorSentinel,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateOperationErrorPayload(tc.payload)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ValidateOperationErrorPayload error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestValidateOperationErrorPayloadAcceptsVariantKeyShapes(t *testing.T) {
	t.Parallel()

	cases := []map[string]any{
		{"error": opErrClassUnexpectedArgument, "hint": "h", "argument": "--bad"},
		{"error": opErrClassUnexpectedArgument, "hint": "h", "field": "src"},
		{"error": opErrClassInvalidInput, "hint": "h"},
		{"error": opErrClassInvalidInput, "hint": "h", "dest": "/tmp/out"},
		{"error": opErrClassInvalidInput, "hint": "h", "out_dir": "/tmp/out"},
		{"error": opErrClassInvalidInput, "hint": "h", "output_path": "/tmp/out"},
		{"error": opErrClassControlCharsInInput, "hint": "h", "field": "src"},
		{"error": opErrClassControlCharsInInput, "hint": "h", "src": "/tmp/in"},
		{"error": opErrClassInternalError, "hint": "h"},
		{"error": opErrClassInternalError, "hint": "h", "src": "/tmp/in"},
		{"error": opErrClassInternalError, "hint": "h", "output_path": "/tmp/out"},
		{"error": opErrClassWriteFailed, "hint": "h"},
		{"error": "missing_required_flag", "hint": "h", "missing_flags": []any{"src", "dest"}},
		{"error": opErrClassMaxFilesExceeded, "hint": "h", "max_files": json.Number("2"), "would_create_count": float64(3)},
	}

	for _, payload := range cases {
		if err := ValidateOperationErrorPayload(payload); err != nil {
			t.Fatalf("ValidateOperationErrorPayload(%#v): %v", payload, err)
		}
	}
}

func TestValidateOperationErrorPayloadRejectsVariantKeyMismatches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload map[string]any
		want    error
	}{
		{
			name:    "static_key_count",
			payload: map[string]any{"error": opErrClassWriteFailed, "hint": "h", "src": "extra"},
			want:    errOpErrJSONKeyCountMismatch,
		},
		{
			name:    "static_wrong_key",
			payload: map[string]any{"error": opErrClassUnknownCommand, "hint": "h", "src": "bad"},
			want:    errOpErrJSONKeysMismatch,
		},
		{
			name:    "invalid_input_no_path_key",
			payload: map[string]any{"error": opErrClassInvalidInput, "hint": "h", "field": "bad"},
			want:    errOpErrJSONInvalidInputKeys,
		},
		{
			name:    "internal_error_too_many_keys",
			payload: map[string]any{"error": opErrClassInternalError, "hint": "h", "src": "in", "output_path": "out"},
			want:    errOpErrJSONInternalErrorKeysInvalid,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateOperationErrorPayload(tc.payload)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ValidateOperationErrorPayload error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestFormatOperationErrorJSONIntegerVariants(t *testing.T) {
	t.Parallel()

	cases := []ErrorOutcome{
		{
			Error: opErrClassEmptyRange, Hint: "empty", ExitCode: ExitValidation, Src: "in.txt",
			IntFields: map[string]int{"range_start": 3, "range_end": 2},
		},
		{
			Error: opErrClassRangeExceedsFile, Hint: "range", ExitCode: ExitValidation, Src: "in.txt",
			IntFields: map[string]int{"file_lines": 1, "range_start": 4, "range_end": 0},
		},
		{
			Error: opErrClassUnclosedBlock, Hint: "unclosed", ExitCode: ExitValidation, Src: "in.txt",
			IntFields: map[string]int{"start_line": 5},
		},
		{
			Error: opErrClassMaxFilesExceeded, Hint: "max", ExitCode: ExitValidation,
			IntFields: map[string]int{"max_files": 2, "would_create_count": 3},
		},
		{
			Error: opErrClassNamesCountMismatch, Hint: "names", ExitCode: ExitValidation,
			IntFields: map[string]int{"names_count": 1, "output_count": 2},
		},
	}

	for _, outcome := range cases {
		payload, err := FormatOperationErrorJSON("/workspace", outcome)
		if err != nil {
			t.Fatalf("FormatOperationErrorJSON(%s): %v", outcome.Error, err)
		}
		if err := ValidateOperationErrorPayload(payload); err != nil {
			t.Fatalf("ValidateOperationErrorPayload(%s): %v", outcome.Error, err)
		}
	}
}
