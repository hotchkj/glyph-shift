// User vision: literal line-span matches should be fast without changing regexp semantics.
package fileops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"regexp/syntax"
	"unicode/utf8"
)

const literalScanChunkBytes = 32 << 10

func literalLineSpanMatch(re *regexp.Regexp) (literal []byte, exact, ok bool) {
	if lit, anchored := anchoredLiteralLinePattern(re.String()); anchored {
		return lit, true, true
	}

	lit, complete := re.LiteralPrefix()
	if !complete || lit == "" || !asciiLiteral(lit) {
		return nil, false, false
	}

	return []byte(lit), false, true
}

func anchoredLiteralLinePattern(pattern string) ([]byte, bool) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, false
	}
	re = re.Simplify()

	mid, ok := anchoredLiteralMiddle(re)
	if !ok || mid.Op != syntax.OpLiteral || mid.Flags&syntax.FoldCase != 0 {
		return nil, false
	}
	literal := string(mid.Rune)
	if literal == "" || !asciiLiteral(literal) {
		return nil, false
	}

	return []byte(literal), true
}

func anchoredLiteralMiddle(re *syntax.Regexp) (*syntax.Regexp, bool) {
	if re.Op != syntax.OpConcat || len(re.Sub) != 3 {
		return nil, false
	}
	if re.Sub[0].Op != syntax.OpBeginText || re.Sub[2].Op != syntax.OpEndText {
		return nil, false
	}

	return re.Sub[1], true
}

func asciiLiteral(literal string) bool {
	for i := range literal {
		if literal[i] >= utf8.RuneSelf {
			return false
		}
	}

	return true
}

func matchLiteralLineSpan(
	ctx context.Context,
	src io.ReadSeeker,
	span LineSpan,
	literal []byte,
	exact bool,
) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	cl, err := seekLineSpanContent(ctx, src, span)
	if err != nil {
		return false, err
	}
	if exact {
		return matchExactLiteralBytes(ctx, src, cl, literal)
	}

	matched, readErr := containsLiteralInReader(ctx, io.LimitReader(src, cl), literal)
	if readErr != nil {
		return false, literalMatchReadError(readErr)
	}
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf(errFmtMatchLineSpan, err)
	}

	return matched, nil
}

func literalMatchReadError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf(errFmtMatchLineSpan, err)
	}

	return fmt.Errorf(errFmtMatchLineSpanRead, err)
}

func matchExactLiteralBytes(ctx context.Context, src io.Reader, contentLen int64, literal []byte) (bool, error) {
	if contentLen != int64(len(literal)) {
		return false, nil
	}

	buf := make([]byte, len(literal))
	if _, err := io.ReadFull(src, buf); err != nil {
		return false, fmt.Errorf(errFmtMatchLineSpanRead, err)
	}
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf(errFmtMatchLineSpan, err)
	}

	return bytes.Equal(buf, literal), nil
}

func containsLiteralInReader(ctx context.Context, reader io.Reader, literal []byte) (bool, error) {
	if len(literal) == 0 {
		return true, nil
	}

	buf := make([]byte, literalScanChunkBytes)
	tail := make([]byte, 0, len(literal)-1)

	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		n, err := reader.Read(buf)
		var matched bool
		tail, matched = appendLiteralWindow(tail, buf[:n], literal)
		if matched {
			return true, nil
		}

		done, readErr := literalReadDone(err)
		if done || readErr != nil {
			return false, readErr
		}
	}
}

func appendLiteralWindow(tail, chunk, literal []byte) ([]byte, bool) {
	if len(chunk) == 0 {
		return tail, false
	}

	tail = append(tail, chunk...)
	if bytes.Contains(tail, literal) {
		return tail, true
	}

	return keepLiteralOverlap(tail, tail, len(literal)-1), false
}

func literalReadDone(err error) (done bool, readErr error) {
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	return false, nil
}

func keepLiteralOverlap(dst, src []byte, overlap int) []byte {
	if overlap <= 0 {
		return dst[:0]
	}
	if len(src) > overlap {
		src = src[len(src)-overlap:]
	}

	dst = dst[:0]
	dst = append(dst, src...)

	return dst
}
