package cmd

import (
	"flag"
	"fmt"
	"io"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

type transformApplyOutput struct {
	Changed           bool  `json:"changed"`
	EndingsChanged    *int  `json:"endings_changed,omitempty"`
	LFFound           *int  `json:"lf_found,omitempty"`
	LFConverted       *int  `json:"lf_converted,omitempty"`
	CRFound           *int  `json:"cr_found,omitempty"`
	CRConverted       *int  `json:"cr_converted,omitempty"`
	CRLFFound         *int  `json:"crlf_found,omitempty"`
	CRLFConverted     *int  `json:"crlf_converted,omitempty"`
	TrailingTrimmed   *int  `json:"trailing_trimmed,omitempty"`
	FinalNewlineAdded *bool `json:"final_newline_added,omitempty"`
}

type transformPreviewOutput struct {
	WouldChange        bool  `json:"would_change"`
	EndingsChanged     *int  `json:"endings_changed,omitempty"`
	LFFound            *int  `json:"lf_found,omitempty"`
	LFConverted        *int  `json:"lf_converted,omitempty"`
	CRFound            *int  `json:"cr_found,omitempty"`
	CRConverted        *int  `json:"cr_converted,omitempty"`
	CRLFFound          *int  `json:"crlf_found,omitempty"`
	CRLFConverted      *int  `json:"crlf_converted,omitempty"`
	TrailingTrimmed    *int  `json:"trailing_trimmed,omitempty"`
	FinalNewlineNeeded *bool `json:"final_newline_needed,omitempty"`
}

type transformFlagValues struct {
	source       string
	lineEndings  string
	trimTrailing bool
	finalNewline bool
	preview      bool
}

func transformUsage(fl *flag.FlagSet, stdout io.Writer) {
	_, _ = fmt.Fprintln(stdout, `Usage: glyph-shift transform --source <path> [options]

Mechanical line-ending and whitespace transforms. Executes by default; use --preview to inspect without writing.

Options:`)
	fl.SetOutput(stdout)
	fl.PrintDefaults()
}

func bindTransformFlags(fl *flag.FlagSet) *transformFlagValues {
	flags := &transformFlagValues{}

	fl.StringVar(&flags.source, "source", "", "source file path")
	fl.StringVar(&flags.lineEndings, "line-endings", "", `target line endings: "lf", "crlf", or "cr"`)
	fl.BoolVar(&flags.trimTrailing, "trim-trailing", false, "trim trailing spaces and tabs on each line")
	fl.BoolVar(&flags.finalNewline, "final-newline", false, "ensure the file ends with a newline")
	fl.BoolVar(&flags.preview, "preview", false, "inspect and report what would change without modifying the file")

	return flags
}

func parseLineEndingTarget(raw string) (*fileops.LineEndingTarget, error) {
	switch raw {
	case "":
		return nil, nil
	case "lf":
		t := fileops.TargetLF

		return &t, nil
	case "crlf":
		t := fileops.TargetCRLF

		return &t, nil
	case "cr":
		t := fileops.TargetCR

		return &t, nil
	default:
		return nil, fmt.Errorf("%w: line-endings must be lf, crlf, or cr", pipeline.ErrInvalidLineEndings)
	}
}

func buildTransformOpts(flags *transformFlagValues) (fileops.TransformOptions, error) {
	le, err := parseLineEndingTarget(flags.lineEndings)
	if err != nil {
		return fileops.TransformOptions{}, err
	}

	return fileops.TransformOptions{
		LineEndings:  le,
		TrimTrailing: flags.trimTrailing,
		FinalNewline: flags.finalNewline,
	}, nil
}

func transformOptsSpecified(flags *transformFlagValues) bool {
	return flags.lineEndings != "" || flags.trimTrailing || flags.finalNewline
}

// runTransform handles the "glyph-shift transform" subcommand.
func runTransform(
	args []string,
	dir string,
	stdout, stderr io.Writer,
	runner pipeline.Runner,
) int {
	fl := flag.NewFlagSet(subcmdTransform, flag.ContinueOnError)

	flags := bindTransformFlags(fl)

	fl.Usage = func() { transformUsage(fl, stdout) }

	if stop, code := parseSubcommandFlags(fl, args, stderr, subcmdTransform, dir); stop {
		return code
	}

	if !flagProvided(fl, "source") {
		writeErrorJSON(stderr, dir, &pipeline.ErrorOutcome{
			Error:    "missing_required_flag",
			Hint:     "--source is required",
			ExitCode: exitValidation,
			StringArrayFields: map[string][]string{
				"missing_flags": {"source"},
			},
		})

		return exitValidation
	}

	if !transformOptsSpecified(flags) {
		return exitCodeForPipelineErr(pipeline.ErrNoTransformSpecified, "", dir, stderr)
	}

	return runTransformExecute(flags, dir, stdout, stderr, runner)
}
