package cmd

import (
	"errors"
	"flag"
	"io"
	"strings"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

func flagNameFromFlagParseError(msg string) string {
	const notDef = "not defined: "
	if i := strings.LastIndex(msg, notDef); i >= 0 {
		return strings.TrimSpace(msg[i+len(notDef):])
	}

	const notDefAlt = "not defined: -"
	if i := strings.Index(msg, notDefAlt); i >= 0 {
		return strings.TrimSpace(msg[i+len("not defined: "):])
	}

	return ""
}

// trailingArgumentHint describes allowed flags/shape when parseSubcommandFlags rejects
// extra positional tokens after fs.Parse succeeds.
func trailingArgumentHint(subCmd string) string {
	switch subCmd {
	case subcmdExtract:
		return `extract accepts --source, --lines, --destination, plus optional --force, --append, ` +
			`--mkdir, and --preview; no trailing positional arguments. Other CLI commands ` +
			`include split, blocks, transform, version, mcp, and help.`
	case subcmdSplit:
		return `split accepts --source, --delimiter, --output-dir, plus optional --extension, --names, ` +
			`--max-files, --strip-delimiter, --force, --mkdir, and --preview; no trailing ` +
			`positional arguments. Other CLI commands include extract, blocks, transform, ` +
			`version, mcp, and help.`
	case subcmdBlocks:
		return `blocks accepts --source, --start-line, --end-line, --output-dir, plus optional --extension, ` +
			`--names, --max-files, --include-delimiters, --force, --mkdir, and --preview; ` +
			`no trailing positional arguments. Other CLI commands include extract, split, ` +
			`transform, version, mcp, and help.`
	case subcmdTransform:
		return `transform accepts required --source plus optional --line-endings (lf|crlf|cr), ` +
			`--trim-trailing, --final-newline, and --preview; no trailing positional ` +
			`arguments. Other CLI commands include extract, split, blocks, version, mcp, ` +
			`and help.`
	case "version":
		return `version accepts optional -h or --help only; no positional arguments or other ` +
			`flags. Other CLI commands include extract, split, blocks, transform, mcp, ` +
			`and help.`
	case "mcp":
		return `mcp accepts optional --workspace-root; no trailing positional arguments. ` +
			`Related CLI commands include extract, split, blocks, transform, version, ` +
			`and help.`
	default:
		return ""
	}
}

// parseSubcommandFlags parses args with FlagSet output discarded. The standard library's
// flag.failf calls Usage before returning; we temporarily replace Usage with a no-op during
// Parse so undefined flags do not print usage to stdout. When ErrHelp is returned, the
// original Usage was suppressed during parseOne; we invoke it here so --help still works.
// Other parse errors write JSON invalid_flag to stderr and expect exit exitValidation.
// When trailingSubCmdHintKey is non-empty and fs.Arg(0+) remain after parsing, writes
// unexpected_argument JSON to stderr and returns exitValidation.
func parseSubcommandFlags(
	fs *flag.FlagSet,
	args []string,
	stderr io.Writer,
	trailingSubCmdHintKey string,
	workspaceRoot string,
) (stop bool, exitCode int) {
	orig := fs.Usage

	fs.Usage = func() {
		// Std flag.failf invokes Usage; suppressed here and invoked after Parse for ErrHelp only.
	}
	fs.SetOutput(io.Discard)

	err := fs.Parse(args)

	fs.Usage = orig

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			orig()

			return true, 0
		}

		flagName := flagNameFromFlagParseError(err.Error())
		if strings.TrimSpace(flagName) == "" {
			flagName = "unknown"
		}

		writeErrorJSON(stderr, workspaceRoot, &pipeline.ErrorOutcome{
			Error:    "invalid_flag",
			Hint:     err.Error(),
			ExitCode: exitValidation,
			StringFields: map[string]string{
				"flag": flagName,
			},
		})

		return true, exitValidation
	}

	if trailingSubCmdHintKey != "" {
		if hint := trailingArgumentHint(trailingSubCmdHintKey); hint != "" && fs.NArg() > 0 {
			writeErrorJSON(stderr, workspaceRoot, &pipeline.ErrorOutcome{
				Error:    "unexpected_argument",
				Hint:     hint,
				ExitCode: exitValidation,
				StringFields: map[string]string{
					"argument": fs.Arg(0),
				},
			})

			return true, exitValidation
		}
	}

	return false, 0
}

func flagProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			provided = true
		}
	})

	return provided
}

func allFlagsProvided(fs *flag.FlagSet, names ...string) bool {
	for _, name := range names {
		if !flagProvided(fs, name) {
			return false
		}
	}

	return true
}
