package pipeline

import "errors"

// Operation error class strings from docs/glyph-shift-json-contract.md (goconst / single source of truth).
const (
	opErrClassInvalidPattern       = "invalid_pattern"
	opErrClassPatternTooLong       = "pattern_too_long"
	opErrClassControlCharsInInput  = "control_chars_in_input"
	opErrClassInvalidInput         = "invalid_input"
	opErrClassUnexpectedArgument   = "unexpected_argument"
	opErrClassMaxFilesExceeded     = "max_files_exceeded"
	opErrClassNamesCountMismatch   = "names_count_mismatch"
	opErrClassEmptyRange           = "empty_range"
	opErrClassRangeExceedsFile     = "range_exceeds_file"
	opErrClassUnclosedBlock        = "unclosed_block"
	opErrClassInternalError        = "internal_error"
	opErrClassWriteFailed          = "write_failed"
	opErrClassDestinationExists    = "destination_exists"
	opErrClassSourceFingerprintMM  = "source_fingerprint_mismatch"
	opErrClassUnknownCommand       = "unknown_command"
	opErrClassInvalidFlag          = "invalid_flag"
	opErrClassMissingRequiredFlag  = "missing_required_flag"
	internalErrorJSONKeyCountBase  = 2
	internalErrorJSONKeyCountPaths = 3
)

// Sentinel errors for FormatOperationErrorJSON and JSON contract validation (err113).
var (
	errOperationErrorEmptyClass = errors.New("operation error: empty error class")

	errOperationErrorMissingNonEmptyHint = errors.New("operation error: missing non-empty hint")

	errOperationUnexpectedArgumentArgumentAndField = errors.New(
		"operation error unexpected_argument: argument and field are mutually exclusive",
	)

	errOperationUnexpectedStringField = errors.New("operation error: unexpected string field")

	errOperationUnexpectedStringArrayContext = errors.New(
		"operation error: unexpected string array context",
	)

	errOperationMissingRequiredFlagMissingFlags = errors.New(
		"operation error missing_required_flag: missing_flags must contain at least one non-empty string",
	)

	errOperationMissingRequiredIntegerContext = errors.New(
		"operation error: missing required integer context",
	)

	errOperationUnexpectedIntegerField = errors.New("operation error: unexpected integer field")

	errOperationMissingRequiredIntegerField = errors.New(
		"operation error: missing required integer field",
	)

	errRenderPathEmptyPath = errors.New("render path: empty path")

	errOpErrJSONNilMap = errors.New("operation error JSON: nil map")

	errOpErrJSONKeyIsNull = errors.New("operation error JSON: key is null")

	errOpErrJSONUnsupportedValueType = errors.New("operation error JSON: unsupported value type")

	errOpErrJSONKeyEmptyString = errors.New("operation error JSON: key is empty string")

	errOpErrJSONKeyEmptyArray = errors.New("operation error JSON: key is empty array")

	errOpErrJSONArrayStringElemEmpty = errors.New("operation error JSON: string array element is empty")

	errOpErrJSONArrayElemNotString = errors.New("operation error JSON: array element is not string")

	errOpErrJSONMissingErrorSentinel = errors.New("operation error JSON: missing error sentinel")

	errOpErrJSONInternalErrorKeysInvalid = errors.New(
		"operation error JSON: internal_error key set invalid",
	)

	errOpErrJSONUnknownErrorSentinel = errors.New("operation error JSON: unknown error sentinel")

	errOpErrJSONInvalidInputKeys = errors.New("operation error JSON: invalid_input key set")

	errOpErrJSONKeyCountMismatch = errors.New("operation error JSON: key count mismatch")

	errOpErrJSONKeysMismatch = errors.New("operation error JSON: keys mismatch")
)
