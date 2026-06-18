package mcpserver

import (
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fsnorm"
)

func TestNewGlyphShiftServerFromRunner_workspaceRootMatchesNormalizeContract(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string([]rune{filepath.Separator}), "x", "mcp-ws-contract")

	runner := constructorRunner{}
	resolver := &constructorPathResolver{}

	srv, err := NewGlyphShiftServerFromRunner(root, "v", runner, resolver)
	if err != nil {
		t.Fatalf("NewGlyphShiftServerFromRunner: %v", err)
	}

	native := fsnorm.DirNative(root)
	absWant, absErr := filepath.Abs(native)
	if absErr != nil {
		t.Fatalf("test fixture filepath.Abs(%q): %v", native, absErr)
	}
	wantRoot := filepath.Clean(absWant)
	if srv.WorkspaceRoot != wantRoot {
		t.Fatalf("WorkspaceRoot %q does not match expected contract root %q", srv.WorkspaceRoot, wantRoot)
	}
}
