package mcpserver

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

var (
	errTestAccessDenied = errors.New("open D:\\GIT\\GLYPH-SHIFT\\file.txt: access denied")
	errTestNotFound     = errors.New("open D:/Git/glyph-shift/file.txt: not found")
	errTestUNCPath      = errors.New("open \\\\server\\share\\file.txt: error")
)

func assertSanitizedMessage(t *testing.T, err error, root, want string) {
	t.Helper()
	sanitized := sanitizeError(err, root)
	if sanitized.Error() != want {
		t.Fatalf("unexpected sanitized message: %q", sanitized.Error())
	}
}

func TestSanitizeError_MixedCaseWindows(t *testing.T) {
	t.Parallel()
	assertSanitizedMessage(t, errTestAccessDenied, "D:\\Git\\glyph-shift", "open file.txt: access denied")
}

func TestSanitizeError_ForwardSlashPath(t *testing.T) {
	t.Parallel()
	assertSanitizedMessage(t, errTestNotFound, "D:\\Git\\glyph-shift", "open file.txt: not found")
}

func TestSanitizeError_UNCPath(t *testing.T) {
	t.Parallel()
	assertSanitizedMessage(t, errTestUNCPath, "\\\\server\\share", "open file.txt: error")
}

func TestNewGlyphShiftServerFromRunner_storesSamePathResolverInstance(t *testing.T) {
	t.Parallel()

	resolver := &constructorPathResolver{}
	server := mustNewGlyphShiftServerFromRunner(t, "/workspace", "test", constructorRunner{}, resolver)
	if server.resolver != resolver {
		t.Fatalf("resolver: got %#v want same instance %#v", server.resolver, resolver)
	}
}

func TestNewGlyphShiftServerFromRunner_rejectsNULWorkspaceRoot(t *testing.T) {
	t.Parallel()

	_, err := NewGlyphShiftServerFromRunner(
		string([]byte{0}), "test", constructorRunner{}, &constructorPathResolver{},
	)
	if !errors.Is(err, ErrWorkspaceRoot) {
		t.Fatalf("error = %v want %v", err, ErrWorkspaceRoot)
	}
}

func TestNewMCPServerRegistersToolSet(t *testing.T) {
	t.Parallel()

	server := mustNewGlyphShiftServerFromRunner(
		t, "/workspace", "test-version", constructorRunner{}, &constructorPathResolver{},
	)
	if got := server.NewMCPServer(); got == nil {
		t.Fatal("NewMCPServer returned nil")
	}
}

type constructorPathResolver struct{}

func (*constructorPathResolver) Lstat(string) (fs.FileInfo, error) {
	return nil, fs.ErrNotExist
}

func (*constructorPathResolver) EvalSymlinks(path string) (string, error) {
	return path, nil
}

type constructorRunner struct{}

func (constructorRunner) RunExtract(context.Context, pipeline.ExtractParams) (fileops.ExtractResult, error) {
	return fileops.ExtractResult{}, nil
}

func (constructorRunner) RunSplit(context.Context, pipeline.SplitParams) (pipeline.SplitPipelineResult, error) {
	return pipeline.SplitPipelineResult{}, nil
}

func (constructorRunner) RunBlocks(context.Context, pipeline.BlocksParams) (pipeline.BlocksPipelineResult, error) {
	return pipeline.BlocksPipelineResult{}, nil
}

func (constructorRunner) RunTransform(
	context.Context,
	pipeline.TransformParams,
) (pipeline.TransformPipelineResult, error) {
	return pipeline.TransformPipelineResult{}, nil
}
