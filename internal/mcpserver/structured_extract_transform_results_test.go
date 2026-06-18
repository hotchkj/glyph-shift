package mcpserver

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

func TestExtractToolSourceNotFoundCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, _, _ := newServer(t, testWorkspaceRoot())

	result, _, err := srv.handleExtractTool(context.Background(), nil, ExtractInput{
		Source:      missingTextRelPath,
		Lines:       "1-1",
		Destination: "out.txt",
	})
	if err != nil {
		t.Fatalf("handleExtractTool: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected non-nil IsError result")
	}

	mustValidateStructuredContentAgainstSchema(t, toolExtract, result.StructuredContent)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != sourceNotFoundError {
		t.Fatalf("error: got %q want source_not_found", opErrMustString(t, payload, "error"))
	}
	wantSrc := mustToolPath(t, srv, missingTextRelPath)
	src := opErrMustString(t, payload, "src")
	if filepath.Clean(src) != filepath.Clean(wantSrc) {
		t.Fatalf("Source: got %q want absolute native workspace path %q", src, wantSrc)
	}
	if !filepath.IsAbs(src) {
		t.Fatalf("src must be absolute native path per contract, got %q", src)
	}
}

func TestExtractToolSuccessStructuredContentMatchesOutputSchema(t *testing.T) {
	t.Parallel()

	srv, srcMem, _ := newServer(t, testWorkspaceRoot())

	srcPath, err := srv.validateToolPath("doc.txt")
	if err != nil {
		t.Fatalf("validate source path: %v", err)
	}
	if writeErr := afero.WriteFile(srcMem.Fs, srcPath, []byte("one\ntwo\n"), 0o600); writeErr != nil {
		t.Fatalf("write source: %v", writeErr)
	}

	result, _, err := srv.handleExtractTool(context.Background(), nil, ExtractInput{
		Source:      "doc.txt",
		Lines:       "1-1",
		Destination: "out.txt",
	})
	if err != nil {
		t.Fatalf("handleExtractTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected success, isError=%v", result != nil && result.IsError)
	}

	mustValidateStructuredContentAgainstSchema(t, toolExtract, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)
}

func TestExtractToolPreviewStructuredContentMatchesOutputSchema(t *testing.T) {
	t.Parallel()

	srv, srcMem, _ := newServer(t, testWorkspaceRoot())

	mustWriteSrvSourceBytes(t, srv, srcMem.Fs, "doc.txt", []byte("one\ntwo\n"))

	result, _, err := srv.handleExtractTool(context.Background(), nil, ExtractInput{
		Source:      "doc.txt",
		Lines:       "1-2",
		Destination: "out.txt",
		Preview:     true,
	})
	if err != nil {
		t.Fatalf("handleExtractTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected success, isError=%v", result != nil && result.IsError)
	}

	mustValidateStructuredContentAgainstSchema(t, toolExtract, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)
	assertExtractPreviewStructuredExactly(t, srv, result.StructuredContent)
}

func mustWriteSrvSourceBytes(t *testing.T, srv *GlyphShiftServer, fs afero.Fs, logicalName string, body []byte) {
	t.Helper()

	srcPath, err := srv.validateToolPath(logicalName)
	if err != nil {
		t.Fatalf("validate source path: %v", err)
	}
	if writeErr := afero.WriteFile(fs, srcPath, body, 0o600); writeErr != nil {
		t.Fatalf("write source: %v", writeErr)
	}
}

func assertExtractPreviewStructuredExactly(t *testing.T, srv *GlyphShiftServer, structuredContent any) {
	t.Helper()

	encoded, marshalErr := json.Marshal(structuredContent)
	if marshalErr != nil {
		t.Fatalf("marshal structuredContent: %v", marshalErr)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("decode structuredContent: %v", err)
	}
	wantKeys := map[string]struct{}{
		"would_extract_lines": {},
		"would_create":        {},
	}
	if len(got) != len(wantKeys) {
		t.Fatalf("structuredContent keys: got %v want would_extract_lines,would_create", got)
	}
	for k := range got {
		if _, ok := wantKeys[k]; !ok {
			t.Fatalf("unexpected structuredContent key %q in %#v", k, got)
		}
	}
	if got["would_extract_lines"] != float64(2) {
		t.Fatalf("would_extract_lines: got %v want 2", got["would_extract_lines"])
	}
	wouldCreate, ok := got["would_create"].(string)
	if !ok {
		t.Fatalf("would_create: not string, got %T %v", got["would_create"], got["would_create"])
	}
	if !filepath.IsAbs(wouldCreate) {
		t.Fatalf("would_create must be absolute native path per glyph-shift-json-contract.md, got %q", wouldCreate)
	}
	wantDest := mustToolPath(t, srv, "out.txt")
	if filepath.Clean(wouldCreate) != filepath.Clean(wantDest) {
		t.Fatalf("would_create: got %q want resolved workspace destination %q (absolute native)",
			wouldCreate, wantDest)
	}
}

func TestTransformToolSourceNotFoundCarriesStructuredContent(t *testing.T) {
	t.Parallel()

	srv, _, _, _, _ := newUnitGlyphShiftServerParts(t, testWorkspaceRoot())

	result, _, err := srv.handleTransformTool(context.Background(), nil, TransformInput{
		Source:      missingTextRelPath,
		LineEndings: "lf",
	})
	if err != nil {
		t.Fatalf("handleTransformTool: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected non-nil IsError result")
	}

	mustValidateStructuredContentAgainstSchema(t, toolTransform, result.StructuredContent)

	payload := mustValidatedOperationErrorMap(t, result.StructuredContent)
	if opErrMustString(t, payload, "error") != sourceNotFoundError {
		t.Fatalf("error: got %q want source_not_found", opErrMustString(t, payload, "error"))
	}
	wantSrc := mustToolPath(t, srv, missingTextRelPath)
	src := opErrMustString(t, payload, "src")
	if filepath.Clean(src) != filepath.Clean(wantSrc) {
		t.Fatalf("Source: got %q want absolute native workspace path %q", src, wantSrc)
	}
	if !filepath.IsAbs(src) {
		t.Fatalf("src must be absolute native path per contract, got %q", src)
	}
}

func TestTransformToolSuccessStructuredContentMatchesOutputSchema(t *testing.T) {
	t.Parallel()

	srv, _, _, stMem, sessMem := newUnitGlyphShiftServerParts(t, testWorkspaceRoot())
	mustSeedTransformTwinFS(t, srv, stMem, sessMem, "doc.txt", []byte("one\r\ntwo\r\n"))

	result, _, err := srv.handleTransformTool(context.Background(), nil, TransformInput{
		Source:      "doc.txt",
		LineEndings: "lf",
	})
	if err != nil {
		t.Fatalf("handleTransformTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected success, isError=%v", result != nil && result.IsError)
	}

	mustValidateStructuredContentAgainstSchema(t, toolTransform, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)
}

func TestTransformToolPreviewStructuredContentMatchesOutputSchema(t *testing.T) {
	t.Parallel()

	srv, _, _, stMem, sessMem := newUnitGlyphShiftServerParts(t, testWorkspaceRoot())
	mustSeedTransformTwinFS(t, srv, stMem, sessMem, "doc.txt", []byte("one\r\ntwo\r\n"))

	preview := true
	result, _, err := srv.handleTransformTool(context.Background(), nil, TransformInput{
		Source:      "doc.txt",
		LineEndings: "lf",
		Preview:     &preview,
	})
	if err != nil {
		t.Fatalf("handleTransformTool: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected success, isError=%v", result != nil && result.IsError)
	}

	mustValidateStructuredContentAgainstSchema(t, toolTransform, result.StructuredContent)
	mustAssertToolStructuredJSONMatchesText(t, result)
}
