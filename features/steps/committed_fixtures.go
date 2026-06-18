package steps

// User vision: BDD correctness depends on comparing workspace outputs against
// committed byte-faithful golden files. OS-backed reads of those paths go through
// internal/goldenreader only (see docs/glyph-shift-intent narrow exemption).

import (
	"io/fs"

	"github.com/hotchkj/glyph-shift/internal/goldenreader"
)

// readCommittedFileRelativeToFeatures reads a committed file using a path
// expressed relative to the features/ directory (e.g. testdata/inputs/a.md).
func readCommittedFileRelativeToFeatures(featuresRel string) ([]byte, error) {
	return goldenreader.ReadGolden(featuresRel)
}

func readCommittedDirRelativeToFeatures(featuresRel string) ([]fs.DirEntry, error) {
	return goldenreader.ReadGoldenDir(featuresRel)
}
