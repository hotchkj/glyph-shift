package pipeline

import (
	"fmt"
	"path/filepath"

	"github.com/hotchkj/glyph-shift/internal/fsnorm"
)

// PreparePath resolves rawPath lexically against workspaceRoot without ValidatePath security checks:
// Canonical → ResolveUnderWorkspace → filepath.Abs → filepath.Clean.
// Leading and trailing spaces are preserved; empty rawPath yields ErrEmptyPreparedPath.
func PreparePath(rawPath, workspaceRoot string) (string, error) {
	if rawPath == "" {
		return "", fmt.Errorf("%w", ErrEmptyPreparedPath)
	}

	candidate := fsnorm.ResolveUnderWorkspace(fsnorm.Canonical(rawPath), workspaceRoot)

	absPath, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("prepare path: %w", err)
	}

	return filepath.Clean(absPath), nil
}
