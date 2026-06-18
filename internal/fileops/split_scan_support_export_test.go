package fileops_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

func TestTestingTrimIncompleteUTF8Suffix_TrailingPartialCodepoint(t *testing.T) {
	t.Parallel()

	raw := append([]byte("prefix"), '\xff')
	got := string(fileops.TestingTrimIncompleteUTF8Suffix(raw))
	if got != "prefix" {
		t.Fatalf(`trim lone continuation: got %q want "prefix"`, got)
	}
}

func TestTestingStreamSourcePrefixBytesAppend_ByteLenZeroReturnsNilSlice(t *testing.T) {
	t.Parallel()

	got, err := fileops.TestingStreamSourcePrefixBytesAppend(context.Background(), bytes.NewReader([]byte("abc")), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty slice for byteLen zero, got %d bytes", len(got))
	}
}

func TestTestingStreamSourcePrefixBytesAppend_UnexpectedEOF(t *testing.T) {
	t.Parallel()

	_, err := fileops.TestingStreamSourcePrefixBytesAppend(context.Background(), bytes.NewReader([]byte("tiny")), 10_000)
	if err == nil {
		t.Fatal("want error when source truncates declared byte length")
	}
}
