package mcpserver

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/testutil"
)

func testWorkspaceRoot() string {
	return filepath.Join(string([]rune{filepath.Separator}), "mcpserver-max-files-test-root")
}

func TestSplit_explicitMaxFilesZeroIsRejected(t *testing.T) {
	t.Parallel()

	root := testWorkspaceRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	st := testutil.NewMemFileStater()
	sess := testutil.NewMemPublishSessionForOutput(out)

	srv := mustNewGlyphShiftServer(
		t, root,
		src,
		out,
		st,
		sess,
	)

	explicitZero := 0
	_, err := srv.handleSplit(context.Background(), nil, SplitInput{
		Source:    "in.txt",
		Delimiter: "^a$",
		OutputDir: "out",
		Mkdir:     true,
		MaxFiles:  intPtr(explicitZero),
	})
	if err == nil {
		t.Fatal("expected error for max_files 0")
	}
}

func TestBlocks_explicitMaxFilesZeroIsRejected(t *testing.T) {
	t.Parallel()

	root := testWorkspaceRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	st := testutil.NewMemFileStater()
	sess := testutil.NewMemPublishSessionForOutput(out)

	srv := mustNewGlyphShiftServer(
		t, root,
		src,
		out,
		st,
		sess,
	)

	explicitZero := 0
	_, err := srv.handleBlocks(context.Background(), nil, BlocksInput{
		Source:    "in.txt",
		StartLine: "^x$",
		EndLine:   "^x$",
		OutputDir: "out",
		Mkdir:     true,
		MaxFiles:  intPtr(explicitZero),
	})
	if err == nil {
		t.Fatal("expected error for max_files 0")
	}
}
