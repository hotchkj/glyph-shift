package pipeline

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hotchkj/glyph-shift/internal/fsnorm"
)

// FormatOperationErrorJSON builds the JSON-edge object for an operation error outcome.
// Path slots are rendered as absolute native paths when workspaceRoot and the path slot exist.
// The returned map satisfies docs/glyph-shift-json-contract.md error oneOf (flat object, no nulls,
// no empty strings, no empty arrays; write_failed carries error+hint only).
//
//nolint:gocritic // hugeParam: value semantics leave the caller's ErrorOutcome unchanged
func FormatOperationErrorJSON(workspaceRoot string, outcome ErrorOutcome) (map[string]any, error) {
	if outcome.Error == "" {
		return nil, fmt.Errorf("%w", errOperationErrorEmptyClass)
	}

	staging := outcome
	normalizeInternalErrorOutcome(&staging)

	if staging.Error == opErrClassWriteFailed {
		return formatWriteFailedOperationError(staging.Hint)
	}

	return formatNonWriteOperationError(workspaceRoot, &staging)
}

func formatWriteFailedOperationError(hint string) (map[string]any, error) {
	payload := map[string]any{
		"error": opErrClassWriteFailed,
		"hint":  hint,
	}
	if err := validateFormattedOperationError(payload); err != nil {
		return nil, err
	}

	return payload, nil
}

func formatNonWriteOperationError(workspaceRoot string, staging *ErrorOutcome) (map[string]any, error) {
	if strings.TrimSpace(staging.Hint) == "" {
		return nil, fmt.Errorf("%w: class %q", errOperationErrorMissingNonEmptyHint, staging.Error)
	}

	payload := make(map[string]any)
	payload["error"] = staging.Error
	payload["hint"] = staging.Hint

	if err := mergeFormattedPathFields(workspaceRoot, staging.Error, staging, payload); err != nil {
		return nil, err
	}

	if err := mergeFormattedStringAndArrayFields(*staging, payload); err != nil {
		return nil, err
	}

	if err := mergeFormattedIntFields(*staging, payload); err != nil {
		return nil, err
	}

	postProcessInternalErrorPaths(payload)

	if err := validateFormattedOperationError(payload); err != nil {
		return nil, err
	}

	return payload, nil
}

// IsOperationOutcomeRenderableAtJSONEdge reports whether FormatOperationErrorJSON succeeds for outcome.
func IsOperationOutcomeRenderableAtJSONEdge(workspaceRoot string, outcome *ErrorOutcome) bool {
	if outcome == nil {
		return false
	}

	_, ferr := FormatOperationErrorJSON(workspaceRoot, *outcome)

	return ferr == nil
}

// OperationErrorPayload formats an operation error for transports that must always emit a JSON object.
// If formatter validation fails, the payload degrades to the documented write_failed variant while
// retaining suppressed classification diagnostics in the hint alongside the formatter error chain.
func OperationErrorPayload(workspaceRoot string, outcome *ErrorOutcome) map[string]any {
	if outcome == nil {
		fallback := WriteFailedOutcome(errOperationErrorEmptyClass)
		return operationErrorPayloadFromWriteFailed(&fallback)
	}

	payload, err := FormatOperationErrorJSON(workspaceRoot, *outcome)
	if err == nil {
		return payload
	}

	fallback := WriteFailedOutcomeFromFormattingError(err, outcome)

	return operationErrorPayloadFromWriteFailed(&fallback)
}

// WriteFailedOutcome returns the shared operation-error contract outcome for JSON write failures.
func WriteFailedOutcome(err error) ErrorOutcome {
	hint := "operation error JSON write failed"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		hint = err.Error()
	}

	return ErrorOutcome{
		Error:    opErrClassWriteFailed,
		Hint:     hint,
		ExitCode: ExitGeneral,
	}
}

func operationErrorPayloadFromWriteFailed(outcome *ErrorOutcome) map[string]any {
	return map[string]any{
		"error": outcome.Error,
		"hint":  outcome.Hint,
	}
}

func normalizeInternalErrorOutcome(outcome *ErrorOutcome) {
	if outcome == nil || outcome.Error != opErrClassInternalError {
		return
	}

	hasSrc := strings.TrimSpace(outcome.Src) != ""
	hasOut := strings.TrimSpace(outcome.OutputPath) != ""
	hasDest := strings.TrimSpace(outcome.Dest) != ""
	hasOutDir := strings.TrimSpace(outcome.OutDir) != ""

	if normalizeInternalOutputPath(outcome, hasOut) {
		return
	}

	if normalizeInternalSourcePath(outcome, hasSrc) {
		return
	}

	normalizeInternalSecondaryPath(outcome, hasDest, hasOutDir)
}

func normalizeInternalOutputPath(outcome *ErrorOutcome, hasOut bool) bool {
	if !hasOut {
		return false
	}

	// Prefer output_path; strip other path slots.
	outcome.Src = ""
	outcome.Dest = ""
	outcome.OutDir = ""

	return true
}

func normalizeInternalSourcePath(outcome *ErrorOutcome, hasSrc bool) bool {
	if !hasSrc {
		return false
	}

	outcome.Dest = ""
	outcome.OutDir = ""

	return true
}

func normalizeInternalSecondaryPath(outcome *ErrorOutcome, hasDest, hasOutDir bool) {
	if !hasDest && !hasOutDir {
		return
	}

	// Contract allows only base internal_error or src/output_path — collapse dest/out_dir into hint.
	ctx := outcome.PathContextString()
	if ctx != "" {
		outcome.Hint = ctx + ": " + outcome.Hint
	}

	outcome.Dest = ""
	outcome.OutDir = ""
}

func mergeFormattedPathFields(workspaceRoot, errClass string, outcome *ErrorOutcome, payload map[string]any) error {
	switch errClass {
	case opErrClassWriteFailed, opErrClassUnexpectedArgument,
		opErrClassUnknownCommand, opErrClassInvalidFlag, opErrClassMissingRequiredFlag,
		opErrClassMaxFilesExceeded, opErrClassNamesCountMismatch,
		opErrClassInvalidPattern, opErrClassPatternTooLong:
		// MCP decode uses field for unexpected_argument; CLI uses argument — no path keys.
		return nil
	case opErrClassDestinationExists:
		return mergeAbsolutePathField(workspaceRoot, payload, "dest", outcome.Dest)
	case opErrClassSourceFingerprintMM:
		return mergeAbsolutePathField(workspaceRoot, payload, "output_path", outcome.OutputPath)
	}

	if needsPrimarySrc(errClass) {
		return mergeAbsolutePathField(workspaceRoot, payload, "src", outcome.Src)
	}

	if isPathScopedInvalidInput(errClass, outcome) {
		return mergePathScopedInvalidInputPathSlots(workspaceRoot, outcome, payload)
	}

	if errClass == opErrClassInternalError {
		return mergeInternalErrorPathSlots(workspaceRoot, outcome, payload)
	}

	return nil
}

func mergePathScopedInvalidInputPathSlots(workspaceRoot string, outcome *ErrorOutcome, payload map[string]any) error {
	switch {
	case strings.TrimSpace(outcome.Src) != "":
		return mergeAbsolutePathField(workspaceRoot, payload, "src", outcome.Src)
	case strings.TrimSpace(outcome.Dest) != "":
		return mergeAbsolutePathField(workspaceRoot, payload, "dest", outcome.Dest)
	case strings.TrimSpace(outcome.OutDir) != "":
		return mergeAbsolutePathField(workspaceRoot, payload, "out_dir", outcome.OutDir)
	case strings.TrimSpace(outcome.OutputPath) != "":
		return mergeAbsolutePathField(workspaceRoot, payload, "output_path", outcome.OutputPath)
	default:
		return nil
	}
}

func mergeInternalErrorPathSlots(workspaceRoot string, outcome *ErrorOutcome, payload map[string]any) error {
	if strings.TrimSpace(outcome.OutputPath) != "" {
		return mergeAbsolutePathField(workspaceRoot, payload, "output_path", outcome.OutputPath)
	}

	if strings.TrimSpace(outcome.Src) != "" {
		return mergeAbsolutePathField(workspaceRoot, payload, "src", outcome.Src)
	}

	return nil
}

func needsPrimarySrc(errClass string) bool {
	switch errClass {
	case "source_not_found", "binary_source", "directory_not_file", "not_regular_file",
		"no_delimiter_match", "no_blocks_found",
		opErrClassEmptyRange, opErrClassRangeExceedsFile, opErrClassUnclosedBlock:
		return true
	default:
		return false
	}
}

func isPathScopedInvalidInput(errClass string, outcome *ErrorOutcome) bool {
	if errClass != opErrClassInvalidInput && errClass != opErrClassControlCharsInInput {
		return false
	}

	if outcome.StringFields == nil {
		return true
	}

	_, hasField := outcome.StringFields["field"]

	return !hasField
}

func postProcessInternalErrorPaths(payload map[string]any) {
	if payload == nil {
		return
	}

	errVal, _ := payload["error"].(string)
	if errVal != opErrClassInternalError {
		return
	}

	_, hasSrc := payload["src"]
	_, hasOut := payload["output_path"]
	if hasSrc && hasOut {
		delete(payload, "src")
	}
}

func mergeAbsolutePathField(workspaceRoot string, payload map[string]any, jsonKey, raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	abs, err := renderAbsoluteWorkspacePath(workspaceRoot, raw)
	if err != nil {
		return err
	}

	payload[jsonKey] = abs

	return nil
}

func renderAbsoluteWorkspacePath(workspaceRoot, raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("%w", errRenderPathEmptyPath)
	}

	root := fsnorm.DirNative(workspaceRoot)
	if root == "" || root == "." {
		return filepath.Abs(fsnorm.DirNative(raw))
	}

	prepared, err := PreparePath(raw, root)
	if err != nil {
		return "", err
	}

	return prepared, nil
}
