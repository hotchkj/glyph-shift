package steps

import "errors"

var (
	errExitCodeMismatch   = errors.New("exit code mismatch")
	errStdoutMismatch     = errors.New("stdout mismatch")
	errContentMismatch    = errors.New("content mismatch")
	errNoStoredSource     = errors.New("no stored content for file")
	errFileChanged        = errors.New("file changed")
	errLineCountMismatch  = errors.New("line count mismatch")
	errUnknownSource      = errors.New("unknown source file")
	errBytesMismatch      = errors.New("bytes mismatch")
	errPrefixMismatch     = errors.New("prefix mismatch")
	errSuffixMismatch     = errors.New("suffix mismatch")
	errInvalidRange       = errors.New("invalid line range")
	errUnknownTerminator  = errors.New("unknown line terminator")
	errLineOutOfRange     = errors.New("line out of range")
	errTerminatorMismatch = errors.New("line terminator mismatch")
	errFileCountMismatch  = errors.New("file count mismatch")
	errMissingFile        = errors.New("file does not exist")
	errExpectedFile       = errors.New("expected a file")
	errLineTrailingWS     = errors.New("line has trailing space or tab")
	errEmptyFileBytes     = errors.New("empty file")
	errMissingFinalNL     = errors.New("does not end with newline")
	errMultipleFinalNL    = errors.New("ends with more than one newline")

	// Godog step glue: err113 prefers static sentinels with contextual %w wrapping.
	errExpectedExitZero               = errors.New("expected exit code 0")
	errExpectedNonzeroExit            = errors.New("expected non-zero exit")
	errPathTraversalNilSegment        = errors.New("path traversal: nil value at segment")
	errPathTraversalExpectedArray     = errors.New("path traversal: expected array at segment")
	errPathTraversalIndexOutOfRange   = errors.New("path traversal: index out of range for array")
	errPathTraversalExpectedMap       = errors.New("path traversal: expected map at segment")
	errPathTraversalMissingMapKey     = errors.New("path traversal: missing key in map")
	errUnknownResultNoun              = errors.New("unknown result noun (expected lines, files, blocks, or endings)")
	errUnknownResultVerb              = errors.New("unknown verb for result noun")
	errExpectedJSONArrayAtPath        = errors.New("expected JSON array at path")
	errExpectedNumberAtPath           = errors.New("expected number at JSON path")
	errResultNounVerbCountMismatch    = errors.New("result count mismatch for noun and verb")
	errExpectedChangedTrue            = errors.New("expected changed=true")
	errExpectedChangedFalse           = errors.New("expected changed=false")
	errExpectedSkippedTrue            = errors.New("expected skipped=true")
	errUnknownFileChangeStatus        = errors.New("unknown file change status")
	errStderrErrorFieldNotString      = errors.New("expected stderr error field to be a string")
	errExpectedNumberWouldExtract     = errors.New("expected number at would_extract_lines")
	errWouldExtractLineCountMismatch  = errors.New("would_extract_lines count mismatch")
	errFileExtensionMismatch          = errors.New("output file extension mismatch")
	errStdoutWouldCreateMissing       = errors.New("missing field would_create in stdout JSON")
	errStdoutWouldCreateNotArray      = errors.New("field would_create is not an array")
	errStdoutWouldCreateNotString     = errors.New("field would_create is not a string")
	errWouldCreatePathMismatch        = errors.New("would_create path mismatch")
	errEmptyOutputFileListEntry       = errors.New("empty entry in comma-separated output file list")
	errEmptyCommaSeparatedPathSegment = errors.New("empty workspace-relative path segment in comma-separated paths")
	errNoOutputBasenames              = errors.New("no output basenames in list")
	errDirectoryShouldNotExistButDoes = errors.New("directory should not exist but does")
	errStdoutChangedMissingBool       = errors.New("missing or non-bool changed field in stdout JSON")
	errStdoutChangedExpectedFalse     = errors.New("expected changed=false, got changed=true")
	errStdoutWouldChangeMissingBool   = errors.New("missing or non-bool would_change field in stdout JSON")
	errWouldChangeExpectedTrue        = errors.New("expected would_change=true, got would_change=false")
	errWouldChangeExpectedFalse       = errors.New("expected would_change=false, got would_change=true")
	errFileShouldNotEndWithNewline    = errors.New("file should not end with a newline")
	errUnknownExtractOption           = errors.New("unknown extract option")
	errTooManyExtractOptions          = errors.New("extract: at most one with-option phrase is allowed")
	errUnknownSplitOption             = errors.New("unknown split option")
	errUnknownBlocksOption            = errors.New("unknown blocks option")
	errEscapedFixtureDecode           = errors.New("escaped fixture decode failed")

	errStdoutJSONMissingNumericField    = errors.New("stdout JSON missing numeric field")
	errStdoutJSONFieldWantNumberGotKind = errors.New("stdout JSON field must be a number")

	errTransformLFEndingStatsMismatch     = errors.New("transform stdout lf line-ending stats mismatch")
	errTransformCRLineEndingStatsMismatch = errors.New("transform stdout cr line-ending stats mismatch")
	errTransformCRLFEndingStatsMismatch   = errors.New("transform stdout crlf line-ending stats mismatch")

	errGlyphShiftErrorJSONObjectNotFound = errors.New(
		`stderr is not exactly one JSON object with non-empty string field "error"`,
	)

	errStderrExtraContentAfterJSON = errors.New(
		"stderr has non-whitespace after the JSON error object",
	)

	errGlyphShiftStderrJSONFieldContract = errors.New(
		`stderr JSON must have non-empty string "error" and valid optional string "hint"`,
	)

	errStdoutJSONExtraField         = errors.New("stdout JSON has unexpected field not in contract shape")
	errStdoutJSONMissingField       = errors.New("stdout JSON is missing required field")
	errStdoutJSONFieldTypeMismatch  = errors.New("stdout JSON field has wrong type")
	errStdoutJSONFieldValueMismatch = errors.New("stdout JSON field has wrong value")
	errStdoutJSONNotObject          = errors.New("stdout JSON is not an object")
	// Layer 2 / run helper glue (err113: static sentinels, wrap with %w for context).
	errUnknownPipelineSentinel    = errors.New("unknown pipeline error sentinel")
	errBlocksFileCountMismatch    = errors.New("file count does not match content blocks count")
	errEmptyCLIRenderLine         = errors.New("empty CLI render line")
	errCLIRenderMissingGlyphShift = errors.New("CLI render step expects leading glyph-shift")
	errExpectedMCPSuccessGotError = errors.New("expected MCP success but got isError=true")
	errMCPStructuredContentNil    = errors.New("MCP structuredContent is nil")
	errLayer1GenericCLINotAllowed = errors.New(
		"layer 1 runGlyphShiftSubcommand allows only the mcp subcommand; use direct operation runners or runGlyphShiftMocked",
	)
	errOperationErrorClassMismatch         = errors.New("operation error class mismatch")
	errMCPContentJSONNil                   = errors.New("MCP content JSON is nil")
	errUnknownOperationErrorJSONSource     = errors.New("unknown operation error JSON source")
	errOperationJSONSourceNeedsLayer1Error = errors.New(
		"operation error JSON source requires a recorded operation failure",
	)
	errOperationErrorJSONSourceAmbiguous = errors.New(
		"operation error JSON source is ambiguous; specify source explicitly",
	)
	errOperationErrorJSONAssert = errors.New("operation error JSON assertion failed")

	errMCPContentJSONTrailingNonJSON = errors.New("MCP content JSON has trailing non-JSON")
	errEmptyFieldNameList            = errors.New("empty field name list")
)
