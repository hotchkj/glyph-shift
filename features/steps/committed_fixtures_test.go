package steps

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/goldenreader"
)

func TestReadCommittedRelative_rejectsLeavingFeaturesDirectory(t *testing.T) {
	t.Parallel()

	_, err := readCommittedFileRelativeToFeatures(filepath.Join("testdata", "..", "..", "..", "go.mod"))
	if err == nil {
		t.Fatal("want error")
	}

	if !errors.Is(err, goldenreader.ErrFixturePath) {
		t.Fatalf("want errors.Is(err, goldenreader.ErrFixturePath): %v", err)
	}
}
