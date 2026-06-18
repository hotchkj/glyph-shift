package fileops_test

import (
	"regexp"
	"slices"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

func TestTextForFromContentStringsCoversFallbacks(t *testing.T) {
	t.Parallel()

	re := regexp.MustCompile(`^---\s*`)
	cases := []struct {
		name      string
		delimText string
		outLines  []string
		fullSec   []string
		strip     bool
		want      string
	}{
		{name: "strip_first_output", delimText: "--- title", outLines: []string{"body"}, strip: true, want: "body"},
		{name: "strip_no_output", delimText: "--- title", strip: true, want: "--- title"},
		{name: "suffix_after_delimiter", delimText: "--- title", want: "title"},
		{name: "full_section_second_line", delimText: "---   ", fullSec: []string{"---   ", "next"}, want: "next"},
		{name: "delimiter_fallback", delimText: "plain", want: "plain"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := fileops.TestingTextForFromContentStrings(re, tc.delimText, tc.outLines, tc.fullSec, tc.strip)
			if got != tc.want {
				t.Fatalf("text = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestThinStringsForSplitNamingCoversStripAndNonStrip(t *testing.T) {
	t.Parallel()

	first := "first"
	second := "second"

	outThin, fullThin := fileops.TestingThinStringsForSplitNaming(true, "---", 3, &first, &second)
	assertStringSlice(t, outThin, []string{"first", "second"})
	assertStringSlice(t, fullThin, []string{"---", "first", "second"})

	outThin, fullThin = fileops.TestingThinStringsForSplitNaming(true, "---", 1, nil, nil)
	assertStringSlice(t, outThin, nil)
	assertStringSlice(t, fullThin, []string{"---"})

	outThin, fullThin = fileops.TestingThinStringsForSplitNaming(false, "---", 3, &first, &second)
	assertStringSlice(t, outThin, []string{"---", "first", "second"})
	assertStringSlice(t, fullThin, []string{"---", "first", "second"})
}

func TestChooseSectionFilenameFromStringsCoversStrategies(t *testing.T) {
	t.Parallel()

	re := regexp.MustCompile(`^---\s*`)
	cases := []struct {
		name     string
		strategy fileops.NamingStrategy
		existing map[string]bool
		want     string
	}{
		{name: "delimiter", strategy: fileops.FromDelimiter, want: "title.md"},
		{name: "content", strategy: fileops.FromContent, want: "body.md"},
		{name: "sequential_dedup", strategy: fileops.Sequential, existing: map[string]bool{"001.md": true}, want: "001-2.md"},
		{name: "unknown_defaults_sequential", strategy: fileops.NamingStrategy(99), want: "001.md"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := fileops.SplitOptions{Delimiter: re, Naming: tc.strategy, StripDelimiter: true}
			got := fileops.TestingChooseSectionFilenameFromStrings(
				opts,
				1,
				"--- title",
				[]string{"body"},
				[]string{"--- title", "body"},
				".md",
				tc.existing,
			)
			if got != tc.want {
				t.Fatalf("filename = %q, want %q", got, tc.want)
			}
		})
	}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Fatalf("slice = %#v, want %#v", got, want)
	}
}
