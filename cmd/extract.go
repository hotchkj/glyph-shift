package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/linparse"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

type extractJSONOutput struct {
	LinesExtracted int `json:"lines_extracted"`
}

type extractPreviewOutput struct {
	WouldExtractLines int    `json:"would_extract_lines"`
	WouldCreate       string `json:"would_create"`
}

type extractFlagValues struct {
	source      string
	lines       string
	destination string
	force       bool
	appendMode  bool
	mkdir       bool
	preview     bool
}

func extractUsage(fs *flag.FlagSet, stdout io.Writer) {
	_, _ = fmt.Fprintln(stdout, `Usage: glyph-shift extract --source <path> --lines <range> --destination <path> [options]

Extract a 1-based inclusive line range from the source file to the destination.

Line range examples:
  45-55     lines 45 through 55 inclusive
  95-       from line 95 through end of file
  -10       from line 1 through line 10

Options:`)
	fs.SetOutput(stdout)
	fs.PrintDefaults()
}

func bindExtractFlags(fs *flag.FlagSet) *extractFlagValues {
	flags := &extractFlagValues{}

	fs.StringVar(&flags.source, "source", "", "source file path")
	fs.StringVar(&flags.lines, "lines", "", "line range (e.g. 45-55, 95-, -10)")
	fs.StringVar(&flags.destination, "destination", "", "destination file path")
	fs.BoolVar(&flags.force, "force", false, "overwrite existing destination")
	fs.BoolVar(&flags.appendMode, "append", false, "append to existing destination")
	fs.BoolVar(&flags.mkdir, "mkdir", false, "create destination parent directories")
	fs.BoolVar(&flags.preview, "preview", false, "report what would be extracted without writing the destination")

	return flags
}

func runExtractExecute(
	flags *extractFlagValues,
	start, end int,
	dir string,
	stdout, stderr io.Writer,
	runner pipeline.Runner,
) int {
	srcPath, prepErr := pipeline.PreparePath(flags.source, dir)
	if prepErr != nil {
		wrapped := pipeline.WithPathRole(pipeline.PathRoleSrc, flags.source, prepErr)
		return exitCodeForPipelineErr(wrapped, flags.source, dir, stderr)
	}

	destPath, prepErr := pipeline.PreparePath(flags.destination, dir)
	if prepErr != nil {
		wrapped := pipeline.WithPathRole(pipeline.PathRoleDest, flags.destination, prepErr)
		return exitCodeForPipelineErr(wrapped, flags.destination, dir, stderr)
	}

	params := pipeline.ExtractParams{
		SrcPath:  srcPath,
		DestPath: destPath,
		Root:     dir,
		Lines:    fileops.LineRange{Start: start, End: end},
		Force:    flags.force,
		Append:   flags.appendMode,
		Mkdir:    flags.mkdir,
		Preview:  flags.preview,
	}

	res, err := runner.RunExtract(context.Background(), params)
	if err != nil {
		return exitCodeForPipelineErr(err, srcPath, dir, stderr)
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")

	var outPayload any

	if flags.preview {
		outPayload = extractPreviewOutput{
			WouldExtractLines: res.LinesExtracted,
			WouldCreate:       res.WouldCreatePath,
		}
	} else {
		outPayload = extractJSONOutput{LinesExtracted: res.LinesExtracted}
	}

	if err := enc.Encode(outPayload); err != nil {
		writeFailedJSON(stderr, dir, err)

		return 1
	}

	return 0
}

// runExtract handles the "glyph-shift extract" subcommand.
func runExtract(
	args []string,
	dir string,
	stdout, stderr io.Writer,
	runner pipeline.Runner,
) int {
	fs := flag.NewFlagSet(subcmdExtract, flag.ContinueOnError)

	flags := bindExtractFlags(fs)

	fs.Usage = func() { extractUsage(fs, stdout) }

	if stop, code := parseSubcommandFlags(fs, args, stderr, subcmdExtract, dir); stop {
		return code
	}

	if !allFlagsProvided(fs, "source", "lines", "destination") {
		writeErrorJSON(stderr, dir, &pipeline.ErrorOutcome{
			Error:    "missing_required_flag",
			Hint:     "--source, --lines, and --destination are required",
			ExitCode: exitValidation,
			StringArrayFields: map[string][]string{
				"missing_flags": {"source", "lines", "destination"},
			},
		})

		return exitValidation
	}

	start, end, perr := linparse.ParseCLIRange(flags.lines)
	if perr != nil {
		return exitCodeForPipelineErr(linparse.NewLineRangeParseError(perr), "", dir, stderr)
	}

	return runExtractExecute(flags, start, end, dir, stdout, stderr, runner)
}
