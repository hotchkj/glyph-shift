package fileops

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// ErrMaxFilesExceeded is returned when a bounded scan exceeds BoundedScanLimits.MaxFiles.
//
// Package pipeline exposes the same value as pipeline.ErrMaxFilesExceeded so either name shares
// errors.Is behavior.
var ErrMaxFilesExceeded = errors.New("maximum output file count exceeded")

// MaxFilesExceededDetailError carries limit and cardinality for contract JSON classification while
// preserving errors.Is(.., ErrMaxFilesExceeded).
type MaxFilesExceededDetailError struct {
	MaxFiles         int
	WouldCreateCount int
}

func (e *MaxFilesExceededDetailError) Error() string {
	return fmt.Sprintf("%v: would create %d outputs (limit %d)", ErrMaxFilesExceeded, e.WouldCreateCount, e.MaxFiles)
}

func (e *MaxFilesExceededDetailError) Unwrap() error {
	return ErrMaxFilesExceeded
}

// BoundedScanLimits caps output-file cardinality during streaming scans.
//
// MaxFiles <= 0 disables the cap.
//
// Phase 4/5: pass effectiveMaxFiles from pipeline before invoking these scans.
type BoundedScanLimits struct {
	MaxFiles int
}

func (l BoundedScanLimits) outputSectionsOK(nextTotal int) bool {
	if l.MaxFiles <= 0 {
		return true
	}

	return nextTotal <= l.MaxFiles
}

func enforceMaxSections(limits BoundedScanLimits, next int) error {
	if limits.outputSectionsOK(next) {
		return nil
	}

	return &MaxFilesExceededDetailError{MaxFiles: limits.MaxFiles, WouldCreateCount: next}
}

// errStopSplitPreambleAfterFirstLineSpan stops seekable line-span scanning after capturing the first logical
// line span for split preamble naming (FromContent). It is internal to preambleSplitOutputFilename.
var errStopSplitPreambleAfterFirstLineSpan = errors.New("fileops: split preamble first line span captured")

var (
	errPreambleNoFirstLineSpan       = errors.New("split preamble naming: no first line span")
	errPreambleUnexpectedScanOutcome = errors.New("split preamble naming: unexpected scan outcome")
)

// preambleSplitOutputFilenameFromContent resolves FromContent naming for split preamble by scanning the
// first logical line from the start of the seekable source.
func preambleSplitOutputFilenameFromContent(
	ctx context.Context,
	rs io.ReadSeeker,
	opts SplitOptions,
	seq int,
	existing map[string]bool,
) (string, error) {
	cursor, err := rs.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", err
	}

	defer func() {
		_, _ = rs.Seek(cursor, io.SeekStart)
	}()

	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	firstSpan, got, scanErr := firstPreambleLineSpan(ctx, rs)
	return preambleFilenameFromFirstSpan(ctx, rs, opts, seq, existing, firstSpan, got, scanErr)
}

func preambleFilenameFromFirstSpan(
	ctx context.Context,
	rs io.ReadSeeker,
	opts SplitOptions,
	seq int,
	existing map[string]bool,
	firstSpan LineSpan,
	got bool,
	scanErr error,
) (string, error) {
	if got && errors.Is(scanErr, errStopSplitPreambleAfterFirstLineSpan) {
		text, txtErr := lineSpanNamingContentUTF8(ctx, rs, firstSpan)
		if txtErr != nil {
			return "", txtErr
		}

		base := GenerateFilename(FromContent, seq, text, opts.Extension)

		return DeduplicateFilename(base, existing), nil
	}

	if scanErr != nil && !errors.Is(scanErr, errStopSplitPreambleAfterFirstLineSpan) {
		return "", fmt.Errorf("split preamble naming: %w", scanErr)
	}

	if !got {
		return "", errPreambleNoFirstLineSpan
	}

	return "", errPreambleUnexpectedScanOutcome
}

func firstPreambleLineSpan(ctx context.Context, rs io.ReadSeeker) (LineSpan, bool, error) {
	var firstSpan LineSpan

	var got bool

	scanErr := scanLineSpansWithOptions(ctx, rs, func(span LineSpan) error {
		firstSpan = span
		got = true

		return errStopSplitPreambleAfterFirstLineSpan
	}, linespanScanOptions{})

	return firstSpan, got, scanErr
}

// preambleSplitOutputFilename assigns the first output section filename when non-empty preamble exists
// before the first delimiter match. seq is the 1-based ordinal among emitted section files (starts at 1 when
// preamble is the first section). IsPreambleSection still tags internal metadata but must not affect naming.
func preambleSplitOutputFilename(
	ctx context.Context,
	rs io.ReadSeeker,
	opts SplitOptions,
	seq int,
	existing map[string]bool,
) (string, error) {
	switch opts.Naming {
	case Sequential:
		return chooseSequentialSectionFilename(seq, opts.Extension, existing), nil
	case FromDelimiter:
		base := GenerateFilename(FromDelimiter, seq, "", opts.Extension)

		return DeduplicateFilename(base, existing), nil
	case FromContent:
		return preambleSplitOutputFilenameFromContent(ctx, rs, opts, seq, existing)
	default:
		return chooseSequentialSectionFilename(seq, opts.Extension, existing), nil
	}
}

func dupLine(ln Line) Line {
	var contentDup []byte
	if len(ln.Content) > 0 {
		contentDup = append([]byte(nil), ln.Content...)
	}

	var termDup []byte
	if len(ln.Terminator) > 0 {
		termDup = append([]byte(nil), ln.Terminator...)
	}

	return Line{Content: contentDup, Terminator: termDup}
}

//nolint:gocritic // tooManyResults: tuple mirrors split segmentation math without allocating a struct sink.
func computeSplitSegOutput(
	strip bool,
	dStart, nextDelimLine, secLen int,
	delimLineByteStart, delimLineByteLen, byteBeforeNextDelim uint64,
) (outStart, outEnd, clen int, bFrom, bTo uint64, skip bool) {
	if strip {
		if secLen <= 1 {
			return 0, 0, 0, 0, 0, true
		}

		outStart = dStart + 1
		outEnd = nextDelimLine - 1
		clen = secLen - 1
		bFrom = delimLineByteStart + delimLineByteLen
		bTo = byteBeforeNextDelim

		return outStart, outEnd, clen, bFrom, bTo, false
	}

	outStart = dStart
	outEnd = nextDelimLine - 1
	clen = secLen
	bFrom = delimLineByteStart
	bTo = byteBeforeNextDelim

	return outStart, outEnd, clen, bFrom, bTo, false
}
