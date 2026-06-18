package steps

import (
	"context"
	"fmt"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/fsnorm"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

// transformDirectInput mirrors cmd transform flags relevant to Layer 1 scenarios.
type transformDirectInput struct {
	Src          string
	Preview      bool
	LineEndings  string // "", "lf", "crlf", "cr", or invalid spellings for negative tests
	TrimTrailing bool
	FinalNewline bool
}

func parseLineEndingTargetStep(raw string) (*fileops.LineEndingTarget, error) {
	switch raw {
	case "":
		return nil, nil
	case "lf":
		t := fileops.TargetLF

		return &t, nil
	case "crlf":
		t := fileops.TargetCRLF

		return &t, nil
	case "cr":
		t := fileops.TargetCR

		return &t, nil
	default:
		return nil, fmt.Errorf("%w: line-endings must be lf, crlf, or cr", pipeline.ErrInvalidLineEndings)
	}
}

func buildTransformOptsStep(in transformDirectInput) (fileops.TransformOptions, error) {
	le, err := parseLineEndingTargetStep(in.LineEndings)
	if err != nil {
		return fileops.TransformOptions{}, err
	}

	return fileops.TransformOptions{
		LineEndings:  le,
		TrimTrailing: in.TrimTrailing,
		FinalNewline: in.FinalNewline,
	}, nil
}

func transformOptsSpecifiedStep(in transformDirectInput) bool {
	return in.LineEndings != "" || in.TrimTrailing || in.FinalNewline
}

// runTransformDirect runs the transform pipeline like cmd.runTransformExecute without populating
// tc.Stdout, tc.Stderr, or tc.ExitCode.
func runTransformDirect(tc *TestContext, in transformDirectInput) error {
	tc.resetOperationResult()

	dir := fsnorm.DirNative(tc.Ws.Root())
	filePath := fsnorm.ResolveUnderWorkspace(fsnorm.Canonical(in.Src), dir)

	opts, optErr := buildTransformOptsStep(in)
	if optErr != nil {
		tc.LastOperationError = optErr
		tc.LastOperationErrorFallbackPath = ""

		return nil
	}

	if !transformOptsSpecifiedStep(in) {
		tc.LastOperationError = pipeline.ErrNoTransformSpecified
		tc.LastOperationErrorFallbackPath = ""

		return nil
	}

	params := pipeline.TransformParams{
		FilePath: filePath,
		Root:     dir,
		Opts:     opts,
		Yes:      !in.Preview,
	}

	runner, ctorErr := newFeatureOperationRunner(tc)
	if ctorErr != nil {
		return ctorErr
	}

	out, runErr := runner.RunTransform(context.Background(), params)
	if runErr != nil {
		tc.LastOperationError = runErr
		tc.LastOperationErrorFallbackPath = filePath

		return nil
	}

	switch {
	case out.Result.Skipped && out.Result.SkipReason == "binary":
		tc.LastOperationError = pipeline.ErrBinarySource
		tc.LastOperationErrorFallbackPath = filePath

		return nil
	case out.Result.Skipped && out.Result.SkipReason == "no transform":
		tc.LastOperationError = pipeline.ErrNoTransformSpecified
		tc.LastOperationErrorFallbackPath = ""

		return nil
	case out.Result.Skipped:
		tc.LastOperationError = pipeline.ErrTransformSkippedUnknown
		tc.LastOperationErrorFallbackPath = filePath

		return nil
	}

	copyOut := out
	tc.LastTransformResult = &copyOut
	tc.LastOperationError = nil
	tc.LastOperationErrorFallbackPath = ""

	return nil
}
