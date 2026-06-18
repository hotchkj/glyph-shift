package fileops

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestRunTransformStream_Measure_DoesNotOpenWhitespaceSpill(t *testing.T) {
	if testing.Short() {
		t.Skip("long-line spill check")
	}

	pendingWhitespaceSpillCreations.Store(0)
	t.Cleanup(func() { pendingWhitespaceSpillCreations.Store(0) })

	opts := TransformOptions{TrimTrailing: true}
	raw := buildHugeWhitespaceOnlyLine(6<<20, []byte{'\n'})

	_, err := runTransformStream(context.Background(), bytes.NewReader(raw), opts, nil, nil)
	if err != nil {
		t.Fatalf("measure stream: %v", err)
	}

	if n := pendingWhitespaceSpillCreations.Load(); n != 0 {
		t.Fatalf("measure path should not spill pending whitespace; got %d spill(s)", n)
	}
}

func TestRunTransformStream_WritePath_TrailingWhitespaceOverBufferSpills(t *testing.T) {
	if testing.Short() {
		t.Skip("long-line spill check")
	}

	pendingWhitespaceSpillCreations.Store(0)
	t.Cleanup(func() { pendingWhitespaceSpillCreations.Store(0) })

	opts := TransformOptions{TrimTrailing: true}
	raw := buildHugeWhitespaceOnlyLine(6<<20, []byte{'\n'})

	spill := memWhitespaceSpillForTests(io.Discard, opts)
	_, err := runTransformStream(context.Background(), bytes.NewReader(raw), opts, io.Discard, spill)
	if err != nil {
		t.Fatalf("write stream: %v", err)
	}

	if pendingWhitespaceSpillCreations.Load() == 0 {
		t.Fatalf("expected spill when writing trim-trailing over large pending run")
	}
}

func wouldChangeByByteStream(t *testing.T, raw []byte, opts TransformOptions) bool {
	t.Helper()

	hIn := newSHA256Reader(bytes.NewReader(raw))
	outTrack := newSHA256CountWriter()

	spill := memWhitespaceSpillForTests(outTrack, opts)
	if _, err := runTransformStream(context.Background(), hIn, opts, outTrack, spill); err != nil {
		t.Fatalf("byte stream: %v", err)
	}

	return hIn.n != outTrack.n || hIn.Digest() != outTrack.Digest()
}

// TestTransformWouldChange_StatsMatchByteEquality checks semantic WouldChange against hashing
// source vs full transform output for a matrix of options and line shapes.
func TestTransformWouldChange_StatsMatchByteEquality(t *testing.T) {
	t.Parallel()

	lf := TargetLF
	crlf := TargetCRLF
	cr := TargetCR

	cases := []struct {
		name string
		raw  []byte
		opts TransformOptions
	}{
		{
			name: "noop_all_opts_normalized",
			raw:  []byte("a\nb\n"),
			opts: TransformOptions{LineEndings: &lf, TrimTrailing: true, FinalNewline: true},
		},
		{name: "crlf_to_lf_mixed", raw: []byte("a\r\nb\nc\r\nd\r"), opts: TransformOptions{LineEndings: &lf}},
		{name: "lf_to_crlf", raw: []byte("x\ny\n"), opts: TransformOptions{LineEndings: &crlf}},
		{name: "trim_trailing_mixed_endings", raw: []byte("a  \r\nb\t \n"), opts: TransformOptions{TrimTrailing: true}},
		{
			name: "trim_and_line_endings_crlf_target",
			raw:  []byte("hello  \r\nworld\t"),
			opts: TransformOptions{LineEndings: &crlf, TrimTrailing: true},
		},
		{name: "final_newline_only", raw: []byte("z"), opts: TransformOptions{FinalNewline: true}},
		{
			name: "final_newline_with_cr_target",
			raw:  []byte("z"),
			opts: TransformOptions{FinalNewline: true, LineEndings: &cr},
		},
		{
			name: "combined_all_three",
			raw:  []byte(" trim me \r\nno trailing here\t \r\nbare"),
			opts: TransformOptions{LineEndings: &lf, TrimTrailing: true, FinalNewline: true},
		},
		{name: "empty", raw: []byte{}, opts: TransformOptions{LineEndings: &lf, TrimTrailing: true}},
		{name: "bare_cr_lines", raw: []byte("one\rtwo\r"), opts: TransformOptions{LineEndings: &lf}},
		{name: "only_final_nl_needed", raw: []byte("x\ny"), opts: TransformOptions{LineEndings: &lf, FinalNewline: true}},
		{
			name: "huge_ws_trim_single_line",
			raw:  buildHugeWhitespaceOnlyLine(200_000, []byte{'\n'}),
			opts: TransformOptions{TrimTrailing: true},
		},
		{
			name: "huge_inner_ws_then_x",
			raw:  buildHugeAltWhitespacePrefixThenX(200_000, []byte{'\n'}),
			opts: TransformOptions{TrimTrailing: true},
		},
	}

	ctx := context.Background()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res, err := runTransformStream(ctx, bytes.NewReader(tc.raw), tc.opts, nil, nil)
			if err != nil {
				t.Fatalf("measure: %v", err)
			}

			got := transformWouldChangeFromStats(tc.opts, &res)
			want := wouldChangeByByteStream(t, tc.raw, tc.opts)

			if got != want {
				t.Fatalf("semantic=%v byte=%v stats=%+v", got, want, res)
			}
		})
	}
}

func TestRunTransformStream_Measure_StatsMatchWriteBuffer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	lf := TargetLF
	crlf := TargetCRLF

	cases := []struct {
		name string
		raw  []byte
		opts TransformOptions
	}{
		{name: "crlf_to_lf", raw: []byte("a\r\nb\n"), opts: TransformOptions{LineEndings: &lf}},
		{name: "trim_only", raw: []byte("x  \n"), opts: TransformOptions{TrimTrailing: true}},
		{name: "final_nl", raw: []byte("q"), opts: TransformOptions{FinalNewline: true}},
		{name: "combined", raw: []byte("a \r\n"), opts: TransformOptions{LineEndings: &crlf, TrimTrailing: true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resMeasure, err := runTransformStream(ctx, bytes.NewReader(tc.raw), tc.opts, nil, nil)
			if err != nil {
				t.Fatalf("measure: %v", err)
			}

			var buf bytes.Buffer
			spill := memWhitespaceSpillForTests(&buf, tc.opts)
			resWrite, err := runTransformStream(ctx, bytes.NewReader(tc.raw), tc.opts, &buf, spill)
			if err != nil {
				t.Fatalf("write: %v", err)
			}

			assertTransformStatsEqual(t, "measure_vs_write", &resMeasure, &resWrite)
		})
	}
}
