package fileops //nolint:cyclop // Many focused TransformLines tests marginally exceed package-average cyclop.

import (
	"bytes"
	"testing"
)

func TestTransformLinesNoOp(t *testing.T) {
	t.Parallel()

	lines := []Line{
		{Content: []byte("a"), Terminator: []byte{'\n'}},
	}
	out, res := TransformLines(lines, TransformOptions{})
	if len(out) != 1 || !bytes.Equal(out[0].Content, lines[0].Content) {
		t.Fatalf("expected original preserved: %#v", out)
	}

	if !res.Skipped || res.SkipReason != transformSkipReasonNoTransform {
		t.Fatalf("expected skipped no transform: %#v", res)
	}

	if &out[0] != &lines[0] {
		t.Fatal("no-op should return the original slice")
	}
}

func TestTransformLinesCRLFToLF(t *testing.T) {
	t.Parallel()

	lf := TargetLF
	lines := []Line{
		{Content: []byte("a"), Terminator: []byte{'\r', '\n'}},
		{Content: []byte("b"), Terminator: []byte{'\r', '\n'}},
	}
	out, res := TransformLines(lines, TransformOptions{LineEndings: &lf})
	if res.EndingsChanged != 2 {
		t.Fatalf("endings changed: %d", res.EndingsChanged)
	}

	if !bytes.Equal(out[0].Terminator, []byte{'\n'}) || !bytes.Equal(out[1].Terminator, []byte{'\n'}) {
		t.Fatalf("terminators: %#v %#v", out[0].Terminator, out[1].Terminator)
	}
}

func TestTransformLinesPreservesNilTerminatorWithoutFinalNewline(t *testing.T) {
	t.Parallel()

	lf := TargetLF
	lines := []Line{
		{Content: []byte("only"), Terminator: nil},
	}
	out, res := TransformLines(lines, TransformOptions{LineEndings: &lf})
	if res.EndingsChanged != 0 {
		t.Fatalf("expected 0 ending changes, got %d", res.EndingsChanged)
	}

	if out[0].Terminator != nil {
		t.Fatalf("want nil terminator got %q", out[0].Terminator)
	}
}

func TestTransformLinesTrimTrailing(t *testing.T) {
	t.Parallel()

	lines := []Line{
		{Content: []byte("a  "), Terminator: []byte{'\n'}},
		{Content: []byte("b\t"), Terminator: []byte{'\n'}},
	}
	out, res := TransformLines(lines, TransformOptions{TrimTrailing: true})
	if res.TrailingTrimmed != 2 {
		t.Fatalf("trimmed count: %d", res.TrailingTrimmed)
	}

	if string(out[0].Content) != "a" || string(out[1].Content) != "b" {
		t.Fatalf("content: %q %q", out[0].Content, out[1].Content)
	}
}

func TestTransformLinesFinalNewlineDefaultLF(t *testing.T) {
	t.Parallel()

	lines := []Line{
		{Content: []byte("x"), Terminator: nil},
	}
	out, res := TransformLines(lines, TransformOptions{FinalNewline: true})
	if !res.FinalNewlineAdded {
		t.Fatal("expected final newline added")
	}

	if !bytes.Equal(out[0].Terminator, []byte{'\n'}) {
		t.Fatalf("terminator %q", out[0].Terminator)
	}
}

func TestTransformLinesFinalNewlineUsesLineEndingTarget(t *testing.T) {
	t.Parallel()

	crlf := TargetCRLF
	lines := []Line{
		{Content: []byte("x"), Terminator: nil},
	}
	out, res := TransformLines(lines, TransformOptions{FinalNewline: true, LineEndings: &crlf})
	if !res.FinalNewlineAdded {
		t.Fatal("expected final newline added")
	}

	if !bytes.Equal(out[0].Terminator, []byte{'\r', '\n'}) {
		t.Fatalf("terminator %q", out[0].Terminator)
	}
}

func TestTransformLinesFinalNewlineUsesCRTarget(t *testing.T) {
	t.Parallel()

	cr := TargetCR
	lines := []Line{
		{Content: []byte("x"), Terminator: nil},
	}
	out, res := TransformLines(lines, TransformOptions{FinalNewline: true, LineEndings: &cr})
	if !res.FinalNewlineAdded {
		t.Fatal("expected final line terminator added")
	}

	if !bytes.Equal(out[0].Terminator, []byte{'\r'}) {
		t.Fatalf("terminator %q", out[0].Terminator)
	}
}

func TestTransformLinesLFToCR(t *testing.T) {
	t.Parallel()

	cr := TargetCR
	lines := []Line{
		{Content: []byte("a"), Terminator: []byte{'\n'}},
		{Content: []byte("b"), Terminator: []byte{'\n'}},
	}
	out, res := TransformLines(lines, TransformOptions{LineEndings: &cr})
	if res.EndingsChanged != 2 {
		t.Fatalf("endings changed: %d", res.EndingsChanged)
	}

	for i, ln := range out {
		if !bytes.Equal(ln.Terminator, []byte{'\r'}) {
			t.Fatalf("line %d terminator %q", i, ln.Terminator)
		}
	}
}

func TestTransformLinesCRLFToCR(t *testing.T) {
	t.Parallel()

	cr := TargetCR
	lines := []Line{
		{Content: []byte("a"), Terminator: []byte{'\r', '\n'}},
	}
	out, res := TransformLines(lines, TransformOptions{LineEndings: &cr})
	if res.EndingsChanged != 1 {
		t.Fatalf("want 1 ending changed, got %d", res.EndingsChanged)
	}

	if !bytes.Equal(out[0].Terminator, []byte{'\r'}) {
		t.Fatalf("terminator %q", out[0].Terminator)
	}
}

func TestTransformLinesMixedEndingsToCR(t *testing.T) {
	t.Parallel()

	cr := TargetCR
	lines := []Line{
		{Content: []byte("a"), Terminator: []byte{'\r', '\n'}},
		{Content: []byte("b"), Terminator: []byte{'\n'}},
		{Content: []byte("c"), Terminator: []byte{'\r'}},
	}
	out, res := TransformLines(lines, TransformOptions{LineEndings: &cr})
	if res.EndingsChanged != 2 {
		t.Fatalf("want 2 endings changed (CRLF and LF), got %d", res.EndingsChanged)
	}
	for _, ln := range out {
		if ln.Terminator != nil && !bytes.Equal(ln.Terminator, []byte{'\r'}) {
			t.Fatalf("want CR terminator, got %q", ln.Terminator)
		}
	}
}

func TestTransformLinesFinalNewlineNoOpWhenAlreadyPresent(t *testing.T) {
	t.Parallel()

	lines := []Line{
		{Content: []byte("x"), Terminator: []byte{'\n'}},
	}
	out, res := TransformLines(lines, TransformOptions{FinalNewline: true})
	if res.FinalNewlineAdded {
		t.Fatal("unexpected final newline added")
	}

	if !bytes.Equal(out[0].Terminator, []byte{'\n'}) {
		t.Fatalf("terminator %q", out[0].Terminator)
	}
}

func TestTransformLinesIdempotentBytes(t *testing.T) {
	t.Parallel()

	lf := TargetLF
	orig := []Line{
		{Content: []byte("a"), Terminator: []byte{'\r', '\n'}},
		{Content: []byte("b"), Terminator: []byte{'\n'}},
	}
	first, _ := TransformLines(orig, TransformOptions{LineEndings: &lf})
	var buf bytes.Buffer
	if err := WriteLinesTo(&buf, first); err != nil {
		t.Fatal(err)
	}
	firstBytes := buf.Bytes()

	second, _ := TransformLines(first, TransformOptions{LineEndings: &lf})
	buf.Reset()
	if err := WriteLinesTo(&buf, second); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, buf.Bytes()) {
		t.Fatalf("second pass changed bytes: %q vs %q", firstBytes, buf.Bytes())
	}
}

func TestTransformLines_LineEndingStatsMatchEndingsChanged(t *testing.T) {
	t.Parallel()

	lf := TargetLF
	lines := []Line{
		{Content: []byte("a"), Terminator: []byte{'\n'}},
		{Content: []byte("b"), Terminator: []byte{'\n'}},
		{Content: []byte("c"), Terminator: []byte{'\n'}},
		{Content: []byte("d"), Terminator: []byte{'\r'}},
		{Content: []byte("e"), Terminator: []byte{'\r'}},
		{Content: []byte("f"), Terminator: []byte{'\r', '\n'}},
	}
	_, res := TransformLines(lines, TransformOptions{LineEndings: &lf})
	sum := res.LFConverted + res.CRConverted + res.CRLFConverted
	if res.EndingsChanged != sum {
		t.Fatalf("endings_changed %d != sum of per-type converted %d", res.EndingsChanged, sum)
	}

	if res.LFFound != 3 || res.LFConverted != 0 {
		t.Fatalf("LF: found %d converted %d", res.LFFound, res.LFConverted)
	}

	if res.CRFound != 2 || res.CRConverted != 2 {
		t.Fatalf("CR: found %d converted %d", res.CRFound, res.CRConverted)
	}

	if res.CRLFFound != 1 || res.CRLFConverted != 1 {
		t.Fatalf("CRLF: found %d converted %d", res.CRLFFound, res.CRLFConverted)
	}
}

func TestTransformLines_MixedEndingsAllNormalized(t *testing.T) {
	t.Parallel()

	lf := TargetLF
	lines := []Line{
		{Content: []byte("a"), Terminator: []byte{'\r', '\n'}},
		{Content: []byte("b"), Terminator: []byte{'\n'}},
		{Content: []byte("c"), Terminator: []byte{'\r'}},
		{Content: []byte("d"), Terminator: nil},
	}
	out, res := TransformLines(lines, TransformOptions{LineEndings: &lf})
	// Lines with CR, CRLF terminators should change; LF stays; nil stays
	if res.EndingsChanged != 2 {
		t.Fatalf("want 2 endings changed, got %d", res.EndingsChanged)
	}
	for i, ln := range out {
		if ln.Terminator != nil && !bytes.Equal(ln.Terminator, []byte{'\n'}) {
			t.Fatalf("line %d: want LF or nil, got %q", i, ln.Terminator)
		}
	}
}

func TestTransformLines_EmptyFile(t *testing.T) {
	t.Parallel()

	var lines []Line
	out, res := TransformLines(lines, TransformOptions{TrimTrailing: true, FinalNewline: true})
	if len(out) != 0 {
		t.Fatalf("want empty output for empty input, got %d lines", len(out))
	}
	if res.TrailingTrimmed != 0 {
		t.Fatalf("want 0 trimmed, got %d", res.TrailingTrimmed)
	}
}

func TestTransformLines_ContentBytesUntouched(t *testing.T) {
	t.Parallel()

	lf := TargetLF
	original := []byte("hello\tworld  ")
	lines := []Line{
		{Content: append([]byte(nil), original...), Terminator: []byte{'\r', '\n'}},
	}
	out, _ := TransformLines(lines, TransformOptions{LineEndings: &lf})
	// Content should be untouched (tabs, trailing spaces preserved) when TrimTrailing is false
	if !bytes.Equal(out[0].Content, original) {
		t.Fatalf("content modified: got %q, want %q", out[0].Content, original)
	}
}

func TestTransformLines_InvalidEndingTargetActsLikeLF(t *testing.T) {
	t.Parallel()

	bogus := LineEndingTarget(127)
	lines := []Line{
		{Content: []byte("only"), Terminator: []byte{'\r'}},
	}
	out, res := TransformLines(lines, TransformOptions{LineEndings: &bogus})
	if res.EndingsChanged != 1 {
		t.Fatalf("want one ending rewritten, got %d", res.EndingsChanged)
	}
	if !bytes.Equal(out[0].Terminator, []byte{'\n'}) {
		t.Fatalf(`terminator got %q want LF`, out[0].Terminator)
	}
}

func TestCountTransformChanges_nilResult(t *testing.T) {
	t.Parallel()

	if n := CountTransformChanges(nil); n != 0 {
		t.Fatalf("nil result: want 0, got %d", n)
	}
}

func TestCountTransformChanges_AccumulatesDistinctEdits(t *testing.T) {
	t.Parallel()

	res := TransformFileResult{
		EndingsChanged:    2,
		TrailingTrimmed:   1,
		FinalNewlineAdded: true,
	}
	if n := CountTransformChanges(&res); n != 4 {
		t.Fatalf("want 4 total edits, got %d", n)
	}
}

func TestTransformLines_UnknownTerminatorNotCountedPerTypeLFTarget(t *testing.T) {
	t.Parallel()

	lf := TargetLF
	lines := []Line{
		{Content: []byte("odd"), Terminator: []byte("xx")},
	}
	_, res := TransformLines(lines, TransformOptions{LineEndings: &lf})
	if res.LFFound != 0 || res.LFConverted != 0 {
		t.Fatalf("unknown terminator should not participate in LF stats: %#v", res)
	}
}
