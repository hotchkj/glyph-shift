package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/fsnorm"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

// Sentinel errors returned by MCP server constructors.

var (
	ErrNilRunner        = errors.New("mcpserver: nil Runner")
	ErrNilPathResolver  = errors.New("mcpserver: nil PathResolver")
	ErrNilSourceOpener  = errors.New("mcpserver: nil SourceOpener")
	ErrNilOutputOpener  = errors.New("mcpserver: nil OutputOpener")
	ErrNilFileStater    = errors.New("mcpserver: nil FileStater")
	ErrNilFileSession   = errors.New("mcpserver: nil FileSession")
	ErrWorkspaceRoot    = errors.New("mcpserver: invalid workspace root")
	ErrWorkspaceRootAbs = errors.New("mcpserver: workspace root absolute resolution")
)

func normalizeMCPServerWorkspaceRoot(workspaceRoot string) (string, error) {
	if strings.ContainsRune(workspaceRoot, 0) {
		return "", fmt.Errorf("%w: NUL byte", ErrWorkspaceRoot)
	}

	root := fsnorm.DirNative(workspaceRoot)

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrWorkspaceRootAbs, err)
	}

	return filepath.Clean(absRoot), nil
}

// GlyphShiftServer configures workspace-scoped MCP tool handlers.
type GlyphShiftServer struct {
	WorkspaceRoot string
	Version       string
	// mu serializes all tool calls to prevent path-conflict races (two concurrent
	// calls writing to the same destination). With single-file-per-call operations
	// the hold time is bounded by one file read + transform + write, which is
	// acceptable for typical workloads. If profiling later shows contention,
	// narrow to per-path locking rather than removing the mutex.
	mu       sync.Mutex
	runner   pipeline.Runner
	resolver validate.PathResolver
}

// NewGlyphShiftServer returns a server config with an absolute, cleaned workspace root.
func NewGlyphShiftServer(workspaceRoot, version string,
	src pipeline.SourceOpener, out pipeline.OutputOpener,
	stater pipeline.FileStater, resolver validate.PathResolver,
	fs fileops.FileSession,
) (*GlyphShiftServer, error) {
	if err := validateGlyphShiftServerDeps(src, out, stater, resolver, fs); err != nil {
		return nil, err
	}

	absRoot, err := normalizeMCPServerWorkspaceRoot(workspaceRoot)
	if err != nil {
		return nil, err
	}

	runner, rErr := pipeline.NewDefaultRunner(src, out, stater, resolver, fs)
	if rErr != nil {
		return nil, rErr
	}

	return &GlyphShiftServer{
		WorkspaceRoot: absRoot,
		Version:       version,
		runner:        runner,
		resolver:      resolver,
	}, nil
}

func validateGlyphShiftServerDeps(
	src pipeline.SourceOpener,
	out pipeline.OutputOpener,
	stater pipeline.FileStater,
	resolver validate.PathResolver,
	fs fileops.FileSession,
) error {
	if src == nil {
		return ErrNilSourceOpener
	}
	if out == nil {
		return ErrNilOutputOpener
	}
	if stater == nil {
		return ErrNilFileStater
	}
	if resolver == nil {
		return ErrNilPathResolver
	}
	if fs == nil {
		return ErrNilFileSession
	}

	return nil
}

// NewGlyphShiftServerFromRunner returns a workspace-scoped server wired with caller-supplied [pipeline.Runner]
// and path resolver (including production OS-backed seams and substitutes used in diagnostics or tests).
func NewGlyphShiftServerFromRunner(workspaceRoot, version string,
	runner pipeline.Runner, resolver validate.PathResolver,
) (*GlyphShiftServer, error) {
	if runner == nil {
		return nil, ErrNilRunner
	}
	if resolver == nil {
		return nil, ErrNilPathResolver
	}

	absRoot, err := normalizeMCPServerWorkspaceRoot(workspaceRoot)
	if err != nil {
		return nil, err
	}

	return &GlyphShiftServer{
		WorkspaceRoot: absRoot,
		Version:       version,
		runner:        runner,
		resolver:      resolver,
	}, nil
}

// validateToolPath resolves rawPath using pipeline.PreparePath (same lexical policy as CLI),
// enforces containment via validate.ValidatePath with s.resolver, and returns an absolute, cleaned path.
func (s *GlyphShiftServer) validateToolPath(rawPath string) (string, error) {
	candidate, err := s.resolveToolPath(rawPath)
	if err != nil {
		return "", err
	}

	if err := validate.ValidatePath(candidate, s.WorkspaceRoot, s.resolver); err != nil {
		return "", err
	}

	return filepath.Clean(candidate), nil
}

// resolveToolPath applies pipeline.PreparePath; it does not trim path strings and rejects only
// an explicitly empty rawPath (invalid_input via ClassifyOperationError).
func (s *GlyphShiftServer) resolveToolPath(rawPath string) (string, error) {
	return pipeline.PreparePath(rawPath, s.WorkspaceRoot)
}

func mustToolInputSchemaForType[T any]() any {
	rt := reflect.TypeFor[T]()
	s, err := jsonschema.ForType(rt, &jsonschema.ForOptions{})
	if err != nil {
		panic(fmt.Sprintf("mcpserver: derive tool input schema for %s: %v", rt.String(), err))
	}

	return s
}

func (s *GlyphShiftServer) extractToolLowLevel(
	ctx context.Context, req *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	input, err := decodeExtractToolArgs(normalizedToolArgumentsJSON(req.Params.Arguments))
	if err != nil {
		return unexpectedArgumentToolResult(s.WorkspaceRoot, toolExtract, err)
	}

	result, _, callErr := s.handleExtractTool(ctx, req, input)

	return result, callErr
}

func (s *GlyphShiftServer) splitToolLowLevel(
	ctx context.Context, req *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	input, err := decodeSplitToolArgs(normalizedToolArgumentsJSON(req.Params.Arguments))
	if err != nil {
		return unexpectedArgumentToolResult(s.WorkspaceRoot, toolSplit, err)
	}

	result, _, callErr := s.handleSplitTool(ctx, req, input)

	return result, callErr
}

func (s *GlyphShiftServer) blocksToolLowLevel(
	ctx context.Context, req *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	input, err := decodeBlocksToolArgs(normalizedToolArgumentsJSON(req.Params.Arguments))
	if err != nil {
		return unexpectedArgumentToolResult(s.WorkspaceRoot, toolBlocks, err)
	}

	result, _, callErr := s.handleBlocksTool(ctx, req, input)

	return result, callErr
}

func (s *GlyphShiftServer) transformToolLowLevel(
	ctx context.Context, req *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	input, err := decodeTransformToolArgs(normalizedToolArgumentsJSON(req.Params.Arguments))
	if err != nil {
		return unexpectedArgumentToolResult(s.WorkspaceRoot, toolTransform, err)
	}

	result, _, callErr := s.handleTransformTool(ctx, req, input)

	return result, callErr
}

// NewMCPServer builds an MCP server with extract, split, blocks, and transform tools.
func (s *GlyphShiftServer) NewMCPServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "glyph-shift", Version: s.Version}, nil)

	srv.AddTool(&mcp.Tool{
		Name:         toolExtract,
		Description:  "Extract a 1-based inclusive line range from source to destination, byte-faithfully.",
		InputSchema:  mustToolInputSchemaForType[ExtractInput](),
		OutputSchema: extractOutputSchema(),
	}, s.extractToolLowLevel)

	srv.AddTool(&mcp.Tool{
		Name:         toolSplit,
		Description:  "Split a file into multiple files at each line matching a delimiter regex.",
		InputSchema:  mustToolInputSchemaForType[SplitInput](),
		OutputSchema: splitOutputSchema(),
	}, s.splitToolLowLevel)

	srv.AddTool(&mcp.Tool{
		Name:         toolBlocks,
		Description:  "Extract lines between start and end delimiter patterns into separate files.",
		InputSchema:  mustToolInputSchemaForType[BlocksInput](),
		OutputSchema: blocksOutputSchema(),
	}, s.blocksToolLowLevel)

	srv.AddTool(&mcp.Tool{
		Name: toolTransform,
		Description: "Transform line endings and whitespace in-place; executes by default " +
			"(set preview true to inspect without writing).",
		InputSchema:  mustToolInputSchemaForType[TransformInput](),
		OutputSchema: transformOutputSchema(),
	}, s.transformToolLowLevel)

	return srv
}
