package fileops

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
)

// ErrSpanFingerprintMismatch indicates replayed source bytes for an output span
// disagree with bounded-scan fingerprint metadata for that span.
var ErrSpanFingerprintMismatch = errors.New("fileops: span fingerprint mismatch")

var errSpanCopyShortWrite = errors.New("write output: short write")

// EmptySpanSHA256 is SHA256("") — fingerprint for zero-byte output spans.
func EmptySpanSHA256() [32]byte {
	return sha256.Sum256(nil)
}

// HashSerializedLineInto appends ln's serialized on-disk bytes to h (content then terminator).
func HashSerializedLineInto(h hash.Hash, ln Line) {
	_, _ = h.Write(ln.Content)
	_, _ = h.Write(ln.Terminator)
}

func sha256SumToArray(h hash.Hash) [32]byte {
	var out [32]byte

	sum := h.Sum(nil)
	copy(out[:], sum)

	return out
}

// seekSourceToAbsolute positions rs at logical byte offset abs measured from stream start without using
// Seek(abs, SeekStart). This avoids an extra Seek(0, SeekStart) when abs==0—a nicety when consumers count
// start-anchor seeks—as long as Seek(0, SeekEnd) resolves the serialized length.
func seekSourceToAbsolute(src io.ReadSeeker, abs int64) error {
	endOff, err := src.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seek source end: %w", err)
	}

	if abs < 0 || abs > endOff {
		return fmt.Errorf("seek beyond source: abs=%d size=%d: %w", abs, endOff, io.ErrUnexpectedEOF)
	}

	if _, err := src.Seek(abs-endOff, io.SeekEnd); err != nil {
		return fmt.Errorf("seek source to absolute: %w", err)
	}

	return nil
}

// SHA256SerializedByteSpan computes the SHA-256 digest of raw bytes covering the half-open serialized
// interval [serializedStart, serializedEndExclusive). Cursor may be arbitrary; positioning uses SeekEnd
// relative arithmetic so callers avoid redundant SeekStart(0) preamble when abs==0.
func SHA256SerializedByteSpan(
	ctx context.Context,
	src io.ReadSeeker,
	serializedStart, serializedEndExclusive uint64,
) ([32]byte, error) {
	var zero [32]byte

	seekOff, byteLen, err := SerializedSpanSeekAndLength(serializedStart, serializedEndExclusive)
	if err != nil {
		return zero, err
	}

	if byteLen == 0 {
		return EmptySpanSHA256(), nil
	}

	if err := seekSourceToAbsolute(src, seekOff); err != nil {
		return zero, fmt.Errorf("seek source for span digest: %w", err)
	}

	hasher := sha256.New()
	if err := copySourceSpanChunks(ctx, hasher, src, byteLen); err != nil {
		return zero, err
	}

	return sha256SumToArray(hasher), nil
}

// CopySpanToWriterWithSHA256Verify seeks src to seekOff, copies exactly byteLen bytes to dst while
// streaming a SHA256 digest, and returns ErrSpanFingerprintMismatch unless the digest matches want.
//
// For byteLen == 0, only the fingerprint equality against EmptySpanSHA256 semantics applies via want.
func CopySpanToWriterWithSHA256Verify(
	ctx context.Context,
	dst io.Writer,
	src io.ReadSeekCloser,
	seekOff, byteLen int64,
	want [32]byte,
) error {
	if byteLen == 0 {
		if want != EmptySpanSHA256() {
			return fmt.Errorf("%w", ErrSpanFingerprintMismatch)
		}

		return nil
	}

	if err := seekSourceToAbsolute(src, seekOff); err != nil {
		return fmt.Errorf("seek source for span copy: %w", err)
	}

	digestHasher := sha256.New()
	mw := io.MultiWriter(dst, digestHasher)

	if err := copySourceSpanChunks(ctx, mw, src, byteLen); err != nil {
		return err
	}

	got := sha256SumToArray(digestHasher)
	if got != want {
		return fmt.Errorf("%w", ErrSpanFingerprintMismatch)
	}

	return nil
}

// copySourceSpanChunks streams exactly byteLen bytes from src into dst honoring ctx cancellation.
func copySourceSpanChunks(ctx context.Context, dst io.Writer, src io.Reader, byteLen int64) error {
	if byteLen <= 0 {
		return nil
	}

	var buf [32 * 1024]byte

	remaining := byteLen

	for remaining > 0 {
		nr, partialEOF, err := copySourceSpanChunk(ctx, dst, src, buf[:], remaining)
		if err != nil {
			return err
		}

		remaining -= int64(nr)
		if partialEOF && remaining > 0 {
			return fmt.Errorf("copy span: %w", io.ErrUnexpectedEOF)
		}
	}

	return nil
}

func copySourceSpanChunk(
	ctx context.Context,
	dst io.Writer,
	src io.Reader,
	buf []byte,
	remaining int64,
) (bytesCopied int, partialEOF bool, err error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}

	nr, partialEOF, rerr := readChunkForSpanCopy(src, buf[:spanCopyChunkLen(len(buf), remaining)])
	if rerr != nil {
		return 0, false, rerr
	}

	if err := writeSpanChunk(dst, buf[:nr]); err != nil {
		return 0, false, err
	}

	return nr, partialEOF, nil
}

func spanCopyChunkLen(bufLen int, remaining int64) int {
	if int64(bufLen) > remaining {
		return int(remaining)
	}

	return bufLen
}

func writeSpanChunk(dst io.Writer, chunk []byte) error {
	nw, werr := dst.Write(chunk)
	if werr != nil {
		return fmt.Errorf("write output: %w", werr)
	}

	if nw != len(chunk) {
		return errSpanCopyShortWrite
	}

	return nil
}

func readChunkForSpanCopy(src io.Reader, chunk []byte) (nr int, partialSourceEOF bool, err error) {
	nr, rerr := io.ReadFull(src, chunk)
	switch {
	case rerr == nil:
		return nr, false, nil
	case errors.Is(rerr, io.ErrUnexpectedEOF):
		if nr <= 0 {
			return 0, false, fmt.Errorf("read source span: %w", io.ErrUnexpectedEOF)
		}

		return nr, true, nil
	default:
		return 0, false, fmt.Errorf("read source span: %w", rerr)
	}
}
