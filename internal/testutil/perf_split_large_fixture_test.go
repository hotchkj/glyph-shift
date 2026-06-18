package testutil

import (
	"bytes"
	"testing"
)

func TestBuildLargeSplitSingleSectionSource_LineLengthZeroProducesEmptyLogicalLines(t *testing.T) {
	t.Parallel()

	delim := []byte("---\n")
	lineCount := 4

	got := BuildLargeSplitSingleSectionSource(lineCount, 0, delim)

	if !bytes.HasPrefix(got, delim) {
		t.Fatalf("must begin with delim %q", delim)
	}

	rest := got[len(delim):]
	wantTail := bytes.Repeat([]byte{'\n'}, lineCount)

	if !bytes.Equal(rest, wantTail) {
		t.Fatalf("body = %q want %d bare newlines after delim", rest, lineCount)
	}
}
