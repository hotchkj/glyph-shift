package testutil

import (
	"errors"
	"testing"

	"github.com/spf13/afero"
)

func TestRemoveMemFsDestinationsForHeapBracketIgnoresMissingDestinations(t *testing.T) {
	t.Parallel()

	memFs := afero.NewMemMapFs()
	if err := removeMemFsDestinationsForHeapBracket(memFs, []string{"/missing.txt"}, "split"); err != nil {
		t.Fatalf("remove missing destination: %v", err)
	}
}

func TestRemoveMemFsDestinationsForHeapBracketPropagatesRemoveError(t *testing.T) {
	t.Parallel()

	memFs := afero.NewReadOnlyFs(afero.NewMemMapFs())
	err := removeMemFsDestinationsForHeapBracket(memFs, []string{"/blocked.txt"}, "blocks")
	if err == nil {
		t.Fatal("remove read-only destination error = nil, want error")
	}
	if errors.Is(err, afero.ErrFileNotFound) {
		t.Fatalf("remove error = %v, want non-missing error", err)
	}
}
