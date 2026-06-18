package steps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/hotchkj/glyph-shift/cmd"
	"github.com/hotchkj/glyph-shift/internal/fsnorm"
	"github.com/hotchkj/glyph-shift/internal/mcpserver"
	"github.com/hotchkj/glyph-shift/internal/testutil"
)

// Layer 1 scenarios (operation behavior specs): extract, split, blocks, and transform
// operation specs dispatch their pipelines directly via runExtractDirect, runSplitDirect,
// runBlocksDirect, and runTransformDirect (no CLI stdout/stderr for those operations).
// Layer 1 may call runGlyphShiftSubcommand only for MCP CLI surface checks (e.g. "mcp --help"),
// which mirrors main.go MCP routing and asserts via stdout/stderr/ExitCode.
//
// Layer 2 scenarios (CLI/MCP JSON contract specs) use runGlyphShiftMocked (CLI) and
// invokeMCPToolMocked, which invokes [mcpserver.GlyphShiftServer.InvokeRegisteredTool] over an
// in-memory MCP transport (tools/call). Those steps pair a mocked [pipeline.Runner]
// with deterministic pipeline results while asserting CLI stdout/stderr JSON, exit codes, stream separation,
// and MCP structuredContent payloads.
// Layer 2 does not assert file byte contents — that belongs to Layer 1.
//
// Binary composition surfaces are RunProductionCLI, DispatchProductionCLI, and RunProductionMCP.
// JSON contract scenarios mirror the production CLI routing using injected [cmd.DispatchCLI] and the
// MCP stdio lifecycle via injected [cmd.RunMCPServer]; this matches broader dependency injection in cmd.
//
// Both layers use in-process invocation (no subprocess binary). Layer 1 does not route
// operation subcommands through cmd.DispatchCLI; Layer 2 injects MockRunner via the
// pipeline.Runner interface for JSON contract proofs.

// runGlyphShiftSubcommand runs argv after the leading "glyph-shift" token. Only the "mcp" subcommand
// is supported here (mirrors main.go routing into RunProductionMCP, but via RunMCPServer
// with a mock runner). Any other argv returns errLayer1GenericCLINotAllowed
// so specs use direct runners or Layer 2 mocked CLI instead of ad hoc CLI strings.
func (tc *TestContext) runGlyphShiftSubcommand(args []string) error {
	var stdout, stderr bytes.Buffer

	dir := fsnorm.DirNative(tc.Ws.Root())

	if len(args) > 0 && args[0] == "mcp" {
		tc.ExitCode = cmd.RunMCPServer(
			args[1:], "test", dir, bytes.NewReader(nil), &stdout, &stderr, cmd.MCPServerDeps{
				Runner:   ensureMockRunner(tc),
				Resolver: testutil.NewMemPathResolverWithFS(tc.Ws.FS()),
			},
		)
		tc.Stdout = stdout.String()
		tc.Stderr = stderr.String()

		return nil
	}

	sub := "(none)"
	if len(args) > 0 {
		sub = args[0]
	}

	return fmt.Errorf("%w (subcommand %q)", errLayer1GenericCLINotAllowed, sub)
}

// runGlyphShiftMocked runs CLI dispatch with the mock runner (Layer 2).
// The workspace root still exists for path validation, but pipeline operations
// return canned results from tc.MockRunner.
func (tc *TestContext) runGlyphShiftMocked(args []string) error {
	ensureMockRunner(tc)

	var stdout, stderr bytes.Buffer

	dir := fsnorm.DirNative(tc.Ws.Root())
	tc.ExitCode = cmd.DispatchCLI(args, "test", dir, &stdout, &stderr, tc.MockRunner)
	tc.Stdout = stdout.String()
	tc.Stderr = stderr.String()

	return nil
}

// invokeMCPToolMocked invokes a registered MCP tool through the server's tools/call path over an in-memory transport.
func (tc *TestContext) invokeMCPToolMocked(toolName string, args map[string]any) error {
	ensureMockRunner(tc)

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("marshal MCP tool arguments: %w", err)
	}

	srv, ctorErr := mcpserver.NewGlyphShiftServerFromRunner(
		tc.Ws.Root(),
		"test",
		tc.MockRunner,
		testutil.NewMemPathResolverWithFS(tc.Ws.FS()),
	)
	if ctorErr != nil {
		return fmt.Errorf("construct MCP server: %w", ctorErr)
	}

	result, err := srv.InvokeRegisteredTool(context.Background(), toolName, argsJSON)
	if err != nil {
		return fmt.Errorf("invoke MCP tool %q: %w", toolName, err)
	}

	return tc.captureLastMCPToolResult(result)
}
