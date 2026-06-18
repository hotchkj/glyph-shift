package fileops

import (
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
)

func TestScanSplitSectionsMeta_FromContentPreambleNamingRestoresSeekPosition(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	src := "Preamble Name\n## First\nbody\n"
	delimiterOffset := int64(strings.Index(src, "## First"))
	reader := bytes.NewReader([]byte(src))

	if _, err := reader.Seek(delimiterOffset, io.SeekStart); err != nil {
		t.Fatalf("seek setup: %v", err)
	}

	gotName, err := preambleSplitOutputFilenameFromContent(
		ctx,
		reader,
		SplitOptions{Naming: FromContent, Extension: ".txt"},
		1,
		map[string]bool{},
	)
	if err != nil {
		t.Fatalf("preambleSplitOutputFilenameFromContent: %v", err)
	}

	wantName := GenerateFilename(FromContent, 1, "Preamble Name", ".txt")
	if gotName != wantName {
		t.Fatalf("preamble name got %q want %q", gotName, wantName)
	}

	gotOffset, err := reader.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatalf("seek current: %v", err)
	}
	if gotOffset != delimiterOffset {
		t.Fatalf("reader offset got %d want %d", gotOffset, delimiterOffset)
	}

	scan, err := ScanSplitSectionsMeta(ctx, SplitOptions{
		Source:    bytes.NewReader([]byte(src)),
		Delimiter: regexp.MustCompile(`^##`),
		Naming:    FromContent,
		Extension: ".txt",
	}, BoundedScanLimits{})
	if err != nil {
		t.Fatalf("ScanSplitSectionsMeta: %v", err)
	}
	if len(scan.Sections) == 0 || scan.Sections[0].Name != wantName {
		t.Fatalf("first section name got %#v want %q", scan.Sections, wantName)
	}
}

func TestScanSplitSectionsMeta_SeekableStripDelimiterSkipSection_DoesNotEmitSkippedOrdinalHole(t *testing.T) {
	t.Parallel()

	scan, err := ScanSplitSectionsMeta(context.Background(), SplitOptions{
		Source:         bytes.NewReader([]byte("## Empty\n## Real\nbody\n")),
		Delimiter:      regexp.MustCompile(`^##`),
		Naming:         Sequential,
		Extension:      ".txt",
		StripDelimiter: true,
	}, BoundedScanLimits{})
	if err != nil {
		t.Fatalf("ScanSplitSectionsMeta: %v", err)
	}

	if len(scan.Sections) != 1 {
		t.Fatalf("section count got %d want 1: %#v", len(scan.Sections), scan.Sections)
	}

	section := scan.Sections[0]
	if section.OrdinalSeq != 1 {
		t.Fatalf("ordinal got %d want 1", section.OrdinalSeq)
	}

	wantName := GenerateFilename(Sequential, 1, "", ".txt")
	if section.Name != wantName {
		t.Fatalf("name got %q want %q", section.Name, wantName)
	}
}

func TestPreambleSplit_FromContent_NoLineSpanSurfaceError(t *testing.T) {
	t.Parallel()

	_, err := preambleSplitOutputFilenameFromContent(
		context.Background(),
		bytes.NewReader(nil),
		SplitOptions{Naming: FromContent, Extension: ".txt"},
		1,
		map[string]bool{},
	)
	if !errors.Is(err, errPreambleNoFirstLineSpan) {
		t.Fatalf("error got %v want %v", err, errPreambleNoFirstLineSpan)
	}
}
