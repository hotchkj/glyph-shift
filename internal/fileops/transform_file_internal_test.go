package fileops

import (
	"errors"
	"io"
	"testing"
)

var errTransformFileInternalRead = errors.New("transform file internal read failed")

type errorAfterBytesReader struct {
	data []byte
	done bool
}

func (r *errorAfterBytesReader) Read(buf []byte) (int, error) {
	if r.done {
		return 0, errTransformFileInternalRead
	}
	r.done = true

	return copy(buf, r.data), nil
}

func TestReadAtMostFromReaderStopsAtBufferLength(t *testing.T) {
	t.Parallel()

	buf := make([]byte, 3)
	n, err := readAtMostFromReader(&errorAfterBytesReader{data: []byte("abcdef")}, buf)
	if err != nil {
		t.Fatalf("readAtMostFromReader error = %v want nil", err)
	}
	if n != len(buf) {
		t.Fatalf("readAtMostFromReader n = %d want %d", n, len(buf))
	}
}

func TestReadAtMostFromReaderPropagatesReadError(t *testing.T) {
	t.Parallel()

	buf := make([]byte, 4)
	n, err := readAtMostFromReader(&errorAfterBytesReader{data: []byte("xy")}, buf)
	if !errors.Is(err, errTransformFileInternalRead) {
		t.Fatalf("readAtMostFromReader error = %v want %v", err, errTransformFileInternalRead)
	}
	if n != 2 {
		t.Fatalf("readAtMostFromReader n = %d want 2", n)
	}
}

func TestReadAtMostFromReaderReturnsEOFForShortInput(t *testing.T) {
	t.Parallel()

	buf := make([]byte, 4)
	n, err := readAtMostFromReader(io.LimitReader(&errorAfterBytesReader{data: []byte("xy")}, 2), buf)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("readAtMostFromReader error = %v want EOF", err)
	}
	if n != 2 {
		t.Fatalf("readAtMostFromReader n = %d want 2", n)
	}
}

func TestTransformWouldChangeFromStats(t *testing.T) {
	t.Parallel()

	target := TargetLF
	if transformWouldChangeFromStats(TransformOptions{LineEndings: &target}, nil) {
		t.Fatal("nil result reported change")
	}
	if !transformWouldChangeFromStats(
		TransformOptions{LineEndings: &target},
		&TransformFileResult{EndingsChanged: 1},
	) {
		t.Fatal("line ending change not detected")
	}
	if !transformWouldChangeFromStats(
		TransformOptions{TrimTrailing: true},
		&TransformFileResult{TrailingTrimmed: 1},
	) {
		t.Fatal("trailing trim change not detected")
	}
	if !transformWouldChangeFromStats(
		TransformOptions{FinalNewline: true},
		&TransformFileResult{FinalNewlineAdded: true},
	) {
		t.Fatal("final newline change not detected")
	}
	if transformWouldChangeFromStats(TransformOptions{}, &TransformFileResult{}) {
		t.Fatal("empty transform stats reported change")
	}
}

func TestTransformNoOptsResultCopiesBody(t *testing.T) {
	t.Parallel()

	res, err := transformNoOptsResult("doc.txt", io.LimitReader(&errorAfterBytesReader{data: []byte("abc")}, 3))
	if err != nil {
		t.Fatalf("transformNoOptsResult: %v", err)
	}
	if !res.Skipped || res.SkipReason != transformSkipReasonNoTransform || res.Path != "doc.txt" {
		t.Fatalf("transformNoOptsResult = %+v", res)
	}
}

func TestTransformNoOptsResultPropagatesCopyError(t *testing.T) {
	t.Parallel()

	_, err := transformNoOptsResult("doc.txt", &errorAfterBytesReader{data: []byte("abc")})
	if !errors.Is(err, errTransformFileInternalRead) {
		t.Fatalf("transformNoOptsResult error = %v want %v", err, errTransformFileInternalRead)
	}
}
