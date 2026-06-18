package fileops_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

// Covers seekable bounded block metadata after many inner lines: the scanner must accumulate only first/last
// inner spans and a count rather than retaining every LineSpan buffer.
func TestSeekableBlocksScan_ManyInnerLines_MetaMatchesExtract(t *testing.T) {
	t.Parallel()

	const innerLines = 2500
	reStart := regexp.MustCompile("^```py")
	reEnd := regexp.MustCompile("^```$")

	var body strings.Builder
	body.WriteString("```py\n")
	for range innerLines {
		body.WriteString("z\n")
	}

	body.WriteString("```\n")
	src := body.String()

	opts := fileops.BlocksOptions{
		StartDelimiter: reStart,
		EndDelimiter:   reEnd,
		Naming:         fileops.FromDelimiter,
		Extension:      ".txt",
	}

	scanOpts := opts
	scanOpts.Source = strings.NewReader(src)

	exOpts := opts
	exOpts.Source = strings.NewReader(src)

	got, err := fileops.ScanBlocksMeta(context.Background(), scanOpts, fileops.BoundedScanLimits{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	ex, err := fileops.ExtractBlocks(context.Background(), exOpts)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if len(got.Metas) != 1 || len(ex.Blocks) != 1 {
		t.Fatalf("want one emitted block: metas=%d blocks=%d", len(got.Metas), len(ex.Blocks))
	}

	meta := got.Metas[0]
	block := ex.Blocks[0]

	switch {
	case meta.InnerContentLineCount != innerLines, len(block.Lines) != innerLines:
		t.Fatalf("inner count: meta=%d extract=%d want %d", meta.InnerContentLineCount, len(block.Lines), innerLines)
	case meta.InnerStartLineNum != 2, meta.InnerEndLineNum != innerLines+1, meta.EndDelimLineNum != innerLines+2:
		t.Fatalf("line framing: inner %d-%d endDelim=%d fixture=%d inner lines",
			meta.InnerStartLineNum, meta.InnerEndLineNum, meta.EndDelimLineNum, innerLines)
	case meta.Name != block.Name:
		t.Fatalf("name mismatch: %q vs %q", meta.Name, block.Name)
	}
}
