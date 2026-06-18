package fileops

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"regexp"
	"testing"
)

func TestScanBlocksMetaRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	_, err := ScanBlocksMeta(context.Background(), BlocksOptions{}, BoundedScanLimits{})
	if !errors.Is(err, errBlocksNilStart) {
		t.Fatalf("nil start error = %v want %v", err, errBlocksNilStart)
	}

	_, err = ScanBlocksMeta(context.Background(), BlocksOptions{
		StartDelimiter: regexp.MustCompile(`^start$`),
	}, BoundedScanLimits{})
	if !errors.Is(err, errBlocksNilEnd) {
		t.Fatalf("nil end error = %v want %v", err, errBlocksNilEnd)
	}
}

func TestScanBlocksMetaRejectsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ScanBlocksMeta(ctx, BlocksOptions{
		Source:         bytes.NewReader(nil),
		StartDelimiter: regexp.MustCompile(`^start$`),
		EndDelimiter:   regexp.MustCompile(`^end$`),
	}, BoundedScanLimits{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context error = %v want %v", err, context.Canceled)
	}
}

func TestScanBlocksMetaRejectsNonSeekableSource(t *testing.T) {
	t.Parallel()

	_, err := ScanBlocksMeta(context.Background(), BlocksOptions{
		Source:         bytes.NewBufferString("start\nend\n"),
		StartDelimiter: regexp.MustCompile(`^start$`),
		EndDelimiter:   regexp.MustCompile(`^end$`),
	}, BoundedScanLimits{})
	if !errors.Is(err, ErrSeekableSourceRequired) {
		t.Fatalf("non-seekable source error = %v want %v", err, ErrSeekableSourceRequired)
	}
}

func TestIsolateMatchLineSpanRejectsOversizedOffset(t *testing.T) {
	t.Parallel()

	span := LineSpan{SerializedEnd: uint64(math.MaxInt64) + 1}
	err := isolateMatchLineSpanOnSharedSeeker(bytes.NewReader(nil), span, func() error {
		return io.ErrUnexpectedEOF
	})
	if !errors.Is(err, ErrLineSpanOffsetTooLarge) {
		t.Fatalf("oversized span error = %v want %v", err, ErrLineSpanOffsetTooLarge)
	}
}
