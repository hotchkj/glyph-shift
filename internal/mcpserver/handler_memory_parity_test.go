package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

// In-process parity for MCP handler integration scenarios: memory seams
// (mem source/output/stater/session), no OS filesystem or MCP stdio.

func memoryBlocksSrvWithFixture(t *testing.T, srcBody []byte) (*GlyphShiftServer, *testutil.MemOutputOpener, string) {
	t.Helper()

	root := testWorkspaceRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	st := testutil.NewMemFileStater()
	sess := testutil.NewMemPublishSessionForOutput(out)
	srv := mustNewGlyphShiftServer(t, root, src, out, st, sess)

	srcPath, err := srv.validateToolPath("in.txt")
	if err != nil {
		t.Fatalf("validate source path: %v", err)
	}

	if writeErr := afero.WriteFile(src.Fs, srcPath, srcBody, 0o600); writeErr != nil {
		t.Fatalf("write source: %v", writeErr)
	}

	outDir, err := srv.validateToolPath("out")
	if err != nil {
		t.Fatalf("validate out path: %v", err)
	}

	if mkdirErr := out.Fs.MkdirAll(outDir, 0o700); mkdirErr != nil {
		t.Fatalf("mkdir out: %v", mkdirErr)
	}

	return srv, out, outDir
}

func mustAssertPathsEndWithSuffix(t *testing.T, paths []string, suffix string) {
	t.Helper()

	for _, p := range paths {
		if !strings.HasSuffix(p, suffix) {
			t.Fatalf("path %q does not end with %q", p, suffix)
		}
	}
}

func mustSeedTransformTwinFS(
	t *testing.T,
	srv *GlyphShiftServer,
	stMem *testutil.MemFileStater,
	sessMem *testutil.MemTestSession,
	rel string,
	seed []byte,
) string {
	t.Helper()

	srcPath, err := srv.validateToolPath(rel)
	if err != nil {
		t.Fatalf("validate source path: %v", err)
	}

	if writeErr := afero.WriteFile(stMem.Fs, srcPath, seed, 0o600); writeErr != nil {
		t.Fatalf("write stater source: %v", writeErr)
	}

	if writeErr := afero.WriteFile(sessMem.Fs, srcPath, seed, 0o600); writeErr != nil {
		t.Fatalf("write session source: %v", writeErr)
	}

	return srcPath
}

func TestExtractHandler_memory_linesExtractedTwoWritesOutputTxt(t *testing.T) {
	t.Parallel()

	srv, srcMem, outMem := newServer(t, testWorkspaceRoot())

	mustWriteSrvSourceBytes(t, srv, srcMem.Fs, "source.txt", []byte("line1\nline2\nline3\n"))

	result, _, err := srv.handleExtractTool(context.Background(), nil, ExtractInput{
		Source:      "source.txt",
		Lines:       "1-2",
		Destination: "output.txt",
	})
	if err != nil {
		t.Fatalf("handleExtractTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected extract tool success")
	}

	mustValidateStructuredContentAgainstSchema(t, toolExtract, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	var payload struct {
		LinesExtracted *int `json:"lines_extracted"`
	}
	encoded, marshalErr := json.Marshal(result.StructuredContent)
	if marshalErr != nil {
		t.Fatalf("marshal structured content: %v", marshalErr)
	}
	if decodeErr := json.Unmarshal(encoded, &payload); decodeErr != nil {
		t.Fatalf("decode success payload: %v", decodeErr)
	}

	if payload.LinesExtracted == nil || *payload.LinesExtracted != 2 {
		t.Fatalf("lines_extracted = %v, want ptr(2)", payload.LinesExtracted)
	}

	destAbs := mustToolPath(t, srv, "output.txt")
	got := outMem.FileContent(destAbs)
	want := []byte("line1\nline2\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("dest content = %q, want %q", got, want)
	}
}

func TestSplitHandler_memory_forceSplitSchemaCoversWrittenSections(t *testing.T) {
	t.Parallel()

	srv, srcMem, outMem := newServer(t, testWorkspaceRoot())

	mustWriteSrvSourceBytes(t, srv, srcMem.Fs, "in.txt", []byte("a\n---\nb\n"))

	outDir, err := srv.validateToolPath("out")
	if err != nil {
		t.Fatalf("validate out path: %v", err)
	}
	if mkdirErr := outMem.Fs.MkdirAll(outDir, 0o700); mkdirErr != nil {
		t.Fatalf("mkdir out: %v", mkdirErr)
	}

	result, _, err := srv.handleSplitTool(context.Background(), nil, SplitInput{
		Source:    "in.txt",
		Delimiter: "^---$",
		OutputDir: "out",
		Force:     true,
		Mkdir:     true,
	})
	if err != nil {
		t.Fatalf("handleSplitTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected split success")
	}

	mustValidateStructuredContentAgainstSchema(t, toolSplit, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	want001 := filepath.Join(outDir, "001.txt")
	want002 := filepath.Join(outDir, "002.txt")

	got1 := outMem.FileContent(want001)
	got2 := outMem.FileContent(want002)

	if !bytes.Equal(got1, []byte("a\n")) {
		t.Fatalf("001.txt = %q, want %q", got1, []byte("a\n"))
	}

	// StripDelimiter defaults false: the delimiter line begins the second emitted section.
	want2 := []byte("---\nb\n")
	if !bytes.Equal(got2, want2) {
		t.Fatalf("002.txt = %q, want %q", got2, want2)
	}
}

func TestBlocksHandler_memory_startEndSchemaCoversInnerContent(t *testing.T) {
	t.Parallel()

	srv, outMem, outDir := memoryBlocksSrvWithFixture(t, []byte("start\ncontent\nend\n"))

	result, _, err := srv.handleBlocksTool(context.Background(), nil, BlocksInput{
		Source:    "in.txt",
		StartLine: "^start$",
		EndLine:   "^end$",
		OutputDir: "out",
		Extension: ".md",
		Force:     true,
	})
	if err != nil {
		t.Fatalf("handleBlocksTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected blocks success")
	}

	mustValidateStructuredContentAgainstSchema(t, toolBlocks, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	var payload struct {
		ContentBlocksFound int      `json:"content_blocks_found"`
		FilesCreated       []string `json:"files_created"`
	}
	encoded, marshalErr := json.Marshal(result.StructuredContent)
	if marshalErr != nil {
		t.Fatalf("marshal structured content: %v", marshalErr)
	}
	if decodeErr := json.Unmarshal(encoded, &payload); decodeErr != nil {
		t.Fatalf("decode success payload: %v", decodeErr)
	}

	if payload.ContentBlocksFound != 1 {
		t.Fatalf("content_blocks_found = %d, want 1", payload.ContentBlocksFound)
	}

	mustAssertPathsEndWithSuffix(t, payload.FilesCreated, ".md")

	firstOut := filepath.Join(outDir, "001.md")
	got := outMem.FileContent(firstOut)
	want := []byte("content\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("001 content = %q, want %q", got, want)
	}
}

//nolint:gocyclo,cyclop // single scenario: structured decode + FS assert
func TestTransformHandler_memory_crlfToLFSchemaCoversUpdatedFile(t *testing.T) {
	t.Parallel()

	srv, _, _, stMem, sessMem := newUnitGlyphShiftServerParts(t, testWorkspaceRoot())

	srcPath := mustSeedTransformTwinFS(t, srv, stMem, sessMem, "t.txt", []byte("a\r\nb\r\n"))

	result, _, err := srv.handleTransformTool(context.Background(), nil, TransformInput{
		Source:      "t.txt",
		LineEndings: "lf",
	})
	if err != nil {
		t.Fatalf("handleTransformTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected transform success")
	}

	mustValidateStructuredContentAgainstSchema(t, toolTransform, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	var payload struct {
		Changed        *bool `json:"changed"`
		EndingsChanged *int  `json:"endings_changed"`
	}
	encoded, marshalErr := json.Marshal(result.StructuredContent)
	if marshalErr != nil {
		t.Fatalf("marshal structured content: %v", marshalErr)
	}
	if decodeErr := json.Unmarshal(encoded, &payload); decodeErr != nil {
		t.Fatalf("decode success payload: %v", decodeErr)
	}

	if payload.Changed == nil || !*payload.Changed {
		t.Fatalf("changed = %v, want true", payload.Changed)
	}

	if payload.EndingsChanged == nil || *payload.EndingsChanged != 2 {
		t.Fatalf("endings_changed = %v, want ptr(2)", payload.EndingsChanged)
	}

	got, readErr := afero.ReadFile(sessMem.Fs, srcPath)
	if readErr != nil {
		t.Fatalf("read file: %v", readErr)
	}

	if string(got) != "a\nb\n" {
		t.Fatalf("expected LF transform, got %q", string(got))
	}
}

//nolint:gocyclo,cyclop // single scenario: preview fields + JSON shape + FS parity
func TestTransformHandler_memory_previewLFOnlyNoWriteReportsZeroFalse(t *testing.T) {
	t.Parallel()

	srv, _, _, stMem, sessMem := newUnitGlyphShiftServerParts(t, testWorkspaceRoot())

	seed := []byte("a\nb\n")
	srcPath := mustSeedTransformTwinFS(t, srv, stMem, sessMem, "t.txt", seed)

	preview := true
	result, _, err := srv.handleTransformTool(context.Background(), nil, TransformInput{
		Source:      "t.txt",
		LineEndings: "lf",
		Preview:     &preview,
	})
	if err != nil {
		t.Fatalf("handleTransformTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected preview success")
	}

	mustValidateStructuredContentAgainstSchema(t, toolTransform, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	var payload struct {
		WouldChange    *bool `json:"would_change"`
		EndingsChanged *int  `json:"endings_changed"`
	}
	encoded, marshalErr := json.Marshal(result.StructuredContent)
	if marshalErr != nil {
		t.Fatalf("marshal structured content: %v", marshalErr)
	}
	if decodeErr := json.Unmarshal(encoded, &payload); decodeErr != nil {
		t.Fatalf("decode success payload: %v", decodeErr)
	}

	if payload.WouldChange == nil || *payload.WouldChange {
		t.Fatalf("would_change = %v, want ptr(false)", payload.WouldChange)
	}

	if payload.EndingsChanged == nil || *payload.EndingsChanged != 0 {
		t.Fatalf("endings_changed = %v, want ptr(0)", payload.EndingsChanged)
	}

	got, readErr := afero.ReadFile(sessMem.Fs, srcPath)
	if readErr != nil {
		t.Fatalf("read file: %v", readErr)
	}

	if !bytes.Equal(got, seed) {
		t.Fatalf("preview must not mutate file bytes: got %q want %q", got, seed)
	}
}

func TestBlocksHandler_memory_previewExtensionWouldCreateSuffix(t *testing.T) {
	t.Parallel()

	srv, _, _ := memoryBlocksSrvWithFixture(t, []byte("start\ncontent\nend\n"))

	result, _, err := srv.handleBlocksTool(context.Background(), nil, BlocksInput{
		Source:    "in.txt",
		StartLine: "^start$",
		EndLine:   "^end$",
		OutputDir: "out",
		Extension: ".md",
		Preview:   true,
	})
	if err != nil {
		t.Fatalf("handleBlocksTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected preview success")
	}

	mustValidateStructuredContentAgainstSchema(t, toolBlocks, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	var payload struct {
		ContentBlocksFound int      `json:"content_blocks_found"`
		WouldCreate        []string `json:"would_create"`
	}
	encoded, marshalErr := json.Marshal(result.StructuredContent)
	if marshalErr != nil {
		t.Fatalf("marshal structured content: %v", marshalErr)
	}
	if decodeErr := json.Unmarshal(encoded, &payload); decodeErr != nil {
		t.Fatalf("decode success payload: %v", decodeErr)
	}

	if payload.ContentBlocksFound != 1 {
		t.Fatalf("content_blocks_found = %d, want 1", payload.ContentBlocksFound)
	}

	if payload.WouldCreate == nil {
		t.Fatal("would_create must be non-nil JSON array for successful preview")
	}

	if len(payload.WouldCreate) != 1 {
		t.Fatalf("would_create = %v, want one path", payload.WouldCreate)
	}

	mustAssertPathsEndWithSuffix(t, payload.WouldCreate, ".md")
}
