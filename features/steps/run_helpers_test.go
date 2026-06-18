package steps

import (
	"errors"
	"testing"
)

func TestRunGlyphShiftSubcommand_RejectsNonMCPSubcommand(t *testing.T) {
	t.Parallel()

	tc := NewTestContext()
	t.Cleanup(func() { tc.Cleanup() })

	err := tc.runGlyphShiftSubcommand([]string{"extract", "--help"})
	if err == nil {
		t.Fatal("expected error for non-mcp subcommand via runGlyphShiftSubcommand")
	}

	if !errors.Is(err, errLayer1GenericCLINotAllowed) {
		t.Fatalf("expected errLayer1GenericCLINotAllowed, got: %v", err)
	}
}

func TestRunGlyphShiftSubcommand_RejectsEmptyArgs(t *testing.T) {
	t.Parallel()

	tc := NewTestContext()
	t.Cleanup(func() { tc.Cleanup() })

	err := tc.runGlyphShiftSubcommand(nil)
	if err == nil {
		t.Fatal("expected error when no subcommand")
	}

	if !errors.Is(err, errLayer1GenericCLINotAllowed) {
		t.Fatalf("expected errLayer1GenericCLINotAllowed, got: %v", err)
	}
}
