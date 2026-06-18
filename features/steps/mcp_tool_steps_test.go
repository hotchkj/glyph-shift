package steps

import (
	"errors"
	"testing"
)

func TestPathContextFromOperationObj_destBeforeOutDir(t *testing.T) {
	t.Parallel()

	got := pathContextFromOperationObj(map[string]interface{}{
		"out_dir": "/out",
		"dest":    "/d",
	})
	if got != "/d" {
		t.Fatalf("want dest before out_dir, got %q", got)
	}
}

func TestPathContextFromOperationObj_outDirBeforeSrc(t *testing.T) {
	t.Parallel()

	got := pathContextFromOperationObj(map[string]interface{}{
		"src":     "/src",
		"out_dir": "/out",
	})
	if got != "/out" {
		t.Fatalf("want out_dir before src for split-style parity, got %q", got)
	}
}

func TestPathContextFromOperationObj_outputPathBeforeOutDir(t *testing.T) {
	t.Parallel()

	got := pathContextFromOperationObj(map[string]interface{}{
		"out_dir":     "/out",
		"output_path": "/op",
	})
	if got != "/op" {
		t.Fatalf("want output_path before out_dir, got %q", got)
	}
}

func TestOperationErrorFieldsFromMap_validContractPayload(t *testing.T) {
	t.Parallel()

	fields, err := operationErrorFieldsFromMap(map[string]interface{}{
		"error": "no_transform_specified",
		"hint":  "provide at least one transform operation",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fields.Error != "no_transform_specified" ||
		fields.Hint != "provide at least one transform operation" {
		t.Fatalf("unexpected fields: %+v", fields)
	}
}

func TestOperationErrorFieldsFromMap_rejectsNonStringHintNumber(t *testing.T) {
	t.Parallel()

	// ValidateOperationErrorPayload accepts JSON numbers generically for integer fields from some
	// transports; hint must still be a string after decode for MCP BDD parity extraction.
	_, err := operationErrorFieldsFromMap(map[string]interface{}{
		"error": "no_transform_specified",
		"hint":  float64(42),
	})
	if err == nil {
		t.Fatal("expected error for non-string hint")
	}
	if !errors.Is(err, errGlyphShiftStderrJSONFieldContract) {
		t.Fatalf("expected contract type error wrapping, got: %v", err)
	}
}
