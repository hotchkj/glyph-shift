package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

type splitJSONOutput struct {
	Created []string `json:"files_created"`
}

type splitPreviewOutput struct {
	WouldCreate []string `json:"would_create"`
}

type splitFlagValues struct {
	source         string
	delimiter      string
	outputDir      string
	extension      string
	namesRaw       string
	maxFiles       int
	stripDelimiter bool
	force          bool
	mkdir          bool
	preview        bool
}

func splitUsage(fs *flag.FlagSet, stdout io.Writer) {
	_, _ = fmt.Fprintln(stdout, `Usage: glyph-shift split --source <path> --delimiter <regex> --output-dir <dir> [options]

Split a file into multiple files at each line matching the delimiter pattern.

Options:`)
	fs.SetOutput(stdout)
	fs.PrintDefaults()
}

func bindSplitFlags(fs *flag.FlagSet) *splitFlagValues {
	flags := &splitFlagValues{}

	fs.StringVar(&flags.source, "source", "", "source file path")
	fs.StringVar(&flags.delimiter, "delimiter", "", "regular expression for delimiter lines")
	fs.StringVar(&flags.outputDir, "output-dir", "", "output directory")
	fs.StringVar(&flags.extension, "extension", "",
		`output filename extension (include leading "."); default: source file extension`)
	fs.StringVar(&flags.namesRaw, "names", "",
		"comma-separated output basenames (stems or full names)")
	fs.IntVar(&flags.maxFiles, "max-files", pipeline.DefaultMaxFiles, "maximum number of output sections")
	fs.BoolVar(&flags.stripDelimiter, "strip-delimiter", false, "omit delimiter line from each section output")
	fs.BoolVar(&flags.force, "force", false, "overwrite existing output files")
	fs.BoolVar(&flags.mkdir, "mkdir", false, "create output directory if missing")
	fs.BoolVar(&flags.preview, "preview", false, "report output basenames without writing files")

	return flags
}

func buildSplitParams(
	flags *splitFlagValues,
	dir string,
) (pipeline.SplitParams, error) {
	srcPath, prepErr := pipeline.PreparePath(flags.source, dir)
	if prepErr != nil {
		return pipeline.SplitParams{}, pipeline.WithPathRole(pipeline.PathRoleSrc, flags.source, prepErr)
	}

	outDir, prepErr := pipeline.PreparePath(flags.outputDir, dir)
	if prepErr != nil {
		return pipeline.SplitParams{}, pipeline.WithPathRole(pipeline.PathRoleOutDir, flags.outputDir, prepErr)
	}

	flags.source = srcPath
	flags.outputDir = outDir

	re, patErr := validate.ValidatePattern(flags.delimiter)
	if patErr != nil {
		if isPatternFieldValidationError(patErr) {
			return pipeline.SplitParams{}, &pipeline.PatternFieldError{Field: "delimiter", Cause: patErr}
		}

		return pipeline.SplitParams{}, patErr
	}

	namesList, nerr := pipeline.ParseCommaSeparatedNames(flags.namesRaw)
	if nerr != nil {
		return pipeline.SplitParams{}, nerr
	}

	return pipeline.SplitParams{
		SrcPath:        flags.source,
		OutDir:         flags.outputDir,
		Root:           dir,
		Delimiter:      re,
		Naming:         fileops.Sequential,
		StripDelimiter: flags.stripDelimiter,
		Extension:      flags.extension,
		Force:          flags.force,
		Mkdir:          flags.mkdir,
		Preview:        flags.preview,
		MaxFiles:       flags.maxFiles,
		Names:          namesList,
	}, nil
}

func runSplitExecute(
	flags *splitFlagValues,
	dir string,
	stdout, stderr io.Writer,
	runner pipeline.Runner,
) int {
	params, prepErr := buildSplitParams(flags, dir)
	if prepErr != nil {
		return exitCodeForPipelineErr(prepErr, flags.source, dir, stderr)
	}

	pres, err := runner.RunSplit(context.Background(), params)
	if err != nil {
		return exitCodeForPipelineErr(err, flags.source, dir, stderr)
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")

	var result any

	if flags.preview {
		result = splitPreviewOutput{WouldCreate: pres.Files}
	} else {
		result = splitJSONOutput{Created: pres.Files}
	}

	if err := enc.Encode(result); err != nil {
		writeFailedJSON(stderr, dir, err)

		return 1
	}

	return 0
}

// runSplit handles the "glyph-shift split" subcommand.
func runSplit(
	args []string,
	dir string,
	stdout, stderr io.Writer,
	runner pipeline.Runner,
) int {
	fs := flag.NewFlagSet(subcmdSplit, flag.ContinueOnError)

	flags := bindSplitFlags(fs)

	fs.Usage = func() { splitUsage(fs, stdout) }

	if stop, code := parseSubcommandFlags(fs, args, stderr, subcmdSplit, dir); stop {
		return code
	}

	if !allFlagsProvided(fs, "source", "delimiter", "output-dir") {
		writeErrorJSON(stderr, dir, &pipeline.ErrorOutcome{
			Error:    "missing_required_flag",
			Hint:     "--source, --delimiter, and --output-dir are required",
			ExitCode: exitValidation,
			StringArrayFields: map[string][]string{
				"missing_flags": {"source", "delimiter", "output_dir"},
			},
		})

		return exitValidation
	}

	if flags.maxFiles < 1 {
		return exitCodeForPipelineErr(pipeline.ErrMaxFilesAtLeastOne, "", dir, stderr)
	}

	return runSplitExecute(flags, dir, stdout, stderr, runner)
}
