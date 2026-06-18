package fileops

import (
	"context"
	"errors"
	"fmt"
	"io"
)

const errExtractWrapFmt = "extract: %w"

// errExtractClosedRangeComplete stops line streaming after the inclusive end line
// is complete for a closed range. It is internal and never returned to callers.
var errExtractClosedRangeComplete = errors.New("fileops: extract closed range complete")

// LineRange represents an inclusive 1-based line range.
type LineRange struct {
	Start int // 1-based start line; 0 means "from beginning" (i.e., line 1)
	End   int // 1-based end line; 0 means "to end of file"
}

// ExtractOptions configures the extract operation.
type ExtractOptions struct {
	Source io.Reader
	Lines  LineRange
	Append bool
}

// ExtractResult contains the result of an extract operation.
type ExtractResult struct {
	LinesExtracted int

	// WouldCreatePath is the absolute native destination path for pipeline preview runs; unset (empty) when
	// not applicable or when produced by streaming Extract helpers alone.
	WouldCreatePath string
}

// extractClosedRangeSelectedLines reads logical lines via ForEachLineFromContext,
// appending only lines in [normStart, lr.End] to selected until the inclusive end line is complete.
func extractClosedRangeSelectedLines(
	ctx context.Context,
	src io.Reader,
	lr LineRange,
	normStart int,
) (selected []Line, lineNum int, err error) {
	err = ForEachLineFromContext(ctx, src, func(ln Line) error {
		lineNum++

		if lineNum >= normStart && lineNum <= lr.End {
			selected = append(selected, ln)
		}

		if lineNum == lr.End {
			return errExtractClosedRangeComplete
		}

		return nil
	})
	if errors.Is(err, errExtractClosedRangeComplete) {
		err = nil
	}

	return selected, lineNum, err
}

// extractOpenEndedToWriter streams lines via ForEachLineFromContext and writes each selected
// line to dest as soon as the line number reaches normStart. No buffering of the tail.
func extractOpenEndedToWriter(
	ctx context.Context,
	src io.Reader,
	dest io.Writer,
	normStart int,
) (linesExtracted, lineNum int, err error) {
	err = ForEachLineFromContext(ctx, src, func(ln Line) error {
		lineNum++
		if lineNum < normStart {
			return nil
		}

		if werr := WriteLinesTo(dest, []Line{ln}); werr != nil {
			return werr
		}

		linesExtracted++

		return nil
	})

	return linesExtracted, lineNum, err
}

// extractDispatchClosed handles inclusive end line selection: buffer selected lines, validate range, write once.
func extractDispatchClosed(
	ctx context.Context,
	opts ExtractOptions,
	dest io.Writer,
	normStart int,
) (ExtractResult, error) {
	selected, lineNum, streamErr := extractClosedRangeSelectedLines(ctx, opts.Source, opts.Lines, normStart)
	if streamErr != nil {
		return ExtractResult{}, fmt.Errorf(errExtractWrapFmt, streamErr)
	}

	lineCount := lineNum
	if lineNum >= opts.Lines.End {
		lineCount = opts.Lines.End
	}

	if err := ValidateExtractRange(opts.Lines, lineCount); err != nil {
		return ExtractResult{}, err
	}

	if werr := WriteLinesTo(dest, selected); werr != nil {
		return ExtractResult{}, fmt.Errorf(errExtractWrapFmt, werr)
	}

	return ExtractResult{LinesExtracted: len(selected)}, nil
}

// extractDispatchOpen streams from normStart through EOF, validates range against lines seen.
func extractDispatchOpen(
	ctx context.Context,
	opts ExtractOptions,
	dest io.Writer,
	normStart int,
) (ExtractResult, error) {
	linesExtracted, lineNum, streamErr := extractOpenEndedToWriter(ctx, opts.Source, dest, normStart)
	if streamErr != nil {
		return ExtractResult{}, fmt.Errorf(errExtractWrapFmt, streamErr)
	}

	if lineNum < normStart {
		return ExtractResult{}, &RangeExceedsFileError{
			FileLines:  lineNum,
			RangeStart: normStart,
			RangeEnd:   opts.Lines.End,
		}
	}

	if err := ValidateExtractRange(opts.Lines, lineNum); err != nil {
		return ExtractResult{}, err
	}

	return ExtractResult{LinesExtracted: linesExtracted}, nil
}

// Extract copies the specified line range from source to dest, preserving byte fidelity.
//
// Seekable sources use extractViaSerializedSpan: span planning plus raw byte copy to dest (no full-line helper
// materialization). Non-seekable sources use full-line helper paths: ForEachLineFromContext replay; closed
// ranges buffer selected Lines in RAM. Callers should prefer seekable sources when bounded memory matters.
func Extract(ctx context.Context, opts ExtractOptions, dest io.Writer) (ExtractResult, error) {
	if err := ctx.Err(); err != nil {
		return ExtractResult{}, fmt.Errorf(errExtractWrapFmt, err)
	}

	normStart := opts.Lines.Start
	if normStart == 0 {
		normStart = 1
	}

	if opts.Lines.End != 0 && normStart > opts.Lines.End {
		return ExtractResult{}, &EmptyRangeError{Start: opts.Lines.Start, End: opts.Lines.End}
	}

	if rs, ok := opts.Source.(io.ReadSeeker); ok {
		return extractViaSerializedSpan(ctx, rs, opts.Lines, dest)
	}

	if opts.Lines.End != 0 {
		return extractDispatchClosed(ctx, opts, dest, normStart)
	}

	return extractDispatchOpen(ctx, opts, dest, normStart)
}
