package fileops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
)

var (
	errExtractTestRead  = errors.New("extract test read failed")
	errExtractTestWrite = errors.New("extract test write failed")
)

type nonSeekableExtractSource struct {
	reader *strings.Reader
}

func newNonSeekableExtractSource(src string) *nonSeekableExtractSource {
	return &nonSeekableExtractSource{reader: strings.NewReader(src)}
}

func (s *nonSeekableExtractSource) Read(p []byte) (int, error) {
	return s.reader.Read(p)
}

type failingExtractReader struct {
	done bool
}

func (r *failingExtractReader) Read([]byte) (int, error) {
	if r.done {
		return 0, errExtractTestRead
	}
	r.done = true

	return 0, errExtractTestRead
}

type failingExtractWriter struct{}

func (failingExtractWriter) Write([]byte) (int, error) {
	return 0, errExtractTestWrite
}

type extractCase struct {
	name          string
	src           string
	lines         LineRange
	wantOut       string
	wantExtracted int
	wantErr       error
}

func (c *extractCase) assertError(t *testing.T) {
	t.Helper()

	var buf bytes.Buffer
	opts := ExtractOptions{Source: strings.NewReader(c.src), Lines: c.lines}

	_, err := Extract(context.Background(), opts, &buf)
	if err == nil {
		t.Fatal("want error")
	}

	if !errors.Is(err, c.wantErr) {
		t.Fatalf("want %v got %v", c.wantErr, err)
	}
}

func (c *extractCase) assertSuccess(t *testing.T) {
	t.Helper()

	var buf bytes.Buffer
	opts := ExtractOptions{Source: strings.NewReader(c.src), Lines: c.lines}

	res, err := Extract(context.Background(), opts, &buf)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if res.LinesExtracted != c.wantExtracted {
		t.Fatalf("LinesExtracted: want %d got %d", c.wantExtracted, res.LinesExtracted)
	}

	if got := buf.String(); got != c.wantOut {
		t.Fatalf("output: want %q got %q", c.wantOut, got)
	}
}

func TestRangeExceedsFileError_emptyFileCarriesFileLinesZero(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	opts := ExtractOptions{Source: strings.NewReader(""), Lines: LineRange{Start: 1, End: 1}}

	_, err := Extract(context.Background(), opts, &buf)
	if err == nil {
		t.Fatal("want error")
	}
	if !errors.Is(err, ErrRangeExceedsFile) {
		t.Fatalf("want ErrRangeExceedsFile, got %v", err)
	}
	var rxf *RangeExceedsFileError
	if !errors.As(err, &rxf) {
		t.Fatalf("want *RangeExceedsFileError, got %v", err)
	}
	if rxf.FileLines != 0 || rxf.RangeStart != 1 || rxf.RangeEnd != 1 {
		t.Fatalf("got FileLines=%d RangeStart=%d RangeEnd=%d", rxf.FileLines, rxf.RangeStart, rxf.RangeEnd)
	}
}

func TestEmptyRangeError_structuredEndpoints(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	opts := ExtractOptions{Source: strings.NewReader("a\nb\n"), Lines: LineRange{Start: 2, End: 1}}

	_, err := Extract(context.Background(), opts, &buf)
	if err == nil {
		t.Fatal("want error")
	}
	if !errors.Is(err, ErrEmptyRange) {
		t.Fatalf("want ErrEmptyRange, got %v", err)
	}
	var er *EmptyRangeError
	if !errors.As(err, &er) {
		t.Fatalf("want *EmptyRangeError, got %v", err)
	}
	if er.Start != 2 || er.End != 1 {
		t.Fatalf("got Start=%d End=%d", er.Start, er.End)
	}
}

func TestExtract_CanceledContextReturnsWrappedErrorBeforeReading(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Extract(ctx, ExtractOptions{
		Source: newNonSeekableExtractSource("a\n"),
		Lines:  LineRange{Start: 1, End: 1},
	}, &bytes.Buffer{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Extract canceled error = %v, want context.Canceled", err)
	}
}

func TestExtract_NonSeekableClosedRangePreservesInclusiveLines(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	res, err := Extract(context.Background(), ExtractOptions{
		Source: newNonSeekableExtractSource("a\nb\nc\nd\n"),
		Lines:  LineRange{Start: 2, End: 3},
	}, &buf)
	if err != nil {
		t.Fatalf("Extract non-seekable closed range: %v", err)
	}
	if res.LinesExtracted != 2 {
		t.Fatalf("LinesExtracted = %d, want 2", res.LinesExtracted)
	}
	if got := buf.String(); got != "b\nc\n" {
		t.Fatalf("output = %q, want b/c lines", got)
	}
}

func TestExtract_NonSeekableOpenEndedStreamsFromStartLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	res, err := Extract(context.Background(), ExtractOptions{
		Source: newNonSeekableExtractSource("a\nb\nc\n"),
		Lines:  LineRange{Start: 2},
	}, &buf)
	if err != nil {
		t.Fatalf("Extract non-seekable open ended: %v", err)
	}
	if res.LinesExtracted != 2 {
		t.Fatalf("LinesExtracted = %d, want 2", res.LinesExtracted)
	}
	if got := buf.String(); got != "b\nc\n" {
		t.Fatalf("output = %q, want tail lines", got)
	}
}

func TestExtract_NonSeekableOpenEndedPastEOFReturnsStructuredRangeError(t *testing.T) {
	t.Parallel()

	_, err := Extract(context.Background(), ExtractOptions{
		Source: newNonSeekableExtractSource("a\n"),
		Lines:  LineRange{Start: 3},
	}, &bytes.Buffer{})
	if !errors.Is(err, ErrRangeExceedsFile) {
		t.Fatalf("Extract error = %v, want ErrRangeExceedsFile", err)
	}
	var rxf *RangeExceedsFileError
	if !errors.As(err, &rxf) {
		t.Fatalf("Extract error = %v, want *RangeExceedsFileError", err)
	}
	if rxf.FileLines != 1 || rxf.RangeStart != 3 || rxf.RangeEnd != 0 {
		t.Fatalf("range error = %+v, want file_lines=1 range_start=3 range_end=0", rxf)
	}
}

func TestExtract_NonSeekableClosedRangeWrapsReadError(t *testing.T) {
	t.Parallel()

	_, err := Extract(context.Background(), ExtractOptions{
		Source: &failingExtractReader{},
		Lines:  LineRange{Start: 1, End: 1},
	}, &bytes.Buffer{})
	if !errors.Is(err, errExtractTestRead) {
		t.Fatalf("Extract read error = %v, want %v", err, errExtractTestRead)
	}
}

func TestExtract_NonSeekableClosedRangeWrapsWriteError(t *testing.T) {
	t.Parallel()

	_, err := Extract(context.Background(), ExtractOptions{
		Source: newNonSeekableExtractSource("a\n"),
		Lines:  LineRange{Start: 1, End: 1},
	}, failingExtractWriter{})
	if !errors.Is(err, errExtractTestWrite) {
		t.Fatalf("Extract write error = %v, want %v", err, errExtractTestWrite)
	}
}

func TestExtract_NonSeekableOpenEndedWrapsWriteError(t *testing.T) {
	t.Parallel()

	_, err := Extract(context.Background(), ExtractOptions{
		Source: newNonSeekableExtractSource("a\n"),
		Lines:  LineRange{Start: 1},
	}, failingExtractWriter{})
	if !errors.Is(err, errExtractTestWrite) {
		t.Fatalf("Extract write error = %v, want %v", err, errExtractTestWrite)
	}
}

func TestExtract(t *testing.T) {
	t.Parallel()

	cases := []extractCase{
		{
			name:          "inclusive_range",
			src:           "a\nb\nc\nd\n",
			lines:         LineRange{Start: 2, End: 3},
			wantOut:       "b\nc\n",
			wantExtracted: 2,
		},
		{
			name:          "open_end_eof",
			src:           "l1\nl2\nl3\n",
			lines:         LineRange{Start: 2, End: 0},
			wantOut:       "l2\nl3\n",
			wantExtracted: 2,
		},
		{
			name:          "open_start_line1",
			src:           "l1\nl2\nl3\n",
			lines:         LineRange{Start: 0, End: 2},
			wantOut:       "l1\nl2\n",
			wantExtracted: 2,
		},
		{
			name:    "empty_range",
			src:     "a\nb\n",
			lines:   LineRange{Start: 2, End: 1},
			wantErr: ErrEmptyRange,
		},
		{
			name:    "range_exceeds_errors",
			src:     "a\nb\nc\n",
			lines:   LineRange{Start: 2, End: 99},
			wantErr: ErrRangeExceedsFile,
		},
		{
			name:    "empty_file_range_exceeds",
			src:     "",
			lines:   LineRange{Start: 1, End: 10},
			wantErr: ErrRangeExceedsFile,
		},
		{
			name:          "preserves_terminators",
			src:           "a\r\nb\nc",
			lines:         LineRange{Start: 1, End: 3},
			wantOut:       "a\r\nb\nc",
			wantExtracted: 3,
		},
		{
			name:          "single_line_no_terminator",
			src:           "only line",
			lines:         LineRange{Start: 1, End: 1},
			wantOut:       "only line",
			wantExtracted: 1,
		},
	}

	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.wantErr != nil {
				tc.assertError(t)

				return
			}

			tc.assertSuccess(t)
		})
	}
}

//nolint:gocyclo // Table-driven subtests cover terminator variants
func TestExtract_SpanFingerprintMatchesIncrementalLineHasher(t *testing.T) {
	t.Parallel()

	raw := "\xef\xbb\xbfline1\r\nLINE2\n"
	lr := LineRange{Start: 1, End: 2}

	rdr := strings.NewReader(raw)

	plan, err := PlanExtractSerializedSpan(context.Background(), rdr, lr)
	if err != nil {
		t.Fatalf("PlanExtractSerializedSpan: %v", err)
	}

	fpSpan, err := SHA256SerializedByteSpan(context.Background(), rdr, plan.SerializedStart, plan.SerializedEndExclusive)
	if err != nil {
		t.Fatalf("SHA256SerializedByteSpan: %v", err)
	}

	incrementalHasher := sha256.New()
	lineNum := 0
	normStart := lr.Start
	if normStart == 0 {
		normStart = 1
	}
	err = ForEachLineFromContext(context.Background(), strings.NewReader(raw), func(ln Line) error {
		lineNum++
		if lineNum >= normStart && (lr.End == 0 || lineNum <= lr.End) {
			HashSerializedLineInto(incrementalHasher, ln)
		}

		if lr.End != 0 && lineNum == lr.End {
			return errExtractClosedRangeComplete
		}

		return nil
	})
	if errors.Is(err, errExtractClosedRangeComplete) {
		err = nil
	}

	if err != nil {
		t.Fatalf("ForEachLineFromContext: %v", err)
	}

	var fpLines [32]byte
	copy(fpLines[:], incrementalHasher.Sum(nil))

	if fpSpan != fpLines {
		t.Fatalf("span fingerprint %x differs from streamed line fingerprint %x", fpSpan, fpLines)
	}
}

func TestExtract_BOMPreservedInExtract(t *testing.T) {
	t.Parallel()

	bom := "\xef\xbb\xbf"
	src := bom + "line1\nline2\n"
	var buf bytes.Buffer
	opts := ExtractOptions{Source: strings.NewReader(src), Lines: LineRange{Start: 1, End: 1}}
	res, err := Extract(context.Background(), opts, &buf)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if res.LinesExtracted != 1 {
		t.Fatalf("want 1 line, got %d", res.LinesExtracted)
	}

	// BOM should be preserved as part of line 1's content
	got := buf.String()
	if !strings.HasPrefix(got, bom) {
		t.Fatalf("BOM not preserved: got %q", got)
	}
}

func TestExtract_SeekableCopyWriteFails(t *testing.T) {
	t.Parallel()

	src := bytes.NewReader([]byte("line1\nline2\nline3\n"))
	opts := ExtractOptions{Source: src, Lines: LineRange{Start: 2, End: 2}}

	_, err := Extract(context.Background(), opts, failingExtractWriter{})
	if !errors.Is(err, errExtractTestWrite) {
		t.Fatalf("want errExtractTestWrite, got %v", err)
	}
}
