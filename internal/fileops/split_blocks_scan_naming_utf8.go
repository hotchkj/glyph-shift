package fileops

import (
	"context"
	"fmt"
	"io"
	"unicode/utf8"
)

// NamingMaterializationMaxBytes is the largest UTF-8 byte prefix read from any single logical line's
// *content* span when seekable split/blocks scans assemble filename slug inputs (GenerateFilename).
// Slug output is already capped (see slugMaxRunes). Naming reads only this prefix via chunked streaming
// into a string builder-never a single allocation sized to the full logical line. SpanFingerprintSHA256
// and line scanning still stream or hash full byte ranges elsewhere without this cap.
const NamingMaterializationMaxBytes uint64 = 8192

// trimIncompleteUTF8Suffix strips zero or more trailing bytes when the capped read ends inside a UTF-8
// code point, so callers never retain a lone leading byte without its continuations or bare continuation
// bytes at the suffix. Incomplete encodings detected by utf8.DecodeLastRune are removed; arbitrary
// invalid UTF-8 earlier in the buffer is unchanged.
func trimIncompleteUTF8Suffix(prefixUTF8 []byte) []byte {
	for len(prefixUTF8) > 0 {
		r, size := utf8.DecodeLastRune(prefixUTF8)
		if size == 0 {
			break
		}

		if r == utf8.RuneError && size == 1 {
			prefixUTF8 = prefixUTF8[:len(prefixUTF8)-1]

			continue
		}

		return prefixUTF8
	}

	return prefixUTF8
}

// streamSourceBytesToPrefixString reads exactly byteLen raw bytes from r (cursor already set) through a
// fixed stack buffer into a byte slice capped to byteLen, then trims trailing incomplete UTF-8 so the
// string is valid UTF-8 whenever the caller cap slices through a multi-byte character.
//
//nolint:varnamelen // mirrors io.Reader streaming shape used across fileops.
func streamSourceBytesToPrefixString(ctx context.Context, r io.Reader, byteLen int64) (string, error) {
	raw, err := streamSourcePrefixBytesAppend(ctx, r, byteLen)
	if err != nil {
		return "", err
	}

	return string(trimIncompleteUTF8Suffix(raw)), nil
}

func streamSourcePrefixBytesAppend(ctx context.Context, sourceReader io.Reader, byteLen int64) ([]byte, error) {
	if byteLen == 0 {
		return nil, nil
	}

	var dst []byte

	if n := int(byteLen); int64(n) == byteLen && n > 0 {
		dst = make([]byte, 0, n)
	}

	var buf [4096]byte

	remaining := byteLen

	for remaining > 0 {
		nr, partialEOF, err := readPrefixStringChunk(ctx, sourceReader, buf[:], remaining)
		if err != nil {
			return nil, err
		}

		dst = append(dst, buf[:nr]...)
		remaining -= int64(nr)
		if err := ensurePrefixChunkComplete(partialEOF, remaining); err != nil {
			return nil, err
		}
	}

	return dst, nil
}

func readPrefixStringChunk(
	ctx context.Context,
	sourceReader io.Reader,
	buf []byte,
	remaining int64,
) (bytesRead int, partialEOF bool, err error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}

	return readPrefixChunk(sourceReader, buf[:prefixChunkCap(len(buf), remaining)])
}

func ensurePrefixChunkComplete(partialEOF bool, remaining int64) error {
	if partialEOF && remaining > 0 {
		return fmt.Errorf("read serialized span prefix: %w", io.ErrUnexpectedEOF)
	}

	return nil
}

func prefixChunkCap(bufLen int, remaining int64) int64 {
	chunkCap := int64(bufLen)
	if chunkCap > remaining {
		return remaining
	}

	return chunkCap
}

func readPrefixChunk(sourceReader io.Reader, chunk []byte) (readCount int, partialEOF bool, err error) {
	return readChunkForSpanCopy(sourceReader, chunk)
}

// readSerializedSpanContentPrefixUTF8 reads up to maxBytes raw source bytes from the half-open content
// interval [contentStart, contentEnd) by streaming through a fixed stack buffer. Longer lines
// contribute only the prefix bounded by maxBytes reads; trailing bytes that slice a UTF-8 code point at
// the cap are discarded so the returned string is never invalid UTF-8 solely due to naming truncation.
func readSerializedSpanContentPrefixUTF8(
	ctx context.Context,
	src io.ReadSeeker,
	contentStart, contentEnd uint64,
	maxBytes uint64,
) (string, error) {
	if contentEnd <= contentStart {
		return "", nil
	}

	spanLen := contentEnd - contentStart
	prefixLen := spanLen

	if prefixLen > maxBytes {
		prefixLen = maxBytes
	}

	endExclusive := contentStart + prefixLen

	seekOff, byteLen, err := SerializedSpanSeekAndLength(contentStart, endExclusive)
	if err != nil {
		return "", err
	}

	if byteLen == 0 {
		return "", nil
	}

	if err := seekSourceToAbsolute(src, seekOff); err != nil {
		return "", err
	}

	return streamSourceBytesToPrefixString(ctx, src, byteLen)
}

func lineSpanNamingContentUTF8(ctx context.Context, src io.ReadSeeker, span LineSpan) (string, error) {
	return readSerializedSpanContentPrefixUTF8(ctx, src, span.ContentStart, span.ContentEnd, NamingMaterializationMaxBytes)
}
