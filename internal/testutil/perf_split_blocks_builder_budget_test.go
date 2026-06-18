package testutil_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/testutil"
)

func TestStreamingBodyResidencyRetainedHeapBudget_NegativeSmallDeltaUsesZeroBaseline(t *testing.T) {
	t.Parallel()

	const noise = int64(testutil.StreamingBodyResidencyRetainedHeapNoiseAllowance)
	const smallDelta = int64(-50_000)

	got := testutil.StreamingBodyResidencyRetainedHeapBudget(smallDelta)
	if got != noise {
		t.Fatalf("budget: got %d want noise-only baseline %d (ratio term must be zero)", got, noise)
	}
}

func TestStreamingBodyResidencyRetainedHeapBudget_PositiveDeltaScalesByRatioPlusNoise(t *testing.T) {
	t.Parallel()

	const smallDelta = int64(128)
	const ratio = int64(testutil.StreamingBodyRetainedHeapMaxLargeToSmallRatio)
	const noise = int64(testutil.StreamingBodyResidencyRetainedHeapNoiseAllowance)

	want := ratio*smallDelta + noise
	if got := testutil.StreamingBodyResidencyRetainedHeapBudget(smallDelta); got != want {
		t.Fatalf("budget: got %d want %d (ratio*delta + noise)", got, want)
	}
}

func TestBuildMaxFilesExceededSplitPrefix_PanicOnEmptyDelimiter(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty delimLine")
		} else if msg, ok := r.(string); !ok || msg != "BuildMaxFilesExceededSplitPrefix: empty delimLine" {
			t.Fatalf("unexpected panic value: %v", r)
		}
	}()

	_ = testutil.BuildMaxFilesExceededSplitPrefix(byte('.'), "", 3)
}

func TestBuildMaxFilesExceededSplitPrefix_PanicOnNonPositiveMaxAllow(t *testing.T) {
	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		t.Parallel()

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for maxAllow=0")
			} else if msg, ok := r.(string); !ok ||
				msg != "BuildMaxFilesExceededSplitPrefix: maxAllow must be positive" {
				t.Fatalf("unexpected panic value: %v", r)
			}
		}()

		_ = testutil.BuildMaxFilesExceededSplitPrefix(byte('.'), "---\n", 0)
	})

	t.Run("negative", func(t *testing.T) {
		t.Parallel()

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for negative maxAllow")
			} else if msg, ok := r.(string); !ok ||
				msg != "BuildMaxFilesExceededSplitPrefix: maxAllow must be positive" {
				t.Fatalf("unexpected panic value: %v", r)
			}
		}()

		_ = testutil.BuildMaxFilesExceededSplitPrefix(byte('.'), "---\n", -1)
	})
}

func TestBuildMaxFilesExceededSplitPrefix_Structure(t *testing.T) {
	t.Parallel()

	const padByte = byte('@')
	const delim = "---\n"
	const maxAllow = 4

	prefix := testutil.BuildMaxFilesExceededSplitPrefix(padByte, delim, maxAllow)

	padUnit := []byte{padByte, '\n'}
	repeat := testutil.BoundednessBinaryCheckReadWindow/len(padUnit) + 1
	wantHeadLen := repeat * len(padUnit)
	if len(prefix) < wantHeadLen {
		t.Fatalf("prefix shorter than padded head: len=%d want>=%d", len(prefix), wantHeadLen)
	}

	head := prefix[:wantHeadLen]
	if bytes.Contains(head, []byte(strings.TrimSuffix(delim, "\n"))) {
		t.Fatalf("delimiter material must not appear in binary preamble window head")
	}

	if !bytes.HasPrefix(prefix[wantHeadLen:], []byte(delim)) {
		t.Fatalf("delimiter section must begin immediately after padded head")
	}

	delimCount := bytes.Count(prefix, []byte(delim))
	wantDelims := maxAllow + 1
	if delimCount != wantDelims {
		t.Fatalf("delimiter line occurrences: got %d want %d", delimCount, wantDelims)
	}

	if prefix[len(prefix)-1] != '\n' {
		t.Fatalf("fixture must end with a newline (last section body line)")
	}
}

func TestBuildMaxFilesExceededBlocksPrefix_PanicOnSmallBlocksCount(t *testing.T) {
	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		t.Parallel()

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for blocksCount=0")
			} else if msg, ok := r.(string); !ok ||
				msg != "BuildMaxFilesExceededBlocksPrefix: blocksCount must be >= 2" {
				t.Fatalf("unexpected panic value: %v", r)
			}
		}()

		_ = testutil.BuildMaxFilesExceededBlocksPrefix(
			byte('.'),
			"",
			"{", "b", "}",
			0,
		)
	})

	t.Run("one", func(t *testing.T) {
		t.Parallel()

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for blocksCount=1")
			} else if msg, ok := r.(string); !ok ||
				msg != "BuildMaxFilesExceededBlocksPrefix: blocksCount must be >= 2" {
				t.Fatalf("unexpected panic value: %v", r)
			}
		}()

		_ = testutil.BuildMaxFilesExceededBlocksPrefix(
			byte('.'),
			"",
			"{", "b", "}",
			1,
		)
	})
}

func TestBuildMaxFilesExceededBlocksPrefix_NewlineHygieneTable(t *testing.T) {
	t.Parallel()

	const padByte = byte('#')
	const blocksCount = 2

	padUnit := []byte{padByte, '\n'}
	repeat := testutil.BoundednessBinaryCheckReadWindow/len(padUnit) + 1
	headLen := repeat * len(padUnit)

	cases := []struct {
		name       string
		headerLine string
		beginLine  string
		bodyLine   string
		endLine    string
		wantTail   string
	}{
		{
			name:       "all_lines_already_newline_terminated",
			headerLine: "H\n",
			beginLine:  "B\n",
			bodyLine:   "L\n",
			endLine:    "E\n",
			wantTail: "H\n" +
				"B\nL\nE\n" +
				"B\nL\nE\n",
		},
		{
			name:       "no_input_lines_terminated_appends_single_newlines",
			headerLine: "H",
			beginLine:  "B",
			bodyLine:   "L",
			endLine:    "E",
			wantTail: "H\n" +
				"B\nL\nE\n" +
				"B\nL\nE\n",
		},
		{
			name:       "empty_strings_still_emit_record_separating_newlines",
			headerLine: "",
			beginLine:  "",
			bodyLine:   "x",
			endLine:    "}",
			wantTail:   "\n" + "x\n}\n" + "\n" + "x\n}\n",
		},
		{
			name:       "mismatched_termination_mixed",
			headerLine: "HDR",
			beginLine:  "{\n",
			bodyLine:   "mid",
			endLine:    "END\n",
			wantTail: "HDR\n" +
				"{\nmid\nEND\n" +
				"{\nmid\nEND\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := testutil.BuildMaxFilesExceededBlocksPrefix(
				padByte,
				tc.headerLine,
				tc.beginLine,
				tc.bodyLine,
				tc.endLine,
				blocksCount,
			)
			if len(got) < headLen {
				t.Fatalf("output shorter than head: len=%d want>=%d", len(got), headLen)
			}

			tail := string(got[headLen:])
			if tail != tc.wantTail {
				t.Fatalf("tail mismatch:\ngot:  %q\nwant: %q", tail, tc.wantTail)
			}
		})
	}
}

func TestBuildLargeSplitSingleSectionSource_DeterministicShape(t *testing.T) {
	t.Parallel()

	const lineCount = 12
	const lineLen = 5
	delim := []byte("---\n")

	got := testutil.BuildLargeSplitSingleSectionSource(lineCount, lineLen, delim)

	if !bytes.HasPrefix(got, delim) {
		t.Fatalf("must begin with delim prefix %q", delim)
	}

	core := bytes.Repeat([]byte{'_'}, lineLen)
	body := got[len(delim):]
	wantLine := append(append([]byte(nil), core...), '\n')
	for i := range lineCount {
		off := i * len(wantLine)
		slice := body[off : off+len(wantLine)]
		if !bytes.Equal(slice, wantLine) {
			t.Fatalf("line %d: got %q want %q", i, slice, wantLine)
		}
	}

	if len(body) != lineCount*len(wantLine) {
		t.Fatalf("body len: got %d want %d", len(body), lineCount*len(wantLine))
	}

	if got[len(got)-1] != '\n' {
		t.Fatalf("must end with newline")
	}
}

func TestBuildLargeBlocksSingleBodySource_DeterministicShape(t *testing.T) {
	t.Parallel()

	const lineCount = 7
	const lineLen = 4

	header := []byte("// head\n")
	begin := []byte("```\n")
	end := []byte("```")

	got := testutil.BuildLargeBlocksSingleBodySource(header, begin, end, lineCount, lineLen)

	if !bytes.HasPrefix(got, header) {
		t.Fatalf("must begin with header %q", header)
	}
	rest := got[len(header):]
	if !bytes.HasPrefix(rest, begin) {
		t.Fatalf("after header must be begin %q", begin)
	}

	body := rest[len(begin):]
	core := bytes.Repeat([]byte{'='}, lineLen)
	wantLine := append(append([]byte(nil), core...), '\n')
	for i := range lineCount {
		off := i * len(wantLine)
		slice := body[off : off+len(wantLine)]
		if !bytes.Equal(slice, wantLine) {
			t.Fatalf("line %d: got %q want %q", i, slice, wantLine)
		}
	}

	afterLines := body[lineCount*len(wantLine):]
	if !bytes.Equal(afterLines, end) {
		t.Fatalf("trailing bytes: got %q want %q", afterLines, end)
	}
}
