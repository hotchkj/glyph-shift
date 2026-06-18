//go:build mage
// +build mage

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/gate"
	"github.com/hotchkj/mage-gate/gatetest"
)

const crossCompileStepLine = "Cross-compile..."

func newDisplayRunnerWithBuffers(
	tb testing.TB,
	inner gate.CommandRunner,
	mode gate.OutputMode,
) (displayOut, displayErr *bytes.Buffer, runner gate.CommandRunner) {
	tb.Helper()

	var outBuf, errBuf bytes.Buffer
	r, err := gate.NewDisplayRunner(inner, mode, &outBuf, &errBuf)
	if err != nil {
		tb.Fatalf("NewDisplayRunner: %v", err)
	}
	return &outBuf, &errBuf, r
}

func injectCrossCompileDisplayRunner(
	t *testing.T,
	inner gate.CommandRunner,
	mode gate.OutputMode,
) (displayOut, displayErr *bytes.Buffer) {
	t.Helper()

	displayOut, displayErr, runner := newDisplayRunnerWithBuffers(t, inner, mode)
	oldNewRunner := newRunner
	newRunner = func() (gate.CommandRunner, error) {
		return runner, nil
	}
	t.Cleanup(func() { newRunner = oldNewRunner })
	return displayOut, displayErr
}

func writeCrossCompileDistFixtures(t *testing.T, mem *gatetest.MemoryFileOps) {
	t.Helper()

	distDir, err := absInRoot("dist")
	if err != nil {
		t.Fatalf("absInRoot dist: %v", err)
	}
	writeDistFixture(t, mem, distDir, "artifacts.json", "artifacts.json")
	writeDistFixture(t, mem, distDir, "metadata.json", "metadata.json")
	writeDistFixture(t, mem, distDir, "checksums.txt", "checksums.txt")
}

func TestCrossCompileAgentModeEmitsStepStartOnly(t *testing.T) {
	mem := withReleaseArtifacts(t)
	writeCrossCompileDistFixtures(t, mem)
	displayOut, displayErr := injectCrossCompileDisplayRunner(t, &releaseFakeRunner{}, gate.OutputModeAgent)
	oldStore := store
	store = newArtifactStore()
	t.Cleanup(func() { store = oldStore })

	if err := CrossCompile(); err != nil {
		t.Fatalf("CrossCompile: %v", err)
	}
	if got := strings.TrimSpace(displayOut.String()); got != crossCompileStepLine {
		t.Fatalf("displayOut = %q, want %s", got, crossCompileStepLine)
	}
	if displayErr.Len() != 0 {
		t.Fatalf("displayErr = %q, want empty on success", displayErr.String())
	}
}

func TestCrossCompileAgentModeFailureDiagnostic(t *testing.T) {
	withReleaseArtifacts(t)
	displayOut, displayErr := injectCrossCompileDisplayRunner(
		t, &releaseFakeRunner{err: errReleaseRunnerFailed}, gate.OutputModeAgent,
	)

	err := CrossCompile()
	var diag *cmdrunner.DiagnosticError
	if !errors.As(err, &diag) {
		t.Fatalf("CrossCompile error = %T(%v), want *cmdrunner.DiagnosticError", err, err)
	}
	if !errors.Is(err, errReleaseRunnerFailed) {
		t.Fatalf("CrossCompile error = %v, want chain to %v", err, errReleaseRunnerFailed)
	}
	if got := strings.TrimSpace(displayOut.String()); got != crossCompileStepLine {
		t.Fatalf("displayOut = %q, want %s", got, crossCompileStepLine)
	}
	if got, want := displayErr.String(), diag.Error()+"\n"; got != want {
		t.Fatalf("expected exact diagnostic stderr display\nwant: %q\ngot:  %q", want, got)
	}
}

func TestCrossCompileVerboseModePassesThrough(t *testing.T) {
	withReleaseArtifacts(t)
	displayOut, displayErr := injectCrossCompileDisplayRunner(
		t, &releaseFakeRunner{err: errReleaseRunnerFailed}, gate.OutputModeVerbose,
	)

	err := CrossCompile()
	if !errors.Is(err, errReleaseRunnerFailed) {
		t.Fatalf("CrossCompile error = %v, want %v", err, errReleaseRunnerFailed)
	}
	var diag *cmdrunner.DiagnosticError
	if errors.As(err, &diag) {
		t.Fatalf("CrossCompile error = %v, want raw cause in verbose mode", err)
	}
	if got := strings.TrimSpace(displayOut.String()); got != crossCompileStepLine {
		t.Fatalf("displayOut = %q, want %s", got, crossCompileStepLine)
	}
	if displayErr.Len() != 0 {
		t.Fatalf("displayErr = %q, want no diagnostic in verbose mode", displayErr.String())
	}
}

func TestEmitGateStepStartNoOpWithoutDisplayRunner(t *testing.T) {
	t.Parallel()

	emitGateStepStart(&releaseFakeRunner{}, "Cross-compile")
}

func TestWrapExternalStepErrorVerboseReturnsCause(t *testing.T) {
	t.Parallel()

	inner := &releaseFakeRunner{}
	displayOut, displayErr, runner := newDisplayRunnerWithBuffers(t, inner, gate.OutputModeVerbose)
	cause := errReleaseRunnerFailed

	err := wrapExternalStepError(runner, cause, "tool output")
	if !errors.Is(err, cause) {
		t.Fatalf("wrapExternalStepError = %v, want %v", err, cause)
	}
	if displayOut.Len() != 0 || displayErr.Len() != 0 {
		t.Fatalf("verbose wrap should not emit diagnostics")
	}
}

func TestWrapExternalStepErrorAgentEmitsDiagnostic(t *testing.T) {
	t.Parallel()

	inner := &releaseFakeRunner{}
	_, displayErr, runner := newDisplayRunnerWithBuffers(t, inner, gate.OutputModeAgent)
	cause := errReleaseRunnerFailed

	err := wrapExternalStepError(runner, cause, "goreleaser stderr")
	var diag *cmdrunner.DiagnosticError
	if !errors.As(err, &diag) {
		t.Fatalf("wrapExternalStepError = %T(%v), want *cmdrunner.DiagnosticError", err, err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("wrapExternalStepError = %v, want chain to %v", err, cause)
	}
	if got, want := displayErr.String(), diag.Error()+"\n"; got != want {
		t.Fatalf("expected exact diagnostic stderr display\nwant: %q\ngot:  %q", want, got)
	}
	if diag.ToolOutput() != "goreleaser stderr" {
		t.Fatalf("diag.ToolOutput() = %q, want goreleaser stderr", diag.ToolOutput())
	}
}

func TestEmitGateStepStartWritesStepLine(t *testing.T) {
	t.Parallel()

	inner := &releaseFakeRunner{}
	displayOut, _, runner := newDisplayRunnerWithBuffers(t, inner, gate.OutputModeAgent)

	emitGateStepStart(runner, "Cross-compile")
	if got := strings.TrimSpace(displayOut.String()); got != crossCompileStepLine {
		t.Fatalf("displayOut = %q, want %s", got, crossCompileStepLine)
	}
}

func TestEmitGateStepStartSkipsEmptyTitle(t *testing.T) {
	t.Parallel()

	inner := &releaseFakeRunner{}
	displayOut, _, runner := newDisplayRunnerWithBuffers(t, inner, gate.OutputModeAgent)

	emitGateStepStart(runner, "")
	if displayOut.Len() != 0 {
		t.Fatalf("displayOut = %q, want empty for blank title", displayOut.String())
	}
}

type releaseFakeRunnerWithOutput struct {
	err    error
	stdout string
	stderr string
}

func (r *releaseFakeRunnerWithOutput) Run(
	_ context.Context,
	_ string,
	stdout, stderr io.Writer,
	_ string,
	_ ...string,
) error {
	if r.stdout != "" {
		_, _ = io.WriteString(stdout, r.stdout)
	}
	if r.stderr != "" {
		_, _ = io.WriteString(stderr, r.stderr)
	}
	return r.err
}

func TestCrossCompileAgentModeCapturesToolOutputOnFailure(t *testing.T) {
	withReleaseArtifacts(t)
	inner := &releaseFakeRunnerWithOutput{
		err:    errReleaseRunnerFailed,
		stderr: "goreleaser: build failed",
	}
	injectCrossCompileDisplayRunner(t, inner, gate.OutputModeAgent)

	err := CrossCompile()
	var diag *cmdrunner.DiagnosticError
	if !errors.As(err, &diag) {
		t.Fatalf("CrossCompile error = %T(%v), want *cmdrunner.DiagnosticError", err, err)
	}
	if diag.ToolOutput() != "goreleaser: build failed" {
		t.Fatalf("diag.ToolOutput() = %q, want goreleaser: build failed", diag.ToolOutput())
	}
	if diag.Name() != "cross-compile" {
		t.Fatalf("diag.Name() = %q, want cross-compile", diag.Name())
	}
}

func TestCrossCompileSurfacesGoreleaserError(t *testing.T) {
	withReleaseArtifacts(t)
	inner := &releaseFakeRunner{err: errReleaseRunnerFailed}
	oldNewRunner := newRunner
	newRunner = func() (gate.CommandRunner, error) {
		return mustNewDisplayRunner(t, inner), nil
	}
	t.Cleanup(func() { newRunner = oldNewRunner })

	err := CrossCompile()
	var diag *cmdrunner.DiagnosticError
	if !errors.As(err, &diag) {
		t.Fatalf("CrossCompile error = %T(%v), want *cmdrunner.DiagnosticError", err, err)
	}
	if !errors.Is(err, errReleaseRunnerFailed) {
		t.Fatalf("CrossCompile error = %v, want chain to %v", err, errReleaseRunnerFailed)
	}
	if diag.Name() != "cross-compile" {
		t.Fatalf("diag.Name() = %q, want cross-compile", diag.Name())
	}
}

func TestTruncateExternalToolOutputReturnsShortInputUnchanged(t *testing.T) {
	t.Parallel()

	const input = "short goreleaser log"
	if got := truncateExternalToolOutput(input); got != input {
		t.Fatalf("truncateExternalToolOutput() = %q, want %q", got, input)
	}
}

func TestTruncateExternalToolOutputTruncatesLongInput(t *testing.T) {
	t.Parallel()

	input := strings.Repeat("x", maxExternalToolOutputBytes+100)
	got := truncateExternalToolOutput(input)
	wantSuffix := externalToolOutputTruncationSuffix(len(input))
	if len(got) < len(wantSuffix) || got[len(got)-len(wantSuffix):] != wantSuffix {
		t.Fatalf("truncated output = %q, want suffix %q", got, wantSuffix)
	}
}

func TestWrapExternalStepErrorTruncatesLongToolOutput(t *testing.T) {
	t.Parallel()

	inner := &releaseFakeRunner{}
	_, displayErr, runner := newDisplayRunnerWithBuffers(t, inner, gate.OutputModeAgent)
	longOutput := strings.Repeat("e", maxExternalToolOutputBytes+50)

	err := wrapExternalStepError(runner, errReleaseRunnerFailed, longOutput)
	var diag *cmdrunner.DiagnosticError
	if !errors.As(err, &diag) {
		t.Fatalf("wrapExternalStepError = %T(%v), want *cmdrunner.DiagnosticError", err, err)
	}
	wantToolOutput := truncateExternalToolOutput(longOutput)
	if diag.ToolOutput() != wantToolOutput {
		t.Fatalf("diag.ToolOutput() = %q, want %q", diag.ToolOutput(), wantToolOutput)
	}
	if displayErr.String() != diag.Error()+"\n" {
		t.Fatalf("displayErr = %q, want %q", displayErr.String(), diag.Error()+"\n")
	}
}
