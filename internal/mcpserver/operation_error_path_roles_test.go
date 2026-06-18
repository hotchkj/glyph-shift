package mcpserver

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

func TestOperationErrorMap_out_dir_prefers_role_over_fallback_src(t *testing.T) {
	t.Parallel()

	srv, _, _ := newServer(t, testWorkspaceRoot())
	outDirLexical := filepath.Join(string(filepath.Separator), "escape",
		"..", "..", "..", "..", "..", "..", "..", "rel-out")
	inner := fmt.Errorf("validate out-dir: %w", validate.ErrPathTraversal)
	pathErr := sanitizeError(pipeline.WithPathRole(pipeline.PathRoleOutDir, outDirLexical, inner), srv.WorkspaceRoot)

	fallbackPreparedSrc := "prepared-src-must-not-appear"
	payload := srv.operationErrorMap(pathErr, fallbackPreparedSrc)

	if validateErr := pipeline.ValidateOperationErrorPayload(payload); validateErr != nil {
		t.Fatalf("operation error map contract: %v", validateErr)
	}

	if _, hasSrc := payload["src"]; hasSrc {
		t.Fatalf("must not populate src fallback; payload=%v", payload)
	}
	outDir, ok := payload["out_dir"].(string)
	if !ok || outDir == "" {
		t.Fatalf("want structured out_dir; payload=%v", payload)
	}
}

func TestOperationErrorMap_dest_prefers_role_over_fallback_src_extract(t *testing.T) {
	t.Parallel()

	srv, _, _ := newServer(t, testWorkspaceRoot())
	destLex := filepath.Join("..", "..", "..", "..", "..", "..", "..", "..", "nested", "out.txt")

	inner := fmt.Errorf("%w", validate.ErrOutsideRoot)
	pathErr := sanitizeError(pipeline.WithPathRole(pipeline.PathRoleDest, destLex, inner), srv.WorkspaceRoot)

	fallbackPreparedSrc := "prep-src-abs"
	payload := srv.operationErrorMap(pathErr, fallbackPreparedSrc)

	if validateErr := pipeline.ValidateOperationErrorPayload(payload); validateErr != nil {
		t.Fatalf("operation error map contract: %v", validateErr)
	}

	if _, hasSrc := payload["src"]; hasSrc {
		t.Fatalf("must not populate src fallback; payload=%v", payload)
	}
	destAbs, ok := payload["dest"].(string)
	if !ok || destAbs == "" {
		t.Fatalf("want structured dest path; payload=%v", payload)
	}
	if payload["error"] != invalidInputErrorName {
		t.Fatalf("error field: got %v want invalid_input", payload["error"])
	}
}
