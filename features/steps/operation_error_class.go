package steps

import (
	"fmt"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

// ErrorMatchesOperationErrorClass reports whether err is classified as expectedClass using
// pipeline.ClassifyOperationError. That classifier is the single source of truth shared with
// cmd/pipeline_exit.go and MCP tooling: it applies errors.Is against pipeline, fileops, validate,
// and linparse sentinels (see internal/pipeline/error_contract.go and docs/glyph-shift-json-contract.md).
//
// Layer 1 Then steps pass LastOperationErrorFallbackPath (set by extract/split/blocks/
// transform direct helpers to mirror cmd exitCodeForPipelineErr's primary-path argument, and ""
// for line-range parse failures and similar cases) into AssertOperationErrorClass so class
// and path-field alignment match CLI stderr JSON.
//
// Supported Layer 1 BDD class strings include (non-exhaustive for the overall contract):
// source_not_found, binary_source, destination_exists, empty_range, range_exceeds_file,
// invalid_input, plus any other class name ClassifyOperationError may produce.
func ErrorMatchesOperationErrorClass(err error, expectedClass string) bool {
	if err == nil || expectedClass == "" {
		return false
	}

	outcome := pipeline.ClassifyOperationError(err, "")

	return outcome.Error == expectedClass
}

// AssertOperationErrorClass returns nil when err classifies as expectedClass; otherwise it
// returns an error wrapping errOperationErrorClassMismatch suitable for godog steps.
// fallbackPath is passed to pipeline.ClassifyOperationError (same role as cmd stderr JSON primary path).
func AssertOperationErrorClass(err error, expectedClass, fallbackPath string) error {
	switch {
	case expectedClass == "":
		return fmt.Errorf("%w: expected class is empty", errOperationErrorClassMismatch)
	case err == nil:
		return fmt.Errorf("%w: expected error class %q but error is nil", errOperationErrorClassMismatch, expectedClass)
	}

	got := pipeline.ClassifyOperationError(err, fallbackPath).Error
	if got != expectedClass {
		return fmt.Errorf("%w: expected error class %q, got %q: %w",
			errOperationErrorClassMismatch, expectedClass, got, err)
	}

	return nil
}
