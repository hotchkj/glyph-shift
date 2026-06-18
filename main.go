// glyph-shift is a byte-faithful file operation binary for agent workflows.
//
// The binary exposes four mechanical operations: extract, split, blocks, and
// transform. Operation subcommands execute by default, write success JSON to
// stdout, and write failure JSON to stderr. Help and version output are plain
// operational metadata and are not part of the operation JSON contract.
//
// The mcp subcommand runs the same operation surface as an MCP server over
// stdio. CLI flags use kebab-case; MCP tool inputs use snake_case for the same
// logical arguments. Both paths delegate to the same pipeline layer so byte
// movement, validation, and error classification stay aligned.
//
// Product intent lives in docs/glyph-shift-intent.md. The operation payload
// contract lives in docs/glyph-shift-json-contract.md.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/hotchkj/glyph-shift/cmd"
)

var version = "dev"

// mainDeps binds process wiring for [dispatchMain]: working-directory lookup, stdio streams, and MCP/CLI entry points.
type mainDeps struct {
	getwd  func() (string, error)
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	runMCP func(args []string, version, cwd string, stdin io.Reader, stdout, stderr io.Writer) int
	runCLI func(args []string, version string) int
}

func defaultMainDeps() mainDeps {
	return mainDeps{
		getwd:  os.Getwd,
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
		runMCP: cmd.RunProductionMCP,
		runCLI: cmd.RunProductionCLI,
	}
}

// dispatchMain routes argv between MCP and CLI entry points and returns a process exit code.
// It must not call [os.Exit]; [main] owns process termination.
func dispatchMain(args []string, version string, deps mainDeps) int {
	if len(args) > 0 && args[0] == "mcp" {
		cwd, err := deps.getwd()
		if err != nil {
			_, _ = fmt.Fprintf(deps.stderr, "get working directory: %v\n", err)
			return 1
		}
		return deps.runMCP(args[1:], version, cwd, deps.stdin, deps.stdout, deps.stderr)
	}
	return deps.runCLI(args, version)
}

func main() {
	os.Exit(dispatchMain(os.Args[1:], version, defaultMainDeps()))
}
