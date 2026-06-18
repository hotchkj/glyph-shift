package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

var errRejectingErrorWriter = errors.New("rejecting error writer")

type failFirstErrorWriter struct {
	buf    bytes.Buffer
	failed bool
}

func (w *failFirstErrorWriter) Write(payload []byte) (int, error) {
	if !w.failed {
		w.failed = true

		return 0, errRejectingErrorWriter
	}

	return w.buf.Write(payload)
}

// Tests JSON edge mapping for classify outcomes that mimic CLI PreparePath / split-build failures where
// a wrong primary fallback path must not populate "src".
func TestFormatOperation_error_JSON_prep_out_dir_path_role_not_fallback_src(t *testing.T) {
	t.Parallel()

	workspaceRoot := testCmdWorkspaceRoot()
	wrongFallbackSrc := filepath.Join(workspaceRoot, "should-not-appear-as-src.md")
	outDirLexical := filepath.Join(string(filepath.Separator), "escape", "..", "..", "..", "..", "rel-out")
	inner := fmt.Errorf("validate out-dir: %w", validate.ErrPathTraversal)

	err := pipeline.WithPathRole(pipeline.PathRoleOutDir, outDirLexical, inner)
	got := pipeline.ClassifyOperationError(err, wrongFallbackSrc)
	if got.Src != "" {
		t.Fatalf("classifier must not stash fallback src path in Src; got %#v", got.Src)
	}

	var stderr bytes.Buffer
	writeErrorJSON(&stderr, workspaceRoot, &got)

	var raw map[string]any
	if uerr := json.Unmarshal(stderr.Bytes(), &raw); uerr != nil {
		t.Fatalf("decode stderr JSON: %v stderr=%q", uerr, stderr.String())
	}
	if _, has := raw["src"]; has {
		t.Fatalf("stderr JSON must omit src slot; stderr=%s", stderr.String())
	}

	outAbs, ok := raw["out_dir"].(string)
	if !ok || outAbs == "" {
		t.Fatalf("stderr JSON wants non-empty absolute out_dir; got %#v stderr=%s", raw["out_dir"], stderr.String())
	}
}

func TestWriteErrorJSONFallsBackWhenEncoderWriteFails(t *testing.T) {
	t.Parallel()

	outcome := pipeline.WriteFailedOutcome(errRejectingErrorWriter)
	w := &failFirstErrorWriter{}
	writeErrorJSON(w, testWorkspaceLexicalDir, &outcome)

	var raw map[string]string
	if err := json.Unmarshal(w.buf.Bytes(), &raw); err != nil {
		t.Fatalf("decode fallback JSON: %v stderr=%q", err, w.buf.String())
	}
	if raw["error"] != "write_failed" {
		t.Fatalf("fallback error = %q, want write_failed", raw["error"])
	}
	if raw["hint"] != errRejectingErrorWriter.Error() {
		t.Fatalf("fallback hint = %q, want %q", raw["hint"], errRejectingErrorWriter.Error())
	}
}

func TestWriteFailedJSONEmitsWriteFailedPayload(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	writeFailedJSON(&stderr, testWorkspaceLexicalDir, errRejectingErrorWriter)

	var raw map[string]string
	if err := json.Unmarshal(stderr.Bytes(), &raw); err != nil {
		t.Fatalf("decode stderr JSON: %v stderr=%q", err, stderr.String())
	}
	if raw["error"] != "write_failed" {
		t.Fatalf("error = %q, want write_failed", raw["error"])
	}
	if raw["hint"] != errRejectingErrorWriter.Error() {
		t.Fatalf("hint = %q, want %q", raw["hint"], errRejectingErrorWriter.Error())
	}
}
