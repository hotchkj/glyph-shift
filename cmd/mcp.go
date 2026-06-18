// User vision: MCP mode should use the same injected, diagnosable pipeline seams as the CLI.
package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/fsnorm"
	"github.com/hotchkj/glyph-shift/internal/mcpserver"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// mcpPreconditionFailurePrefix prefixes plain-text MCP startup precondition diagnostics on stderr.
const mcpPreconditionFailurePrefix = "mcp server precondition failed:"

var (
	newProductionMCPFileSession  = fileops.NewOSFileSession
	newProductionMCPPathResolver = validate.NewOSPathResolver
	newProductionMCPOutputOpener = pipeline.NewOSOutputOpener
	newProductionMCPFileStater   = pipeline.NewOSFileStater
)

// MCPServerDeps are the injected runtime dependencies for [RunMCPServer].
type MCPServerDeps struct {
	Runner   pipeline.Runner
	Resolver validate.PathResolver
}

func mcpUsage(fs *flag.FlagSet, stdout io.Writer) {
	_, _ = fmt.Fprintln(stdout, `Usage: glyph-shift mcp [options]

Run as an MCP (Model Context Protocol) server over stdio.

Options:`)
	fs.SetOutput(stdout)
	fs.PrintDefaults()
}

// RunMCPServer parses MCP server flags, resolves the MCP workspace root,
// builds the workspace-scoped server with the supplied runner and resolver, and runs the JSON-RPC transport
// on stdin/stdout. defaultWorkspace is used when --workspace-root is not set.
//
// When stderr is non-nil, MCP startup precondition failures (nil stdin/stdout/stderr or nil injected
// dependencies) emit a single plain-text line prefixed with `mcp server precondition failed:` listing the failures.
// Workspace-root construction failures ([mcpserver.NewGlyphShiftServerFromRunner]) and transport/runtime errors
// emit plain-text lines prefixed with `mcp server init error` and `mcp server error` respectively.
// Invalid MCP flags are handled via [parseSubcommandFlags], which emits the same stderr JSON envelope
// as other CLI validation paths.
//
// Exit code is non-zero for all of those failure modes. When stderr is nil and preconditions fail,
// returns 1 without writing diagnostics (no fallback writer is synthesized).
func RunMCPServer(
	args []string, version, defaultWorkspace string,
	stdin io.Reader, stdout, stderr io.Writer,
	deps MCPServerDeps,
) int {
	if invalidMCPServerInputs(stdin, stdout, stderr, deps) {
		writeMCPServerPreconditionDiag(stderr, stdin, stdout, deps)

		return 1
	}

	defaultWorkspace = fsnorm.DirNative(defaultWorkspace)

	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)

	var workspaceRoot string
	fs.StringVar(&workspaceRoot, "workspace-root", "", "workspace root directory (default: current directory)")

	fs.Usage = func() { mcpUsage(fs, stdout) }

	if stop, code := parseSubcommandFlags(fs, args, stderr, "mcp", defaultWorkspace); stop {
		return code
	}

	workspaceRoot = mcpWorkspaceRootOrDefault(workspaceRoot, defaultWorkspace)

	glyphShiftServer, ctorErr := mcpserver.NewGlyphShiftServerFromRunner(
		workspaceRoot, version, deps.Runner, deps.Resolver,
	)
	if ctorErr != nil {
		_, _ = fmt.Fprintf(stderr, "mcp server init error: %v\n", ctorErr)

		return 1
	}
	mcpSrv := glyphShiftServer.NewMCPServer()

	transport := &mcp.IOTransport{
		Reader: io.NopCloser(stdin),
		Writer: nopWriteCloser{stdout},
	}

	if err := mcpSrv.Run(context.Background(), transport); err != nil {
		_, _ = fmt.Fprintf(stderr, "mcp server error: %v\n", err)

		return 1
	}

	return 0
}

func invalidMCPServerInputs(stdin io.Reader, stdout, stderr io.Writer, deps MCPServerDeps) bool {
	return stdin == nil || stdout == nil || stderr == nil || deps.Runner == nil || deps.Resolver == nil
}

func writeMCPServerPreconditionDiag(stderr io.Writer, stdin io.Reader, stdout io.Writer, deps MCPServerDeps) {
	if stderr == nil {
		return
	}

	var problems []string
	if stdin == nil {
		problems = append(problems, "stdin is nil")
	}
	if stdout == nil {
		problems = append(problems, "stdout is nil")
	}

	if deps.Runner == nil {
		problems = append(problems, "pipeline runner is nil")
	}
	if deps.Resolver == nil {
		problems = append(problems, "path resolver is nil")
	}

	if len(problems) == 0 {
		return
	}

	_, _ = fmt.Fprintf(stderr, "%s %s\n", mcpPreconditionFailurePrefix, strings.Join(problems, "; "))
}

func mcpWorkspaceRootOrDefault(workspaceRoot, defaultWorkspace string) string {
	if workspaceRoot == "" {
		return defaultWorkspace
	}

	return fsnorm.DirNative(workspaceRoot)
}

// RunProductionMCP parses MCP server flags, constructs the workspace-scoped server with injected
// OS-backed pipeline seams (shared FileSession for source locking and atomic publication plus resolver),
// and runs the JSON-RPC transport on stdin/stdout.
// defaultWorkspace is used when --workspace-root is not set. Diagnostics follow [RunMCPServer].
func RunProductionMCP(args []string, version, defaultWorkspace string, stdin io.Reader, stdout, stderr io.Writer) int {
	deps, initErr := newProductionMCPDeps()
	if initErr != nil {
		_, _ = fmt.Fprintf(stderr, "mcp server init error: %v\n", initErr)

		return 1
	}

	return RunMCPServer(args, version, defaultWorkspace, stdin, stdout, stderr, deps)
}

func newProductionMCPDeps() (MCPServerDeps, error) {
	session := newProductionMCPFileSession()

	srcOpener, openErr := newProductionMCPSourceOpener(session)
	resolver := newProductionMCPPathResolver()
	runner, runErr := newProductionMCPRunner(srcOpener, resolver, session)

	return MCPServerDeps{
		Runner:   runner,
		Resolver: resolver,
	}, firstProductionMCPInitError(openErr, runErr)
}

func firstProductionMCPInitError(openErr, runErr error) error {
	if openErr != nil {
		return openErr
	}

	return runErr
}

func newProductionMCPSourceOpener(session fileops.FileSession) (pipeline.SourceOpener, error) {
	return pipeline.NewOSSourceOpener(session)
}

func newProductionMCPRunner(
	srcOpener pipeline.SourceOpener,
	resolver validate.PathResolver,
	session fileops.FileSession,
) (pipeline.Runner, error) {
	return pipeline.NewDefaultRunner(
		srcOpener, newProductionMCPOutputOpener(),
		newProductionMCPFileStater(), resolver,
		session,
	)
}
