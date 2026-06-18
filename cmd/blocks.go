package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"regexp"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

type blocksJSONOutput struct {
	ContentBlocksFound int      `json:"content_blocks_found"`
	EmptyBlocksFound   int      `json:"empty_blocks_found,omitempty"`
	Created            []string `json:"files_created"`
}

type blocksPreviewOutput struct {
	ContentBlocksFound int      `json:"content_blocks_found"`
	EmptyBlocksFound   int      `json:"empty_blocks_found,omitempty"`
	WouldCreate        []string `json:"would_create"`
}

type blocksFlagValues struct {
	source            string
	startLine         string
	endLine           string
	outputDir         string
	extension         string
	namesRaw          string
	maxFiles          int
	includeDelimiters bool
	force             bool
	mkdir             bool
	preview           bool
}

func blocksUsage(fs *flag.FlagSet, stdout io.Writer) {
	_, _ = fmt.Fprintln(stdout, `Usage: glyph-shift blocks --source <path> --start-line <regex> `+
		`--end-line <regex> --output-dir <dir> [options]

Extract lines between start and end delimiter patterns into separate files.

Options:`)
	fs.SetOutput(stdout)
	fs.PrintDefaults()
}

func bindBlocksFlags(fs *flag.FlagSet) *blocksFlagValues {
	flags := &blocksFlagValues{}

	fs.StringVar(&flags.source, "source", "", "source file path")
	fs.StringVar(&flags.startLine, "start-line", "", "regular expression for start delimiter lines")
	fs.StringVar(&flags.endLine, "end-line", "", "regular expression for end delimiter lines")
	fs.StringVar(&flags.outputDir, "output-dir", "", "output directory")
	fs.StringVar(&flags.extension, "extension", "",
		`output filename extension (include leading "."); default: source file extension`)
	fs.StringVar(&flags.namesRaw, "names", "",
		"comma-separated output basenames per non-empty block")
	fs.IntVar(&flags.maxFiles, "max-files", pipeline.DefaultMaxFiles, "maximum matched blocks (includes empty blocks)")
	fs.BoolVar(&flags.includeDelimiters, "include-delimiters", false, "include delimiter lines in each block output")
	fs.BoolVar(&flags.force, "force", false, "overwrite existing output files")
	fs.BoolVar(&flags.mkdir, "mkdir", false, "create output directory if missing")
	fs.BoolVar(&flags.preview, "preview", false, "report blocks and planned output paths without writing files")

	return flags
}

func buildBlocksParams(
	flags *blocksFlagValues,
	dir string,
) (pipeline.BlocksParams, error) {
	srcPath, prepErr := pipeline.PreparePath(flags.source, dir)
	if prepErr != nil {
		return pipeline.BlocksParams{}, pipeline.WithPathRole(pipeline.PathRoleSrc, flags.source, prepErr)
	}

	outDir, prepErr := pipeline.PreparePath(flags.outputDir, dir)
	if prepErr != nil {
		return pipeline.BlocksParams{}, pipeline.WithPathRole(pipeline.PathRoleOutDir, flags.outputDir, prepErr)
	}

	flags.source = srcPath
	flags.outputDir = outDir

	startRE, patErr := validateBlockDelimiterPattern(flags.startLine, "start_line")
	if patErr != nil {
		return pipeline.BlocksParams{}, patErr
	}

	endRE, patErr := validateBlockDelimiterPattern(flags.endLine, "end_line")
	if patErr != nil {
		return pipeline.BlocksParams{}, patErr
	}

	namesList, nerr := pipeline.ParseCommaSeparatedNames(flags.namesRaw)
	if nerr != nil {
		return pipeline.BlocksParams{}, nerr
	}

	return pipeline.BlocksParams{
		SrcPath:           flags.source,
		OutDir:            flags.outputDir,
		Root:              dir,
		StartDelimiter:    startRE,
		EndDelimiter:      endRE,
		Naming:            fileops.Sequential,
		IncludeDelimiters: flags.includeDelimiters,
		Extension:         flags.extension,
		Force:             flags.force,
		Mkdir:             flags.mkdir,
		Preview:           flags.preview,
		MaxFiles:          flags.maxFiles,
		Names:             namesList,
	}, nil
}

func validateBlockDelimiterPattern(pattern, field string) (*regexp.Regexp, error) {
	compiledPattern, err := validate.ValidatePattern(pattern)
	if err == nil {
		return compiledPattern, nil
	}

	if isPatternFieldValidationError(err) {
		return nil, &pipeline.PatternFieldError{Field: field, Cause: err}
	}

	return nil, err
}

func isPatternFieldValidationError(err error) bool {
	return errors.Is(err, validate.ErrInvalidPattern) ||
		errors.Is(err, validate.ErrEmptyRegexpPattern) ||
		errors.Is(err, validate.ErrPatternTooLong) ||
		errors.Is(err, validate.ErrControlChar)
}

func runBlocksExecute(
	flags *blocksFlagValues,
	dir string,
	stdout, stderr io.Writer,
	runner pipeline.Runner,
) int {
	params, prepErr := buildBlocksParams(flags, dir)
	if prepErr != nil {
		return exitCodeForPipelineErr(prepErr, flags.source, dir, stderr)
	}

	pres, err := runner.RunBlocks(context.Background(), params)
	if err != nil {
		return exitCodeForPipelineErr(err, flags.source, dir, stderr)
	}

	if len(pres.Files) > pres.BlocksFound {
		writeErrorJSON(stderr, dir, &pipeline.ErrorOutcome{
			Error: "invalid_input",
			Hint: "blocks reported more output files than delimiter blocks found; result is internally inconsistent. " +
				"Check --source, --start-line, and --end-line patterns, or retry after upgrading glyph-shift.",
			ExitCode: exitValidation,
		})

		return exitValidation
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")

	var result any

	contentBlocksFound := len(pres.Files)
	emptyBlocksFound := pres.BlocksFound - contentBlocksFound

	if flags.preview {
		result = blocksPreviewOutput{
			ContentBlocksFound: contentBlocksFound,
			EmptyBlocksFound:   emptyBlocksFound,
			WouldCreate:        pres.Files,
		}
	} else {
		result = blocksJSONOutput{
			ContentBlocksFound: contentBlocksFound,
			EmptyBlocksFound:   emptyBlocksFound,
			Created:            pres.Files,
		}
	}

	if err := enc.Encode(result); err != nil {
		writeFailedJSON(stderr, dir, err)

		return 1
	}

	return 0
}

// runBlocks handles the "glyph-shift blocks" subcommand.
func runBlocks(
	args []string,
	dir string,
	stdout, stderr io.Writer,
	runner pipeline.Runner,
) int {
	fs := flag.NewFlagSet(subcmdBlocks, flag.ContinueOnError)

	flags := bindBlocksFlags(fs)

	fs.Usage = func() { blocksUsage(fs, stdout) }

	if stop, code := parseSubcommandFlags(fs, args, stderr, subcmdBlocks, dir); stop {
		return code
	}

	if !allFlagsProvided(fs, "source", "start-line", "end-line", "output-dir") {
		writeErrorJSON(stderr, dir, &pipeline.ErrorOutcome{
			Error:    "missing_required_flag",
			Hint:     "--source, --start-line, --end-line, and --output-dir are required",
			ExitCode: exitValidation,
			StringArrayFields: map[string][]string{
				"missing_flags": {"source", "start_line", "end_line", "output_dir"},
			},
		})

		return exitValidation
	}

	if flags.maxFiles < 1 {
		return exitCodeForPipelineErr(pipeline.ErrMaxFilesAtLeastOne, "", dir, stderr)
	}

	return runBlocksExecute(flags, dir, stdout, stderr, runner)
}
