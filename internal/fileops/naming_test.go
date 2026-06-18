package fileops

import (
	"strings"
	"testing"
)

func TestGenerateFilename_SequentialOne(t *testing.T) {
	t.Parallel()

	got := GenerateFilename(Sequential, 1, "", ".txt")
	if got != "001.txt" {
		t.Fatalf("got %q want %q", got, "001.txt")
	}
}

func TestGenerateFilename_SequentialHundred(t *testing.T) {
	t.Parallel()

	got := GenerateFilename(Sequential, 100, "", ".txt")
	if got != "100.txt" {
		t.Fatalf("got %q want %q", got, "100.txt")
	}
}

func TestGenerateFilename_FromContentSimple(t *testing.T) {
	t.Parallel()

	got := GenerateFilename(FromContent, 0, "Feature: Login", ".txt")
	if got != "feature-login.txt" {
		t.Fatalf("got %q want %q", got, "feature-login.txt")
	}
}

func TestGenerateFilename_FromContentSpecialChars(t *testing.T) {
	t.Parallel()

	got := GenerateFilename(FromContent, 0, "What's the deal? (Part 1)", ".txt")
	if got != "whats-the-deal-part-1.txt" {
		t.Fatalf("got %q want %q", got, "whats-the-deal-part-1.txt")
	}
}

func TestGenerateFilename_FromContentLong(t *testing.T) {
	t.Parallel()

	longText := strings.Repeat("a", 100)
	want := strings.Repeat("a", 60) + ".txt"
	got := GenerateFilename(FromContent, 0, longText, ".txt")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestGenerateFilename_FromContentEmptyFallsBackSequential(t *testing.T) {
	t.Parallel()

	got := GenerateFilename(FromContent, 5, "", ".txt")
	if got != "005.txt" {
		t.Fatalf("got %q want %q", got, "005.txt")
	}
}

func TestGenerateFilename_FromContentOSUnsafe(t *testing.T) {
	t.Parallel()

	got := GenerateFilename(FromContent, 0, "file<name>", ".go")
	if got != "filename.go" {
		t.Fatalf("got %q want %q", got, "filename.go")
	}
}

func TestGenerateFilename_FromContentNoTrailingDotInSlug(t *testing.T) {
	t.Parallel()

	got := GenerateFilename(FromContent, 0, "test.", ".txt")
	if strings.HasSuffix(strings.TrimSuffix(got, ".txt"), ".") {
		t.Fatalf("slug must not end with dot: %q", got)
	}

	if got != "test.txt" {
		t.Fatalf("got %q want %q", got, "test.txt")
	}
}

func TestGenerateFilename_ReservedNameFallsBackToSequential(t *testing.T) {
	t.Parallel()

	cases := []struct {
		text string
		name string
	}{
		{"CON", "001.txt"},
		{"con: something", "con-something.txt"},
		{"AUX", "001.txt"},
		{"COM1", "001.txt"},
		{"NUL", "001.txt"},
		{"PRN", "001.txt"},
		{"LPT1", "001.txt"},
	}

	for _, tc := range cases {
		got := GenerateFilename(FromContent, 1, tc.text, ".txt")
		if got != tc.name {
			t.Errorf("GenerateFilename(FromContent, 1, %q, .txt) = %q, want %q", tc.text, got, tc.name)
		}
	}
}

func TestGenerateFilename_ReservedNameWithExtension(t *testing.T) {
	t.Parallel()

	got := GenerateFilename(FromDelimiter, 0, "CON", ".md")
	if got != "000.md" {
		t.Errorf("got %q, want 000.md", got)
	}
}

func TestGenerateFilename_NonReservedPassesThrough(t *testing.T) {
	t.Parallel()

	got := GenerateFilename(FromContent, 1, "hello world", ".txt")
	if got == "001.txt" {
		t.Error("non-reserved name should not fall back to sequential")
	}

	if !strings.HasSuffix(got, ".txt") {
		t.Errorf("got %q, want .txt suffix", got)
	}
}

func TestDeduplicateFilename_FirstUnique(t *testing.T) {
	t.Parallel()

	existing := map[string]bool{}
	got := DeduplicateFilename("feature-login.txt", existing)
	if got != "feature-login.txt" {
		t.Fatalf("got %q want %q", got, "feature-login.txt")
	}

	if !existing["feature-login.txt"] {
		t.Fatal("existing map must contain returned name")
	}
}

func TestDeduplicateFilename_SecondCollision(t *testing.T) {
	t.Parallel()

	existing := map[string]bool{"feature-login.txt": true}
	got := DeduplicateFilename("feature-login.txt", existing)
	if got != "feature-login-2.txt" {
		t.Fatalf("got %q want %q", got, "feature-login-2.txt")
	}

	if !existing["feature-login-2.txt"] {
		t.Fatal("existing map must contain deduplicated name")
	}
}

func TestDeduplicateFilename_ThirdCollision(t *testing.T) {
	t.Parallel()

	existing := map[string]bool{
		"feature-login.txt":   true,
		"feature-login-2.txt": true,
	}
	got := DeduplicateFilename("feature-login.txt", existing)
	if got != "feature-login-3.txt" {
		t.Fatalf("got %q want %q", got, "feature-login-3.txt")
	}

	if !existing["feature-login-3.txt"] {
		t.Fatal("existing map must contain deduplicated name")
	}
}
