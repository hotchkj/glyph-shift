package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

var errContractWriteFailed = errors.New("contract write failed")

// Stub paths only; no filesystem access (depguard: no os in unit tests).
const errorContractStubBase = "/glyph-shift_error_contract_stub"

func errorContractStubPath(name string) string {
	return filepath.Join(filepath.FromSlash(errorContractStubBase), name)
}

type errorContractRunner struct{}

func (errorContractRunner) RunExtract(context.Context, pipeline.ExtractParams) (fileops.ExtractResult, error) {
	return fileops.ExtractResult{LinesExtracted: 1}, nil
}

func (errorContractRunner) RunSplit(context.Context, pipeline.SplitParams) (pipeline.SplitPipelineResult, error) {
	return pipeline.SplitPipelineResult{Files: []string{errorContractStubPath("001.md")}}, nil
}

func (errorContractRunner) RunBlocks(context.Context, pipeline.BlocksParams) (pipeline.BlocksPipelineResult, error) {
	stub001 := errorContractStubPath("001.md")

	return pipeline.BlocksPipelineResult{
		Blocks:      []fileops.Block{{Name: stub001}},
		BlocksFound: 1,
		Files:       []string{stub001},
	}, nil
}

func (errorContractRunner) RunTransform(
	context.Context,
	pipeline.TransformParams,
) (pipeline.TransformPipelineResult, error) {
	return pipeline.TransformPipelineResult{
		Result: fileops.TransformFileResult{
			WouldChange:     true,
			EndingsChanged:  1,
			LFFound:         1,
			LFConverted:     1,
			CRFound:         0,
			CRConverted:     0,
			CRLFFound:       0,
			CRLFConverted:   0,
			TrailingTrimmed: 0,
		},
		ChangeCount: 1,
	}, nil
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errContractWriteFailed
}

func decodeErrorContract(t *testing.T, stderr *bytes.Buffer) errorJSONOutput {
	t.Helper()

	var got errorJSONOutput
	if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
		t.Fatalf("decode stderr JSON: %v\nstderr=%q", err, stderr.String())
	}

	return got
}

func assertErrorContract(
	t *testing.T,
	stdout *bytes.Buffer,
	stderr *bytes.Buffer,
	wantError string,
	wantHint string,
) {
	t.Helper()

	if stdout != nil && stdout.Len() != 0 {
		t.Fatalf("stdout: want empty, got %q", stdout.String())
	}

	got := decodeErrorContract(t, stderr)
	if got.Error != wantError {
		t.Fatalf("error: got %q want %s", got.Error, wantError)
	}
	if got.Src != "" || got.Dest != "" || got.OutputPath != "" {
		t.Fatalf("path context: want empty, got src=%q dest=%q output_path=%q", got.Src, got.Dest, got.OutputPath)
	}
	if got.Hint != wantHint {
		t.Fatalf("hint: got %q want %q", got.Hint, wantHint)
	}
}

func TestDispatchCLI_missingRequiredFlagSentinelExitCodeTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		args     []string
		wantHint string
	}{
		{subcmdExtract, []string{subcmdExtract}, "--source, --lines, and --destination are required"},
		{subcmdSplit, []string{subcmdSplit}, "--source, --delimiter, and --output-dir are required"},
		{"blocks", []string{"blocks"}, "--source, --start-line, --end-line, and --output-dir are required"},
		{subcmdTransform, []string{subcmdTransform}, "--source is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := DispatchCLI(tc.args, "1.0.0", testWorkspaceLexicalDir, &stdout, &stderr, errorContractRunner{})
			if code != exitValidation {
				t.Fatalf("exit code: got %d want %d", code, exitValidation)
			}

			assertErrorContract(t, &stdout, &stderr, "missing_required_flag", tc.wantHint)
		})
	}
}

func TestDispatchCLI_writeFailedSentinelExitCodeTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{subcmdExtract, []string{subcmdExtract, "--source", "doc.md", "--lines", "1-1", "--destination", "out.md"}},
		{"split", []string{"split", "--source", "doc.md", "--delimiter", "^##", "--output-dir", "out"}},
		{
			"blocks",
			[]string{"blocks", "--source", "doc.md", "--start-line", "^```", "--end-line", "^```$", "--output-dir", "out"},
		},
		{subcmdTransform, []string{subcmdTransform, "--source", "doc.md", "--line-endings", "lf"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			code := DispatchCLI(tc.args, "1.0.0", testWorkspaceLexicalDir, failingWriter{}, &stderr, errorContractRunner{})
			if code != 1 {
				t.Fatalf("exit code: got %d want 1", code)
			}

			assertErrorContract(t, nil, &stderr, "write_failed", errContractWriteFailed.Error())
		})
	}
}
