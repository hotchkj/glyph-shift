package testutil

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

func TestExtractLineTerminatorZeroBytesError(t *testing.T) {
	t.Parallel()

	var term ExtractLineTerminator
	_, err := term.bytes()
	if err == nil {
		t.Fatal("terminator 0: want error")
	}
	if !errors.Is(err, errExtractUnknownLineTerminator) {
		t.Fatalf("want errExtractUnknownLineTerminator, got %v", err)
	}
}

func TestVerifyGoldTerminatorsMatchExtract_NoLinesNoError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	if err := verifyGoldTerminatorsMatchExtract(ctx, []byte("x\n"), []byte{'\n'}, 0); err != nil {
		t.Fatalf("early return: %v", err)
	}
}

func TestVerifyGoldTerminatorsMatchExtract_TermMismatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	termCRLF := []byte{'\r', '\n'}

	// Gold uses LF-only line terminators while the fixture claims CRLF.
	err := verifyGoldTerminatorsMatchExtract(ctx, []byte("a\nb\n"), termCRLF, 2)
	if err == nil {
		t.Fatal("want terminator mismatch error")
	}
	if !errors.Is(err, errExtractGoldenTerminatorMismatch) {
		t.Fatalf("want errExtractGoldenTerminatorMismatch, got %v", err)
	}
}

func TestSourceCountersResetNil(t *testing.T) {
	t.Parallel()

	var sc *sourceCounters
	sc.reset()
}

func TestCountingReaderNilCountersReadSeekClose(t *testing.T) {
	t.Parallel()

	payload := []byte("abc")
	countingReader := NewCountingReader(payload, nil)

	buf := make([]byte, 2)
	if n, readErr := countingReader.Read(buf); n != 2 || readErr != nil {
		t.Fatalf("Read = (%d, %v)", n, readErr)
	}

	if pos, seekErr := countingReader.Seek(-1, io.SeekEnd); pos != 2 || seekErr != nil {
		t.Fatalf("Seek = (%d, %v)", pos, seekErr)
	}

	if closeErr := countingReader.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}
}

func TestCountingWriterNilWrite(t *testing.T) {
	t.Parallel()

	var nilCountingWriter *CountingWriter

	_, err := nilCountingWriter.Write([]byte{'x'})
	if err == nil {
		t.Fatal("nil CountingWriter Write: want error")
	}
	if !errors.Is(err, errNilCountingWriter) {
		t.Fatalf("want errNilCountingWriter, got %v", err)
	}
}

func TestCountingWriterNilAccessors(t *testing.T) {
	t.Parallel()

	var nilWriter *CountingWriter

	if nilWriter.Bytes() != nil {
		t.Fatal("nil Bytes: want nil")
	}
	if nilWriter.BytesWritten() != 0 {
		t.Fatalf("BytesWritten = %d want 0", nilWriter.BytesWritten())
	}
	if nilWriter.WriteOps() != 0 {
		t.Fatalf("WriteOps = %d want 0", nilWriter.WriteOps())
	}
	nilWriter.Reset()
}

func TestCountingWriterResetClearsBufferAndStats(t *testing.T) {
	t.Parallel()

	payloadWriter := &CountingWriter{}
	if _, err := payloadWriter.Write([]byte("pq")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	payloadWriter.Reset()

	if len(payloadWriter.Bytes()) != 0 {
		t.Fatalf("Bytes after Reset len=%d want 0", len(payloadWriter.Bytes()))
	}
	if payloadWriter.BytesWritten() != 0 || payloadWriter.WriteOps() != 0 {
		t.Fatalf(
			"instrumentation after Reset: bytes=%d ops=%d",
			payloadWriter.BytesWritten(),
			payloadWriter.WriteOps(),
		)
	}
}

func TestSyntheticAbsentPathResolverLstatAbsent(t *testing.T) {
	t.Parallel()

	res := NewSyntheticAbsentPathResolver()
	_, err := res.Lstat("/any")
	if err == nil {
		t.Fatal("Lstat: want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Lstat: want fs.ErrNotExist, got %v", err)
	}
	p, err := res.EvalSymlinks("/p/q")
	if err != nil || p != "/p/q" {
		t.Fatalf("EvalSymlinks = (%q, %v) want (/p/q, nil)", p, err)
	}
}

func TestCountingSourceOpener_OpenPathAllowsOnlyConfiguredAllowedPath(t *testing.T) {
	t.Parallel()

	restricted := &CountingSourceOpener{Immutable: []byte("data\n"), AllowedPath: "/want/exact"}

	_, err := restricted.Open("/wrong/path")
	if err == nil {
		t.Fatal("Open wrong path: want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open: want fs.ErrNotExist, got %v", err)
	}

	reader, err := restricted.Open("/want/exact")
	if err != nil {
		t.Fatalf("Open allowed path: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := restricted.Opens(); got != 1 {
		t.Fatalf("Opens=%d want 1 after single successful Open", got)
	}
}

func TestCountingSourceOpener_ResetCountersClearsOpensAndReaders(t *testing.T) {
	t.Parallel()

	opened := &CountingSourceOpener{Immutable: []byte("zz")}

	sourceReader, err := opened.Open("/any")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	buf := make([]byte, 1)
	if _, err := sourceReader.Read(buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if err := sourceReader.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if opened.Opens() != 1 {
		t.Fatalf("Opens before reset = %d", opened.Opens())
	}
	if opened.AggregateSourceBytesRead() == 0 {
		t.Fatal("expected non-zero bytes read before reset")
	}

	opened.ResetCounters()

	if opened.Opens() != 0 || opened.AggregateSourceBytesRead() != 0 {
		t.Fatalf(
			"after reset: opens=%d bytesRead=%d",
			opened.Opens(),
			opened.AggregateSourceBytesRead(),
		)
	}
}

func TestMeasureFileopsExtractAppendMode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	src := []byte("one\ntwo\nthree\n")
	lines := fileops.LineRange{Start: 1, End: 2}

	var wantBuf bytes.Buffer

	_, exErr := fileops.Extract(ctx, fileops.ExtractOptions{
		Source: bytes.NewReader(src),
		Lines:  lines,
		Append: true,
	}, &wantBuf)
	if exErr != nil {
		t.Fatalf("fileops.Extract reference: %v", exErr)
	}

	meas, _, err := MeasureFileopsExtract(ctx, src, lines, true)
	if err != nil {
		t.Fatalf("MeasureFileopsExtract append: %v", err)
	}

	if meas.LinesExtracted != 2 {
		t.Fatalf("LinesExtracted = %d want 2", meas.LinesExtracted)
	}
	if meas.SourceReadCalls < 1 || meas.SourceSeekCalls < 1 {
		t.Fatalf("instrumentation readCalls=%d seekCalls=%d want non-trivial", meas.SourceReadCalls, meas.SourceSeekCalls)
	}

	if !bytes.Equal(meas.OutputBytes, wantBuf.Bytes()) {
		t.Fatalf("OutputBytes mismatch fileops reference")
	}
}
