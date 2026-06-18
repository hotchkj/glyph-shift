package cmd

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/fsnorm"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

// RunProductionCLI is the main-process CLI entry point for non-MCP commands.
// It reads the working directory from the OS and delegates to DispatchProductionCLI
// with process stdout/stderr (production composition root).
func RunProductionCLI(args []string, version string) int {
	cwd, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "get working directory: %v\n", err)

		return 1
	}

	return DispatchProductionCLI(args, version, cwd, os.Stdout, os.Stderr)
}

// DispatchProductionCLI parses CLI flags, dispatches subcommands, and writes JSON output
// to stdout on success and JSON error objects to stderr on failure.
// It constructs pipeline Runner wiring from injected OS-backed fileops and validate seams:
// one shared FileSession for source locking and atomic publication, resolver, and adapters.
// The dir argument is the process working directory used as the
// workspace root for path validation; it must not be read from the environment here.
func DispatchProductionCLI(args []string, version, dir string, stdout, stderr io.Writer) int {
	return dispatchCLIWithRunnerFactory(args, version, dir, stdout, stderr, newProductionPipelineRunner)
}

type pipelineRunnerFactories struct {
	newSession      func() fileops.FileSession
	newSourceOpener func(fileops.FileSession) (pipeline.SourceOpener, error)
	newOutputOpener func() pipeline.OutputOpener
	newFileStater   func() pipeline.FileStater
	newPathResolver func() validate.PathResolver
}

var productionPipelineFactories = pipelineRunnerFactories{
	newSession:      fileops.NewOSFileSession,
	newSourceOpener: pipeline.NewOSSourceOpener,
	newOutputOpener: pipeline.NewOSOutputOpener,
	newFileStater:   pipeline.NewOSFileStater,
	newPathResolver: validate.NewOSPathResolver,
}

func newProductionPipelineRunner() (pipeline.Runner, error) {
	return newPipelineRunnerFromFactories(productionPipelineFactories)
}

func newPipelineRunnerFromFactories(factories pipelineRunnerFactories) (pipeline.Runner, error) {
	sess := factories.newSession()

	srcOpener, err := factories.newSourceOpener(sess)
	if err != nil {
		return nil, err
	}

	return pipeline.NewDefaultRunner(
		srcOpener, factories.newOutputOpener(),
		factories.newFileStater(), factories.newPathResolver(),
		sess,
	)
}

// DispatchCLI dispatches CLI subcommands with the same routing and output policy as [DispatchProductionCLI],
// except the caller injects pipeline.Runner (for example in-memory fakes replacing OS-backed seams).
// Returns 1 if stdout, stderr, or runner is nil.
func DispatchCLI(args []string, version, dir string, stdout, stderr io.Writer, runner pipeline.Runner) int {
	return dispatchCLIWithRunnerFactory(args, version, dir, stdout, stderr, func() (pipeline.Runner, error) {
		return runner, nil
	})
}

func dispatchCLIWithRunnerFactory(
	args []string,
	version, dir string,
	stdout, stderr io.Writer,
	runnerFactory func() (pipeline.Runner, error),
) int {
	runner, factoryErr := runnerFactory()
	if factoryErr != nil {
		writePipelineRunnerFactoryError(stderr, factoryErr)

		return 1
	}
	if stdout == nil || stderr == nil || runner == nil {
		return 1
	}

	dir = fsnorm.DirNative(dir)

	if len(args) == 0 {
		printUsage(stdout)

		return 0
	}

	return dispatchCommand(args, version, dir, stdout, stderr, runner)
}

func writePipelineRunnerFactoryError(stderr io.Writer, factoryErr error) {
	if stderr == nil || factoryErr == nil {
		return
	}

	_, _ = fmt.Fprintf(stderr, "pipeline runner: %v\n", factoryErr)
}

func dispatchCommand(args []string, version, dir string, stdout, stderr io.Writer, runner pipeline.Runner) int {
	first := args[0]

	switch first {
	case "version":
		return runVersion(args[1:], version, dir, stdout, stderr)
	case subcmdExtract:
		return runExtract(args[1:], dir, stdout, stderr, runner)
	case subcmdSplit:
		return runSplit(args[1:], dir, stdout, stderr, runner)
	case subcmdBlocks:
		return runBlocks(args[1:], dir, stdout, stderr, runner)
	case subcmdTransform:
		return runTransform(args[1:], dir, stdout, stderr, runner)
	case "--help", "-h", "help":
		printUsage(stdout)

		return 0
	default:
		return unknownCommand(first, dir, stderr)
	}
}

func versionUsage(fs *flag.FlagSet, stdout io.Writer) {
	_, _ = fmt.Fprintln(stdout, `Usage: glyph-shift version

Print the glyph-shift release version string.`)

	fs.SetOutput(stdout)
	fs.PrintDefaults()
}

func runVersion(args []string, versionStr, dir string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)

	fs.Usage = func() { versionUsage(fs, stdout) }

	if stop, code := parseSubcommandFlags(fs, args, stderr, "version", dir); stop {
		return code
	}

	_, _ = fmt.Fprintf(stdout, "glyph-shift %s\n", versionStr)

	return 0
}

func unknownCommand(name, workspaceRoot string, stderr io.Writer) int {
	writeErrorJSON(stderr, workspaceRoot, &pipeline.ErrorOutcome{
		Error:    "unknown_command",
		Hint:     "Run glyph-shift --help to see available commands.",
		ExitCode: exitValidation,
		StringFields: map[string]string{
			"command": name,
		},
	})

	return exitValidation
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, `glyph-shift - byte-faithful file operations

Usage: glyph-shift <command> [options]
    or: glyph-shift mcp [options]

Commands:
  extract     Extract a line range from a file
  split       Split a file by delimiter pattern
  blocks      Extract fenced blocks from a file
  transform   Transform line endings and whitespace
  version     Print version

Run as MCP server: glyph-shift mcp [--workspace-root <dir>]

Use "glyph-shift <command> --help" for more information about a command.`)
}
