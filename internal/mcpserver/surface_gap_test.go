package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

func TestInvokeRegisteredTool_unknownToolName_returnsError(t *testing.T) {
	t.Parallel()

	srv := mustNewGlyphShiftServerFromRunner(
		t, testWorkspaceRoot(), "surf", constructorRunner{}, &constructorPathResolver{},
	)

	_, err := srv.InvokeRegisteredTool(context.Background(), "not-a-registered-tool", []byte("{}"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func setupBlocksInvariantRunner(t *testing.T) *pipeline.DefaultRunner {
	t.Helper()

	root := testWorkspaceRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	st := testutil.NewMemFileStater()
	session := testutil.NewMemPublishSessionForOutput(out)

	runnerInner := mustNewDefaultRunner(t, src, out, st, session)
	srv := mustNewGlyphShiftServer(t, root, src, out, st, session)

	srcPath, err := srv.validateToolPath("in.txt")
	if err != nil {
		t.Fatalf("validate source path: %v", err)
	}
	if wf := afero.WriteFile(src.Fs, srcPath, []byte("plain\ntext\n"), 0o600); wf != nil {
		t.Fatalf("write source: %v", wf)
	}

	outDir, errOut := srv.validateToolPath("out")
	if errOut != nil {
		t.Fatalf("validate out path: %v", errOut)
	}
	if mk := out.Fs.MkdirAll(outDir, 0o700); mk != nil {
		t.Fatalf("mkdir out: %v", mk)
	}

	return runnerInner
}

type blocksInvariantRunner struct {
	*pipeline.DefaultRunner
}

func (blocksInvariantRunner) RunBlocks(
	context.Context,
	pipeline.BlocksParams,
) (pipeline.BlocksPipelineResult, error) {
	return pipeline.BlocksPipelineResult{
		BlocksFound: 0,
		Files:       []string{filepath.Join("out", "only.txt")},
	}, nil
}

func TestBlocksTool_handlesBlocksOutputInvariantWithInvalidInputStructuredError(t *testing.T) {
	t.Parallel()

	runnerInner := setupBlocksInvariantRunner(t)
	surfRunner := blocksInvariantRunner{DefaultRunner: runnerInner}
	harness := mustNewGlyphShiftServerFromRunner(
		t, testWorkspaceRoot(), "t-surf", surfRunner, testutil.NoSymlinkPathResolver{},
	)

	result, _, err := harness.handleBlocksTool(context.Background(), nil, BlocksInput{
		Source:    "in.txt",
		StartLine: "^plain$",
		EndLine:   "^text$",
		OutputDir: "out",
	})
	if err != nil {
		t.Fatalf("handleBlocksTool: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected tool error envelope, got %+v", result)
	}

	mustValidateStructuredContentAgainstSchema(t, toolBlocks, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != invalidInputErrorName {
		t.Fatalf(
			"error: got %q want invalid_input",
			opErrMustString(t, payload, "error"),
		)
	}
	if opErrMustString(t, payload, "hint") != errBlocksOutputInvariantBroken.Error() {
		t.Fatalf("hint: got %q want invariant message", opErrMustString(t, payload, "hint"))
	}
}

func TestTransformTool_invalidLineEndingsMatchesInvalidLineEndingsClassification(t *testing.T) {
	t.Parallel()

	leWrapped := fmt.Errorf("%w: line-endings must be lf, crlf, or cr", pipeline.ErrInvalidLineEndings)
	wantOutcome := pipeline.ClassifyOperationError(leWrapped, "")

	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	st := testutil.NewMemFileStater()
	sess := testutil.NewMemPublishSessionForOutput(out)
	runnerInner := mustNewDefaultRunner(t, src, out, st, sess)

	root := testWorkspaceRoot()
	harnessSrv := mustNewGlyphShiftServerFromRunner(t, root, "tline", runnerInner, testutil.NoSymlinkPathResolver{})

	args := []byte(`{"source":"doc.md","line_endings":"nope-format"}`)
	result, err := harnessSrv.InvokeRegisteredTool(context.Background(), toolTransform, args)
	if err != nil {
		t.Fatalf("InvokeRegisteredTool: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("want IsError structured tool envelope, got %+v", result)
	}

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != wantOutcome.Error {
		t.Fatalf("error: got %q want %q", opErrMustString(t, payload, "error"), wantOutcome.Error)
	}
	if opErrMustString(t, payload, "hint") != wantOutcome.Hint {
		t.Fatalf("hint: got %q want %q", opErrMustString(t, payload, "hint"), wantOutcome.Hint)
	}
}

func TestTransformTool_binarySkippedStructuredErrorUsesBinarySourceClassification(t *testing.T) {
	t.Parallel()

	root := testWorkspaceRoot()
	prep, prepErr := pipeline.PreparePath("doc.txt", root)
	if prepErr != nil {
		t.Fatalf("PreparePath: %v", prepErr)
	}

	wantOutcome := pipeline.ClassifyOperationError(pipeline.ErrBinarySource, prep)

	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	st := testutil.NewMemFileStater()
	res := testutil.NoSymlinkPathResolver{}
	sess := testutil.NewMemPublishSessionForOutput(out)
	inner := mustNewDefaultRunner(t, src, out, st, sess)

	sk := transformSkipRunnerEmbed{DefaultRunner: inner, skipBinary: true}
	harnessSrv := mustNewGlyphShiftServerFromRunner(t, root, "tbin", sk, res)

	body := []byte(`{"source":"doc.txt","line_endings":"lf"}`)
	resTool, callErr := harnessSrv.InvokeRegisteredTool(context.Background(), toolTransform, body)
	if callErr != nil {
		t.Fatalf("InvokeRegisteredTool: %v", callErr)
	}

	payload := mustValidatedOperationErrorMap(t, resTool.StructuredContent)
	if opErrMustString(t, payload, "error") != wantOutcome.Error {
		t.Fatalf("error mismatch: got %q want %q", opErrMustString(t, payload, "error"), wantOutcome.Error)
	}
	if opErrMustString(t, payload, "hint") != wantOutcome.Hint {
		t.Fatalf("hint mismatch")
	}
}

func TestTransformTool_unknownSkipReasonStructuredInternalError(t *testing.T) {
	t.Parallel()

	root := testWorkspaceRoot()
	prep, prepErr := pipeline.PreparePath("doc.txt", root)
	if prepErr != nil {
		t.Fatalf("PreparePath: %v", prepErr)
	}

	wantOutcome := pipeline.ClassifyOperationError(pipeline.ErrTransformSkippedUnknown, prep)

	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	st := testutil.NewMemFileStater()
	res := testutil.NoSymlinkPathResolver{}
	sess := testutil.NewMemPublishSessionForOutput(out)
	inner := mustNewDefaultRunner(t, src, out, st, sess)

	sk := transformSkipRunnerEmbed{DefaultRunner: inner, skipBinary: false}
	harnessSrv := mustNewGlyphShiftServerFromRunner(t, root, "tunknown", sk, res)

	body := []byte(`{"source":"doc.txt","line_endings":"lf"}`)
	resTool, callErr := harnessSrv.InvokeRegisteredTool(context.Background(), toolTransform, body)
	if callErr != nil {
		t.Fatalf("InvokeRegisteredTool: %v", callErr)
	}

	payload := mustValidatedOperationErrorMap(t, resTool.StructuredContent)
	if opErrMustString(t, payload, "error") != wantOutcome.Error {
		t.Fatalf("error mismatch")
	}
	if opErrMustString(t, payload, "hint") != wantOutcome.Hint {
		t.Fatalf("hint mismatch")
	}
}

func TestInvokeRegisteredTool_explicitPreviewTrueStructuredUsesInspectionShape(t *testing.T) {
	t.Parallel()

	srv, _, _, stMem, sessMem := newUnitGlyphShiftServerParts(t, testWorkspaceRoot())
	mustSeedTransformTwinFS(t, srv, stMem, sessMem, "doc.txt", []byte("a\n"))

	preview := true
	raw := transformDispatchArgs(t, srv, TransformInput{
		Source:      "doc.txt",
		LineEndings: "lf",
		Preview:     &preview,
	})

	jsonRequireKeyAbsent(t, raw, "changed")
	jsonRequireKeyPresent(t, raw, "would_change")
}

func TestInvokeRegisteredTool_omittedPreviewUsesApplyStructuredFieldsNotInspectionFields(t *testing.T) {
	t.Parallel()

	srv, _, _, stMem, sessMem := newUnitGlyphShiftServerParts(t, testWorkspaceRoot())
	mustSeedTransformTwinFS(t, srv, stMem, sessMem, "doc.txt", []byte("a\n"))

	raw := transformDispatchArgs(t, srv, TransformInput{
		Source:      "doc.txt",
		LineEndings: "lf",
	})

	jsonRequireKeyAbsent(t, raw, "would_change")
	jsonRequireKeyPresent(t, raw, "changed")
}

func transformDispatchArgs(tb testing.TB, srv *GlyphShiftServer, in TransformInput) json.RawMessage {
	tb.Helper()

	argsJSON, err := json.Marshal(in)
	if err != nil {
		tb.Fatalf("marshal tool args: %v", err)
	}

	out, invokeErr := srv.InvokeRegisteredTool(context.Background(), toolTransform, argsJSON)
	if invokeErr != nil {
		tb.Fatalf("InvokeRegisteredTool: %v", invokeErr)
	}
	if out == nil || out.IsError {
		tb.Fatalf("expected successful tool envelope, got %+v", out)
	}

	encoded, mErr := json.Marshal(out.StructuredContent)
	if mErr != nil {
		tb.Fatalf("marshal structured transform output: %v", mErr)
	}

	return json.RawMessage(encoded)
}

func jsonRequireKeyAbsent(tb testing.TB, raw json.RawMessage, key string) {
	tb.Helper()

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		tb.Fatalf("unmarshal structured json: %v", err)
	}
	if _, ok := keys[key]; ok {
		tb.Fatalf("expected key %q absent, keys=%v", key, sortedMapKeysForRaw(keys))
	}
}

func jsonRequireKeyPresent(tb testing.TB, raw json.RawMessage, key string) {
	tb.Helper()

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		tb.Fatalf("unmarshal structured json: %v", err)
	}

	if _, ok := keys[key]; !ok {
		tb.Fatalf("expected key %q present, keys=%v", key, sortedMapKeysForRaw(keys))
	}
}

func sortedMapKeysForRaw(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))

	for key := range m {
		out = append(out, key)
	}

	sort.Strings(out)

	return out
}

type transformSkipRunnerEmbed struct {
	*pipeline.DefaultRunner
	skipBinary bool
}

func (sk transformSkipRunnerEmbed) RunTransform(
	_ context.Context,
	p pipeline.TransformParams,
) (pipeline.TransformPipelineResult, error) {
	reason := "unit-test-unknown-skip"
	if sk.skipBinary {
		reason = "binary"
	}

	return pipeline.TransformPipelineResult{
		Result: fileops.TransformFileResult{
			Skipped:    true,
			SkipReason: reason,
		},
	}, nil
}

func TestNewGlyphShiftServerFromRunner_nilRunner_returnsSentinel(t *testing.T) {
	t.Parallel()

	var zero pipeline.Runner
	_, err := NewGlyphShiftServerFromRunner(testWorkspaceRoot(), "vr-test", zero, testutil.NoSymlinkPathResolver{})
	if !errors.Is(err, ErrNilRunner) {
		t.Fatalf("got %v want %v", err, ErrNilRunner)
	}
}

func TestNewGlyphShiftServerFromRunner_nilPathResolver_returnsSentinel(t *testing.T) {
	t.Parallel()

	_, err := NewGlyphShiftServerFromRunner(testWorkspaceRoot(), "vr-pr", constructorRunner{}, nil)
	if !errors.Is(err, ErrNilPathResolver) {
		t.Fatalf("got %v want %v", err, ErrNilPathResolver)
	}
}

func TestNewGlyphShiftServer_nilSourceOpener_returnsSentinel(t *testing.T) {
	t.Parallel()

	root := testWorkspaceRoot()

	outOK := testutil.NewMemOutputOpener()
	stOK := testutil.NewMemFileStater()
	fsOK := testutil.NewMemPublishSessionForOutput(outOK)
	resOK := testutil.NoSymlinkPathResolver{}

	var nilSrc pipeline.SourceOpener
	_, err := NewGlyphShiftServer(
		root, "v",
		nilSrc,
		outOK,
		stOK,
		resOK,
		fsOK,
	)
	if !errors.Is(err, ErrNilSourceOpener) {
		t.Fatalf("got %v want %v", err, ErrNilSourceOpener)
	}
}

func TestNewGlyphShiftServer_nilOutputOpener_returnsSentinel(t *testing.T) {
	t.Parallel()

	root := testWorkspaceRoot()

	srcOK := testutil.NewMemSourceOpener()
	outOK := testutil.NewMemOutputOpener()
	stOK := testutil.NewMemFileStater()
	fsOK := testutil.NewMemPublishSessionForOutput(outOK)
	resOK := testutil.NoSymlinkPathResolver{}

	var nilOut pipeline.OutputOpener
	_, err := NewGlyphShiftServer(
		root, "v",
		srcOK,
		nilOut,
		stOK,
		resOK,
		fsOK,
	)
	if !errors.Is(err, ErrNilOutputOpener) {
		t.Fatalf("got %v want %v", err, ErrNilOutputOpener)
	}
}

func TestNewGlyphShiftServer_nilFileStater_returnsSentinel(t *testing.T) {
	t.Parallel()

	root := testWorkspaceRoot()

	srcOK := testutil.NewMemSourceOpener()
	outOK := testutil.NewMemOutputOpener()
	fsOK := testutil.NewMemPublishSessionForOutput(outOK)
	resOK := testutil.NoSymlinkPathResolver{}

	var nilSt pipeline.FileStater
	_, err := NewGlyphShiftServer(
		root, "v",
		srcOK,
		outOK,
		nilSt,
		resOK,
		fsOK,
	)
	if !errors.Is(err, ErrNilFileStater) {
		t.Fatalf("got %v want %v", err, ErrNilFileStater)
	}
}

func TestNewGlyphShiftServer_nilPathResolver_returnsSentinel(t *testing.T) {
	t.Parallel()

	root := testWorkspaceRoot()

	srcOK := testutil.NewMemSourceOpener()
	outOK := testutil.NewMemOutputOpener()
	stOK := testutil.NewMemFileStater()
	fsOK := testutil.NewMemPublishSessionForOutput(outOK)

	var nilR validate.PathResolver
	_, err := NewGlyphShiftServer(
		root, "v",
		srcOK,
		outOK,
		stOK,
		nilR,
		fsOK,
	)
	if !errors.Is(err, ErrNilPathResolver) {
		t.Fatalf("got %v want %v", err, ErrNilPathResolver)
	}
}

func TestNewGlyphShiftServer_nilFileSession_returnsSentinel(t *testing.T) {
	t.Parallel()

	root := testWorkspaceRoot()

	srcOK := testutil.NewMemSourceOpener()
	outOK := testutil.NewMemOutputOpener()
	stOK := testutil.NewMemFileStater()
	resOK := testutil.NoSymlinkPathResolver{}

	var nilS fileops.FileSession
	_, err := NewGlyphShiftServer(
		root, "v",
		srcOK,
		outOK,
		stOK,
		resOK,
		nilS,
	)
	if !errors.Is(err, ErrNilFileSession) {
		t.Fatalf("got %v want %v", err, ErrNilFileSession)
	}
}
