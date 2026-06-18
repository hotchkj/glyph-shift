package pipeline_test

import (
	"errors"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

func TestParseCommaSeparatedNames(t *testing.T) {
	t.Parallel()

	got, err := pipeline.ParseCommaSeparatedNames("a, b , c")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("got %#v", got)
	}

	empty, err := pipeline.ParseCommaSeparatedNames("   ")
	if err != nil {
		t.Fatal(err)
	}
	if empty != nil {
		t.Fatalf("want nil slice, got %#v", empty)
	}

	_, emptySegErr := pipeline.ParseCommaSeparatedNames("a,,b")
	if emptySegErr == nil || !errors.Is(emptySegErr, pipeline.ErrEmptyNamesListEntry) {
		t.Fatalf("expected ErrEmptyNamesListEntry, got %v", emptySegErr)
	}
}

func TestParseCommaSeparatedNames_errorsIsNotMapped(t *testing.T) {
	t.Parallel()

	_, err := pipeline.ParseCommaSeparatedNames("x,")
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, pipeline.ErrNamesCountMismatch) {
		t.Fatal("parse error should not wrap ErrNamesCountMismatch")
	}
}

func TestParseNamingStrategy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  string
		want fileops.NamingStrategy
	}{
		{raw: "", want: fileops.Sequential},
		{raw: " \t ", want: fileops.Sequential},
		{raw: "sequential", want: fileops.Sequential},
		{raw: "SEQUENTIAL", want: fileops.Sequential},
		{raw: "content", want: fileops.FromContent},
		{raw: " Content ", want: fileops.FromContent},
		{raw: "match", want: fileops.FromDelimiter},
		{raw: "MATCH", want: fileops.FromDelimiter},
	}

	for _, tc := range cases {
		got, err := pipeline.ParseNamingStrategy(tc.raw)
		if err != nil {
			t.Fatalf("ParseNamingStrategy(%q): %v", tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("ParseNamingStrategy(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestParseNamingStrategyUnknownWrapsSentinel(t *testing.T) {
	t.Parallel()

	_, err := pipeline.ParseNamingStrategy("unknown")
	if !errors.Is(err, pipeline.ErrUnknownNamingStrategy) {
		t.Fatalf("ParseNamingStrategy error = %v, want %v", err, pipeline.ErrUnknownNamingStrategy)
	}
}
