package fileops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

const (
	errFmtReadLinesFrom = "read lines: %v"
	errFmtWriteLinesTo  = "WriteLinesTo: %v"
	errFmtIsBinary      = "IsBinary: %v"
)

var errLineIOTerminatorWrite = errors.New("terminator write")

type wantLine struct {
	content string
	term    string
}

func assertReadLinesResult(t *testing.T, input string, want []wantLine) {
	t.Helper()

	lines, err := ReadLinesFrom(strings.NewReader(input))
	if err != nil {
		t.Fatalf(errFmtReadLinesFrom, err)
	}

	if len(lines) != len(want) {
		t.Fatalf("line count: want %d got %d", len(want), len(lines))
	}

	for idx := range want {
		if string(lines[idx].Content) != want[idx].content {
			t.Errorf("line %d content: got %q want %q", idx, lines[idx].Content, want[idx].content)
		}

		wt := want[idx].term
		if wt == "" {
			if len(lines[idx].Terminator) != 0 {
				t.Errorf("line %d term: want empty got %q", idx, lines[idx].Terminator)
			}
		} else if string(lines[idx].Terminator) != wt {
			t.Errorf("line %d term: got %q want %q", idx, lines[idx].Terminator, wt)
		}
	}
}

func TestReadLinesFrom_LFOnly(t *testing.T) {
	t.Parallel()

	assertReadLinesResult(t, "alpha\nbravo\ncharlie\n", []wantLine{
		{"alpha", "\n"},
		{"bravo", "\n"},
		{"charlie", "\n"},
	})
}

func TestReadLinesFrom_CRLFOnly(t *testing.T) {
	t.Parallel()

	assertReadLinesResult(t, "alpha\r\nbravo\r\ncharlie\r\n", []wantLine{
		{"alpha", "\r\n"},
		{"bravo", "\r\n"},
		{"charlie", "\r\n"},
	})
}

func TestReadLinesFrom_MixedLFAndCRLF(t *testing.T) {
	t.Parallel()

	assertReadLinesResult(t, "alpha\nbravo\r\ncharlie\n", []wantLine{
		{"alpha", "\n"},
		{"bravo", "\r\n"},
		{"charlie", "\n"},
	})
}

func TestReadLinesFrom_BareCR(t *testing.T) {
	t.Parallel()

	assertReadLinesResult(t, "alpha\rbravo\rcharlie\r", []wantLine{
		{"alpha", "\r"},
		{"bravo", "\r"},
		{"charlie", "\r"},
	})
}

func TestReadLinesFrom_NoFinalNewline(t *testing.T) {
	t.Parallel()

	assertReadLinesResult(t, "alpha\nbravo", []wantLine{
		{"alpha", "\n"},
		{"bravo", ""},
	})
}

func TestReadLinesFrom_EndsWithNewline(t *testing.T) {
	t.Parallel()

	assertReadLinesResult(t, "alpha\nbravo\n", []wantLine{
		{"alpha", "\n"},
		{"bravo", "\n"},
	})
}

func TestReadLinesFrom_EmptyFile(t *testing.T) {
	t.Parallel()

	assertReadLinesResult(t, "", nil)
}

func TestReadLinesFrom_SingleLineNoTerminator(t *testing.T) {
	t.Parallel()

	assertReadLinesResult(t, "hello", []wantLine{{"hello", ""}})
}

func TestRoundTrip_LF(t *testing.T) {
	t.Parallel()

	assertRoundTrip(t, "a\nb\nc\n")
}

func TestRoundTrip_CRLF(t *testing.T) {
	t.Parallel()

	assertRoundTrip(t, "a\r\nb\r\nc\r\n")
}

func TestRoundTrip_Mixed(t *testing.T) {
	t.Parallel()

	assertRoundTrip(t, "a\nb\r\nc\n")
}

func TestRoundTrip_BareCR(t *testing.T) {
	t.Parallel()

	assertRoundTrip(t, "a\rb\rc\r")
}

func TestRoundTrip_NoFinalNewline(t *testing.T) {
	t.Parallel()

	assertRoundTrip(t, "a\nb")
}

type writeFullShortWriter struct {
	buf bytes.Buffer
}

func (w *writeFullShortWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return w.buf.Write(p[:1])
}

func TestWriteFullRetriesShortWrites(t *testing.T) {
	t.Parallel()

	var w writeFullShortWriter
	if err := WriteFull(&w, []byte("abc")); err != nil {
		t.Fatalf("WriteFull: %v", err)
	}
	if got := w.buf.String(); got != "abc" {
		t.Fatalf("written = %q, want %q", got, "abc")
	}
}

type invalidCountWriter struct{}

func (invalidCountWriter) Write([]byte) (int, error) {
	return -1, nil
}

func TestWriteFullRejectsNegativeWriteCount(t *testing.T) {
	t.Parallel()

	if err := WriteFull(invalidCountWriter{}, []byte("abc")); !errors.Is(err, errWriteFullInvalidCount) {
		t.Fatalf("WriteFull negative count = %v, want errWriteFullInvalidCount", err)
	}
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestWriteLinesToSurfacesTerminatorWriteError(t *testing.T) {
	t.Parallel()

	lines := []Line{{Content: nil, Terminator: []byte("\n")}}
	err := WriteLinesTo(failingWriter{err: errLineIOTerminatorWrite}, lines)
	if !errors.Is(err, errLineIOTerminatorWrite) {
		t.Fatalf("WriteLinesTo terminator error = %v, want %v", err, errLineIOTerminatorWrite)
	}
}

func TestRoundTrip_BOMPreserved(t *testing.T) {
	t.Parallel()

	bom := []byte{0xEF, 0xBB, 0xBF}
	input := make([]byte, 0, len(bom)+len("hello\n"))
	input = append(input, bom...)
	input = append(input, []byte("hello\n")...)

	lines, err := ReadLinesFrom(bytes.NewReader(input))
	if err != nil {
		t.Fatalf(errFmtReadLinesFrom, err)
	}

	if len(lines) != 1 {
		t.Fatalf("line count: want 1 got %d", len(lines))
	}

	wantContent := append(append([]byte(nil), bom...), []byte("hello")...)
	if !bytes.Equal(lines[0].Content, wantContent) {
		t.Fatalf("content mismatch: got %q want %q", lines[0].Content, wantContent)
	}

	if !bytes.Equal(lines[0].Terminator, []byte{'\n'}) {
		t.Fatalf("terminator: got %q want \\n", lines[0].Terminator)
	}

	var buf bytes.Buffer
	if err := WriteLinesTo(&buf, lines); err != nil {
		t.Fatalf(errFmtWriteLinesTo, err)
	}

	if !bytes.Equal(buf.Bytes(), input) {
		t.Fatalf("round-trip mismatch: got %q want %q", buf.Bytes(), input)
	}
}

func TestIsBinary_True(t *testing.T) {
	t.Parallel()

	input := []byte("hello\x00world")
	got, err := IsBinary(bytes.NewReader(input))
	if err != nil {
		t.Fatalf(errFmtIsBinary, err)
	}

	if !got {
		t.Fatal("want true for input containing null byte")
	}
}

func TestIsBinary_False(t *testing.T) {
	t.Parallel()

	got, err := IsBinary(strings.NewReader("hello world"))
	if err != nil {
		t.Fatalf(errFmtIsBinary, err)
	}

	if got {
		t.Fatal("want false for normal text")
	}
}

func TestIsBinary_EmptyReader(t *testing.T) {
	t.Parallel()

	got, err := IsBinary(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf(errFmtIsBinary, err)
	}

	if got {
		t.Fatal("want false for empty reader")
	}
}

func TestIsBinary_ZeroReadReader(t *testing.T) {
	t.Parallel()

	// Custom reader that returns (0, nil) 20 times then real data
	r := &zeroReadReader{zerosRemaining: 20, data: []byte("hello")}
	got, err := IsBinary(r)
	if err != nil {
		t.Fatalf("IsBinary: %v", err)
	}
	if got {
		t.Fatal("want false for text content after zero reads")
	}
}

func TestIsBinary_NullByteAfterWindow(t *testing.T) {
	t.Parallel()

	// 8000 bytes of text followed by a null byte -- IsBinary only checks the first 8000 (Git's FIRST_FEW_BYTES)
	data := make([]byte, 8001)
	for i := range data {
		data[i] = 'A'
	}
	data[8000] = 0
	got, err := IsBinary(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("IsBinary: %v", err)
	}
	if got {
		t.Fatal("null byte after 8000-byte window should not trigger binary detection")
	}
}

func TestIsBinary_ExactlyAtWindowBoundary(t *testing.T) {
	t.Parallel()

	// Null byte at the last position within the 8000-byte window (index 7999)
	data := make([]byte, 8000)
	for i := range data {
		data[i] = 'A'
	}
	data[7999] = 0
	got, err := IsBinary(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("IsBinary: %v", err)
	}
	if !got {
		t.Fatal("null byte at window boundary should trigger binary detection")
	}
}

type zeroReadReader struct {
	zerosRemaining int
	data           []byte
	pos            int
}

func (r *zeroReadReader) Read(buf []byte) (int, error) {
	if r.zerosRemaining > 0 {
		r.zerosRemaining--
		return 0, nil
	}
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(buf, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func TestReadLinesFrom_LargeLineOver64KB(t *testing.T) {
	t.Parallel()

	const largeSize = 100_000
	large := bytes.Repeat([]byte{'x'}, largeSize)
	lines, err := ReadLinesFrom(bytes.NewReader(large))
	if err != nil {
		t.Fatalf(errFmtReadLinesFrom, err)
	}

	if len(lines) != 1 {
		t.Fatalf("line count: want 1 got %d", len(lines))
	}

	if len(lines[0].Content) != largeSize {
		t.Fatalf("content length: want %d got %d", largeSize, len(lines[0].Content))
	}

	if !bytes.Equal(lines[0].Content, large) {
		t.Fatal("content bytes mismatch")
	}

	if len(lines[0].Terminator) != 0 {
		t.Fatalf("terminator: want empty, got %q", lines[0].Terminator)
	}
}

func TestWriteLinesTo_VerbatimWrites(t *testing.T) {
	t.Parallel()

	lines := []Line{
		{Content: []byte("a"), Terminator: []byte{'\n'}},
		{Content: []byte("b"), Terminator: []byte{'\r', '\n'}},
		{Content: []byte("c"), Terminator: nil},
	}

	var buf bytes.Buffer
	if err := WriteLinesTo(&buf, lines); err != nil {
		t.Fatalf(errFmtWriteLinesTo, err)
	}

	want := "a\nb\r\nc"
	if buf.String() != want {
		t.Fatalf("got %q want %q", buf.String(), want)
	}
}

func TestReadLinesFromContext_CancelledMidStream(t *testing.T) {
	t.Parallel()

	// Build input with well over 1000 lines so context check fires
	var sb strings.Builder
	for i := 0; i < 2500; i++ {
		fmt.Fprintf(&sb, "line-%d\n", i)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ReadLinesFromContext(ctx, strings.NewReader(sb.String()))
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled error chain, got %v", err)
	}
}

func TestReadLinesFromContext_NotCancelled(t *testing.T) {
	t.Parallel()

	lines, err := ReadLinesFromContext(context.Background(), strings.NewReader("a\nb\n"))
	if err != nil {
		t.Fatalf("ReadLinesFromContext: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("line count: want 2 got %d", len(lines))
	}
}

func assertRoundTrip(t *testing.T, input string) {
	t.Helper()

	original := []byte(input)
	lines, err := ReadLinesFrom(bytes.NewReader(original))
	if err != nil {
		t.Fatalf(errFmtReadLinesFrom, err)
	}

	var buf bytes.Buffer
	if err := WriteLinesTo(&buf, lines); err != nil {
		t.Fatalf(errFmtWriteLinesTo, err)
	}

	if !bytes.Equal(buf.Bytes(), original) {
		t.Fatalf("round-trip mismatch:\nwant %q\ngot  %q", original, buf.Bytes())
	}
}
