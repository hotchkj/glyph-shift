package pipeline

// OutputWriteIntent describes how a destination file should be opened for writing.
// Only production [OutputOpener] implementations map intents to OS-specific flags.
type OutputWriteIntent int

const (
	// OutputCreateExclusive fails when the logical destination already exists.
	OutputCreateExclusive OutputWriteIntent = iota
	// OutputCreateOrReplace truncates an existing destination or creates a new file.
	OutputCreateOrReplace
	// OutputAppend seeds from existing content when present.
	OutputAppend
)
