package mcpserver

import (
	"errors"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

var errPatternWrapTestOtherValidation = errors.New("other validation failure")

func TestWrapValidatePatternAsFieldErrorWrapsEmptyPattern(t *testing.T) {
	t.Parallel()

	err := wrapValidatePatternAsFieldError("delimiter", validate.ErrEmptyRegexpPattern)

	var fieldErr *pipeline.PatternFieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("wrapped error: got %T want *pipeline.PatternFieldError", err)
	}
	if fieldErr.Field != "delimiter" {
		t.Fatalf("field: got %q want delimiter", fieldErr.Field)
	}
	if !errors.Is(fieldErr.Cause, validate.ErrEmptyRegexpPattern) {
		t.Fatalf("cause: got %v want ErrEmptyRegexpPattern", fieldErr.Cause)
	}
}

func TestWrapValidatePatternAsFieldErrorPassesThroughUnclassifiedErrors(t *testing.T) {
	t.Parallel()

	err := wrapValidatePatternAsFieldError("delimiter", errPatternWrapTestOtherValidation)
	if !errors.Is(err, errPatternWrapTestOtherValidation) {
		t.Fatalf("error: got %v want %v", err, errPatternWrapTestOtherValidation)
	}

	var fieldErr *pipeline.PatternFieldError
	if errors.As(err, &fieldErr) {
		t.Fatalf("wrapped error: got PatternFieldError for unclassified error")
	}
}
