package fileops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
)

// ExtractSerializedSpanPlan describes the contiguous on-disk interval that serializes exactly the logical
// lines selected by lr. Interval is half-open [SerializedStart, SerializedEndExclusive).
//
// LinesInFile counts logical lines emitted by ScanLineSpans (last LineNum seen).
// LinesSelected counts lines counted toward the extraction output span.
type ExtractSerializedSpanPlan struct {
	SerializedStart        uint64
	SerializedEndExclusive uint64
	LinesInFile            int
	LinesSelected          int
}

func validationLineCountForExtract(lr LineRange, linesInFile int) int {
	lineCount := linesInFile
	if lr.End != 0 && linesInFile >= lr.End {
		lineCount = lr.End
	}

	return lineCount
}

var errExtractMissingSerializedSpan = errors.New("extract: missing serialized span despite validated range")

type spanPlanScan struct {
	lr               LineRange
	normStart        int
	normEndInclusive int

	plan *ExtractSerializedSpanPlan
}

func (s *spanPlanScan) onLine(span LineSpan) error {
	s.plan.LinesInFile = span.LineNum

	inRange := span.LineNum >= s.normStart && (s.lr.End == 0 || span.LineNum <= s.normEndInclusive)
	if inRange {
		if s.plan.LinesSelected == 0 {
			s.plan.SerializedStart = span.SerializedStart
		}

		s.plan.SerializedEndExclusive = span.SerializedEnd
		s.plan.LinesSelected++
	}

	if s.lr.End != 0 && span.LineNum == s.normEndInclusive {
		return errExtractClosedRangeComplete
	}

	return nil
}

// PlanExtractSerializedSpan scans line spans until the inclusive end boundary is satisfied or EOF,
// records the contiguous serialized-byte interval for lr, and validates the range via normalizeExtractRange
// (sentinels ErrEmptyRange and ErrRangeExceedsFile are preserved). The scan does not materialize full
// logical Line bodies.
func PlanExtractSerializedSpan(
	ctx context.Context,
	src io.ReadSeeker,
	lr LineRange,
) (ExtractSerializedSpanPlan, error) {
	var zero ExtractSerializedSpanPlan

	normStart := lr.Start
	if normStart == 0 {
		normStart = 1
	}

	normEndInclusive := lr.End

	var plan ExtractSerializedSpanPlan

	scan := spanPlanScan{lr: lr, normStart: normStart, normEndInclusive: normEndInclusive, plan: &plan}

	err := scanLineSpansWithOptions(ctx, src, scan.onLine, linespanScanOptions{
		chunkSizeBytes: boundedEarlyExitLinespanChunkSize,
	})
	if errors.Is(err, errExtractClosedRangeComplete) {
		err = nil
	}

	if err != nil {
		return zero, fmt.Errorf(errExtractWrapFmt, err)
	}

	if err := validateExtractSpanPlan(lr, normStart, &plan); err != nil {
		return zero, err
	}

	return plan, nil
}

func validateExtractSpanPlan(lr LineRange, normStart int, plan *ExtractSerializedSpanPlan) error {
	if lr.End == 0 && plan.LinesInFile < normStart {
		return &RangeExceedsFileError{
			FileLines:  plan.LinesInFile,
			RangeStart: normStart,
			RangeEnd:   lr.End,
		}
	}

	lineCount := validationLineCountForExtract(lr, plan.LinesInFile)
	if valErr := normalizeExtractRange(lr, lineCount); valErr != nil {
		return valErr
	}

	if plan.LinesSelected == 0 {
		return fmt.Errorf(errExtractWrapFmt, errExtractMissingSerializedSpan)
	}

	return nil
}

// extractViaSerializedSpan implements the seekable Extract path: plan serialized byte bounds, then copy that
// span without materializing full logical Line values (unlike full-line helper replay in extract.go).
func extractViaSerializedSpan(
	ctx context.Context,
	src io.ReadSeeker,
	lr LineRange,
	dest io.Writer,
) (ExtractResult, error) {
	plan, err := PlanExtractSerializedSpan(ctx, src, lr)
	if err != nil {
		return ExtractResult{}, err
	}

	if err := copyExtractPlanBytesToWriter(ctx, dest, src, plan); err != nil {
		return ExtractResult{}, fmt.Errorf(errExtractWrapFmt, err)
	}

	return ExtractResult{LinesExtracted: plan.LinesSelected}, nil
}

// SerializedSpanSeekAndLength maps half-open serialized bounds to values suitable for Seek and fixed-size
// chunk copying.
func SerializedSpanSeekAndLength(serializedStart, serializedEndExclusive uint64) (seekOff, byteLen int64, err error) {
	if serializedEndExclusive < serializedStart {
		return 0, 0, errSerializedSpanBoundsInverted
	}

	delta := serializedEndExclusive - serializedStart
	if delta > uint64(math.MaxInt64) {
		return 0, 0, ErrLineSpanContentTooLarge
	}

	if serializedStart > uint64(math.MaxInt64) {
		return 0, 0, ErrLineSpanOffsetTooLarge
	}

	return int64(serializedStart), int64(delta), nil
}

var errSerializedSpanBoundsInverted = errors.New("serialized span end before start")

func copyExtractPlanBytesToWriter(
	ctx context.Context,
	dest io.Writer,
	src io.ReadSeeker,
	plan ExtractSerializedSpanPlan,
) error {
	seekOff, byteLen, berr := SerializedSpanSeekAndLength(plan.SerializedStart, plan.SerializedEndExclusive)
	if berr != nil {
		return berr
	}

	if byteLen == 0 {
		return nil
	}

	if err := seekSourceToAbsolute(src, seekOff); err != nil {
		return fmt.Errorf("seek source for extract span: %w", err)
	}

	if err := copySourceSpanChunks(ctx, dest, src, byteLen); err != nil {
		return err
	}

	return nil
}
