package steps

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/linparse"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

// Test-only sentinel errors satisfy err113 (no dynamic errors.New inside tests).
var (
	errTestBadLines = errors.New("bad lines")
	errTestBoom     = errors.New("boom")
	errTestSyntax   = errors.New("syntax")
)

type operationErrorClassCase struct {
	name  string
	err   error
	class string
	want  bool
}

func operationErrorClassBddCases() []operationErrorClassCase {
	cases := operationErrorClassSourceCases()
	cases = append(cases, operationErrorClassRangeCases()...)
	cases = append(cases, operationErrorClassInvalidInputCases()...)
	cases = append(cases, operationErrorClassWrappedCases()...)

	return append(cases, operationErrorClassNegativeCases()...)
}

func operationErrorClassSourceCases() []operationErrorClassCase {
	return []operationErrorClassCase{
		{name: "source_not_found", err: pipeline.ErrSourceNotFound, class: "source_not_found", want: true},
		{name: "binary_source", err: pipeline.ErrBinarySource, class: "binary_source", want: true},
		{
			name:  "destination_exists",
			err:   &pipeline.DestinationExistsError{Path: "out/001.md"},
			class: "destination_exists",
			want:  true,
		},
	}
}

func operationErrorClassRangeCases() []operationErrorClassCase {
	return []operationErrorClassCase{
		{
			name:  "empty_range structured",
			err:   &fileops.EmptyRangeError{Start: 2, End: 1},
			class: "empty_range",
			want:  true,
		},
		{name: "empty_range bare is internal_error", err: fileops.ErrEmptyRange, class: "empty_range", want: false},
		{name: "empty_range bare matches internal_error", err: fileops.ErrEmptyRange, class: "internal_error", want: true},
		{
			name:  "range_exceeds_file structured",
			err:   &fileops.RangeExceedsFileError{FileLines: 5, RangeStart: 1, RangeEnd: 99},
			class: "range_exceeds_file",
			want:  true,
		},
		{
			name:  "range_exceeds_file bare is internal_error",
			err:   fileops.ErrRangeExceedsFile,
			class: "range_exceeds_file",
			want:  false,
		},
	}
}

func operationErrorClassInvalidInputCases() []operationErrorClassCase {
	return []operationErrorClassCase{
		{
			name:  "invalid_input max_files",
			err:   pipeline.ErrMaxFilesAtLeastOne,
			class: "invalid_input",
			want:  true,
		},
		{name: "invalid_input extension", err: validate.ErrInvalidExtension, class: "invalid_input", want: true},
		{name: "invalid_input traversal", err: validate.ErrPathTraversal, class: "invalid_input", want: true},
		{name: "invalid_input outside root", err: validate.ErrOutsideRoot, class: "invalid_input", want: true},
		{name: "invalid_input reserved", err: validate.ErrReservedName, class: "invalid_input", want: true},
		{
			name:  "invalid_input line range parse sentinel",
			err:   linparse.ErrLineRangeParse,
			class: "invalid_input",
			want:  true,
		},
		{name: "invalid_input invalid line range", err: linparse.ErrInvalidLineRange, class: "invalid_input", want: true},
		{name: "invalid_input empty line range", err: linparse.ErrEmptyLineRange, class: "invalid_input", want: true},
		{
			name:  "invalid_input explicit name",
			err:   pipeline.ErrInvalidExplicitName,
			class: "invalid_input",
			want:  true,
		},
		{
			name:  "invalid_input duplicate names",
			err:   pipeline.ErrDuplicateExplicitNames,
			class: "invalid_input",
			want:  true,
		},
		{
			name:  "invalid_input line range parse wrapper",
			err:   linparse.NewLineRangeParseError(errTestBadLines),
			class: "invalid_input",
			want:  true,
		},
	}
}

func operationErrorClassWrappedCases() []operationErrorClassCase {
	return []operationErrorClassCase{
		{
			name: "wrapped source_not_found",
			err: fmt.Errorf(
				"open: %w",
				pipeline.ErrSourceNotFound,
			),
			class: "source_not_found",
			want:  true,
		},
	}
}

func operationErrorClassNegativeCases() []operationErrorClassCase {
	return []operationErrorClassCase{
		{
			name:  "mismatch class",
			err:   pipeline.ErrSourceNotFound,
			class: "binary_source",
			want:  false,
		},
		{
			name:  "internal_error not source_not_found",
			err:   errTestBoom,
			class: "source_not_found",
			want:  false,
		},
		{name: "nil error", err: nil, class: "source_not_found", want: false},
		{name: "empty expected class", err: pipeline.ErrSourceNotFound, class: "", want: false},
	}
}

func TestErrorMatchesOperationErrorClass_BddClasses(t *testing.T) {
	t.Parallel()

	for _, tc := range operationErrorClassBddCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errUnderTest := tc.err

			if got := ErrorMatchesOperationErrorClass(errUnderTest, tc.class); got != tc.want {
				classified := ""
				if errUnderTest != nil {
					classified = pipeline.ClassifyOperationError(errUnderTest, "").Error
				}

				t.Fatalf(
					"ErrorMatchesOperationErrorClass(...) = %v, want %v (classifier error field: %q)",
					got,
					tc.want,
					classified,
				)
			}
		})
	}
}

func TestErrorMatchesOperationErrorClass_WrappedErrorsUseErrorsIs(t *testing.T) {
	t.Parallel()

	wrappedBare := fmt.Errorf("pipeline: %w", fileops.ErrRangeExceedsFile)
	if !errors.Is(wrappedBare, fileops.ErrRangeExceedsFile) {
		t.Fatal("precondition: setup should use errors.Is chain")
	}

	if ErrorMatchesOperationErrorClass(wrappedBare, "range_exceeds_file") {
		t.Fatalf("bare wrapped range error must not classify as range_exceeds_file without typed detail, got %q",
			pipeline.ClassifyOperationError(wrappedBare, "").Error)
	}

	if !ErrorMatchesOperationErrorClass(wrappedBare, "internal_error") {
		t.Fatalf(
			"wrapped bare range error should classify as internal_error, got %q",
			pipeline.ClassifyOperationError(wrappedBare, "").Error,
		)
	}

	structuredWrapped := fmt.Errorf("wrap: %w", &fileops.RangeExceedsFileError{
		FileLines: 10, RangeStart: 1, RangeEnd: 50,
	})
	if !ErrorMatchesOperationErrorClass(structuredWrapped, "range_exceeds_file") {
		t.Fatalf(
			"wrapped typed range error should classify as range_exceeds_file, got %q",
			pipeline.ClassifyOperationError(structuredWrapped, "").Error,
		)
	}

	parseWrapped := fmt.Errorf("cli: %w", linparse.NewLineRangeParseError(errTestSyntax))
	if !errors.Is(parseWrapped, linparse.ErrLineRangeParse) {
		t.Fatal("precondition: line range parse wrapper should unwrap to ErrLineRangeParse")
	}

	if !ErrorMatchesOperationErrorClass(parseWrapped, "invalid_input") {
		t.Fatalf(
			"wrapped parse error should classify as invalid_input, got %q",
			pipeline.ClassifyOperationError(parseWrapped, "").Error,
		)
	}
}

func TestAssertOperationErrorClass(t *testing.T) {
	t.Parallel()

	if err := AssertOperationErrorClass(pipeline.ErrSourceNotFound, "source_not_found", ""); err != nil {
		t.Fatalf("AssertOperationErrorClass success: %v", err)
	}

	if err := AssertOperationErrorClass(nil, "source_not_found", ""); err == nil {
		t.Fatal("expected non-nil error when err is nil")
	} else if !errors.Is(err, errOperationErrorClassMismatch) {
		t.Fatalf("expected %v, got %v", errOperationErrorClassMismatch, err)
	}

	if err := AssertOperationErrorClass(pipeline.ErrSourceNotFound, "", ""); err == nil {
		t.Fatal("expected non-nil error when expected class is empty")
	} else if !errors.Is(err, errOperationErrorClassMismatch) {
		t.Fatalf("expected %v, got %v", errOperationErrorClassMismatch, err)
	}

	if err := AssertOperationErrorClass(pipeline.ErrBinarySource, "source_not_found", ""); err == nil {
		t.Fatal("expected class mismatch error")
	} else if !errors.Is(err, errOperationErrorClassMismatch) {
		t.Fatalf("expected %v, got %v", errOperationErrorClassMismatch, err)
	}
}
