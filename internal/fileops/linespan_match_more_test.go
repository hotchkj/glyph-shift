package fileops

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"math"
	"regexp"
	"strings"
	"testing"
)

var errLiteralReaderFailed = errors.New("literal reader failed")

type failingLiteralReader struct{}

func (failingLiteralReader) Read([]byte) (int, error) {
	return 0, errLiteralReaderFailed
}

func TestMatchLineSpan_NilRegexp(t *testing.T) {
	t.Parallel()

	span := LineSpan{LineNum: 1, ContentStart: 0, ContentEnd: 1, Terminator: LineTerminatorNone}
	_, err := MatchLineSpan(context.Background(), bytes.NewReader([]byte("x")), span, nil)
	if !errors.Is(err, errMatchLineSpanNilRegexp) {
		t.Fatalf("want errMatchLineSpanNilRegexp, got %v", err)
	}
}

func TestFindLineSpanSubmatchIndex_NilRegexp(t *testing.T) {
	t.Parallel()

	span := LineSpan{LineNum: 1, ContentStart: 0, ContentEnd: 1, Terminator: LineTerminatorNone}
	_, err := FindLineSpanSubmatchIndex(context.Background(), bytes.NewReader([]byte("x")), span, nil)
	if !errors.Is(err, errFindLineSpanSubmatchNilRegexp) {
		t.Fatalf("want errFindLineSpanSubmatchNilRegexp, got %v", err)
	}
}

func TestMatchLineSpan_InvertedContentOffsets(t *testing.T) {
	t.Parallel()

	span := LineSpan{LineNum: 1, ContentStart: 5, ContentEnd: 2, Terminator: LineTerminatorLF}
	_, err := MatchLineSpan(context.Background(), bytes.NewReader([]byte("hello\n")), span, regexp.MustCompile(`x`))
	if !errors.Is(err, errLineSpanReaderInvertedOffsets) {
		t.Fatalf("want errLineSpanReaderInvertedOffsets, got %v", err)
	}
}

func TestFindLineSpanSubmatchIndex_InvertedContentOffsets(t *testing.T) {
	t.Parallel()

	span := LineSpan{LineNum: 1, ContentStart: 40, ContentEnd: 10, Terminator: LineTerminatorLF}
	rdr := bytes.NewReader([]byte("aa\n"))
	subRe := regexp.MustCompile(`x`)
	_, err := FindLineSpanSubmatchIndex(context.Background(), rdr, span, subRe)
	if !errors.Is(err, errLineSpanReaderInvertedOffsets) {
		t.Fatalf("want errLineSpanReaderInvertedOffsets, got %v", err)
	}
}

func TestMatchLineSpan_RejectsUnrepresentableContentLength(t *testing.T) {
	t.Parallel()

	span := LineSpan{LineNum: 1, ContentStart: 0, ContentEnd: uint64(math.MaxInt64) + 1}
	_, err := MatchLineSpan(context.Background(), bytes.NewReader([]byte("x")), span, regexp.MustCompile(`x`))
	if !errors.Is(err, ErrLineSpanContentTooLarge) {
		t.Fatalf("want ErrLineSpanContentTooLarge, got %v", err)
	}
}

func TestMatchLineSpan_RejectsUnrepresentableContentOffset(t *testing.T) {
	t.Parallel()

	start := uint64(math.MaxInt64) + 1
	span := LineSpan{LineNum: 1, ContentStart: start, ContentEnd: start}
	_, err := MatchLineSpan(context.Background(), bytes.NewReader([]byte("x")), span, regexp.MustCompile(`x`))
	if !errors.Is(err, ErrLineSpanOffsetTooLarge) {
		t.Fatalf("want ErrLineSpanOffsetTooLarge, got %v", err)
	}
}

func TestLineSpanRuneReaderReadRuneHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rr := &ctxBufRuneReader{ctx: ctx, br: bufio.NewReader(bytes.NewReader([]byte("x")))}
	_, _, err := rr.ReadRune()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ReadRune error = %v, want context.Canceled", err)
	}
}

func TestMatchLineSpan_LiteralPathCanceledContextUsesMatchDiagnostic(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	raw := []byte("plain\n")
	reader := &cancelingReadSeeker{Reader: bytes.NewReader(raw), cancel: cancel}
	_, err := MatchLineSpan(ctx, reader, singleLineSpanForLF(len(raw)), regexp.MustCompile("MARK"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("MatchLineSpan error = %v, want context.Canceled", err)
	}
}

type cancelingReadSeeker struct {
	*bytes.Reader
	cancel context.CancelFunc
}

func (r *cancelingReadSeeker) Read(payload []byte) (int, error) {
	r.cancel()

	return r.Reader.Read(payload)
}

func TestMatchLineSpan_FastPathsMatchRegexpSemantics(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		pattern string
		line    string
	}{
		{name: "literal substring", pattern: "MARK", line: "xxMARKyy\n"},
		{name: "anchored literal", pattern: "^---$", line: "---\n"},
		{name: "escaped anchored literal", pattern: `^\.$`, line: ".\n"},
		{name: "char class not treated as literal", pattern: `^a[0-9]$`, line: "a7\n"},
		{name: "escaped digit class not treated as literal", pattern: `^a\d$`, line: "a7\n"},
		{name: "case insensitive not treated as literal", pattern: `(?i)^abc$`, line: "ABC\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw := []byte(tc.line)
			span := singleLineSpanForLF(len(raw))
			re := regexp.MustCompile(tc.pattern)

			got, err := MatchLineSpan(context.Background(), bytes.NewReader(raw), span, re)
			if err != nil {
				t.Fatalf("MatchLineSpan: %v", err)
			}

			want := re.MatchString(tc.line[:len(tc.line)-1])
			if got != want {
				t.Fatalf("MatchLineSpan = %v, want regexp semantics %v", got, want)
			}
		})
	}
}

func TestLiteralLineSpanMatchDetection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		pattern   string
		wantOK    bool
		wantExact bool
		wantLit   string
	}{
		{name: "anchored ascii literal", pattern: "^---$", wantOK: true, wantExact: true, wantLit: "---"},
		{name: "anchored escaped literal", pattern: `^\.$`, wantOK: true, wantExact: true, wantLit: "."},
		{name: "plain complete literal", pattern: "MARK", wantOK: true, wantExact: false, wantLit: "MARK"},
		{name: "char class rejected", pattern: `^a[0-9]$`},
		{name: "escaped digit class rejected", pattern: `^a\d$`},
		{name: "case fold rejected", pattern: `(?i)^abc$`},
		{name: "non ascii literal rejected", pattern: "micro-µ"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			lit, exact, ok := literalLineSpanMatch(regexp.MustCompile(tc.pattern))
			if ok != tc.wantOK || exact != tc.wantExact || string(lit) != tc.wantLit {
				t.Fatalf(
					"literalLineSpanMatch(%q) = lit %q exact %v ok %v; want lit %q exact %v ok %v",
					tc.pattern, lit, exact, ok, tc.wantLit, tc.wantExact, tc.wantOK,
				)
			}
		})
	}
}

func TestContainsLiteralInReaderFindsChunkBoundaryMatch(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat("a", literalScanChunkBytes-2) + "MARK"

	got, err := containsLiteralInReader(context.Background(), strings.NewReader(raw), []byte("MARK"))
	if err != nil {
		t.Fatalf("containsLiteralInReader: %v", err)
	}
	if !got {
		t.Fatal("literal spanning chunk boundary was not found")
	}
}

func TestContainsLiteralInReaderPropagatesReadError(t *testing.T) {
	t.Parallel()

	_, err := containsLiteralInReader(context.Background(), failingLiteralReader{}, []byte("MARK"))
	if !errors.Is(err, errLiteralReaderFailed) {
		t.Fatalf("containsLiteralInReader error = %v, want %v", err, errLiteralReaderFailed)
	}
}

func TestMatchLineSpan_LiteralFastPathMissesMatchRegexpSemantics(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		pattern string
		line    string
	}{
		{name: "anchored length mismatch", pattern: "^---$", line: "----\n"},
		{name: "substring absent", pattern: "MARK", line: "plain text\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw := []byte(tc.line)
			span := singleLineSpanForLF(len(raw))

			got, err := MatchLineSpan(context.Background(), bytes.NewReader(raw), span, regexp.MustCompile(tc.pattern))
			if err != nil {
				t.Fatalf("MatchLineSpan: %v", err)
			}
			if got {
				t.Fatalf("MatchLineSpan(%q, %q) = true, want false", tc.pattern, tc.line)
			}
		})
	}
}

func TestAnchoredLiteralLinePatternRejectsInvalidSyntax(t *testing.T) {
	t.Parallel()

	if lit, ok := anchoredLiteralLinePattern("["); ok || lit != nil {
		t.Fatalf("invalid syntax literal = %q ok=%v, want rejected", lit, ok)
	}
}
