package fileops

import (
	"errors"
	"io"
	"strings"
	"testing"
)

var errLinespanInternalRead = errors.New("linespan internal read failed")

type failingLinespanReadSeeker struct{}

func (failingLinespanReadSeeker) Read([]byte) (int, error) {
	return 0, errLinespanInternalRead
}

func (failingLinespanReadSeeker) Seek(int64, int) (int64, error) {
	return 0, nil
}

func TestScannerStateGrowBufInitializesAndPreservesFill(t *testing.T) {
	t.Parallel()

	state := &scannerState{buf: []byte("abc"), fill: 2}
	state.growBuf()

	if len(state.buf) != defaultLinespanChunkSize {
		t.Fatalf("buffer length = %d, want %d", len(state.buf), defaultLinespanChunkSize)
	}
	if got := string(state.buf[:state.fill]); got != "ab" {
		t.Fatalf("buffer prefix = %q, want ab", got)
	}
}

func TestScannerStateEnsureSecondByteForCRLFCoversBufferedEOFAndRefillError(t *testing.T) {
	t.Parallel()

	buffered := &scannerState{
		src:  strings.NewReader(""),
		buf:  []byte{'\r', '\n'},
		fill: 2,
	}
	hasSecond, err := buffered.ensureSecondByteForCRLF(1)
	if err != nil {
		t.Fatalf("buffered ensure second byte error = %v", err)
	}
	if !hasSecond {
		t.Fatal("buffered second byte not detected")
	}

	atEOF := &scannerState{
		src:       strings.NewReader(""),
		buf:       []byte{'\r'},
		fill:      1,
		streamEOF: true,
	}
	hasSecond, err = atEOF.ensureSecondByteForCRLF(1)
	if err != nil {
		t.Fatalf("EOF ensure second byte error = %v", err)
	}
	if hasSecond {
		t.Fatal("EOF second byte should not be present")
	}

	failing := &scannerState{
		src:  failingLinespanReadSeeker{},
		buf:  []byte{'\r'},
		fill: 1,
	}
	_, err = failing.ensureSecondByteForCRLF(1)
	if !errors.Is(err, errLinespanInternalRead) && !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("refill error = %v, want read sentinel", err)
	}

	refilled := &scannerState{
		src:    strings.NewReader("\n"),
		buf:    []byte{'\r'},
		fill:   1,
		cursor: 1,
	}
	hasSecond, err = refilled.awaitSecondByteForCRLF(1)
	if err != nil {
		t.Fatalf("refilled ensure second byte error = %v", err)
	}
	if !hasSecond {
		t.Fatal("refilled second byte not detected")
	}
}
