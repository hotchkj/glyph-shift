package testutil

import "github.com/hotchkj/glyph-shift/internal/pipeline"

// Test-facing aliases for [pipeline.OutputWriteIntent] so test helpers avoid OS flag constants.
const (
	OutputWriteExclusiveCreate = pipeline.OutputCreateExclusive
	OutputWriteTruncCreate     = pipeline.OutputCreateOrReplace
	OutputWriteAppendCreate    = pipeline.OutputAppend
)
