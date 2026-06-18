package fsnorm

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonical_Empty(t *testing.T) {
	t.Parallel()

	if got := Canonical(""); got != "" {
		t.Fatalf("Canonical(\"\") = %q, want \"\"", got)
	}
}

func TestCanonical_RelativeBackslashes(t *testing.T) {
	t.Parallel()

	const want = "internal/harness/config.go"
	if got := Canonical(`internal\harness\config.go`); got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_MixedSeparators(t *testing.T) {
	t.Parallel()

	const want = "internal/harness/config.go"
	if got := Canonical(`internal\harness/config.go`); got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_UnixAbsolute(t *testing.T) {
	t.Parallel()

	const want = "/home/dev/repo/internal/harness/config.go"
	if got := Canonical("/home/dev/repo/internal/harness/config.go"); got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_WindowsStyleDrivePath(t *testing.T) {
	t.Parallel()

	const in = `C:\Users\dev\repo\internal\harness\config.go`
	const want = "C:/Users/dev/repo/internal/harness/config.go"
	if got := Canonical(in); got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_DotSegments(t *testing.T) {
	t.Parallel()

	if got, want := Canonical("x/y/../z"), "x/z"; got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_Idempotent(t *testing.T) {
	t.Parallel()

	const in = `C:\a\b\..\c`
	once := Canonical(in)
	twice := Canonical(once)
	if once != twice {
		t.Fatalf("Canonical not idempotent: once=%q twice=%q", once, twice)
	}
}

func TestCanonical_TrailingSlash(t *testing.T) {
	t.Parallel()

	if got, want := Canonical("a/b/"), "a/b"; got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_DotOnly(t *testing.T) {
	t.Parallel()

	if got, want := Canonical("."), "."; got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_DoubleDotOnly(t *testing.T) {
	t.Parallel()

	if got, want := Canonical(".."), ".."; got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_UNCStyleDoubleSlash(t *testing.T) {
	t.Parallel()

	got := Canonical(`//host/share/path`)
	if got == "" {
		t.Fatal("unexpected empty")
	}
	if Canonical(got) != got {
		t.Fatalf("expected idempotent, got follow-up %q", Canonical(got))
	}
	// filepath.Clean collapses duplicate slashes in the prefix (GOOS-dependent details).
	if !strings.HasPrefix(got, "/") {
		t.Fatalf("expected rooted slash form, got %q", got)
	}
}

func TestDirNative_EmptyBecomesDot(t *testing.T) {
	t.Parallel()

	if got := DirNative(""); got != "." {
		t.Fatalf("DirNative(\"\") = %q, want \".\"", got)
	}
}

func TestDirNative_MixedSeparatorsResolvesDotSegments(t *testing.T) {
	t.Parallel()

	in := `a\b/../c`
	got := DirNative(in)
	if filepath.Base(got) != "c" {
		t.Fatalf("DirNative(%q) basename want c, got %q", in, got)
	}
}
