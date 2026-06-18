package pipeline_test

import (
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fsnorm"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

const goosWindows = "windows"

func testPreparePathWorkspaceRoot(t *testing.T) string {
	t.Helper()

	raw := filepath.Join(string([]rune{filepath.Separator}), "pipeline-path-prepare-root")
	absRoot, err := filepath.Abs(raw)
	if err != nil {
		t.Fatal(err)
	}

	return filepath.Clean(absRoot)
}

func TestPreparePath_RejectsEmpty(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string([]rune{filepath.Separator}), "ws")
	_, err := pipeline.PreparePath("", root)
	if !errors.Is(err, pipeline.ErrEmptyPreparedPath) {
		t.Fatalf("got err %v, want ErrEmptyPreparedPath", err)
	}
}

func TestPreparePath_MatchesReferenceLexicalCompose(t *testing.T) {
	t.Parallel()

	root := testPreparePathWorkspaceRoot(t)
	rel := filepath.FromSlash("rel/sub/file.txt")

	got, err := pipeline.PreparePath(rel, root)
	if err != nil {
		t.Fatalf("PreparePath: %v", err)
	}

	want, absErr := filepath.Abs(fsnorm.ResolveUnderWorkspace(fsnorm.Canonical(rel), root))
	if absErr != nil {
		t.Fatalf("Abs: %v", absErr)
	}
	want = filepath.Clean(want)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPreparePath_PreservesInternalSpacingWithoutTrimSpace(t *testing.T) {
	t.Parallel()

	root := testPreparePathWorkspaceRoot(t)
	rel := filepath.FromSlash(" spaced-name.txt ")
	got, err := pipeline.PreparePath(rel, root)
	if err != nil {
		t.Fatalf("PreparePath: %v", err)
	}

	want, absErr := filepath.Abs(fsnorm.ResolveUnderWorkspace(fsnorm.Canonical(rel), root))
	if absErr != nil {
		t.Fatalf("Abs: %v", absErr)
	}
	want = filepath.Clean(want)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	gotBase := filepath.Base(got)
	switch runtime.GOOS {
	case goosWindows:
		wantBase := filepath.Base(want)
		if gotBase != wantBase {
			t.Fatalf(`basename GOOS windows: got %q want host reference %q`, gotBase, wantBase)
		}
	default:
		if gotBase != rel {
			t.Fatalf(`basename got %q want %q`, gotBase, rel)
		}
	}
}

func TestPreparePath_LeadingSpaceBasenamePOSIX(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("POSIX-only: Windows path normalization differs for spaced basenames")
	}
	t.Parallel()

	root := testPreparePathWorkspaceRoot(t)
	const rel = " spaced.txt "
	got, err := pipeline.PreparePath(rel, root)
	if err != nil {
		t.Fatalf("PreparePath: %v", err)
	}

	if filepath.Base(got) != rel {
		t.Fatalf(`basename got %q want %q`, filepath.Base(got), rel)
	}
}

func TestPreparePath_WhitespaceOnlyIsLiteralRelativeSegmentPOSIX(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("POSIX-only: space-only lexical path segments are not exercised on Windows runners")
	}
	t.Parallel()

	root := testPreparePathWorkspaceRoot(t)
	const wsOnly = "   "
	got, err := pipeline.PreparePath(wsOnly, root)
	if err != nil {
		t.Fatalf("PreparePath: %v", err)
	}

	if filepath.Base(got) != wsOnly {
		t.Fatalf(`basename got %q want literal %q`, filepath.Base(got), wsOnly)
	}
}
