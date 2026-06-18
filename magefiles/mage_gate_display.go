//go:build mage
// +build mage

package main

import (
	"fmt"
	"unicode/utf8"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/gate"
)

const (
	maxExternalToolOutputBytes = 4096
	crossCompileStepName       = "cross-compile"
)

// gateStepDisplay matches gate's displayRunner exported methods for step lines and diagnostics.
type gateStepDisplay interface {
	EmitStepStartLine(line string)
	EmitDiagnostic(diagnostic string)
}

func emitGateStepStart(runner gate.CommandRunner, title string) {
	if d, ok := runner.(gateStepDisplay); ok && title != "" {
		d.EmitStepStartLine(title + "...")
	}
}

func wrapExternalStepError(runner gate.CommandRunner, cause error, toolOutput string) error {
	if gate.RunnerOutputMode(runner) == gate.OutputModeVerbose {
		return cause
	}
	diag := cmdrunner.NewDiagnosticError(
		crossCompileStepName,
		crossCompileStepName+" failed",
		"review .goreleaser.yml and release hooks",
		"re-run with CI=1 or mage crosscompile for full goreleaser output",
		&cmdrunner.DiagnosticOptions{
			ToolOutput: truncateExternalToolOutput(toolOutput),
			Cause:      cause,
		},
	)
	if d, ok := runner.(gateStepDisplay); ok {
		d.EmitDiagnostic(diag.Error())
	}
	return diag
}

func truncateExternalToolOutput(toolOutput string) string {
	if len(toolOutput) <= maxExternalToolOutputBytes {
		return toolOutput
	}
	cut := maxExternalToolOutputBytes
	for cut > 0 && !utf8.ValidString(toolOutput[:cut]) {
		cut--
	}
	return toolOutput[:cut] + externalToolOutputTruncationSuffix(len(toolOutput))
}

func externalToolOutputTruncationSuffix(totalBytes int) string {
	return fmt.Sprintf(
		"\n... (truncated, %d bytes total — full output in verbose mode)",
		totalBytes,
	)
}
