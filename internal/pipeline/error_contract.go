package pipeline

import (
	"errors"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/linparse"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

const (
	ExitGeneral        = 1
	ExitSourceNotFound = 2
	ExitBinarySource   = 3
	ExitDestExists     = 4
	ExitNotRegularFile = 5
	ExitValidation     = 6
)

// ErrorOutcome is the transport-independent operation error contract: sentinel key in Error,
// user hint, exit category, at most one primary path vocabulary slot for pipeline classification,
// and optional grouped primitive maps for variant-specific context (no nested objects).
type ErrorOutcome struct {
	Error    string
	Hint     string
	ExitCode int

	Src        string
	Dest       string
	OutDir     string
	OutputPath string

	StringFields      map[string]string
	IntFields         map[string]int
	StringArrayFields map[string][]string
}

type pathSlot int

const (
	slotNone pathSlot = iota
	slotUseFallbackSrc
	slotUseFallbackOutputPath
)

// sentinelRule maps a single sentinel error to a fixed-hint outcome where applicable.
type sentinelRule struct {
	sentinel   error
	class      string
	pathSlot   pathSlot
	fixedHint  string
	useErrHint bool
	exitCode   int
}

// sentinelRules is the ordered lookup table for ClassifyOperationError after special-case wrappers.
// ErrDestinationExists and PatternFieldError are handled before this table.
var sentinelRules = []sentinelRule{
	{ErrSourceNotFound, "source_not_found", slotUseFallbackSrc, HintSourceNotFound, false, ExitSourceNotFound},
	{ErrBinarySource, "binary_source", slotUseFallbackSrc, HintBinarySource, false, ExitBinarySource},
	{ErrDirectoryNotFile, "directory_not_file", slotUseFallbackSrc, HintDirectoryNotFile, false, ExitNotRegularFile},
	{ErrNotRegularFile, "not_regular_file", slotUseFallbackSrc, HintNotRegularFile, false, ExitNotRegularFile},
	{fileops.ErrNoBlocksFound, "no_blocks_found", slotUseFallbackSrc, HintNoBlocksFound, false, ExitValidation},
	{fileops.ErrNoDelimiterMatch, "no_delimiter_match", slotUseFallbackSrc, HintNoDelimiterMatch, false, ExitValidation},
	{fileops.ErrSpanFingerprintMismatch, "source_fingerprint_mismatch", slotUseFallbackOutputPath, "", true, ExitGeneral},
	{validate.ErrPatternTooLong, "invalid_input", slotNone, "", true, ExitValidation},
	{validate.ErrControlChar, "invalid_input", slotNone, "", true, ExitValidation},
	{fileops.ErrPathContainsNUL, "invalid_input", slotNone, "", true, ExitValidation},
	{validate.ErrPathContainsNUL, "invalid_input", slotNone, "", true, ExitValidation},
	{validate.ErrInvalidPattern, "invalid_input", slotNone, "", true, ExitValidation},
	{validate.ErrInvalidExtension, "invalid_input", slotNone, "", true, ExitValidation},
	{validate.ErrPathTraversal, "invalid_input", slotUseFallbackSrc, "", true, ExitValidation},
	{validate.ErrOutsideRoot, "invalid_input", slotUseFallbackSrc, "", true, ExitValidation},
	{validate.ErrReservedName, "invalid_input", slotUseFallbackSrc, "", true, ExitValidation},
	{ErrEmptyPreparedPath, "invalid_input", slotNone, "", true, ExitValidation},
	{ErrEmptyRegexpPattern, "invalid_input", slotNone, "", true, ExitValidation},
	{linparse.ErrLineRangeParse, "invalid_input", slotNone, "", true, ExitValidation},
	{linparse.ErrInvalidLineRange, "invalid_input", slotNone, "", true, ExitValidation},
	{linparse.ErrEmptyLineRange, "invalid_input", slotNone, "", true, ExitValidation},
	{ErrEmptyNamesListEntry, "invalid_input", slotNone, "", true, ExitValidation},
	{ErrInvalidExplicitName, "invalid_input", slotNone, "", true, ExitValidation},
	{ErrDuplicateExplicitNames, "invalid_input", slotNone, "", true, ExitValidation},
	{ErrInvalidLineEndings, "invalid_line_endings", slotNone, "", true, ExitValidation},
	{ErrNoTransformSpecified, "no_transform_specified", slotNone, HintNoTransformSpecified, false, ExitValidation},
	{ErrMaxFilesAtLeastOne, "invalid_input", slotNone, "", true, ExitValidation},
	{ErrTransformSkippedUnknown, "internal_error", slotUseFallbackSrc, HintTransformSkippedUnknown, false, ExitGeneral},
}

func classifyPatternFieldOutcome(err error) (ErrorOutcome, bool) {
	var pf *PatternFieldError
	if !errors.As(err, &pf) || pf.Cause == nil {
		return ErrorOutcome{}, false
	}

	out := ErrorOutcome{
		Hint:     pf.Cause.Error(),
		ExitCode: ExitValidation,
		StringFields: map[string]string{
			"field": pf.Field,
		},
	}

	switch {
	case errors.Is(pf.Cause, validate.ErrInvalidPattern),
		errors.Is(pf.Cause, validate.ErrEmptyRegexpPattern):
		out.Error = opErrClassInvalidPattern
	case errors.Is(pf.Cause, validate.ErrPatternTooLong):
		out.Error = opErrClassPatternTooLong
	case errors.Is(pf.Cause, validate.ErrControlChar):
		out.Error = opErrClassControlCharsInInput
	default:
		out.Error = opErrClassInvalidInput
		out.StringFields = nil
	}

	return out, true
}

func classifyDestinationExistsOutcome(err error, primaryPath string) (ErrorOutcome, bool) {
	if !errors.Is(err, ErrDestinationExists) {
		return ErrorOutcome{}, false
	}

	dest := primaryPath
	if d, ok := DestinationPathFromError(err); ok {
		dest = d
	}

	return ErrorOutcome{
		Error:    "destination_exists",
		Hint:     HintDestinationExists,
		ExitCode: ExitDestExists,
		Dest:     dest,
	}, true
}

// ClassifyOperationError maps internal sentinels to the public operation error contract shared by CLI
// stderr JSON and MCP structuredContent.
// primaryPath is the operation's primary source path when known (for src-keyed variants);
// destination fingerprint and similar surfaces use OutputPath slots via rules or PathContextError.
func ClassifyOperationError(err error, primaryPath string) ErrorOutcome {
	if err == nil {
		return ErrorOutcome{Error: "internal_error", ExitCode: ExitGeneral}
	}

	if out, ok := classifyPatternFieldOutcome(err); ok {
		return out
	}

	if out, ok := classifyDestinationExistsOutcome(err, primaryPath); ok {
		return out
	}

	if out, ok := classifyStructuredOperationError(err, primaryPath); ok {
		return out
	}

	if out, ok := classifyFromSentinelRules(err, primaryPath); ok {
		return out
	}

	hint := err.Error()

	return classifyInternalError(err, primaryPath, hint)
}

func classifyStructuredOperationError(err error, primaryPath string) (ErrorOutcome, bool) {
	if out, ok := classifyEmptyRangeOutcome(err, primaryPath); ok {
		return out, true
	}
	if out, ok := classifyRangeExceedsFileOutcome(err, primaryPath); ok {
		return out, true
	}
	if out, ok := classifyUnclosedBlockOutcome(err, primaryPath); ok {
		return out, true
	}
	if out, ok := classifyMaxFilesExceededOutcome(err, primaryPath); ok {
		return out, true
	}

	return classifyNamesCountMismatchOutcome(err, primaryPath)
}

func classifyFromSentinelRules(err error, primaryPath string) (ErrorOutcome, bool) {
	for _, rule := range sentinelRules {
		if !errors.Is(err, rule.sentinel) {
			continue
		}

		outcome := outcomeFromRule(rule, primaryPath, err)
		return enrichHintFromErr(outcome, err, rule.useErrHint), true
	}

	return ErrorOutcome{}, false
}

// classifyEmptyRangeOutcome returns ok when err wraps [fileops.ErrEmptyRange].
// The empty_range contract variant requires [fileops.EmptyRangeError] for range_start/range_end;
// a bare sentinel without typed endpoints classifies as internal_error (no placeholder integers).
func classifyEmptyRangeOutcome(err error, primaryPath string) (ErrorOutcome, bool) {
	if !errors.Is(err, fileops.ErrEmptyRange) {
		return ErrorOutcome{}, false
	}

	var er *fileops.EmptyRangeError
	if !errors.As(err, &er) {
		return classifyInternalError(err, primaryPath, HintEmptyRange), true
	}

	outcome := applyPathSlotFromError(
		outcomeFromRuleSlot("empty_range", HintEmptyRange, false, ExitValidation),
		slotUseFallbackSrc,
		primaryPath,
		err,
	)
	outcome.IntFields = map[string]int{
		"range_start": er.Start,
		"range_end":   er.End,
	}

	return enrichHintFromErr(outcome, err, false), true
}

// classifyRangeExceedsFileOutcome returns ok when err wraps [fileops.ErrRangeExceedsFile].
// Typed [fileops.RangeExceedsFileError] is required to emit range_exceeds_file with integers.
func classifyRangeExceedsFileOutcome(err error, primaryPath string) (ErrorOutcome, bool) {
	if !errors.Is(err, fileops.ErrRangeExceedsFile) {
		return ErrorOutcome{}, false
	}

	var rxf *fileops.RangeExceedsFileError
	if !errors.As(err, &rxf) {
		return classifyInternalError(err, primaryPath, err.Error()), true
	}

	out := ErrorOutcome{
		Error:    "range_exceeds_file",
		ExitCode: ExitValidation,
		Src:      primaryPath,
		IntFields: map[string]int{
			"file_lines":  rxf.FileLines,
			"range_start": rxf.RangeStart,
			"range_end":   rxf.RangeEnd,
		},
	}

	out = applyPathSlotFromError(out, slotUseFallbackSrc, primaryPath, err)

	return enrichHintFromErr(out, err, true), true
}

// classifyUnclosedBlockOutcome returns ok when err wraps [fileops.ErrUnclosedBlock].
// start_line must come from [fileops.UnclosedBlockDetailError] with a positive line number.
func classifyUnclosedBlockOutcome(err error, primaryPath string) (ErrorOutcome, bool) {
	if !errors.Is(err, fileops.ErrUnclosedBlock) {
		return ErrorOutcome{}, false
	}

	var ub *fileops.UnclosedBlockDetailError
	if !errors.As(err, &ub) || ub.StartLine <= 0 {
		return classifyInternalError(err, primaryPath, err.Error()), true
	}

	out := ErrorOutcome{
		Error:    "unclosed_block",
		ExitCode: ExitValidation,
		IntFields: map[string]int{
			"start_line": ub.StartLine,
		},
	}
	out = applyPathSlotFromError(out, slotUseFallbackSrc, primaryPath, err)

	return enrichHintFromErr(out, err, true), true
}

// classifyMaxFilesExceededOutcome returns ok when err wraps [ErrMaxFilesExceeded]
// (same sentinel as [fileops.ErrMaxFilesExceeded]).
func classifyMaxFilesExceededOutcome(err error, primaryPath string) (ErrorOutcome, bool) {
	if !errors.Is(err, ErrMaxFilesExceeded) {
		return ErrorOutcome{}, false
	}

	var mfd *fileops.MaxFilesExceededDetailError
	if !errors.As(err, &mfd) {
		return classifyInternalError(err, primaryPath, err.Error()), true
	}

	out := ErrorOutcome{
		Error:    "max_files_exceeded",
		ExitCode: ExitValidation,
		IntFields: map[string]int{
			"max_files":          mfd.MaxFiles,
			"would_create_count": mfd.WouldCreateCount,
		},
	}

	return enrichHintFromErr(out, err, true), true
}

// classifyNamesCountMismatchOutcome returns ok when err wraps [ErrNamesCountMismatch].
func classifyNamesCountMismatchOutcome(err error, primaryPath string) (ErrorOutcome, bool) {
	if !errors.Is(err, ErrNamesCountMismatch) {
		return ErrorOutcome{}, false
	}

	var nm *NamesCountMismatchError
	if !errors.As(err, &nm) {
		return classifyInternalError(err, primaryPath, err.Error()), true
	}

	out := ErrorOutcome{
		Error:    "names_count_mismatch",
		ExitCode: ExitValidation,
		IntFields: map[string]int{
			"names_count":  nm.NamesCount,
			"output_count": nm.OutputCount,
		},
	}

	return enrichHintFromErr(out, err, true), true
}

func classifyInternalError(err error, primaryPath, hint string) ErrorOutcome {
	var pc *PathContextError
	if errors.As(err, &pc) && pc.Err != nil {
		return internalErrorOutcomeFromPathContext(pc.Context, primaryPath, hint)
	}

	out := ErrorOutcome{
		Error:    "internal_error",
		Hint:     hint,
		ExitCode: ExitGeneral,
		Src:      primaryPath,
	}

	return out
}

func internalErrorOutcomeFromPathContext(context PathContext, primaryPath, hint string) ErrorOutcome {
	out := ErrorOutcome{
		Error:    "internal_error",
		Hint:     hint,
		ExitCode: ExitGeneral,
	}

	switch context.Role {
	case PathRoleSrc:
		out.Src = context.Path
	case PathRoleDest:
		out.Dest = context.Path
	case PathRoleOutDir:
		out.OutDir = context.Path
	case PathRoleOutputPath:
		out.OutputPath = context.Path
	case PathRoleNone:
		if primaryPath != "" {
			out.Src = primaryPath
		}
	}

	return out
}

func outcomeFromRule(rule sentinelRule, primaryPath string, classifyErr error) ErrorOutcome {
	staging := outcomeFromRuleSlot(rule.class, rule.fixedHint, rule.useErrHint, rule.exitCode)

	return applyPathSlotFromError(staging, rule.pathSlot, primaryPath, classifyErr)
}

func outcomeFromRuleSlot(class, fixedHint string, useErrHint bool, exit int) ErrorOutcome {
	hintVal := fixedHint
	if useErrHint {
		hintVal = ""
	}

	return ErrorOutcome{
		Error:    class,
		Hint:     hintVal,
		ExitCode: exit,
	}
}

//nolint:gocritic // hugeParam: staging ErrorOutcome by value keeps classification helpers side-effect local
func applyPathSlot(staging ErrorOutcome, slot pathSlot, primaryPath string) ErrorOutcome {
	switch slot {
	case slotUseFallbackSrc:
		staging.Src = primaryPath
	case slotUseFallbackOutputPath:
		staging.OutputPath = primaryPath
	case slotNone:
	}

	return staging
}

// applyPathSlotFromError prefers an explicit [PathContextError] path role in classifyErr
// over fallback primaryPath slots.
//
//nolint:gocritic // hugeParam: staging ErrorOutcome by value keeps classification helpers side-effect local
func applyPathSlotFromError(staging ErrorOutcome, slot pathSlot, primaryPath string, classifyErr error) ErrorOutcome {
	var pc *PathContextError
	if errors.As(classifyErr, &pc) && pc != nil && pc.Context.Role != PathRoleNone && pc.Context.Path != "" {
		return applyPathContextSlot(staging, pc.Context)
	}

	return applyPathSlot(staging, slot, primaryPath)
}

//nolint:gocritic // hugeParam: staging ErrorOutcome by value keeps classification helpers side-effect local
func applyPathContextSlot(staging ErrorOutcome, context PathContext) ErrorOutcome {
	switch context.Role {
	case PathRoleSrc:
		staging.Src = context.Path
	case PathRoleDest:
		staging.Dest = context.Path
	case PathRoleOutDir:
		staging.OutDir = context.Path
	case PathRoleOutputPath:
		staging.OutputPath = context.Path
	case PathRoleNone:
	}

	return staging
}

//nolint:gocritic // hugeParam: copy mirrors existing merge-style helpers; avoids mutating shared refs
func enrichHintFromErr(staging ErrorOutcome, err error, useErrHint bool) ErrorOutcome {
	if !useErrHint {
		return staging
	}

	if err != nil {
		staging.Hint = err.Error()
	}

	return staging
}

// PathContextString returns the legacy-equivalent single path diagnostic string
// (dest, planned output path, src, then out_dir).
func (out *ErrorOutcome) PathContextString() string {
	if out == nil {
		return ""
	}
	if out.Dest != "" {
		return out.Dest
	}

	if out.OutputPath != "" {
		return out.OutputPath
	}

	if out.Src != "" {
		return out.Src
	}

	return out.OutDir
}
