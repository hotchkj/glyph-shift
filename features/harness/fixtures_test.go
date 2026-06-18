package harness

import (
	"bytes"
	"testing"
)

func TestLineTerminator_names(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		want []byte
	}{
		{"LF", []byte{'\n'}},
		{"CRLF", []byte{'\r', '\n'}},
		{"CR", []byte{'\r'}},
	} {
		got, err := LineTerminator(tc.name)
		if err != nil {
			t.Fatalf("LineTerminator(%q): %v", tc.name, err)
		}
		if !bytes.Equal(got, tc.want) {
			t.Fatalf("LineTerminator(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestNumberedLineContent_matchesLFAndCRLFHelpers(t *testing.T) {
	t.Parallel()

	lf, err := NumberedLineContent(3, "LF")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(lf, LFLineContent(3)) {
		t.Fatalf("LF mismatch:\n%s", lf)
	}

	crlf, err := NumberedLineContent(3, "CRLF")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(crlf, CRLFLineContent(3)) {
		t.Fatalf("CRLF mismatch:\n%s", crlf)
	}
}

func TestBinaryFixtures_includeNullInFirst8KB(t *testing.T) {
	t.Parallel()

	for name, data := range map[string][]byte{
		"BDD": BinaryFileFixture(),
		"CLI": BinarySourceCLIFixture(),
	} {
		window := data
		if len(window) > 8192 {
			window = window[:8192]
		}

		if !bytes.Contains(window, []byte{0}) {
			t.Fatalf("%s fixture: expected null byte in first 8KB", name)
		}
	}
}

func TestDecodeEscapedFixture_roundTrip(t *testing.T) {
	t.Parallel()

	raw := []byte("```go\\n\\xEF\\xBB\\xBFpayload\\n```\\n\n")
	got, err := DecodeEscapedFixture(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 4 || got[0] != '`' {
		t.Fatalf("unexpected decode prefix: %q", got)
	}
}
