package cmd

import (
	"io"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

const (
	exitSourceNotFound = pipeline.ExitSourceNotFound
	exitBinarySource   = pipeline.ExitBinarySource
	exitDestExists     = pipeline.ExitDestExists
	exitNotRegularFile = pipeline.ExitNotRegularFile
	exitValidation     = pipeline.ExitValidation
)

// exitCodeForPipelineErr maps pipeline sentinel errors to intent-spec exit codes,
// writing a JSON error object to stderr.
func exitCodeForPipelineErr(err error, src, workspaceRoot string, stderr io.Writer) int {
	outcome := pipeline.ClassifyOperationError(err, src)
	writeErrorJSON(stderr, workspaceRoot, &outcome)

	return outcome.ExitCode
}
