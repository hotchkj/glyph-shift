//go:build integration

// Integration tests for MCP server handlers: real OS filesystem I/O through the handler layer.
//
// Run: mage integration. Diagnostic: go test -tags integration ./integrations/...
//
//nolint:cyclop // Handler matrix coverage; package-average budget is tight for integration tests.
package mcpserver_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/mcpserver"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

func newTestGlyphShiftServer(t *testing.T, dir string) *mcpserver.GlyphShiftServer {
	t.Helper()

	session := fileops.NewOSFileSession()

	srcOpener, openErr := pipeline.NewOSSourceOpener(session)
	if openErr != nil {
		t.Fatalf("NewOSSourceOpener: %v", openErr)
	}

	srv, err := mcpserver.NewGlyphShiftServer(
		dir, "test",
		srcOpener,
		pipeline.NewOSOutputOpener(),
		pipeline.NewOSFileStater(),
		validate.NewOSPathResolver(),
		session,
	)
	if err != nil {
		t.Fatalf("NewGlyphShiftServer: %v", err)
	}

	return srv
}

func TestToolRegistration(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srv := newTestGlyphShiftServer(t, tmpDir)
	mcpSrv := srv.NewMCPServer()
	if mcpSrv == nil {
		t.Fatal("expected non-nil MCP server")
	}
}

func TestValidateToolPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srv := newTestGlyphShiftServer(t, tmpDir)

	got, err := mcpserver.IntegrationValidateToolPath(srv, "foo.txt")
	if err != nil {
		t.Fatalf("IntegrationValidateToolPath: %v", err)
	}

	want := filepath.Join(tmpDir, "foo.txt")
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("got %q, want %q", got, want)
	}

	_, err = mcpserver.IntegrationValidateToolPath(srv, "../../outside")
	if err == nil {
		t.Fatal("expected error for path outside workspace")
	}
}

func prepareValidateToolPathParityFixture(t *testing.T) (srv *mcpserver.GlyphShiftServer, wantClean string) {
	t.Helper()

	tmpDir := t.TempDir()
	srv = newTestGlyphShiftServer(t, tmpDir)

	nested := filepath.Join(tmpDir, "sub", "file.txt")
	if err := os.MkdirAll(filepath.Dir(nested), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(nested, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	wantAbs, err := filepath.Abs(nested)
	if err != nil {
		t.Fatalf("abs nested: %v", err)
	}
	wantClean = filepath.Clean(wantAbs)

	return srv, wantClean
}

func TestValidateToolPath_forwardSlashRelative_matches_abs_nested(t *testing.T) {
	t.Parallel()

	srv, want := prepareValidateToolPathParityFixture(t)

	gotFromSlash, err := mcpserver.IntegrationValidateToolPath(srv, "sub/file.txt")
	if err != nil {
		t.Fatalf("IntegrationValidateToolPath(sub/file.txt): %v", err)
	}
	if filepath.Clean(gotFromSlash) != want {
		t.Fatalf("sub/file.txt: got %q, want %q", gotFromSlash, want)
	}
}

func TestValidateToolPath_nativeSeparatorRelative_matches_abs_nested(t *testing.T) {
	t.Parallel()

	srv, want := prepareValidateToolPathParityFixture(t)

	relNative := filepath.Join("sub", "file.txt")
	gotNative, err := mcpserver.IntegrationValidateToolPath(srv, relNative)
	if err != nil {
		t.Fatalf("IntegrationValidateToolPath(%q): %v", relNative, err)
	}
	if filepath.Clean(gotNative) != want {
		t.Fatalf("native rel: got %q, want %q", gotNative, want)
	}
}

func TestValidateToolPath_absolute_under_workspace_matches_abs_nested(t *testing.T) {
	t.Parallel()

	srv, want := prepareValidateToolPathParityFixture(t)

	nestedForAbs := filepath.Join(srv.WorkspaceRoot, "sub", "file.txt")
	absNested, err := filepath.Abs(nestedForAbs)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	gotAbs, err := mcpserver.IntegrationValidateToolPath(srv, absNested)
	if err != nil {
		t.Fatalf("IntegrationValidateToolPath(abs): %v", err)
	}
	if filepath.Clean(gotAbs) != want {
		t.Fatalf("absolute under workspace: got %q, want %q", gotAbs, want)
	}
}

func TestHandlerErrorsSanitizeWorkspacePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srv := newTestGlyphShiftServer(t, tmpDir)

	input := mcpserver.ExtractInput{
		Source:      "does-not-exist.txt",
		Lines:       "1-2",
		Destination: "out.txt",
	}

	_, _, err := mcpserver.IntegrationHandleExtract(context.Background(), srv, nil, input)
	if err == nil {
		t.Fatal("expected error opening missing source file")
	}

	if !errors.Is(err, pipeline.ErrSourceNotFound) {
		t.Fatalf("expected ErrSourceNotFound, got: %v", err)
	}

	const wantSanitized = "source file not found"
	if err.Error() != wantSanitized {
		t.Fatalf(
			"sanitized error message = %q, want %q (must not leak workspace path %q)",
			err.Error(), wantSanitized, tmpDir,
		)
	}
}

func TestPathTraversal(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srv := newTestGlyphShiftServer(t, tmpDir)
	input := mcpserver.ExtractInput{
		Source:      "../../etc/passwd",
		Lines:       "1-10",
		Destination: "out.txt",
	}

	_, _, err := mcpserver.IntegrationHandleExtract(context.Background(), srv, nil, input)
	if err == nil {
		t.Fatal("expected error for path traversal")
	}

	if !errors.Is(err, validate.ErrOutsideRoot) && !errors.Is(err, validate.ErrPathTraversal) {
		t.Fatalf("error %q should be path traversal outside root", err.Error())
	}
}

func TestValidateToolPathEmpty(t *testing.T) {
	t.Parallel()

	srv := newTestGlyphShiftServer(t, t.TempDir())
	_, err := mcpserver.IntegrationValidateToolPath(srv, "")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}
