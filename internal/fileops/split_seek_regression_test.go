package fileops

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// meteringReadSeeker counts Read bytes (not Seeks) for bounded-read regression tests.
type meteringReadSeeker struct {
	io.ReadSeeker
	readBytes int64
}

func (m *meteringReadSeeker) Read(p []byte) (int, error) {
	n, err := m.ReadSeeker.Read(p)

	m.readBytes += int64(n)

	return n, err
}

func TestReadSerializedSpanContentPrefix_UTF8ReadsAtMostMaxBytes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	content := strings.Repeat("z", 100_000)
	mrs := &meteringReadSeeker{ReadSeeker: bytes.NewReader([]byte(content))}

	_, err := readSerializedSpanContentPrefixUTF8(ctx, mrs, 0, uint64(len(content)), NamingMaterializationMaxBytes)
	if err != nil {
		t.Fatalf("readSerializedSpanContentPrefixUTF8: %v", err)
	}

	wantExact := int64(NamingMaterializationMaxBytes)

	if mrs.readBytes != wantExact {
		t.Fatalf("read %d bytes, want exactly %d for span longer than naming cap", mrs.readBytes, wantExact)
	}
}

func TestReadSerializedSpanContentPrefixUTF8_TrimsIncompleteLastRuneAtCap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Hiragana あ is UTF-8 {0xE3, 0x81, 0x82}. A byte cap that stops after the first two bytes reads an
	// incomplete code point; naming must not retain those units or introduce replacement characters.
	repeat := strings.Repeat("a", 8)
	jp := "\u3042"
	content := repeat + jp
	maxBytes := uint64(len(repeat)) + 2

	mrs := &meteringReadSeeker{ReadSeeker: bytes.NewReader([]byte(content))}

	got, err := readSerializedSpanContentPrefixUTF8(ctx, mrs, 0, uint64(len(content)), maxBytes)
	if err != nil {
		t.Fatalf("readSerializedSpanContentPrefixUTF8: %v", err)
	}

	wantRawReadBytes := int64(len(repeat)) + 2
	if mrs.readBytes != wantRawReadBytes {
		t.Fatalf("read %d raw bytes, want %d (cap unchanged; trim is in-memory only)", mrs.readBytes, wantRawReadBytes)
	}

	if got != repeat {
		t.Fatalf("got %q want %q", got, repeat)
	}

	if !utf8.ValidString(got) {
		t.Fatalf("result not valid UTF-8: %#v", []byte(got))
	}

	if strings.ContainsRune(got, '\uFFFD') {
		t.Fatalf("replacement rune must not appear in %q", got)
	}
}

func TestSeekableSplitScan_MultiChunkLongPreamble_MultipleDelimiters_NamesMatchSplit(t *testing.T) {
	t.Parallel()

	re := regexp.MustCompile(`^##\s`)

	longPreambleLine := strings.Repeat("x", 70*1024) + "\n"
	src := longPreambleLine +
		"## FIRST\ninner1\n" +
		"## SECOND\ninner2\n" +
		"tail\n"

	scanOpts := SplitOptions{
		Source:         strings.NewReader(src),
		Delimiter:      re,
		Naming:         FromDelimiter,
		Extension:      ".txt",
		StripDelimiter: false,
	}

	got, err := ScanSplitSectionsMeta(context.Background(), scanOpts, BoundedScanLimits{})
	if err != nil {
		t.Fatalf("ScanSplitSectionsMeta: %v", err)
	}

	full, err := Split(context.Background(), scanOpts)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	if len(got.Sections) != len(full.Sections) {
		t.Fatalf("section count meta %d split %d", len(got.Sections), len(full.Sections))
	}

	for i := range full.Sections {
		if got.Sections[i].Name != full.Sections[i].Name {
			t.Fatalf("name[%d]: meta %q split %q", i, got.Sections[i].Name, full.Sections[i].Name)
		}
	}
}

func TestSeekableBlocksScan_MultiChunkInnerLines_FromDelimiterMatchesExtract(t *testing.T) {
	t.Parallel()

	startRE := regexp.MustCompile("^```py")
	endRE := regexp.MustCompile("^```$")

	hugeInner := strings.Repeat("y", 70*1024) + "\n"

	src := "```py\n" + hugeInner + "inner-tail\n```\noutro\n"

	opts := BlocksOptions{
		Source:         strings.NewReader(src),
		StartDelimiter: startRE,
		EndDelimiter:   endRE,
		Naming:         FromDelimiter,
		Extension:      ".txt",
	}

	got, err := ScanBlocksMeta(context.Background(), opts, BoundedScanLimits{})
	if err != nil {
		t.Fatalf("ScanBlocksMeta: %v", err)
	}

	ex, err := ExtractBlocks(context.Background(), BlocksOptions{
		Source:         strings.NewReader(src),
		StartDelimiter: startRE,
		EndDelimiter:   endRE,
		Naming:         FromDelimiter,
		Extension:      ".txt",
	})
	if err != nil {
		t.Fatalf("ExtractBlocks: %v", err)
	}

	if len(got.Metas) != len(ex.Blocks) {
		t.Fatalf("meta %d blocks %d", len(got.Metas), len(ex.Blocks))
	}

	for i := range ex.Blocks {
		if got.Metas[i].Name != ex.Blocks[i].Name {
			t.Fatalf("name[%d]: meta %q extract %q", i, got.Metas[i].Name, ex.Blocks[i].Name)
		}
	}
}

func TestSeekableBlocksScan_FromContentHugeFirstInner_LineMatchesExtract(t *testing.T) {
	t.Parallel()

	startRE := regexp.MustCompile("^```gherkin")
	endRE := regexp.MustCompile("^```$")

	firstInnerBody := strings.Repeat("w", 200*1024) + "\n"

	src := "```gherkin\n" + firstInnerBody + "```\n"

	opts := BlocksOptions{
		Source:         strings.NewReader(src),
		StartDelimiter: startRE,
		EndDelimiter:   endRE,
		Naming:         FromContent,
		Extension:      ".feat",
	}

	got, err := ScanBlocksMeta(context.Background(), opts, BoundedScanLimits{})
	if err != nil {
		t.Fatalf("ScanBlocksMeta: %v", err)
	}

	ex, err := ExtractBlocks(context.Background(), BlocksOptions{
		Source:         strings.NewReader(src),
		StartDelimiter: startRE,
		EndDelimiter:   endRE,
		Naming:         FromContent,
		Extension:      ".feat",
	})
	if err != nil {
		t.Fatalf("ExtractBlocks: %v", err)
	}

	if len(got.Metas) != len(ex.Blocks) {
		t.Fatalf("meta %d blocks %d", len(got.Metas), len(ex.Blocks))
	}

	if len(ex.Blocks) != 1 {
		t.Fatalf("want 1 block got %d", len(ex.Blocks))
	}

	if got.Metas[0].Name != ex.Blocks[0].Name {
		t.Fatalf("name meta %q extract %q", got.Metas[0].Name, ex.Blocks[0].Name)
	}
}

func TestSeekableSplitScan_FromContent_DelimMatchAfterNamingPrefix_UsesMatchEndSuffix(t *testing.T) {
	t.Parallel()

	// First 8192 bytes are 'x' only; delimiter `## ` appears after NamingMaterializationMaxBytes.
	// FromContent must use match-end-relative suffix on the seekable span, not FindStringIndex on the
	// capped prefix string.
	re := regexp.MustCompile(`##\s`)

	pad := int(NamingMaterializationMaxBytes) + 400
	longDelimLine := strings.Repeat("x", pad) + "## hello\n"
	src := longDelimLine +
		"inner\n" +
		"## s2\n" +
		"tail\n"

	scanOpts := SplitOptions{
		Source:         strings.NewReader(src),
		Delimiter:      re,
		Naming:         FromContent,
		Extension:      ".txt",
		StripDelimiter: false,
	}

	got, err := ScanSplitSectionsMeta(context.Background(), scanOpts, BoundedScanLimits{})
	if err != nil {
		t.Fatalf("ScanSplitSectionsMeta: %v", err)
	}

	full, err := Split(context.Background(), scanOpts)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	if len(got.Sections) != len(full.Sections) {
		t.Fatalf("section count meta %d split %d", len(got.Sections), len(full.Sections))
	}

	for i := range full.Sections {
		if got.Sections[i].Name != full.Sections[i].Name {
			t.Fatalf("name[%d]: meta %q split %q", i, got.Sections[i].Name, full.Sections[i].Name)
		}
	}
}

func TestSeekableSplitScan_FromContentHugeSectionBody_NamesMatchSplit(t *testing.T) {
	t.Parallel()

	re := regexp.MustCompile(`^##\s`)

	body := strings.Repeat("v", 200*1024)

	src := "## delim\n" + body + "\n## next\nfinal\n"

	scanOpts := SplitOptions{
		Source:         strings.NewReader(src),
		Delimiter:      re,
		Naming:         FromContent,
		Extension:      ".txt",
		StripDelimiter: false,
	}

	got, err := ScanSplitSectionsMeta(context.Background(), scanOpts, BoundedScanLimits{})
	if err != nil {
		t.Fatalf("ScanSplitSectionsMeta: %v", err)
	}

	full, err := Split(context.Background(), scanOpts)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	if len(got.Sections) != len(full.Sections) {
		t.Fatalf("section count meta %d split %d", len(got.Sections), len(full.Sections))
	}

	for i := range full.Sections {
		if got.Sections[i].Name != full.Sections[i].Name {
			t.Fatalf("name[%d]: meta %q split %q", i, got.Sections[i].Name, full.Sections[i].Name)
		}
	}
}

//nolint:varnamelen // short names for paired regression rows (tiny vs default chunk scan)
func TestScanLineSpans_TinyChunks_IsolatedCallbackSHA256_DoesNotCorruptNextLine(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	opts := linespanScanOptions{chunkSizeBytes: 64}

	raw := []byte("aa\nbb\ncc\n")

	r := bytes.NewReader(raw)

	var gotNums []int

	err := scanLineSpansWithOptions(ctx, r, func(sp LineSpan) error {
		return isolateMatchLineSpanOnSharedSeeker(r, sp, func() error {
			gotNums = append(gotNums, sp.LineNum)

			if sp.LineNum == 1 {
				end := sp.SerializedEnd

				_, he := SHA256SerializedByteSpan(ctx, r, 0, end)
				if he != nil {
					return he
				}
			}

			return nil
		})
	}, opts)
	if err != nil {
		t.Fatalf("scanLineSpansWithOptions: %v", err)
	}

	want := []int{1, 2, 3}

	if len(gotNums) != len(want) {
		t.Fatalf("got %v line nums, want %v", gotNums, want)
	}

	for i := range want {
		if gotNums[i] != want[i] {
			t.Fatalf("line num idx %d: got %d want %d", i, gotNums[i], want[i])
		}
	}
}

//nolint:varnamelen // metering wrapper field wiring is clearer with short handle
func TestLineSpanNamingContentUTF8_MeteringReadsAtMostNamingCap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	content := strings.Repeat("n", 100_000)
	raw := []byte(content + "\n")

	span := LineSpan{
		LineNum:         1,
		SerializedStart: 0,
		ContentStart:    0,
		ContentEnd:      uint64(len(content)),
		SerializedEnd:   uint64(len(raw)),
		Terminator:      LineTerminatorLF,
	}

	m := &meteringReadSeeker{ReadSeeker: bytes.NewReader(raw)}

	_, err := lineSpanNamingContentUTF8(ctx, m, span)
	if err != nil {
		t.Fatalf("lineSpanNamingContentUTF8: %v", err)
	}

	want := int64(NamingMaterializationMaxBytes)

	if m.readBytes != want {
		t.Fatalf("read %d bytes through naming helper, want %d (capped prefix)", m.readBytes, want)
	}
}

//nolint:varnamelen // row compare loop uses tiny vs default aliases
func TestSeekableSplitScan_TinyChunks_MultipleDelimiters_FromContent_MatchesDefaultChunkMeta(t *testing.T) {
	t.Parallel()

	re := regexp.MustCompile(`^##\s`)

	inner := strings.Repeat("p", 500) + "\n"
	src := "## A\n" + inner + "## B\n" + "tail\n"

	opts := SplitOptions{
		Source:         bytes.NewReader([]byte(src)),
		Delimiter:      re,
		Naming:         FromContent,
		Extension:      ".txt",
		StripDelimiter: false,
	}

	ctx := context.Background()

	gotTiny, err := scanSplitSectionsMetaSeekable(
		ctx,
		bytes.NewReader([]byte(src)),
		opts,
		BoundedScanLimits{},
		linespanScanOptions{chunkSizeBytes: 64},
	)
	if err != nil {
		t.Fatalf("scan tiny chunk: %v", err)
	}

	gotDef, err := scanSplitSectionsMetaSeekable(
		ctx,
		bytes.NewReader([]byte(src)),
		opts,
		BoundedScanLimits{},
		linespanScanOptions{},
	)
	if err != nil {
		t.Fatalf("scan default chunk: %v", err)
	}

	if len(gotTiny.Sections) != len(gotDef.Sections) {
		t.Fatalf("section count tiny %d default %d", len(gotTiny.Sections), len(gotDef.Sections))
	}

	for i := range gotDef.Sections {
		a, b := gotTiny.Sections[i], gotDef.Sections[i]
		if a.Name != b.Name {
			t.Fatalf("name[%d]: tiny %q default %q", i, a.Name, b.Name)
		}

		if a.SpanFingerprintSHA256 != b.SpanFingerprintSHA256 {
			t.Fatalf("fingerprint[%d]: differs", i)
		}

		if a.OutputStartLineNum != b.OutputStartLineNum || a.OutputEndLineNum != b.OutputEndLineNum {
			t.Fatalf("lines[%d]: tiny %d-%d default %d-%d",
				i, a.OutputStartLineNum, a.OutputEndLineNum, b.OutputStartLineNum, b.OutputEndLineNum)
		}
	}
}

//nolint:varnamelen // row compare loop uses tiny vs default aliases
func TestSeekableBlocksScan_TinyChunks_TwoBlocks_FromDelimiter_MatchesDefaultChunkMeta(t *testing.T) {
	t.Parallel()

	startRE := regexp.MustCompile("^```py")
	endRE := regexp.MustCompile("^```$")

	inner1 := strings.Repeat("q", 400) + "\n"
	inner2 := "small\n"

	src := "```py\n" + inner1 + "```\n" + "```py\n" + inner2 + "```\n" + "done\n"

	opts := BlocksOptions{
		Source:         bytes.NewReader([]byte(src)),
		StartDelimiter: startRE,
		EndDelimiter:   endRE,
		Naming:         FromDelimiter,
		Extension:      ".txt",
	}

	ctx := context.Background()

	gotTiny, err := scanBlocksMetaSeekable(
		ctx,
		bytes.NewReader([]byte(src)),
		opts,
		BoundedScanLimits{},
		linespanScanOptions{chunkSizeBytes: 64},
	)
	if err != nil {
		t.Fatalf("scan tiny chunk: %v", err)
	}

	gotDef, err := scanBlocksMetaSeekable(
		ctx,
		bytes.NewReader([]byte(src)),
		opts,
		BoundedScanLimits{},
		linespanScanOptions{},
	)
	if err != nil {
		t.Fatalf("scan default chunk: %v", err)
	}

	if len(gotTiny.Metas) != len(gotDef.Metas) {
		t.Fatalf("meta count tiny %d default %d", len(gotTiny.Metas), len(gotDef.Metas))
	}

	for i := range gotDef.Metas {
		a, b := gotTiny.Metas[i], gotDef.Metas[i]
		if a.Name != b.Name {
			t.Fatalf("name[%d]: tiny %q default %q", i, a.Name, b.Name)
		}

		if a.SpanFingerprintSHA256 != b.SpanFingerprintSHA256 {
			t.Fatalf("fingerprint[%d]: differs", i)
		}

		if a.StartDelimLineNum != b.StartDelimLineNum || a.EndDelimLineNum != b.EndDelimLineNum {
			t.Fatalf("delim lines[%d]: differ", i)
		}
	}
}
