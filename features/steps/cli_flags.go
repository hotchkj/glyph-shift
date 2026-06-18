package steps

// Shared Godog option strings and CLI flags (goconst: occurrences across when_* helpers).
const (
	stepOptOverwrite         = "overwrite"
	stepOptCreateDirectories = "create directories"
	stepFlagSource           = "--source"
	stepFlagLines            = "--lines"
	stepFlagDestination      = "--destination"
	stepFlagDelimiter        = "--delimiter"
	stepFlagOutputDir        = "--output-dir"
	stepFlagStartLine        = "--start-line"
	stepFlagEndLine          = "--end-line"
	stepFlagExtension        = "--extension"
	stepFlagLineEndings      = "--line-endings"
	stepFlagTrimTrailing     = "--trim-trailing"
	stepFlagFinalNewline     = "--final-newline"
	stepFlagPreview          = "--preview"
	stepFlagForce            = "--force"
	stepFlagMkdir            = "--mkdir"
)
