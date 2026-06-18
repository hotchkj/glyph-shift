package fileops

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
)

// ErrLineSpanContentTooLarge is returned when the half-open content span length does not fit in int64,
// so a bounded LimitReader cannot be constructed without silent truncation on some platforms.
var ErrLineSpanContentTooLarge = errors.New("fileops: line span content length overflows int64 for bounded reads")

// ErrLineSpanOffsetTooLarge is returned when ContentStart does not fit in int64,
// so io.Seek(offset, io.SeekStart) cannot be invoked without silently truncating the offset.
var ErrLineSpanOffsetTooLarge = errors.New("fileops: line span content start overflows int64 for seek")

var (
	errMatchLineSpanNilRegexp        = errors.New("fileops.MatchLineSpan: nil regexp")
	errFindLineSpanSubmatchNilRegexp = errors.New("fileops.FindLineSpanSubmatchIndex: nil regexp")
	errLineSpanReaderInvertedOffsets = errors.New("new line span reader: inverted content offsets")
)

const (
	errFmtMatchLineSpanRead = "match line span read: %w"
	errFmtMatchLineSpan     = "match line span: %w"
)

// MatchLineSpan reports whether regexp re matches within the CONTENT byte span
// ([ContentStart, ContentEnd)) only.
// The implementation seeks to ContentStart then constrains Reads with io.LimitReader so regexp cannot
// observe bytes past the trailing content edge.
//
// Matching does not materialize the full line body in a single []byte allocation; only the bounded
// reader sees stream bytes incrementally.
//
// Stdlib regexp reader paths treat arbitrary ReadRune errors as textual end-of-input; this helper
// therefore verifies ctx.Err after the engine returns before reporting any definitive false negative.
func MatchLineSpan(ctx context.Context, src io.ReadSeeker, span LineSpan, re *regexp.Regexp) (bool, error) {
	if re == nil {
		return false, errMatchLineSpanNilRegexp
	}

	if literal, exact, ok := literalLineSpanMatch(re); ok {
		return matchLiteralLineSpan(ctx, src, span, literal, exact)
	}

	rr, err := newLineSpanRuneReader(ctx, src, span)
	if err != nil {
		return false, err
	}

	matched := re.MatchReader(rr)

	if rr.rdSticky != nil {
		return false, fmt.Errorf(errFmtMatchLineSpanRead, rr.rdSticky)
	}

	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf(errFmtMatchLineSpan, err)
	}

	return matched, nil
}

// FindLineSpanSubmatchIndex returns index pairs analogous to regexp.FindReaderSubmatchIndex confined
// to the CONTENT span.
// Returned indices are UTF-8 byte offsets counted from ContentStart within the line body; shift by
// span.ContentStart for absolute file offsets.
func FindLineSpanSubmatchIndex(
	ctx context.Context,
	src io.ReadSeeker,
	span LineSpan,
	re *regexp.Regexp,
) ([]int, error) {
	if re == nil {
		return nil, errFindLineSpanSubmatchNilRegexp
	}

	rr, err := newLineSpanRuneReader(ctx, src, span)
	if err != nil {
		return nil, err
	}

	idx := re.FindReaderSubmatchIndex(rr)

	if rr.rdSticky != nil {
		return nil, fmt.Errorf("find line span submatch read: %w", rr.rdSticky)
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("find line span submatch: %w", err)
	}

	return idx, nil
}

func newLineSpanRuneReader(ctx context.Context, src io.ReadSeeker, span LineSpan) (*ctxBufRuneReader, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	cl, err := seekLineSpanContent(ctx, src, span)
	if err != nil {
		return nil, err
	}

	return &ctxBufRuneReader{
		ctx: ctx,
		br:  bufio.NewReader(io.LimitReader(src, cl)),
	}, nil
}

func seekLineSpanContent(ctx context.Context, src io.ReadSeeker, span LineSpan) (int64, error) {
	if span.ContentEnd < span.ContentStart {
		return 0, errLineSpanReaderInvertedOffsets
	}

	delta := span.ContentEnd - span.ContentStart
	if delta > uint64(math.MaxInt64) {
		return 0, ErrLineSpanContentTooLarge
	}

	if span.ContentStart > uint64(math.MaxInt64) {
		return 0, ErrLineSpanOffsetTooLarge
	}

	cl := int64(delta)
	if _, err := src.Seek(int64(span.ContentStart), io.SeekStart); err != nil {
		return 0, fmt.Errorf("new line span reader seek: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("new line span reader: %w", err)
	}

	return cl, nil
}

type ctxBufRuneReader struct {
	ctx      context.Context
	br       *bufio.Reader
	rdSticky error
}

func (c *ctxBufRuneReader) ReadRune() (rn rune, size int, err error) {
	if err := c.ctx.Err(); err != nil {
		return 0, 0, err
	}

	ch, sz, er := c.br.ReadRune()
	switch {
	case er == nil, errors.Is(er, io.EOF):
	default:
		c.rdSticky = er
	}

	return ch, sz, er
}
