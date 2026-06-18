package fileops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"testing"
)

var errSpanFPTestSeekEnd = errors.New("test seek end failure")

type seekEndFailReader struct {
	data []byte
	pos  int64
}

func (s *seekEndFailReader) Read(p []byte) (int, error) {
	if s.pos >= int64(len(s.data)) {
		return 0, io.EOF
	}
	n := copy(p, s.data[s.pos:])
	s.pos += int64(n)

	return n, nil
}

func (s *seekEndFailReader) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekEnd {
		return 0, errSpanFPTestSeekEnd
	}

	switch whence {
	case io.SeekStart:
		s.pos = offset
	case io.SeekCurrent:
		s.pos += offset
	default:
		return 0, errInvalidWhenceForMeteringTest
	}

	return s.pos, nil
}

type readSeekNopClose struct{ *bytes.Reader }

func (readSeekNopClose) Close() error {
	return nil
}

func TestSeekSourceToAbsolute_SeekEndFails(t *testing.T) {
	t.Parallel()

	src := &seekEndFailReader{data: []byte("abc")}
	if err := seekSourceToAbsolute(src, 0); !errors.Is(err, errSpanFPTestSeekEnd) {
		t.Fatalf("want seek end error, got %v", err)
	}
}

func TestSeekSourceToAbsolute_AbsBeyondSize(t *testing.T) {
	t.Parallel()

	src := bytes.NewReader([]byte("hi"))
	if err := seekSourceToAbsolute(src, 99); err == nil {
		t.Fatal("expected error for offset beyond source size")
	}
}

func TestSeekSourceToAbsolute_AbsNegative(t *testing.T) {
	t.Parallel()

	src := bytes.NewReader([]byte("x"))
	if err := seekSourceToAbsolute(src, -1); err == nil {
		t.Fatal("expected error for negative absolute offset")
	}
}

func TestEmptySpanSHA256_MatchesSUM256Nil(t *testing.T) {
	t.Parallel()

	want := sha256.Sum256(nil)
	if EmptySpanSHA256() != want {
		t.Fatalf("EmptySpanSHA256 mismatch")
	}
}

func TestHashSerializedLineInto_AggregatesBytes(t *testing.T) {
	t.Parallel()

	ln := Line{Content: []byte("ab"), Terminator: []byte{'\n'}}
	h := sha256.New()
	HashSerializedLineInto(h, ln)

	want := sha256.Sum256([]byte("ab\n"))
	var got [32]byte
	copy(got[:], h.Sum(nil))
	if got != want {
		t.Fatalf("HashSerializedLineInto digest %x want %x", got, want)
	}
}

func TestSHA256SerializedByteSpan_EmptyInterval(t *testing.T) {
	t.Parallel()

	src := bytes.NewReader([]byte("ignored"))
	got, err := SHA256SerializedByteSpan(context.Background(), src, 5, 5)
	if err != nil {
		t.Fatalf("SHA256SerializedByteSpan: %v", err)
	}
	if got != EmptySpanSHA256() {
		t.Fatalf("empty span digest mismatch")
	}
}

func TestCopySpanToWriterWithSHA256Verify_ZeroLenWrongFingerprint(t *testing.T) {
	t.Parallel()

	dst := bytes.NewBuffer(nil)
	src := readSeekNopClose{Reader: bytes.NewReader([]byte("zzz"))}

	var bogus [32]byte
	bogus[0] = 0x37

	err := CopySpanToWriterWithSHA256Verify(context.Background(), dst, src, 0, 0, bogus)
	if !errors.Is(err, ErrSpanFingerprintMismatch) {
		t.Fatalf("want ErrSpanFingerprintMismatch, got %v", err)
	}
	if dst.Len() != 0 {
		t.Fatalf("expected no writes, got %d bytes", dst.Len())
	}
}

func TestCopySpanToWriterWithSHA256Verify_WritesAndVerifies(t *testing.T) {
	t.Parallel()

	payload := []byte("exact-bytes")
	want := sha256.Sum256(payload)

	dst := bytes.NewBuffer(nil)
	src := readSeekNopClose{Reader: bytes.NewReader(payload)}

	if err := CopySpanToWriterWithSHA256Verify(
		context.Background(),
		dst,
		src,
		0,
		int64(len(payload)),
		want,
	); err != nil {
		t.Fatalf("CopySpanToWriterWithSHA256Verify: %v", err)
	}
	if !bytes.Equal(dst.Bytes(), payload) {
		t.Fatalf("output mismatch")
	}
}

func TestCopySpanToWriterWithSHA256Verify_UnexpectedEOF(t *testing.T) {
	t.Parallel()

	dst := bytes.NewBuffer(nil)
	// Declare length 50 but underlying reader yields EOF before filling the chunk machinery's full read.
	r := bytes.NewReader([]byte("tiny"))
	src := readSeekNopClose{Reader: r}
	want := sha256.Sum256([]byte("nope"))

	err := CopySpanToWriterWithSHA256Verify(context.Background(), dst, src, 0, 50, want)
	if err == nil {
		t.Fatal("expected error for short source")
	}
}

func TestCopySourceSpanChunks_ShortWriter(t *testing.T) {
	t.Parallel()

	src := bytes.NewReader([]byte("hello"))
	sw := shortWriter{}

	if err := copySourceSpanChunks(context.Background(), sw, src, 5); !errors.Is(err, errSpanCopyShortWrite) {
		t.Fatalf("want errSpanCopyShortWrite, got %v", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return len(p) - 1, nil
}

func TestCopySourceSpanChunks_CanceledDuringLoop(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	body := bytes.Repeat([]byte("q"), 10000)
	src := bytes.NewReader(body)

	if err := copySourceSpanChunks(ctx, io.Discard, src, int64(len(body))); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}
