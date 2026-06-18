package pipeline

// User-facing hint strings shared by CLI JSON stderr and MCP structured tool errors
// for the same error class keys.

const (
	HintSourceNotFound = "Check that the source file exists and is accessible."

	HintBinarySource = "Source file is binary and cannot be processed as text."

	// HintDestinationExists covers CLI (--force) and MCP (JSON force field).
	HintDestinationExists = "Use --force on the CLI or force: true in MCP JSON to overwrite, " +
		"or append when the operation supports append mode."

	HintDirectoryNotFile = "Path must point to a regular file; directories are not valid sources."

	HintNotRegularFile = "Path must point to a regular file, not a directory or device."

	// HintEmptyRange covers CLI --lines and MCP lines argument.
	HintEmptyRange = "The requested line range is empty (start after end). Adjust --lines (CLI) or the lines input (MCP)."

	HintNoBlocksFound = "The start and end patterns did not match any complete blocks."

	HintNoDelimiterMatch = "The delimiter pattern did not match any source lines."

	HintNoTransformSpecified = "specify at least one of --line-endings, --trim-trailing, or --final-newline"

	HintTransformSkippedUnknown = "transform skipped for unknown reason"
)
