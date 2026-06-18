package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/mcpserver"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

var errMCPSeamsPipelineFactory = errors.New("cmd mcp seams: pipeline factory failed")

func TestDispatchCLI_nilRunner_returnsExit1(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := DispatchCLI([]string{"version"}, "1.0.0", testWorkspaceLexicalDir, &stdout, &stderr, nil)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
}

func TestWritePipelineRunnerFactoryError_writesDiagnostic(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	writePipelineRunnerFactoryError(&stderr, errMCPSeamsPipelineFactory)

	want := "pipeline runner: " + errMCPSeamsPipelineFactory.Error() + "\n"
	if got := stderr.String(); got != want {
		t.Fatalf("stderr: got %q want %q", got, want)
	}
}

func TestWritePipelineRunnerFactoryError_nilInputsAreSilent(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	writePipelineRunnerFactoryError(&stderr, nil)
	writePipelineRunnerFactoryError(nil, errMCPSeamsPipelineFactory)

	if stderr.Len() != 0 {
		t.Fatalf("stderr: want empty, got %q", stderr.String())
	}
}

func TestRunMCPServer_nilRunner_stderrPreconditionDiag(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := RunMCPServer(
		[]string{"--help"}, "1.0.0", testWorkspaceLexicalDir,
		bytes.NewReader(nil), &stdout, &stderr, MCPServerDeps{
			Runner:   nil,
			Resolver: testutil.NoSymlinkPathResolver{},
		},
	)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}

	want := mcpPreconditionFailurePrefix + " pipeline runner is nil\n"
	if got := stderr.String(); got != want {
		t.Fatalf("stderr: got %q want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout: expected empty prior to diagnostics, got %q", stdout.String())
	}
}

func TestRunMCPServer_nilPathResolver_stderrPreconditionDiag(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := RunMCPServer(
		[]string{"--help"}, "1.0.0", testWorkspaceLexicalDir,
		bytes.NewReader(nil), &stdout, &stderr, MCPServerDeps{
			Runner:   errorContractRunner{},
			Resolver: nil,
		},
	)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}

	want := mcpPreconditionFailurePrefix + " path resolver is nil\n"
	if got := stderr.String(); got != want {
		t.Fatalf("stderr: got %q want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout: expected empty prior to diagnostics, got %q", stdout.String())
	}
}

func TestRunMCPServer_nilStdin_stderrPreconditionDiag(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	deps := MCPServerDeps{Runner: errorContractRunner{}, Resolver: testutil.NoSymlinkPathResolver{}}
	code := RunMCPServer(
		[]string{"--help"}, "1.0.0", testWorkspaceLexicalDir,
		nil, &stdout, &stderr, deps,
	)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}

	want := mcpPreconditionFailurePrefix + " stdin is nil\n"
	if got := stderr.String(); got != want {
		t.Fatalf("stderr: got %q want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout: want empty prior to precondition exit, got %q", stdout.String())
	}
}

func TestRunMCPServer_nilStdout_stderrPreconditionDiag(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	deps := MCPServerDeps{Runner: errorContractRunner{}, Resolver: testutil.NoSymlinkPathResolver{}}
	code := RunMCPServer(
		[]string{"--help"}, "1.0.0", testWorkspaceLexicalDir,
		bytes.NewReader(nil), nil, &stderr, deps,
	)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}

	want := mcpPreconditionFailurePrefix + " stdout is nil\n"
	if got := stderr.String(); got != want {
		t.Fatalf("stderr: got %q want %q", got, want)
	}
}

func TestRunMCPServer_nilStderr_returnsExit1Silent(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	deps := MCPServerDeps{Runner: errorContractRunner{}, Resolver: testutil.NoSymlinkPathResolver{}}

	code := RunMCPServer(
		[]string{"--help"}, "1.0.0", testWorkspaceLexicalDir,
		bytes.NewReader(nil), &stdout, nil, deps,
	)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout: want empty prior to precondition exit (help not printed), got %q", stdout.String())
	}
}

func TestRunMCPServer_multiplePreconditions_stderrJoinsProblems(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := RunMCPServer(
		[]string{"--help"}, "1.0.0", testWorkspaceLexicalDir,
		nil, &stdout, &stderr, MCPServerDeps{
			Runner:   nil,
			Resolver: testutil.NoSymlinkPathResolver{},
		},
	)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}

	want := mcpPreconditionFailurePrefix + " stdin is nil; pipeline runner is nil\n"
	if got := stderr.String(); got != want {
		t.Fatalf("stderr: got %q want %q", got, want)
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout: want empty prior to precondition exit, got %q", stdout.String())
	}
}

func TestRunMCPServer_emptyInputExitsCleanly(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := RunMCPServer(
		nil, "1.0.0", testWorkspaceLexicalDir,
		bytes.NewReader(nil), &stdout, &stderr,
		MCPServerDeps{Runner: errorContractRunner{}, Resolver: testutil.NoSymlinkPathResolver{}},
	)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%q", code, stderr.String())
	}
}

func TestNewGlyphShiftServerFromRunner_nilRunner_returnsSentinel(t *testing.T) {
	t.Parallel()

	_, err := mcpserver.NewGlyphShiftServerFromRunner(
		testWorkspaceLexicalDir,
		"test",
		nil,
		testutil.NoSymlinkPathResolver{},
	)
	if err == nil {
		t.Fatal("expected error for nil runner")
	}
	if !errors.Is(err, mcpserver.ErrNilRunner) {
		t.Fatalf("error: got %v want %v", err, mcpserver.ErrNilRunner)
	}
}

func TestNewGlyphShiftServerFromRunner_nilPathResolver_returnsSentinel(t *testing.T) {
	t.Parallel()

	_, err := mcpserver.NewGlyphShiftServerFromRunner(
		testWorkspaceLexicalDir,
		"test",
		errorContractRunner{},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for nil path resolver")
	}
	if !errors.Is(err, mcpserver.ErrNilPathResolver) {
		t.Fatalf("error: got %v want %v", err, mcpserver.ErrNilPathResolver)
	}
}

func TestInvokeRegisteredTool_injectedPathResolverUsedByPipelineValidatePath(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	src, out := seedMCPExtractMemory(t, root)
	resolver := &recordingMCPPathResolver{err: fs.ErrPermission}
	runner := mustNewMCPExtractRunner(t, src, out, resolver)

	srv, ctorErr := mcpserver.NewGlyphShiftServerFromRunner(root, "test", runner, resolver)
	if ctorErr != nil {
		t.Fatalf("NewGlyphShiftServerFromRunner: %v", ctorErr)
	}

	result, err := srv.InvokeRegisteredTool(
		context.Background(),
		subcmdExtract,
		[]byte(`{"source":"doc.md","lines":"1-1","destination":"out.txt"}`),
	)
	if err != nil {
		t.Fatalf("InvokeRegisteredTool: %v", err)
	}
	if resolver.lstatCalls == 0 {
		t.Fatal("injected resolver was not used")
	}
	if result == nil || !result.IsError {
		t.Fatalf("result: got %#v want operation error", result)
	}
}

func TestInvokeRegisteredTool_memPathResolverMatchesMemRunner_extractSuccess(t *testing.T) {
	t.Parallel()

	root := testCmdWorkspaceRoot()
	src, out := seedMCPExtractMemory(t, root)
	resolver := testutil.NewMemPathResolverWithFS(src.Fs)
	runner := mustNewMCPExtractRunner(t, src, out, resolver)

	srv, ctorErr := mcpserver.NewGlyphShiftServerFromRunner(root, "test", runner, resolver)
	if ctorErr != nil {
		t.Fatalf("NewGlyphShiftServerFromRunner: %v", ctorErr)
	}

	result, err := srv.InvokeRegisteredTool(
		context.Background(),
		subcmdExtract,
		[]byte(`{"source":"doc.md","lines":"1-1","destination":"out.txt"}`),
	)
	if err != nil {
		t.Fatalf("InvokeRegisteredTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("result: got %#v want success", result)
	}

	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structuredContent: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode structuredContent: %v", err)
	}
	if payload["lines_extracted"] != float64(1) {
		t.Fatalf("lines_extracted: got %#v want 1", payload["lines_extracted"])
	}
}

type recordingMCPPathResolver struct {
	err        error
	lstatCalls int
}

func (r *recordingMCPPathResolver) Lstat(string) (fs.FileInfo, error) {
	r.lstatCalls++

	return nil, r.err
}

func (r *recordingMCPPathResolver) EvalSymlinks(path string) (string, error) {
	return path, nil
}

func seedMCPExtractMemory(
	t *testing.T,
	root string,
) (*testutil.MemSourceOpener, *testutil.MemOutputOpener) {
	t.Helper()

	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	if err := src.Fs.MkdirAll(root, 0o750); err != nil {
		t.Fatalf("mkdir source root: %v", err)
	}
	if err := out.Fs.MkdirAll(root, 0o750); err != nil {
		t.Fatalf("mkdir output root: %v", err)
	}
	if err := afero.WriteFile(src.Fs, filepath.Join(root, "doc.md"), []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	return src, out
}

func mustNewMCPExtractRunner(
	t *testing.T,
	src *testutil.MemSourceOpener,
	out *testutil.MemOutputOpener,
	resolver validate.PathResolver,
) pipeline.Runner {
	t.Helper()

	runner, err := pipeline.NewDefaultRunner(
		src,
		out,
		testutil.NewMemFileStater(),
		resolver,
		testutil.NewMemPublishSessionForOutput(out),
	)
	if err != nil {
		t.Fatalf("NewDefaultRunner: %v", err)
	}

	return runner
}

func mustDecodedMCPOperationEnvelope(tb testing.TB, structuredContent any) map[string]any {
	tb.Helper()

	encoded, marshalErr := json.Marshal(structuredContent)
	if marshalErr != nil {
		tb.Fatalf("marshal MCP structured payload: %v", marshalErr)
	}

	dec := json.NewDecoder(bytes.NewReader(encoded))
	dec.UseNumber()

	var payload map[string]any
	if decodeErr := dec.Decode(&payload); decodeErr != nil {
		tb.Fatalf("decode operation error envelope: %v", decodeErr)
	}

	if validateErr := pipeline.ValidateOperationErrorPayload(payload); validateErr != nil {
		tb.Fatalf("payload contract validation: %v JSON=%s", validateErr, string(encoded))
	}

	return payload
}

func mcpEnvelopeStringField(tb testing.TB, payload map[string]any, key string) string {
	tb.Helper()

	raw, ok := payload[key]
	if !ok {
		tb.Fatalf("operation envelope missing key %q (have %+v)", key, payload)
	}

	strVal, typed := raw.(string)
	if !typed {
		tb.Fatalf("%q wanted string got %T", key, raw)
	}

	return strVal
}

func TestInvokeRegisteredTool_invalidLineEndings_payloadMatchesClassification(t *testing.T) {
	t.Parallel()

	leWrapped := fmt.Errorf("%w: line-endings must be lf, crlf, or cr", pipeline.ErrInvalidLineEndings)
	wantOutcome := pipeline.ClassifyOperationError(leWrapped, "")

	root := testCmdWorkspaceRoot()
	src, out := seedMCPExtractMemory(t, root)
	resolver := testutil.NewMemPathResolverWithFS(src.Fs)
	runner := mustNewMCPExtractRunner(t, src, out, resolver)

	srv, ctorErr := mcpserver.NewGlyphShiftServerFromRunner(root, "test", runner, resolver)
	if ctorErr != nil {
		t.Fatalf("NewGlyphShiftServerFromRunner: %v", ctorErr)
	}

	result, invokeErr := srv.InvokeRegisteredTool(
		context.Background(),
		subcmdTransform,
		[]byte(`{"source":"doc.md","line_endings":"bad-target"}`),
	)
	if invokeErr != nil {
		t.Fatalf("InvokeRegisteredTool: %v", invokeErr)
	}
	if result == nil || !result.IsError {
		t.Fatalf("want IsError tool envelope, got %#v", result)
	}

	payload := mustDecodedMCPOperationEnvelope(t, result.StructuredContent)
	errStr := mcpEnvelopeStringField(t, payload, "error")
	hintStr := mcpEnvelopeStringField(t, payload, "hint")

	if errStr != wantOutcome.Error {
		t.Fatalf("error: got %q want %q", errStr, wantOutcome.Error)
	}

	if hintStr != wantOutcome.Hint {
		t.Fatalf("hint: got %q want %q", hintStr, wantOutcome.Hint)
	}
}
