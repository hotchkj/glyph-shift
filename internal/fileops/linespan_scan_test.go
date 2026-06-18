package fileops

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"regexp"
	"sync"
	"testing"
)

var errInvalidWhenceForMeteringTest = errors.New("invalid whence")

func TestScanLineSpans_Empty(t *testing.T) {
	t.Parallel()

	var got int
	err := ScanLineSpans(context.Background(), bytes.NewReader(nil), func(LineSpan) error {
		got++

		return nil
	})
	if err != nil {
		t.Fatalf("ScanLineSpans: %v", err)
	}
	if got != 0 {
		t.Fatalf("expected zero lines, got %d", got)
	}
}

type terminatorsTrailingCase struct {
	name   string
	raw    []byte
	want   []LineSpan
	parity bool // also compare ReadLinesFromContext reconstruction
}

func terminatorsAndTrailingCases() []terminatorsTrailingCase {
	return []terminatorsTrailingCase{
		{
			name: "lf_empty_line",
			raw:  []byte("\n"),
			want: []LineSpan{
				{LineNum: 1, SerializedStart: 0, SerializedEnd: 1, ContentStart: 0, ContentEnd: 0, Terminator: LineTerminatorLF},
			},
			parity: true,
		},
		{
			name: "crlf_empty_line",
			raw:  []byte("\r\n"),
			want: []LineSpan{
				{LineNum: 1, SerializedStart: 0, SerializedEnd: 2, ContentStart: 0, ContentEnd: 0, Terminator: LineTerminatorCRLF},
			},
			parity: true,
		},
		{
			name: "bare_cr_empty_line",
			raw:  []byte("\r"),
			want: []LineSpan{
				{LineNum: 1, SerializedStart: 0, SerializedEnd: 1, ContentStart: 0, ContentEnd: 0, Terminator: LineTerminatorCR},
			},
			parity: true,
		},
		{
			name: "trailing_without_terminator",
			raw:  []byte("ab"),
			want: []LineSpan{
				{LineNum: 1, SerializedStart: 0, SerializedEnd: 2, ContentStart: 0, ContentEnd: 2, Terminator: LineTerminatorNone},
			},
			parity: true,
		},
		{
			name: "lf_then_eof_trailer",
			raw:  []byte("a\nbc"),
			want: []LineSpan{
				{LineNum: 1, SerializedStart: 0, SerializedEnd: 2, ContentStart: 0, ContentEnd: 1, Terminator: LineTerminatorLF},
				{LineNum: 2, SerializedStart: 2, SerializedEnd: 4, ContentStart: 2, ContentEnd: 4, Terminator: LineTerminatorNone},
			},
			parity: true,
		},
		{
			name: "bare_cr_splits_next_byte",
			raw:  []byte("ab\rc"),
			want: []LineSpan{
				{LineNum: 1, SerializedStart: 0, SerializedEnd: 3, ContentStart: 0, ContentEnd: 2, Terminator: LineTerminatorCR},
				{LineNum: 2, SerializedStart: 3, SerializedEnd: 4, ContentStart: 3, ContentEnd: 4, Terminator: LineTerminatorNone},
			},
			parity: true,
		},
		{
			name: "mixed",
			raw:  []byte("a\nb\r\nc\rd"),
			want: []LineSpan{
				{LineNum: 1, SerializedStart: 0, SerializedEnd: 2, ContentStart: 0, ContentEnd: 1, Terminator: LineTerminatorLF},
				{LineNum: 2, SerializedStart: 2, SerializedEnd: 5, ContentStart: 2, ContentEnd: 3, Terminator: LineTerminatorCRLF},
				{LineNum: 3, SerializedStart: 5, SerializedEnd: 7, ContentStart: 5, ContentEnd: 6, Terminator: LineTerminatorCR},
				{LineNum: 4, SerializedStart: 7, SerializedEnd: 8, ContentStart: 7, ContentEnd: 8, Terminator: LineTerminatorNone},
			},
			parity: true,
		},
	}
}

func assertSpansEqualWant(t *testing.T, spans, want []LineSpan) {
	t.Helper()

	if len(spans) != len(want) {
		t.Fatalf("got %d lines want %d: %#v vs %#v", len(spans), len(want), spans, want)
	}

	for i := range spans {
		if spans[i] != want[i] {
			t.Fatalf("line %d\n got  %#v\n want %#v", i+1, spans[i], want[i])
		}
	}
}

func assertParityReadLinesAndRoundTripSerialized(t *testing.T, parity bool, raw []byte, spans []LineSpan) {
	t.Helper()

	if !parity {
		return
	}

	assertParityReadLinesForScan(t, raw)

	got := reconstructFromSpansSeek(t, raw, spans)
	if !bytes.Equal(got, raw) {
		t.Fatalf("serialized round-trip mismatch:\n got %q\nwant %q", got, raw)
	}
}

func TestScanLineSpans_TerminatorsAndTrailing(t *testing.T) {
	t.Parallel()

	for _, tc := range terminatorsAndTrailingCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spans := scanAllSpans(t, tc.raw)
			assertSpansEqualWant(t, spans, tc.want)
			assertSpansMonotonic(t, spans)
			assertParityReadLinesAndRoundTripSerialized(t, tc.parity, tc.raw, spans)
		})
	}
}

func TestScanLineSpans_CRLFCrossesTinyReadBuffer(t *testing.T) {
	t.Parallel()

	raw := []byte("abc\r\ndef\n")
	opts := linespanScanOptions{chunkSizeBytes: 4}
	want := []LineSpan{
		{LineNum: 1, SerializedStart: 0, SerializedEnd: 5, ContentStart: 0, ContentEnd: 3, Terminator: LineTerminatorCRLF},
		{LineNum: 2, SerializedStart: 5, SerializedEnd: 9, ContentStart: 5, ContentEnd: 8, Terminator: LineTerminatorLF},
	}

	var got []LineSpan
	err := scanLineSpansWithOptions(context.Background(), bytes.NewReader(raw), func(sp LineSpan) error {
		got = append(got, sp)

		return nil
	}, opts)
	if err != nil {
		t.Fatalf("ScanLineSpans: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d want %d spans: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("span %d %#v want %#v", i+1, got[i], want[i])
		}
	}
	assertParityReadLinesForScan(t, raw)
}

func TestScanLineSpans_ContextCancelledImmediately(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ScanLineSpans(ctx, bytes.NewReader([]byte("x\n")), func(LineSpan) error { return nil })
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
}

func TestScanLineSpans_ContextCancelledDuringLongLine(t *testing.T) {
	t.Parallel()

	opts := linespanScanOptions{contextCheckBytes: 3}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	body := bytes.Repeat([]byte{'z'}, 200_000)

	// Cancel after the first successful Read fills the scanner buffer — before byte-cadence checks
	// consume the long line — so cancellation is deterministic with no wall-clock race.
	rs := cancelAfterNonemptyReadSeeker{r: bytes.NewReader(body), cancel: cancel}

	err := scanLineSpansWithOptions(ctx, &rs, func(LineSpan) error { return nil }, opts)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled, got %v", err)
	}
}

func TestScanLineSpans_RefillTail_ReadZeroWithoutEOF_ReturnsError(t *testing.T) {
	t.Parallel()

	err := ScanLineSpans(context.Background(), zeroProgressReadSeeker{}, func(LineSpan) error {
		t.Fatal("callback must not run after zero-progress read")

		return nil
	})
	if !errors.Is(err, errScanLineSpansReadZeroNoEOF) {
		t.Fatalf("want errScanLineSpansReadZeroNoEOF, got %v", err)
	}
}

type zeroProgressReadSeeker struct{}

func (zeroProgressReadSeeker) Read([]byte) (int, error) {
	return 0, nil
}

func (zeroProgressReadSeeker) Seek(int64, int) (int64, error) {
	return 0, nil
}

// cancelAfterNonemptyReadSeeker delegates to r and invokes cancel exactly once after the first Read
// returns n > 0, modeling deferred cancellation observed while scanning a long logical line.
type cancelAfterNonemptyReadSeeker struct {
	r      io.ReadSeeker
	cancel context.CancelFunc
	once   sync.Once
}

func (s *cancelAfterNonemptyReadSeeker) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	if n > 0 {
		s.once.Do(s.cancel)
	}

	return n, err
}

func (s *cancelAfterNonemptyReadSeeker) Seek(offset int64, whence int) (int64, error) {
	return s.r.Seek(offset, whence)
}

func TestMatchLineSpan_ContentLengthOverflowsInt64(t *testing.T) {
	t.Parallel()

	span := LineSpan{
		LineNum:         1,
		SerializedStart: 0,
		SerializedEnd:   0,
		ContentStart:    0,
		ContentEnd:      uint64(math.MaxInt64) + 1,
		Terminator:      LineTerminatorNone,
	}

	_, err := MatchLineSpan(context.Background(), bytes.NewReader(nil), span, regexp.MustCompile(`x`))
	if err == nil {
		t.Fatal("expected error for span length overflowing int64")
	}
	if !errors.Is(err, ErrLineSpanContentTooLarge) {
		t.Fatalf("expected ErrLineSpanContentTooLarge, got %v", err)
	}

	_, err = FindLineSpanSubmatchIndex(context.Background(), bytes.NewReader(nil), span, regexp.MustCompile(`x`))
	if err == nil {
		t.Fatal("expected error for span length overflowing int64 (FindLineSpanSubmatchIndex)")
	}
	if !errors.Is(err, ErrLineSpanContentTooLarge) {
		t.Fatalf("expected ErrLineSpanContentTooLarge, got %v", err)
	}
}

func TestMatchLineSpan_ContentStartOverflowsInt64(t *testing.T) {
	t.Parallel()

	span := LineSpan{
		LineNum:         1,
		SerializedStart: 0,
		SerializedEnd:   0,
		ContentStart:    uint64(math.MaxInt64) + 1,
		ContentEnd:      uint64(math.MaxInt64) + 2,
		Terminator:      LineTerminatorNone,
	}

	_, err := MatchLineSpan(context.Background(), bytes.NewReader(nil), span, regexp.MustCompile(`x`))
	if err == nil {
		t.Fatal("expected error for ContentStart overflowing int64")
	}
	if !errors.Is(err, ErrLineSpanOffsetTooLarge) {
		t.Fatalf("expected ErrLineSpanOffsetTooLarge, got %v", err)
	}

	_, err = FindLineSpanSubmatchIndex(context.Background(), bytes.NewReader(nil), span, regexp.MustCompile(`x`))
	if err == nil {
		t.Fatal("expected error for ContentStart overflowing int64 (FindLineSpanSubmatchIndex)")
	}
	if !errors.Is(err, ErrLineSpanOffsetTooLarge) {
		t.Fatalf("expected ErrLineSpanOffsetTooLarge, got %v", err)
	}
}

func TestMatchLineSpan_BoundedToContent(t *testing.T) {
	t.Parallel()

	raw := []byte("first line\nSECRETPAST")
	re := regexp.MustCompile(`line`)
	span := LineSpan{
		LineNum: 1, SerializedStart: 0, SerializedEnd: 11, ContentStart: 0, ContentEnd: 10, Terminator: LineTerminatorLF,
	}

	ms := newMeteringSeeker(raw)
	ok, err := MatchLineSpan(context.Background(), ms, span, re)
	if err != nil {
		t.Fatalf("MatchLineSpan: %v", err)
	}
	if !ok {
		t.Fatal("expected match within first line")
	}

	if ms.maxOffsetSeen > int64(span.ContentEnd) { //nolint:gosec // G115: fixture span fits test buffer indices
		t.Fatalf("seek/read advanced past content end: max %d want <= %d", ms.maxOffsetSeen, span.ContentEnd)
	}
}

func TestFindLineSpanSubmatchIndex_RelativeOffsets(t *testing.T) {
	t.Parallel()

	raw := []byte("alpha beta\n")
	span := LineSpan{
		LineNum: 1, SerializedStart: 0, SerializedEnd: 11, ContentStart: 0, ContentEnd: 10, Terminator: LineTerminatorLF,
	}

	re := regexp.MustCompile(`be(ta)`)
	idx, err := FindLineSpanSubmatchIndex(context.Background(), bytes.NewReader(raw), span, re)
	if err != nil {
		t.Fatalf("FindLineSpanSubmatchIndex: %v", err)
	}
	if len(idx) < 4 {
		t.Fatalf("unexpected idx %v", idx)
	}

	//nolint:gosec // G115: test uses small fixture offsets
	cs := int(span.ContentStart)
	full := raw[cs+idx[0] : cs+idx[1]] //nolint:gosec // G115: UTF-8 indices within small fixture
	sub := raw[cs+idx[2] : cs+idx[3]]  //nolint:gosec // G115: UTF-8 indices within small fixture
	if string(full) != "beta" || string(sub) != "ta" {
		t.Fatalf("unexpected text full=%q sub=%q idx=%v", full, sub, idx)
	}
}

func assertSpansMonotonic(t *testing.T, spans []LineSpan) {
	t.Helper()

	for i := 1; i < len(spans); i++ {
		prev, cur := spans[i-1], spans[i]
		if prev.SerializedEnd != cur.SerializedStart {
			t.Fatalf("non-contiguous spans at %d: prev.End=%d cur.Start=%d", i, prev.SerializedEnd, cur.SerializedStart)
		}
		if prev.LineNum+1 != cur.LineNum {
			t.Fatalf("line numbers not sequential: %#v then %#v", prev, cur)
		}
	}
}

//nolint:gocyclo,varnamelen // Parity harness compares ReadLines vs span scan across terminator kinds.
func assertParityReadLinesForScan(t *testing.T, raw []byte) {
	t.Helper()

	lines, err := ReadLinesFromContext(context.Background(), bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadLinesFromContext: %v", err)
	}

	var spans []LineSpan

	err = ScanLineSpans(context.Background(), bytes.NewReader(raw), func(ln LineSpan) error {
		spans = append(spans, ln)

		return nil
	})
	if err != nil {
		t.Fatalf("ScanLineSpans: %v", err)
	}
	if len(lines) != len(spans) {
		t.Fatalf("line count mismatch scan=%d readlines=%d", len(spans), len(lines))
	}

	for i := range lines {
		wantBody := lines[i].Content
		wantTerm := lines[i].Terminator

		var gotBody []byte
		s := spans[i]
		gotBody = raw[s.ContentStart:s.ContentEnd]

		if !bytes.Equal(gotBody, wantBody) {
			t.Fatalf("line %d body mismatch got %q want %q", i+1, gotBody, wantBody)
		}

		var gotTerm []byte
		switch spans[i].Terminator {
		case LineTerminatorNone:
		case LineTerminatorLF:
			gotTerm = []byte{'\n'}
		case LineTerminatorCR:
			gotTerm = []byte{'\r'}
		case LineTerminatorCRLF:
			gotTerm = []byte{'\r', '\n'}
		}
		if !bytes.Equal(gotTerm, wantTerm) {
			t.Fatalf("line %d terminator mismatch got %q want %q", i+1, gotTerm, wantTerm)
		}
	}
}

//nolint:varnamelen // Reader buffer/shift indices use short names matching stdlib conventions.
func reconstructFromSpansSeek(t *testing.T, raw []byte, spans []LineSpan) []byte {
	t.Helper()

	r := bytes.NewReader(raw)
	var out bytes.Buffer
	for _, sp := range spans {
		if _, err := r.Seek(int64(sp.SerializedStart), io.SeekStart); err != nil { //nolint:gosec // G115: test buffer spans
			t.Fatalf("Seek: %v", err)
		}

		seg := make([]byte, sp.SerializedEnd-sp.SerializedStart)
		if _, err := io.ReadFull(r, seg); err != nil {
			t.Fatalf("ReadFull segment: %v", err)
		}

		if _, err := out.Write(seg); err != nil {
			t.Fatalf("buffer write: %v", err)
		}
	}

	return out.Bytes()
}

func scanAllSpans(t *testing.T, raw []byte) []LineSpan {
	t.Helper()

	var spans []LineSpan

	err := ScanLineSpans(context.Background(), bytes.NewReader(raw), func(ln LineSpan) error {
		spans = append(spans, ln)

		return nil
	})
	if err != nil {
		t.Fatalf("ScanLineSpans: %v", err)
	}

	return spans
}

// meteringSeeker verifies reads never reposition beyond scanned tail for MatchLineSpan.
type meteringSeeker struct {
	data          []byte
	pos           int64
	maxOffsetSeen int64
}

func newMeteringSeeker(b []byte) *meteringSeeker {
	return &meteringSeeker{data: b}
}

//nolint:varnamelen // bytes.Reader-style Read contract
func (m *meteringSeeker) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	if int(m.pos) >= len(m.data) {
		return 0, io.EOF
	}

	n := copy(p, m.data[m.pos:])
	m.pos += int64(n)
	if m.pos > m.maxOffsetSeen {
		m.maxOffsetSeen = m.pos
	}

	return n, nil
}

func (m *meteringSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		m.pos = offset
	case io.SeekCurrent:
		m.pos += offset
	case io.SeekEnd:
		m.pos = int64(len(m.data)) + offset
	default:
		return 0, errInvalidWhenceForMeteringTest
	}

	if m.pos > m.maxOffsetSeen {
		m.maxOffsetSeen = m.pos
	}

	return m.pos, nil
}

var errSeekStreamStartTest = errors.New("scan test: seek stream start fails")

type seekStreamStartFailRS struct {
	*bytes.Reader
}

func (seekStreamStartFailRS) Seek(int64, int) (int64, error) {
	return 0, errSeekStreamStartTest
}

func TestLineTerminatorKind_LenUnknownKind(t *testing.T) {
	t.Parallel()

	if got := (LineTerminatorKind(255)).Len(); got != 0 {
		t.Fatalf("unknown terminator kind: want 0 len, got %d", got)
	}
}

func TestScanLineSpans_SeekStreamStartFails(t *testing.T) {
	t.Parallel()

	raw := []byte("x\n")
	rs := seekStreamStartFailRS{Reader: bytes.NewReader(raw)}
	err := ScanLineSpans(context.Background(), rs, func(LineSpan) error { return nil })
	if !errors.Is(err, errSeekStreamStartTest) {
		t.Fatalf("want seek start error, got %v", err)
	}
}

func TestScanLineSpans_GrowBufSingleVeryLongLine(t *testing.T) {
	t.Parallel()

	// One logical line longer than defaultLinespanChunkSize forces sliding-window growth.
	body := bytes.Repeat([]byte("M"), 70*1024)
	raw := make([]byte, len(body)+1)
	copy(raw, body)
	raw[len(body)] = '\n'

	var got int
	err := ScanLineSpans(context.Background(), bytes.NewReader(raw), func(LineSpan) error {
		got++

		return nil
	})
	if err != nil {
		t.Fatalf("ScanLineSpans: %v", err)
	}
	if got != 1 {
		t.Fatalf("want 1 span, got %d", got)
	}
}

func TestScanLineSpans_ContextCanceledAfterLineEmitProbe(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	raw := []byte("a\nb\n")

	opts := linespanScanOptions{contextCheckLines: 1}
	err := scanLineSpansWithOptions(ctx, bytes.NewReader(raw), func(span LineSpan) error {
		if span.LineNum == 1 {
			cancel()
		}

		return nil
	}, opts)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled after first line probe, got %v", err)
	}
}
