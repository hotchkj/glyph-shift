package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

type binarySkippedTransformRunner struct {
	errorContractRunner
}

func (binarySkippedTransformRunner) RunTransform(
	_ context.Context,
	_ pipeline.TransformParams,
) (pipeline.TransformPipelineResult, error) {
	return pipeline.TransformPipelineResult{
		Result: fileops.TransformFileResult{
			Skipped:    true,
			SkipReason: "binary",
		},
	}, nil
}

type noTransformSkippedRunner struct {
	errorContractRunner
}

func (noTransformSkippedRunner) RunTransform(
	_ context.Context,
	_ pipeline.TransformParams,
) (pipeline.TransformPipelineResult, error) {
	return pipeline.TransformPipelineResult{
		Result: fileops.TransformFileResult{
			Skipped:    true,
			SkipReason: "no transform",
		},
	}, nil
}

type unknownSkippedTransformRunner struct {
	errorContractRunner
}

func (unknownSkippedTransformRunner) RunTransform(
	_ context.Context,
	_ pipeline.TransformParams,
) (pipeline.TransformPipelineResult, error) {
	return pipeline.TransformPipelineResult{
		Result: fileops.TransformFileResult{
			Skipped:    true,
			SkipReason: "unit-test-unknown-skip-reason",
		},
	}, nil
}

func TestDispatchCLI_transformInvalidLineEndings_stderrMatchesClassifiedOutcome(t *testing.T) {
	t.Parallel()

	_, errOpts := buildTransformOpts(&transformFlagValues{lineEndings: "bogus"})
	if errOpts == nil {
		t.Fatal("expected invalid line-endings error from buildTransformOpts")
	}

	want := pipeline.ClassifyOperationError(errOpts, "")

	var stdout, stderr bytes.Buffer
	code := DispatchCLI(
		[]string{"transform", "--source", "doc.md", "--line-endings", "bogus"},
		"1.0.0",
		testWorkspaceLexicalDir,
		&stdout,
		&stderr,
		errorContractRunner{},
	)
	if code != want.ExitCode {
		t.Fatalf("exit code: got %d want %d", code, want.ExitCode)
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout want empty got %q", stdout.String())
	}

	var got errorJSONOutput
	if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
		t.Fatalf("stderr json: %v body=%q", err, stderr.String())
	}

	if got.Error != want.Error {
		t.Fatalf("error: got %q want %q", got.Error, want.Error)
	}
	if got.Hint != want.Hint {
		t.Fatalf("hint: got %q want %q", got.Hint, want.Hint)
	}
}

func TestTransformRunErrorExitClassifiesPipelineError(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	code := transformRunErrorExit(pipeline.ErrNoTransformSpecified, "src.txt", testWorkspaceLexicalDir, &stderr)
	if code != exitValidation {
		t.Fatalf("exit code = %d, want %d", code, exitValidation)
	}
	var got errorJSONOutput
	if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
		t.Fatalf("stderr json: %v body=%q", err, stderr.String())
	}
	if got.Error != "no_transform_specified" {
		t.Fatalf("error = %q, want no_transform_specified", got.Error)
	}
}

func TestParseLineEndingTargetRecognizedValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want *fileops.LineEndingTarget
	}{
		{name: "empty", raw: "", want: nil},
		{name: "lf", raw: "lf", want: ptrLineEndingTarget(fileops.TargetLF)},
		{name: "crlf", raw: "crlf", want: ptrLineEndingTarget(fileops.TargetCRLF)},
		{name: "cr", raw: "cr", want: ptrLineEndingTarget(fileops.TargetCR)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseLineEndingTarget(tc.raw)
			if err != nil {
				t.Fatalf("parseLineEndingTarget(%q): %v", tc.raw, err)
			}
			if tc.want == nil {
				if got != nil {
					t.Fatalf("parseLineEndingTarget(%q) = %v, want nil", tc.raw, *got)
				}

				return
			}
			if got == nil || *got != *tc.want {
				t.Fatalf("parseLineEndingTarget(%q) = %v, want %v", tc.raw, got, *tc.want)
			}
		})
	}
}

func ptrLineEndingTarget(target fileops.LineEndingTarget) *fileops.LineEndingTarget {
	return &target
}

func classifyTransformSkipOutcome(t *testing.T, sentinel error, workspace, relSrc string) pipeline.ErrorOutcome {
	t.Helper()

	prepared, err := pipeline.PreparePath(relSrc, workspace)
	if err != nil {
		t.Fatalf("PreparePath: %v", err)
	}

	return pipeline.ClassifyOperationError(sentinel, prepared)
}

func TestDispatchCLI_transformSkippedBinary_stderrMatchesClassification(t *testing.T) {
	t.Parallel()

	runner := binarySkippedTransformRunner{}
	want := classifyTransformSkipOutcome(t, pipeline.ErrBinarySource, testWorkspaceLexicalDir, "doc.md")

	var stdout, stderr bytes.Buffer
	code := DispatchCLI(
		[]string{"transform", "--source", "doc.md", "--line-endings", "lf"},
		"1.0.0",
		testWorkspaceLexicalDir,
		&stdout,
		&stderr,
		runner,
	)
	if code != want.ExitCode {
		t.Fatalf("exit code: got %d want %d", code, want.ExitCode)
	}

	var got errorJSONOutput
	if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
		t.Fatalf("stderr json: %v body=%q", err, stderr.String())
	}

	if got.Error != want.Error || got.Hint != want.Hint || got.Src != want.Src {
		t.Fatalf(
			"stderr JSON mismatch: got error=%q hint=%q src=%q want error=%q hint=%q src=%q",
			got.Error,
			got.Hint,
			got.Src,
			want.Error,
			want.Hint,
			want.Src,
		)
	}
}

func TestDispatchCLI_transformSkippedNoTransform_stderrMatchesClassification(t *testing.T) {
	t.Parallel()

	runner := noTransformSkippedRunner{}
	want := classifyTransformSkipOutcome(t, pipeline.ErrNoTransformSpecified, testWorkspaceLexicalDir, "doc.md")

	var stdout, stderr bytes.Buffer
	code := DispatchCLI(
		[]string{"transform", "--source", "doc.md", "--line-endings", "lf"},
		"1.0.0",
		testWorkspaceLexicalDir,
		&stdout,
		&stderr,
		runner,
	)
	if code != want.ExitCode {
		t.Fatalf("exit code: got %d want %d", code, want.ExitCode)
	}

	var got errorJSONOutput
	if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
		t.Fatalf("stderr json: %v body=%q", err, stderr.String())
	}

	if got.Error != want.Error || got.Hint != want.Hint || got.Src != want.Src {
		t.Fatalf(
			"stderr JSON mismatch: got error=%q hint=%q src=%q want error=%q hint=%q src=%q",
			got.Error,
			got.Hint,
			got.Src,
			want.Error,
			want.Hint,
			want.Src,
		)
	}
}

func TestDispatchCLI_transformSkippedUnknownReason_stderrMatchesClassification(t *testing.T) {
	t.Parallel()

	runner := unknownSkippedTransformRunner{}
	want := classifyTransformSkipOutcome(t, pipeline.ErrTransformSkippedUnknown, testWorkspaceLexicalDir, "doc.md")

	var stdout, stderr bytes.Buffer
	code := DispatchCLI(
		[]string{"transform", "--source", "doc.md", "--line-endings", "lf"},
		"1.0.0",
		testWorkspaceLexicalDir,
		&stdout,
		&stderr,
		runner,
	)
	if code != want.ExitCode {
		t.Fatalf("exit code: got %d want %d", code, want.ExitCode)
	}

	var got errorJSONOutput
	if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
		t.Fatalf("stderr json: %v body=%q", err, stderr.String())
	}

	if got.Error != want.Error || got.Hint != want.Hint || got.Src != want.Src {
		t.Fatalf(
			"stderr JSON mismatch: got error=%q hint=%q src=%q want error=%q hint=%q src=%q",
			got.Error,
			got.Hint,
			got.Src,
			want.Error,
			want.Hint,
			want.Src,
		)
	}
}
