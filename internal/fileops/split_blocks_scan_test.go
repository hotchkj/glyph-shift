//nolint:revive // File-length: related split scan cases and fixtures stay in one test module.
package fileops_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

type countReadSeeker struct {
	r io.ReadSeeker
	n int64
}

func (c *countReadSeeker) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)

	return n, err
}

func (c *countReadSeeker) Seek(offset int64, whence int) (int64, error) {
	return c.r.Seek(offset, whence)
}

type readOnlySource struct {
	r io.Reader
}

func (s readOnlySource) Read(p []byte) (int, error) {
	return s.r.Read(p)
}

func TestScanSplitSectionsMeta_matchesSplitOutputShape(t *testing.T) {
	t.Parallel()

	src := "Preamble text\n## Feature: Login\nL1\n## Feature: Signup\nL2\n## Feature: Profile\nL3\n"
	re := regexp.MustCompile(`^##\sFeature:`)

	scanOpts := fileops.SplitOptions{
		Source:    strings.NewReader(src),
		Delimiter: re,
		Naming:    fileops.Sequential,
		Extension: ".txt",
	}

	scan, err := fileops.ScanSplitSectionsMeta(context.Background(), scanOpts, fileops.BoundedScanLimits{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	splitOpts := fileops.SplitOptions{
		Source:    strings.NewReader(src),
		Delimiter: re,
		Naming:    fileops.Sequential,
		Extension: ".txt",
	}

	full, err := fileops.Split(context.Background(), splitOpts)
	if err != nil {
		t.Fatalf("split: %v", err)
	}

	if scan.DelimiterLineCount != 3 {
		t.Fatalf("delim lines: want 3 got %d", scan.DelimiterLineCount)
	}

	if scan.OutputSectionCount != len(full.Sections) {
		t.Fatalf("sections: scan %d split %d", scan.OutputSectionCount, len(full.Sections))
	}

	if len(scan.Sections) != len(full.Sections) {
		t.Fatalf("meta len: scan %d split %d", len(scan.Sections), len(full.Sections))
	}

	for secIdx := range full.Sections {
		if scan.Sections[secIdx].Name != full.Sections[secIdx].Name {
			t.Fatalf("name[%d]: scan %q split %q", secIdx, scan.Sections[secIdx].Name, full.Sections[secIdx].Name)
		}

		if scan.Sections[secIdx].ContentLineCount != len(full.Sections[secIdx].Lines) {
			t.Fatalf("lines[%d]: scan %d split %d",
				secIdx, scan.Sections[secIdx].ContentLineCount, len(full.Sections[secIdx].Lines))
		}

		if scan.Sections[secIdx].StripDelimiterInOutput != scanOpts.StripDelimiter {
			t.Fatalf("strip flag[%d]", secIdx)
		}
	}
}

func TestScanSplitSectionsMeta_rejectsNonSeekableSource(t *testing.T) {
	t.Parallel()

	opts := fileops.SplitOptions{
		Source:    readOnlySource{r: strings.NewReader("## A\nbody\n")},
		Delimiter: regexp.MustCompile(`^##\s`),
		Naming:    fileops.Sequential,
		Extension: ".txt",
	}

	_, err := fileops.ScanSplitSectionsMeta(context.Background(), opts, fileops.BoundedScanLimits{})
	if !errors.Is(err, fileops.ErrSeekableSourceRequired) {
		t.Fatalf("want ErrSeekableSourceRequired, got %v", err)
	}
}

func TestScanSplitSectionsMeta_stripDelimiterParity(t *testing.T) {
	t.Parallel()

	scanOpts := fileops.SplitOptions{
		Source:         strings.NewReader("## A\nBody\n"),
		Delimiter:      regexp.MustCompile(`^##\s`),
		Naming:         fileops.Sequential,
		Extension:      ".txt",
		StripDelimiter: true,
	}

	scan, err := fileops.ScanSplitSectionsMeta(context.Background(), scanOpts, fileops.BoundedScanLimits{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	splitOpts := fileops.SplitOptions{
		Source:         strings.NewReader("## A\nBody\n"),
		Delimiter:      regexp.MustCompile(`^##\s`),
		Naming:         fileops.Sequential,
		Extension:      ".txt",
		StripDelimiter: true,
	}

	full, err := fileops.Split(context.Background(), splitOpts)
	if err != nil {
		t.Fatalf("split: %v", err)
	}

	if scan.OutputSectionCount != len(full.Sections) {
		t.Fatalf("counts: scan %d split %d", scan.OutputSectionCount, len(full.Sections))
	}

	if scan.Sections[0].ContentLineCount != 1 {
		t.Fatalf("want 1 output line got %d", scan.Sections[0].ContentLineCount)
	}
}

func TestScanSplitSectionsMeta_maxFilesStopsEarly(t *testing.T) {
	t.Parallel()

	src := "p1\np2\n## A\nx\n## B\ny\n"

	tail := strings.Repeat("Z", 512*1024)

	full := strings.NewReader(src + tail)

	cr := &countReadSeeker{r: full}

	opts := fileops.SplitOptions{
		Source:    cr,
		Delimiter: regexp.MustCompile(`^##\s`),
		Naming:    fileops.Sequential,
		Extension: ".txt",
	}

	_, err := fileops.ScanSplitSectionsMeta(context.Background(), opts, fileops.BoundedScanLimits{MaxFiles: 2})
	if err == nil {
		t.Fatal("want error")
	}

	if !errors.Is(err, fileops.ErrMaxFilesExceeded) {
		t.Fatalf("want ErrMaxFilesExceeded, got %v", err)
	}

	var mfd *fileops.MaxFilesExceededDetailError
	if !errors.As(err, &mfd) {
		t.Fatalf("want *MaxFilesExceededDetailError, got %v", err)
	}
	if mfd.MaxFiles != 2 || mfd.WouldCreateCount != 3 {
		t.Fatalf("detail: max_files=%d would_create=%d want 2 and 3", mfd.MaxFiles, mfd.WouldCreateCount)
	}

	if cr.n >= int64(len(src)+len(tail)) {
		t.Fatalf("expected early stop: read %d of %d", cr.n, len(src)+len(tail))
	}
}

func TestScanSplitSectionsMeta_maxFilesSeekableCarriesDetail(t *testing.T) {
	t.Parallel()

	src := "p1\np2\n## A\nx\n## B\ny\n## C\nz\n"

	opts := fileops.SplitOptions{
		Source:    bytes.NewReader([]byte(src)),
		Delimiter: regexp.MustCompile(`^##\s`),
		Naming:    fileops.Sequential,
		Extension: ".txt",
	}

	_, err := fileops.ScanSplitSectionsMeta(context.Background(), opts, fileops.BoundedScanLimits{MaxFiles: 2})
	if err == nil {
		t.Fatal("want error")
	}
	if !errors.Is(err, fileops.ErrMaxFilesExceeded) {
		t.Fatalf("want ErrMaxFilesExceeded, got %v", err)
	}
	var mfd *fileops.MaxFilesExceededDetailError
	if !errors.As(err, &mfd) {
		t.Fatalf("want *MaxFilesExceededDetailError, got %v", err)
	}
	if mfd.MaxFiles != 2 || mfd.WouldCreateCount != 3 {
		t.Fatalf("detail: max_files=%d would_create=%d want 2 and 3", mfd.MaxFiles, mfd.WouldCreateCount)
	}
}

func TestScanSplitSectionsMeta_preservesCRLFByteSpanRoundTrip(t *testing.T) {
	t.Parallel()

	raw := []byte("## A\r\ntext\r\n")

	opts := fileops.SplitOptions{
		Source:    bytes.NewReader(raw),
		Delimiter: regexp.MustCompile("^##"),
		Naming:    fileops.Sequential,
		Extension: ".txt",
	}

	scan, err := fileops.ScanSplitSectionsMeta(context.Background(), opts, fileops.BoundedScanLimits{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(scan.Sections) != 1 {
		t.Fatalf("sections: %d", len(scan.Sections))
	}

	sl := scan.Sections[0]

	snippet := raw[sl.ByteSpanStart:sl.ByteSpanEndExclusive]

	var buf bytes.Buffer

	if err := fileops.WriteLinesTo(&buf, []fileops.Line{{Content: snippet, Terminator: nil}}); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(buf.Bytes(), []byte("## A\r\ntext\r\n")) {
		t.Fatalf("want CRLF round trip, got %q", buf.Bytes())
	}
}

func assertSplitScanNamesMatchSplit(
	t *testing.T,
	src string,
	re *regexp.Regexp,
	naming fileops.NamingStrategy,
	ext string,
	strip bool,
) {
	t.Helper()

	scan, err := fileops.ScanSplitSectionsMeta(context.Background(), fileops.SplitOptions{
		Source:         strings.NewReader(src),
		Delimiter:      re,
		Naming:         naming,
		Extension:      ext,
		StripDelimiter: strip,
	}, fileops.BoundedScanLimits{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	full, err := fileops.Split(context.Background(), fileops.SplitOptions{
		Source:         strings.NewReader(src),
		Delimiter:      re,
		Naming:         naming,
		Extension:      ext,
		StripDelimiter: strip,
	})
	if err != nil {
		t.Fatalf("split: %v", err)
	}

	if len(scan.Sections) != len(full.Sections) {
		t.Fatalf("section count: scan %d split %d", len(scan.Sections), len(full.Sections))
	}

	for i := range full.Sections {
		if scan.Sections[i].Name != full.Sections[i].Name {
			t.Fatalf("name[%d]: scan %q split %q", i, scan.Sections[i].Name, full.Sections[i].Name)
		}
	}
}

func assertBlocksScanNamesMatchExtract(
	t *testing.T,
	src string,
	startRE, endRE *regexp.Regexp,
	naming fileops.NamingStrategy,
	ext string,
) {
	t.Helper()

	scan, err := fileops.ScanBlocksMeta(context.Background(), fileops.BlocksOptions{
		Source:         strings.NewReader(src),
		StartDelimiter: startRE,
		EndDelimiter:   endRE,
		Naming:         naming,
		Extension:      ext,
	}, fileops.BoundedScanLimits{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	ex, err := fileops.ExtractBlocks(context.Background(), fileops.BlocksOptions{
		Source:         strings.NewReader(src),
		StartDelimiter: startRE,
		EndDelimiter:   endRE,
		Naming:         naming,
		Extension:      ext,
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if len(scan.Metas) != len(ex.Blocks) {
		t.Fatalf("emitted count: scan %d extract %d", len(scan.Metas), len(ex.Blocks))
	}

	for i := range ex.Blocks {
		if scan.Metas[i].Name != ex.Blocks[i].Name {
			t.Fatalf("name[%d]: scan %q extract %q", i, scan.Metas[i].Name, ex.Blocks[i].Name)
		}
	}
}

func TestScanSplitSectionsMeta_namingStrategyParityWithSplit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		desc   string
		src    string
		delim  string
		naming fileops.NamingStrategy
		ext    string
		strip  bool
	}{
		{
			desc:   "from_delimiter_two_sections",
			src:    "## Part One Label\nA\n## Part Two Label\nB\n",
			delim:  `^##`,
			naming: fileops.FromDelimiter,
			ext:    ".txt",
		},
		{
			desc:   "from_content_feature_suffix",
			src:    "## Feature: User Login\nLogin steps\n",
			delim:  `^##\sFeature:`,
			naming: fileops.FromContent,
			ext:    ".feature",
		},
		{
			desc:   "from_content_strip_uses_first_inner_line",
			src:    "## X\nBody Line\n",
			delim:  `^##\s`,
			naming: fileops.FromContent,
			ext:    ".txt",
			strip:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			assertSplitScanNamesMatchSplit(
				t,
				tc.src,
				regexp.MustCompile(tc.delim),
				tc.naming,
				tc.ext,
				tc.strip,
			)
		})
	}
}

func TestScanBlocksMeta_namingStrategyParityWithExtractBlocks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		desc       string
		src        string
		startDelim string
		naming     fileops.NamingStrategy
		ext        string
	}{
		{
			desc:       "from_delimiter_distinct_start_lines",
			src:        "```python\na\n```\n```ruby\nb\n```\n",
			startDelim: "^(```python|```ruby)",
			naming:     fileops.FromDelimiter,
			ext:        ".txt",
		},
		{
			desc:       "from_content_gherkin",
			src:        "```gherkin\nFeature: My Story\n```\n",
			startDelim: "^```gherkin",
			naming:     fileops.FromContent,
			ext:        ".feature",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			assertBlocksScanNamesMatchExtract(
				t,
				tc.src,
				regexp.MustCompile(tc.startDelim),
				regexp.MustCompile("^```$"),
				tc.naming,
				tc.ext,
			)
		})
	}
}

func TestScanSplitSectionsMeta_sentinel_NoDelimiterMatch(t *testing.T) {
	t.Parallel()

	opts := fileops.SplitOptions{
		Source:    strings.NewReader("plain text\n"),
		Delimiter: regexp.MustCompile(`^---$`),
		Naming:    fileops.Sequential,
		Extension: ".txt",
	}

	_, err := fileops.ScanSplitSectionsMeta(context.Background(), opts, fileops.BoundedScanLimits{})
	if err == nil {
		t.Fatal("want error")
	}

	if !errors.Is(err, fileops.ErrNoDelimiterMatch) {
		t.Fatalf("want ErrNoDelimiterMatch, got %v", err)
	}
}

func parseInnerOnly(lines []fileops.Line, inc bool) []fileops.Line {
	switch {
	case len(lines) == 0:
		return nil
	case inc && len(lines) >= 3:
		return lines[1 : len(lines)-1]
	case !inc:
		return lines
	default:
		return nil
	}
}

//nolint:gocyclo // Exhaustive parity rows cover extract vs scan metadata for multiple fixture shapes.
func TestScanBlocksMeta_matchesExtractBlocksStats(t *testing.T) {
	t.Parallel()

	src := "intro\n```gherkin\nFeature: A\n```\n\n```gherkin\nFeature: B\n```\n"
	start := regexp.MustCompile("^```gherkin")
	end := regexp.MustCompile("^```$")

	scanOpts := fileops.BlocksOptions{
		Source:         strings.NewReader(src),
		StartDelimiter: start,
		EndDelimiter:   end,
		Naming:         fileops.Sequential,
		Extension:      ".txt",
	}

	scan, err := fileops.ScanBlocksMeta(context.Background(), scanOpts, fileops.BoundedScanLimits{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	extOpts := fileops.BlocksOptions{
		Source:         strings.NewReader(src),
		StartDelimiter: start,
		EndDelimiter:   end,
		Naming:         fileops.Sequential,
		Extension:      ".txt",
	}

	ex, err := fileops.ExtractBlocks(context.Background(), extOpts)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if scan.BlocksFound != ex.BlocksFound {
		t.Fatalf("BlocksFound scan %d extract %d", scan.BlocksFound, ex.BlocksFound)
	}

	if scan.OutputBlockFileCount != len(ex.Blocks) {
		t.Fatalf("emitted scan %d extract %d", scan.OutputBlockFileCount, len(ex.Blocks))
	}

	if len(scan.Metas) != len(ex.Blocks) {
		t.Fatalf("metas %d blocks %d", len(scan.Metas), len(ex.Blocks))
	}

	wantEmpty := ex.BlocksFound - len(ex.Blocks)

	if scan.EmptyBlocksDiscarded != wantEmpty {
		t.Fatalf("empty discarded: want %d got %d", wantEmpty, scan.EmptyBlocksDiscarded)
	}

	for blkIdx := range ex.Blocks {
		if scan.Metas[blkIdx].Name != ex.Blocks[blkIdx].Name {
			t.Fatalf("name[%d]: scan %q extract %q", blkIdx, scan.Metas[blkIdx].Name, ex.Blocks[blkIdx].Name)
		}

		if scan.Metas[blkIdx].IncludeDelimiters != scanOpts.IncludeDelimiters {
			t.Fatalf("include delims[%d]", blkIdx)
		}

		if scan.Metas[blkIdx].InnerContentLineCount !=
			len(parseInnerOnly(ex.Blocks[blkIdx].Lines, scanOpts.IncludeDelimiters)) {
			t.Fatalf("inner line count mismatch idx %d", blkIdx)
		}
	}
}

func TestScanBlocksMeta_rejectsNonSeekableSource(t *testing.T) {
	t.Parallel()

	opts := fileops.BlocksOptions{
		Source:         readOnlySource{r: strings.NewReader("```go\nbody\n```\n")},
		StartDelimiter: regexp.MustCompile("^```go"),
		EndDelimiter:   regexp.MustCompile("^```$"),
		Naming:         fileops.Sequential,
		Extension:      ".txt",
	}

	_, err := fileops.ScanBlocksMeta(context.Background(), opts, fileops.BoundedScanLimits{})
	if !errors.Is(err, fileops.ErrSeekableSourceRequired) {
		t.Fatalf("want ErrSeekableSourceRequired, got %v", err)
	}
}

func TestScanBlocksMeta_includeDelimitersByteSpan(t *testing.T) {
	t.Parallel()

	src := "```go\nx\n```\n"
	raw := []byte(src)

	opts := fileops.BlocksOptions{
		Source:            strings.NewReader(src),
		StartDelimiter:    regexp.MustCompile("^```go"),
		EndDelimiter:      regexp.MustCompile("^```$"),
		Naming:            fileops.Sequential,
		IncludeDelimiters: true,
		Extension:         ".txt",
	}

	scan, err := fileops.ScanBlocksMeta(context.Background(), opts, fileops.BoundedScanLimits{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(scan.Metas) != 1 {
		t.Fatalf("want 1 meta got %d", len(scan.Metas))
	}

	got := raw[scan.Metas[0].OutputByteStart:scan.Metas[0].OutputByteEndExclusive]

	if string(got) != src {
		t.Fatalf("output span want %q got %q", src, string(got))
	}
}

func TestScanBlocksMeta_maxFilesStopsEarlyBeforeTail(t *testing.T) {
	t.Parallel()

	// Two allowed output blocks, then a third file-producing block; the large tail must not be consumed.
	prefix := "\x60\x60\x60a\nb\n\x60\x60\x60\n\x60\x60\x60a\nc\n\x60\x60\x60\n\x60\x60\x60a\nd\n\x60\x60\x60\n"

	tail := strings.Repeat("\n", 256*1024)

	cr := &countReadSeeker{r: strings.NewReader(prefix + tail)}

	opts := fileops.BlocksOptions{
		Source:         cr,
		StartDelimiter: regexp.MustCompile("^`{3}a"),
		EndDelimiter:   regexp.MustCompile("^`{3}$"),
		Naming:         fileops.Sequential,
		Extension:      ".txt",
	}

	_, err := fileops.ScanBlocksMeta(context.Background(), opts, fileops.BoundedScanLimits{MaxFiles: 2})
	if err == nil {
		t.Fatal("want error")
	}

	if !errors.Is(err, fileops.ErrMaxFilesExceeded) {
		t.Fatalf("want ErrMaxFilesExceeded, got %v", err)
	}

	if cr.n >= int64(len(prefix)+len(tail)) {
		t.Fatalf("read entire source %d unexpectedly", cr.n)
	}
}

func TestScanBlocksMeta_maxFilesCountsOutputFilesNotEmptyBlocks(t *testing.T) {
	t.Parallel()

	src := "\x60\x60\x60a\n\x60\x60\x60\n\x60\x60\x60a\nbody\n\x60\x60\x60\n"
	opts := fileops.BlocksOptions{
		Source:         strings.NewReader(src),
		StartDelimiter: regexp.MustCompile("^`{3}a"),
		EndDelimiter:   regexp.MustCompile("^`{3}$"),
		Naming:         fileops.Sequential,
		Extension:      ".txt",
	}

	scan, err := fileops.ScanBlocksMeta(context.Background(), opts, fileops.BoundedScanLimits{MaxFiles: 1})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if scan.BlocksFound != 2 {
		t.Fatalf("blocks found: got %d want 2", scan.BlocksFound)
	}
	if scan.EmptyBlocksDiscarded != 1 {
		t.Fatalf("empty blocks discarded: got %d want 1", scan.EmptyBlocksDiscarded)
	}
	if scan.OutputBlockFileCount != 1 || len(scan.Metas) != 1 {
		t.Fatalf("output files: count=%d metas=%d want 1 and 1", scan.OutputBlockFileCount, len(scan.Metas))
	}
}

func TestScanBlocksMeta_sentinel_NoBlocksFound(t *testing.T) {
	t.Parallel()

	opts := fileops.BlocksOptions{
		Source:         strings.NewReader("plain\ntext\n"),
		StartDelimiter: regexp.MustCompile("^```gherkin"),
		EndDelimiter:   regexp.MustCompile("^```$"),
		Naming:         fileops.Sequential,
		Extension:      ".txt",
	}

	_, err := fileops.ScanBlocksMeta(context.Background(), opts, fileops.BoundedScanLimits{})
	if err == nil {
		t.Fatal("want error")
	}

	if !errors.Is(err, fileops.ErrNoBlocksFound) {
		t.Fatalf("want ErrNoBlocksFound, got %v", err)
	}
}

func TestScanBlocksMeta_sentinel_UnclosedBlock(t *testing.T) {
	t.Parallel()

	opts := fileops.BlocksOptions{
		Source:         strings.NewReader("\x60\x60\x60gherkin\norphan\n"),
		StartDelimiter: regexp.MustCompile("^```gherkin"),
		EndDelimiter:   regexp.MustCompile("^```$"),
		Naming:         fileops.Sequential,
		Extension:      ".txt",
	}

	_, err := fileops.ScanBlocksMeta(context.Background(), opts, fileops.BoundedScanLimits{})
	if err == nil {
		t.Fatal("want error")
	}

	if !errors.Is(err, fileops.ErrUnclosedBlock) {
		t.Fatalf("want ErrUnclosedBlock, got %v", err)
	}
}

func TestScanBlocksMeta_emptyInnerRecordsFoundAndDiscard(t *testing.T) {
	t.Parallel()

	opts := fileops.BlocksOptions{
		Source:         strings.NewReader("\x60\x60\x60gherkin\n\x60\x60\x60\n"),
		StartDelimiter: regexp.MustCompile("^```gherkin"),
		EndDelimiter:   regexp.MustCompile("^```$"),
		Naming:         fileops.Sequential,
		Extension:      ".txt",
	}

	scan, err := fileops.ScanBlocksMeta(context.Background(), opts, fileops.BoundedScanLimits{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if scan.BlocksFound != 1 {
		t.Fatalf("BlocksFound want 1 got %d", scan.BlocksFound)
	}

	if scan.OutputBlockFileCount != 0 {
		t.Fatalf("emissions want 0 got %d", scan.OutputBlockFileCount)
	}

	if scan.EmptyBlocksDiscarded != 1 {
		t.Fatalf("empty want 1 got %d", scan.EmptyBlocksDiscarded)
	}
}
