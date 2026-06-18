package mcpserver

import (
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

func mustNewDefaultRunner(
	tb testing.TB,
	src pipeline.SourceOpener,
	out pipeline.OutputOpener,
	stater pipeline.FileStater,
	fs fileops.FileSession,
) *pipeline.DefaultRunner {
	tb.Helper()

	runner, err := pipeline.NewDefaultRunner(
		src,
		out,
		stater,
		testutil.NoSymlinkPathResolver{},
		fs,
	)
	if err != nil {
		tb.Fatalf("NewDefaultRunner: %v", err)
	}

	return runner
}

func mustNewGlyphShiftServer(
	tb testing.TB,
	workspaceRoot string,
	src pipeline.SourceOpener,
	out pipeline.OutputOpener,
	stater pipeline.FileStater,
	fs fileops.FileSession,
) *GlyphShiftServer {
	tb.Helper()

	srv, err := NewGlyphShiftServer(workspaceRoot, "t", src, out, stater, testutil.NoSymlinkPathResolver{}, fs)
	if err != nil {
		tb.Fatalf("NewGlyphShiftServer: %v", err)
	}

	return srv
}

func mustNewGlyphShiftServerFromRunner(
	tb testing.TB,
	workspaceRoot, version string,
	runner pipeline.Runner,
	resolver validate.PathResolver,
) *GlyphShiftServer {
	tb.Helper()

	srv, err := NewGlyphShiftServerFromRunner(workspaceRoot, version, runner, resolver)
	if err != nil {
		tb.Fatalf("NewGlyphShiftServerFromRunner: %v", err)
	}

	return srv
}
