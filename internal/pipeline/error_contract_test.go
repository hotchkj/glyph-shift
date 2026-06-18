package pipeline

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/linparse"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

var (
	errBadLines               = errors.New("bad lines")
	errClassifyTestUnexpected = errors.New("something unexpected happened")
	errClassifyTestDeep       = errors.New("deep error")
)

const classifyTestPrimaryPath = "doc.md"

type errorOutcomeCase struct {
	name string
	err  error
	want ErrorOutcome
}

func runErrorOutcomeCases(t *testing.T, cases []errorOutcomeCase) {
	t.Helper()
	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifyOperationError(tc.err, classifyTestPrimaryPath)
			if !outcomesEqual(&got, &tc.want) {
				t.Fatalf("outcome: got %#v want %#v", got, tc.want)
			}
		})
	}
}

func sourceErrorOutcome(errorName, hint string, exitCode int) ErrorOutcome {
	return ErrorOutcome{
		Error:    errorName,
		Src:      classifyTestPrimaryPath,
		Hint:     hint,
		ExitCode: exitCode,
	}
}

func destinationErrorOutcome(errorName, dest, hint string, exitCode int) ErrorOutcome {
	return ErrorOutcome{
		Error:    errorName,
		Dest:     dest,
		Hint:     hint,
		ExitCode: exitCode,
	}
}

func TestClassifyOperationError_SourceAndFileErrors(t *testing.T) {
	t.Parallel()
	runErrorOutcomeCases(t, []errorOutcomeCase{
		{
			name: "source not found",
			err:  ErrSourceNotFound,
			want: sourceErrorOutcome("source_not_found", HintSourceNotFound, ExitSourceNotFound),
		},
		{
			name: "binary source",
			err:  ErrBinarySource,
			want: sourceErrorOutcome("binary_source", HintBinarySource, ExitBinarySource),
		},
		{
			name: "directory not file",
			err:  ErrDirectoryNotFile,
			want: sourceErrorOutcome("directory_not_file", HintDirectoryNotFile, ExitNotRegularFile),
		},
		{
			name: "not regular file",
			err:  ErrNotRegularFile,
			want: sourceErrorOutcome("not_regular_file", HintNotRegularFile, ExitNotRegularFile),
		},
		{
			name: "unknown transform skip",
			err:  ErrTransformSkippedUnknown,
			want: sourceErrorOutcome("internal_error", HintTransformSkippedUnknown, ExitGeneral),
		},
		{
			name: "span fingerprint mismatch sentinel",
			err:  fileops.ErrSpanFingerprintMismatch,
			want: ErrorOutcome{
				Error:      "source_fingerprint_mismatch",
				OutputPath: classifyTestPrimaryPath,
				Hint:       fileops.ErrSpanFingerprintMismatch.Error(),
				ExitCode:   ExitGeneral,
			},
		},
		{
			name: "wrapped span fingerprint mismatch",
			err:  fmt.Errorf("copy span: %w", fileops.ErrSpanFingerprintMismatch),
			want: ErrorOutcome{
				Error:      "source_fingerprint_mismatch",
				OutputPath: classifyTestPrimaryPath,
				Hint:       "copy span: fileops: span fingerprint mismatch",
				ExitCode:   ExitGeneral,
			},
		},
	})
}

func TestClassifyOperationError_ValidationErrors(t *testing.T) {
	t.Parallel()
	runErrorOutcomeCases(t, operationValidationErrorCases())
}

func operationValidationErrorCases() []errorOutcomeCase {
	return append(operationRangeValidationCases(), operationPatternValidationCases()...)
}

func operationRangeValidationCases() []errorOutcomeCase {
	return append(operationLineRangeValidationCases(), operationBlockMatchValidationCases()...)
}

func operationLineRangeValidationCases() []errorOutcomeCase {
	return []errorOutcomeCase{
		{
			name: "empty range bare sentinel classifies internal_error",
			err:  fileops.ErrEmptyRange,
			want: ErrorOutcome{
				Error:    "internal_error",
				Src:      classifyTestPrimaryPath,
				Hint:     HintEmptyRange,
				ExitCode: ExitGeneral,
			},
		},
		{
			name: "empty range with endpoints",
			err:  &fileops.EmptyRangeError{Start: 2, End: 1},
			want: ErrorOutcome{
				Error:    "empty_range",
				Src:      classifyTestPrimaryPath,
				Hint:     HintEmptyRange,
				ExitCode: ExitValidation,
				IntFields: map[string]int{
					"range_start": 2,
					"range_end":   1,
				},
			},
		},
		{
			name: "range exceeds file bare sentinel classifies internal_error",
			err:  fileops.ErrRangeExceedsFile,
			want: ErrorOutcome{
				Error:    "internal_error",
				Src:      classifyTestPrimaryPath,
				Hint:     fileops.ErrRangeExceedsFile.Error(),
				ExitCode: ExitGeneral,
			},
		},
		{
			name: "range exceeds file structured",
			err: &fileops.RangeExceedsFileError{
				FileLines:  50,
				RangeStart: 45,
				RangeEnd:   120,
			},
			want: ErrorOutcome{
				Error:    "range_exceeds_file",
				Src:      classifyTestPrimaryPath,
				Hint:     (&fileops.RangeExceedsFileError{FileLines: 50, RangeStart: 45, RangeEnd: 120}).Error(),
				ExitCode: ExitValidation,
				IntFields: map[string]int{
					"file_lines":  50,
					"range_start": 45,
					"range_end":   120,
				},
			},
		},
	}
}

func operationBlockMatchValidationCases() []errorOutcomeCase {
	return []errorOutcomeCase{
		{
			name: "unclosed block bare sentinel classifies internal_error",
			err:  fileops.ErrUnclosedBlock,
			want: ErrorOutcome{
				Error:    "internal_error",
				Src:      classifyTestPrimaryPath,
				Hint:     fileops.ErrUnclosedBlock.Error(),
				ExitCode: ExitGeneral,
			},
		},
		{
			name: "unclosed block zero start_line classifies internal_error",
			err:  &fileops.UnclosedBlockDetailError{StartLine: 0},
			want: ErrorOutcome{
				Error:    "internal_error",
				Src:      classifyTestPrimaryPath,
				Hint:     fileops.ErrUnclosedBlock.Error(),
				ExitCode: ExitGeneral,
			},
		},
		{
			name: "unclosed block with start line",
			err:  &fileops.UnclosedBlockDetailError{StartLine: 12},
			want: ErrorOutcome{
				Error:     "unclosed_block",
				Src:       classifyTestPrimaryPath,
				Hint:      (&fileops.UnclosedBlockDetailError{StartLine: 12}).Error(),
				ExitCode:  ExitValidation,
				IntFields: map[string]int{"start_line": 12},
			},
		},
		{
			name: "no blocks found",
			err:  fileops.ErrNoBlocksFound,
			want: sourceErrorOutcome("no_blocks_found", HintNoBlocksFound, ExitValidation),
		},
		{
			name: "split delimiter miss",
			err:  fmt.Errorf("split: %w", fileops.ErrNoDelimiterMatch),
			want: sourceErrorOutcome("no_delimiter_match", HintNoDelimiterMatch, ExitValidation),
		},
	}
}

func operationPatternValidationCases() []errorOutcomeCase {
	return []errorOutcomeCase{
		{
			name: "invalid pattern sentinel",
			err:  validate.ErrInvalidPattern,
			want: ErrorOutcome{Error: "invalid_input", Hint: validate.ErrInvalidPattern.Error(), ExitCode: ExitValidation},
		},
		{
			name: "invalid pattern with field",
			err:  &PatternFieldError{Field: "delimiter", Cause: validate.ErrInvalidPattern},
			want: ErrorOutcome{
				Error:        "invalid_pattern",
				Hint:         validate.ErrInvalidPattern.Error(),
				ExitCode:     ExitValidation,
				StringFields: map[string]string{"field": "delimiter"},
			},
		},
		{
			name: "empty pattern with field",
			err:  &PatternFieldError{Field: "delimiter", Cause: validate.ErrEmptyRegexpPattern},
			want: ErrorOutcome{
				Error:        "invalid_pattern",
				Hint:         validate.ErrEmptyRegexpPattern.Error(),
				ExitCode:     ExitValidation,
				StringFields: map[string]string{"field": "delimiter"},
			},
		},
		{
			name: "pattern too long sentinel",
			err:  validate.ErrPatternTooLong,
			want: ErrorOutcome{Error: "invalid_input", Hint: validate.ErrPatternTooLong.Error(), ExitCode: ExitValidation},
		},
		{
			name: "control char in pattern sentinel",
			err:  validate.ErrControlChar,
			want: ErrorOutcome{Error: "invalid_input", Hint: validate.ErrControlChar.Error(), ExitCode: ExitValidation},
		},
		{
			name: "path contains nul sentinel",
			err:  fileops.ErrPathContainsNUL,
			want: ErrorOutcome{Error: "invalid_input", Hint: fileops.ErrPathContainsNUL.Error(), ExitCode: ExitValidation},
		},
		{
			name: "validate path contains nul sentinel",
			err:  validate.ErrPathContainsNUL,
			want: ErrorOutcome{Error: "invalid_input", Hint: validate.ErrPathContainsNUL.Error(), ExitCode: ExitValidation},
		},
	}
}

func TestClassifyOperationError_DestAndExitErrors(t *testing.T) {
	t.Parallel()
	runErrorOutcomeCases(t, operationDestinationAndExitErrorCases())
}

func operationDestinationAndExitErrorCases() []errorOutcomeCase {
	return append(operationDestinationLimitCases(), operationTransformInputCases()...)
}

func operationDestinationLimitCases() []errorOutcomeCase {
	return append(operationMaxFileLimitCases(), operationNameCountLimitCases()...)
}

func operationMaxFileLimitCases() []errorOutcomeCase {
	return []errorOutcomeCase{
		{
			name: "destination exists carries destination",
			err:  newDestinationExistsError("out/001.md"),
			want: destinationErrorOutcome("destination_exists", "out/001.md", HintDestinationExists, ExitDestExists),
		},
		{
			name: "max files exceeded bare sentinel classifies internal_error",
			err:  ErrMaxFilesExceeded,
			want: ErrorOutcome{
				Error:    "internal_error",
				Src:      classifyTestPrimaryPath,
				Hint:     ErrMaxFilesExceeded.Error(),
				ExitCode: ExitGeneral,
			},
		},
		{
			name: "max files exceeded structured",
			err:  &fileops.MaxFilesExceededDetailError{MaxFiles: 3, WouldCreateCount: 4},
			want: ErrorOutcome{
				Error:    "max_files_exceeded",
				Hint:     (&fileops.MaxFilesExceededDetailError{MaxFiles: 3, WouldCreateCount: 4}).Error(),
				ExitCode: ExitValidation,
				IntFields: map[string]int{
					"max_files":          3,
					"would_create_count": 4,
				},
			},
		},
		{
			name: "max files exceeded fileops sentinel classifies internal_error",
			err:  fileops.ErrMaxFilesExceeded,
			want: ErrorOutcome{
				Error:    "internal_error",
				Src:      classifyTestPrimaryPath,
				Hint:     fileops.ErrMaxFilesExceeded.Error(),
				ExitCode: ExitGeneral,
			},
		},
		{
			name: "wrapped fileops max files exceeded classifies internal_error",
			err:  fmt.Errorf("bounded scan: %w", fileops.ErrMaxFilesExceeded),
			want: ErrorOutcome{
				Error:    "internal_error",
				Src:      classifyTestPrimaryPath,
				Hint:     "bounded scan: maximum output file count exceeded",
				ExitCode: ExitGeneral,
			},
		},
		{
			name: "wrapped pipeline max files exceeded classifies internal_error",
			err:  fmt.Errorf("split: %w", ErrMaxFilesExceeded),
			want: ErrorOutcome{
				Error:    "internal_error",
				Src:      classifyTestPrimaryPath,
				Hint:     "split: maximum output file count exceeded",
				ExitCode: ExitGeneral,
			},
		},
		{
			name: "max files at least one",
			err:  ErrMaxFilesAtLeastOne,
			want: ErrorOutcome{Error: "invalid_input", Hint: ErrMaxFilesAtLeastOne.Error(), ExitCode: ExitValidation},
		},
	}
}

func operationNameCountLimitCases() []errorOutcomeCase {
	return []errorOutcomeCase{
		{
			name: "names count mismatch bare sentinel classifies internal_error",
			err:  ErrNamesCountMismatch,
			want: ErrorOutcome{
				Error:    "internal_error",
				Src:      classifyTestPrimaryPath,
				Hint:     ErrNamesCountMismatch.Error(),
				ExitCode: ExitGeneral,
			},
		},
		{
			name: "names count mismatch structured",
			err:  &NamesCountMismatchError{NamesCount: 2, OutputCount: 3},
			want: ErrorOutcome{
				Error:    "names_count_mismatch",
				Hint:     (&NamesCountMismatchError{NamesCount: 2, OutputCount: 3}).Error(),
				ExitCode: ExitValidation,
				IntFields: map[string]int{
					"names_count":  2,
					"output_count": 3,
				},
			},
		},
		{
			name: "names count mismatch preserves zero output_count",
			err:  &NamesCountMismatchError{NamesCount: 1, OutputCount: 0},
			want: ErrorOutcome{
				Error:    "names_count_mismatch",
				Hint:     (&NamesCountMismatchError{NamesCount: 1, OutputCount: 0}).Error(),
				ExitCode: ExitValidation,
				IntFields: map[string]int{
					"names_count":  1,
					"output_count": 0,
				},
			},
		},
	}
}

func operationTransformInputCases() []errorOutcomeCase {
	return []errorOutcomeCase{
		{
			name: "invalid line endings",
			err:  fmt.Errorf("%w: line-endings must be lf, crlf, or cr", ErrInvalidLineEndings),
			want: ErrorOutcome{
				Error:    "invalid_line_endings",
				Hint:     "invalid line-endings value: line-endings must be lf, crlf, or cr",
				ExitCode: ExitValidation,
			},
		},
		{
			name: "no transform specified",
			err:  ErrNoTransformSpecified,
			want: ErrorOutcome{Error: "no_transform_specified", Hint: HintNoTransformSpecified, ExitCode: ExitValidation},
		},
		{
			name: "line range parse",
			err:  linparse.NewLineRangeParseError(errBadLines),
			want: ErrorOutcome{Error: "invalid_input", Hint: "parse lines: bad lines", ExitCode: ExitValidation},
		},
	}
}

func TestClassifyOperationError_PathAndNameValidation(t *testing.T) {
	t.Parallel()
	runErrorOutcomeCases(t, []errorOutcomeCase{
		{
			name: "invalid extension",
			err:  validate.ErrInvalidExtension,
			want: ErrorOutcome{Error: "invalid_input", Hint: validate.ErrInvalidExtension.Error(), ExitCode: ExitValidation},
		},
		{
			name: "path traversal",
			err:  validate.ErrPathTraversal,
			want: sourceErrorOutcome("invalid_input", validate.ErrPathTraversal.Error(), ExitValidation),
		},
		{
			name: "outside root",
			err:  validate.ErrOutsideRoot,
			want: sourceErrorOutcome("invalid_input", validate.ErrOutsideRoot.Error(), ExitValidation),
		},
		{
			name: "reserved name",
			err:  validate.ErrReservedName,
			want: sourceErrorOutcome("invalid_input", validate.ErrReservedName.Error(), ExitValidation),
		},
		{
			name: "empty regexp pattern sentinel (pipeline)",
			err:  ErrEmptyRegexpPattern,
			want: ErrorOutcome{Error: "invalid_input", Hint: validate.ErrEmptyRegexpPattern.Error(), ExitCode: ExitValidation},
		},
		{
			name: "empty regexp pattern sentinel (validate)",
			err:  validate.ErrEmptyRegexpPattern,
			want: ErrorOutcome{Error: "invalid_input", Hint: validate.ErrEmptyRegexpPattern.Error(), ExitCode: ExitValidation},
		},
		{
			name: "invalid line range sentinel",
			err:  linparse.ErrInvalidLineRange,
			want: ErrorOutcome{Error: "invalid_input", Hint: linparse.ErrInvalidLineRange.Error(), ExitCode: ExitValidation},
		},
		{
			name: "empty line range sentinel",
			err:  linparse.ErrEmptyLineRange,
			want: ErrorOutcome{Error: "invalid_input", Hint: linparse.ErrEmptyLineRange.Error(), ExitCode: ExitValidation},
		},
		{
			name: "invalid explicit output name",
			err:  ErrInvalidExplicitName,
			want: ErrorOutcome{Error: "invalid_input", Hint: ErrInvalidExplicitName.Error(), ExitCode: ExitValidation},
		},
		{
			name: "empty explicit names list entry",
			err:  ErrEmptyNamesListEntry,
			want: ErrorOutcome{Error: "invalid_input", Hint: ErrEmptyNamesListEntry.Error(), ExitCode: ExitValidation},
		},
		{
			name: "duplicate explicit output names",
			err:  ErrDuplicateExplicitNames,
			want: ErrorOutcome{Error: "invalid_input", Hint: ErrDuplicateExplicitNames.Error(), ExitCode: ExitValidation},
		},
	})
}

func TestClassifyOperationError_GenericFallback(t *testing.T) {
	t.Parallel()
	runErrorOutcomeCases(t, []errorOutcomeCase{
		{
			name: "unrecognized error produces internal_error",
			err:  errClassifyTestUnexpected,
			want: sourceErrorOutcome("internal_error", "something unexpected happened", ExitGeneral),
		},
		{
			name: "wrapped unrecognized error produces internal_error",
			err:  fmt.Errorf("wrapper: %w", errClassifyTestDeep),
			want: sourceErrorOutcome("internal_error", "wrapper: deep error", ExitGeneral),
		},
	})
}
