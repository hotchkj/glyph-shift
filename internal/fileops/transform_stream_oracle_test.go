package fileops

import (
	"bytes"
	"context"
	"testing"
)

func oracleTransformBytes(t *testing.T, raw []byte, opts TransformOptions) ([]byte, TransformFileResult) {
	t.Helper()

	lines, err := ReadLinesFrom(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("read lines: %v", err)
	}

	outLines, res := TransformLines(lines, opts)

	var buf bytes.Buffer
	if err := WriteLinesTo(&buf, outLines); err != nil {
		t.Fatalf("write lines: %v", err)
	}

	return buf.Bytes(), res
}

//nolint:gocyclo // Repetitive oracle field parity checks; clarity over abstraction.
func assertTransformStatsEqual(t *testing.T, phase string, got, want *TransformFileResult) {
	t.Helper()

	if got == nil || want == nil {
		t.Fatalf("%s: nil stats", phase)
	}

	if got.EndingsChanged != want.EndingsChanged {
		t.Fatalf("%s: EndingsChanged %d want %d", phase, got.EndingsChanged, want.EndingsChanged)
	}

	if got.LFFound != want.LFFound {
		t.Fatalf("%s: LFFound %d want %d", phase, got.LFFound, want.LFFound)
	}

	if got.LFConverted != want.LFConverted {
		t.Fatalf("%s: LFConverted %d want %d", phase, got.LFConverted, want.LFConverted)
	}

	if got.CRFound != want.CRFound {
		t.Fatalf("%s: CRFound %d want %d", phase, got.CRFound, want.CRFound)
	}

	if got.CRConverted != want.CRConverted {
		t.Fatalf("%s: CRConverted %d want %d", phase, got.CRConverted, want.CRConverted)
	}

	if got.CRLFFound != want.CRLFFound {
		t.Fatalf("%s: CRLFFound %d want %d", phase, got.CRLFFound, want.CRLFFound)
	}

	if got.CRLFConverted != want.CRLFConverted {
		t.Fatalf("%s: CRLFConverted %d want %d", phase, got.CRLFConverted, want.CRLFConverted)
	}

	if got.TrailingTrimmed != want.TrailingTrimmed {
		t.Fatalf("%s: TrailingTrimmed %d want %d", phase, got.TrailingTrimmed, want.TrailingTrimmed)
	}

	if got.FinalNewlineAdded != want.FinalNewlineAdded {
		t.Fatalf("%s: FinalNewlineAdded %v want %v", phase, got.FinalNewlineAdded, want.FinalNewlineAdded)
	}
}

func TestRunTransformStream_MatchesTransformLinesOracle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	lf := TargetLF
	crlf := TargetCRLF
	cr := TargetCR

	cases := []struct {
		name string
		raw  []byte
		opts TransformOptions
	}{
		{
			name: "crlf_to_lf_mixed",
			raw:  []byte("a\r\nb\nc\r\nd\r"),
			opts: TransformOptions{LineEndings: &lf},
		},
		{
			name: "lf_to_crlf",
			raw:  []byte("x\ny\n"),
			opts: TransformOptions{LineEndings: &crlf},
		},
		{
			name: "trim_trailing_mixed_endings",
			raw:  []byte("a  \r\nb\t \n"),
			opts: TransformOptions{TrimTrailing: true},
		},
		{
			name: "trim_and_line_endings_crlf_target",
			raw:  []byte("hello  \r\nworld\t"),
			opts: TransformOptions{LineEndings: &crlf, TrimTrailing: true},
		},
		{
			name: "final_newline_only",
			raw:  []byte("z"),
			opts: TransformOptions{FinalNewline: true},
		},
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
		{
			name: "cr_only_lines",
			raw:  []byte("one\rtwo\r"),
			opts: TransformOptions{LineEndings: &lf},
		},
		{
			name: "empty_then_content_crlf",
			raw:  []byte("\r\nx\r\n"),
			opts: TransformOptions{LineEndings: &lf, TrimTrailing: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			wantBytes, wantRes := oracleTransformBytes(t, tc.raw, tc.opts)

			var out bytes.Buffer
			spill := memWhitespaceSpillForTests(&out, tc.opts)
			gotRes, err := runTransformStream(ctx, bytes.NewReader(tc.raw), tc.opts, &out, spill)
			if err != nil {
				t.Fatalf("stream: %v", err)
			}

			if !bytes.Equal(out.Bytes(), wantBytes) {
				t.Fatalf("bytes mismatch:\n got %q\nwant %q", out.Bytes(), wantBytes)
			}

			assertTransformStatsEqual(t, "stream_vs_oracle", &gotRes, &wantRes)
		})
	}
}
