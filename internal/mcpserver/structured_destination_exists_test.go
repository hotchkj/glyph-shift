package mcpserver

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/spf13/afero"
)

const testMCPJSONErrDestinationExists = "destination_exists"

func mustToolPath(t *testing.T, srv *GlyphShiftServer, rel string) string {
	t.Helper()

	p, err := srv.validateToolPath(rel)
	if err != nil {
		t.Fatalf("validateToolPath %q: %v", rel, err)
	}

	return p
}

func mustMkdirMem(t *testing.T, fs afero.Fs, path string) {
	t.Helper()

	if err := fs.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func mustWriteMem(t *testing.T, fs afero.Fs, path string, content []byte) {
	t.Helper()

	if err := afero.WriteFile(fs, path, content, 0o600); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func mustAssertDestinationExistsPayload(
	t *testing.T,
	srv *GlyphShiftServer,
	toolName string,
	result *mcp.CallToolResult,
	logicalDestRel string,
) {
	t.Helper()

	mustValidateStructuredContentAgainstSchema(t, toolName, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != testMCPJSONErrDestinationExists {
		t.Fatalf("error: got %q want %s", opErrMustString(t, payload, "error"), testMCPJSONErrDestinationExists)
	}

	gotDest := opErrMustString(t, payload, "dest")
	absPathMustMatchToolPath(t, srv, logicalDestRel, gotDest, "dest")

	if opErrMustString(t, payload, "hint") != pipeline.HintDestinationExists {
		t.Fatalf("hint: got %q", opErrMustString(t, payload, "hint"))
	}
}

func TestSplitToolDestinationExistsCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, srcMem, outMem := newServer(t, testWorkspaceRoot())

	srcPath := mustToolPath(t, srv, "in.txt")
	mustWriteMem(t, srcMem.Fs, srcPath, []byte(testSplitMultiSectionSource))

	outDir := mustToolPath(t, srv, "out")
	mustMkdirMem(t, outMem.Fs, outDir)

	firstOut := filepath.Join(outDir, "001.txt")
	mustWriteMem(t, outMem.Fs, firstOut, []byte("exists\n"))

	result, _, err := srv.handleSplitTool(context.Background(), nil, SplitInput{
		Source:    "in.txt",
		Delimiter: "^---$",
		OutputDir: "out",
		Extension: ".txt",
	})
	if err != nil {
		t.Fatalf("handleSplitTool: %v", err)
	}

	if result == nil || !result.IsError {
		t.Fatal("expected non-nil IsError result")
	}

	mustAssertDestinationExistsPayload(t, srv, toolSplit, result, filepath.Join("out", "001.txt"))
}

func TestBlocksToolDestinationExistsCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, srcMem, outMem := newServer(t, testWorkspaceRoot())

	srcPath := mustToolPath(t, srv, "doc.md")
	mustWriteMem(t, srcMem.Fs, srcPath, []byte("```go\nx\n```\n"))

	outDir := mustToolPath(t, srv, "out")
	mustMkdirMem(t, outMem.Fs, outDir)

	firstOut := filepath.Join(outDir, "001.md")
	mustWriteMem(t, outMem.Fs, firstOut, []byte("exists\n"))

	result, _, err := srv.handleBlocksTool(context.Background(), nil, BlocksInput{
		Source:    "doc.md",
		StartLine: "^```go$",
		EndLine:   "^```$",
		OutputDir: "out",
	})
	if err != nil {
		t.Fatalf("handleBlocksTool: %v", err)
	}

	if result == nil || !result.IsError {
		t.Fatal("expected non-nil IsError result")
	}

	mustAssertDestinationExistsPayload(t, srv, toolBlocks, result, filepath.Join("out", "001.md"))
}

func TestExtractToolDestinationExistsCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, srcMem, outMem := newServer(t, testWorkspaceRoot())

	srcPath := mustToolPath(t, srv, "doc.txt")
	mustWriteMem(t, srcMem.Fs, srcPath, []byte("one\ntwo\n"))

	destPath := mustToolPath(t, srv, "out.txt")
	mustWriteMem(t, outMem.Fs, destPath, []byte("exists\n"))

	result, _, err := srv.handleExtractTool(context.Background(), nil, ExtractInput{
		Source:      "doc.txt",
		Lines:       "1-1",
		Destination: "out.txt",
	})
	if err != nil {
		t.Fatalf("handleExtractTool: %v", err)
	}

	if result == nil || !result.IsError {
		t.Fatal("expected non-nil IsError result")
	}

	mustAssertDestinationExistsPayload(t, srv, toolExtract, result, "out.txt")
}

func TestExtractToolPreviewDestinationExistsCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, srcMem, outMem := newServer(t, testWorkspaceRoot())

	srcPath := mustToolPath(t, srv, "doc.txt")
	mustWriteMem(t, srcMem.Fs, srcPath, []byte("one\ntwo\n"))

	destPath := mustToolPath(t, srv, "out.txt")
	mustWriteMem(t, outMem.Fs, destPath, []byte("exists\n"))

	result, _, err := srv.handleExtractTool(context.Background(), nil, ExtractInput{
		Source:      "doc.txt",
		Lines:       "1-1",
		Destination: "out.txt",
		Preview:     true,
	})
	if err != nil {
		t.Fatalf("handleExtractTool: %v", err)
	}

	if result == nil || !result.IsError {
		t.Fatal("expected non-nil IsError result")
	}

	mustAssertDestinationExistsPayload(t, srv, toolExtract, result, "out.txt")
}

func TestSplitToolPreviewDestinationExistsCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, srcMem, outMem := newServer(t, testWorkspaceRoot())

	srcPath := mustToolPath(t, srv, "in.txt")
	mustWriteMem(t, srcMem.Fs, srcPath, []byte(testSplitMultiSectionSource))

	outDir := mustToolPath(t, srv, "out")
	mustMkdirMem(t, outMem.Fs, outDir)

	firstOut := filepath.Join(outDir, "001.txt")
	mustWriteMem(t, outMem.Fs, firstOut, []byte("exists\n"))

	result, _, err := srv.handleSplitTool(context.Background(), nil, SplitInput{
		Source:    "in.txt",
		Delimiter: "^---$",
		OutputDir: "out",
		Extension: ".txt",
		Preview:   true,
	})
	if err != nil {
		t.Fatalf("handleSplitTool: %v", err)
	}

	if result == nil || !result.IsError {
		t.Fatal("expected non-nil IsError result")
	}

	mustAssertDestinationExistsPayload(t, srv, toolSplit, result, filepath.Join("out", "001.txt"))
}

func TestBlocksToolPreviewDestinationExistsCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, srcMem, outMem := newServer(t, testWorkspaceRoot())

	srcPath := mustToolPath(t, srv, "doc.md")
	mustWriteMem(t, srcMem.Fs, srcPath, []byte("```go\nx\n```\n"))

	outDir := mustToolPath(t, srv, "out")
	mustMkdirMem(t, outMem.Fs, outDir)

	firstOut := filepath.Join(outDir, "001.md")
	mustWriteMem(t, outMem.Fs, firstOut, []byte("exists\n"))

	result, _, err := srv.handleBlocksTool(context.Background(), nil, BlocksInput{
		Source:    "doc.md",
		StartLine: "^```go$",
		EndLine:   "^```$",
		OutputDir: "out",
		Preview:   true,
	})
	if err != nil {
		t.Fatalf("handleBlocksTool: %v", err)
	}

	if result == nil || !result.IsError {
		t.Fatal("expected non-nil IsError result")
	}

	mustAssertDestinationExistsPayload(t, srv, toolBlocks, result, filepath.Join("out", "001.md"))
}
