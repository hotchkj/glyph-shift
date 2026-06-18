package pipeline

import (
	"errors"
	"testing"
)

func TestValidateOperationErrorPayload_rejectsEmptyHintString(t *testing.T) {
	t.Parallel()

	err := ValidateOperationErrorPayload(map[string]any{
		"error": "no_transform_specified",
		"hint":  "",
	})
	if err == nil {
		t.Fatal("expected error for empty hint")
	}
	if !errors.Is(err, errOpErrJSONKeyEmptyString) {
		t.Fatalf("expected errOpErrJSONKeyEmptyString, got %v", err)
	}
}

func TestValidateOperationErrorPayload_rejectsUnknownErrorClass(t *testing.T) {
	t.Parallel()

	err := ValidateOperationErrorPayload(map[string]any{
		"error": "not_a_real_sentinel_class",
		"hint":  "x",
	})
	if err == nil {
		t.Fatal("expected error for unknown sentinel")
	}
	if !errors.Is(err, errOpErrJSONUnknownErrorSentinel) {
		t.Fatalf("expected errOpErrJSONUnknownErrorSentinel, got %v", err)
	}
}
