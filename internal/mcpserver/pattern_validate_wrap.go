package mcpserver

import (
	"errors"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

func wrapValidatePatternAsFieldError(field string, err error) error {
	if errors.Is(err, validate.ErrInvalidPattern) ||
		errors.Is(err, validate.ErrEmptyRegexpPattern) ||
		errors.Is(err, validate.ErrPatternTooLong) ||
		errors.Is(err, validate.ErrControlChar) {
		return &pipeline.PatternFieldError{Field: field, Cause: err}
	}

	return err
}
