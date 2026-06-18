package pipeline

import (
	"fmt"
	"path/filepath"

	"github.com/hotchkj/glyph-shift/internal/validate"
)

func validateSourceAndOutDir(srcPath, outDir, root string, resolver validate.PathResolver) error {
	if err := validate.ValidatePath(srcPath, root, resolver); err != nil {
		return pathContextError(PathRoleSrc, srcPath, fmt.Errorf("validate source: %w", err))
	}

	if err := validate.ValidatePath(outDir, root, resolver); err != nil {
		return pathContextError(PathRoleOutDir, outDir, fmt.Errorf("validate out-dir: %w", err))
	}

	return nil
}

func mkdirOutDirIfRequested(out OutputOpener, outDir string, mkdir bool) error {
	if !mkdir {
		return nil
	}

	if mkErr := out.MkdirAll(outDir, DirPerm); mkErr != nil {
		return pathContextError(PathRoleOutDir, outDir, fmt.Errorf("create output directory: %w", mkErr))
	}

	return nil
}

// absolutePlannedOutputPath joins outDir and basename, resolves to an absolute native path, and applies
// filepath.Clean. When the joined path is already absolute, filepath.Abs is skipped so root-relative paths
// (for example `\work\out\001.txt` on Windows) retain their original root.
func absolutePlannedOutputPath(outDir, basename string) (string, error) {
	joined := filepath.Join(outDir, basename)
	cleaned := filepath.Clean(joined)

	if filepath.IsAbs(cleaned) {
		return cleaned, nil
	}

	absJoined, err := filepath.Abs(cleaned)
	if err != nil {
		return "", pathContextError(PathRoleOutputPath, cleaned, fmt.Errorf("planned output path: %w", err))
	}

	return filepath.Clean(absJoined), nil
}

// canonicalAbsoluteNativePath normalizes a filesystem path to a cleaned absolute path for stable error
// resources and preview contract fields. Paths already recognized as absolute by filepath.IsAbs are only
// cleaned, not passed through filepath.Abs (avoids rewriting root-relative paths on Windows).
func canonicalAbsoluteNativePath(path string) (string, error) {
	cp := filepath.Clean(path)

	if filepath.IsAbs(cp) {
		return cp, nil
	}

	absPath, err := filepath.Abs(cp)
	if err != nil {
		return "", err
	}

	return filepath.Clean(absPath), nil
}

// validatePlannedOutputPaths checks each planned file under outDir resolves within root
// (same checks as write path) without opening files. Returns absolute planned paths on success.
func validatePlannedOutputPaths(
	outDir, root string,
	basenames []string,
	resolver validate.PathResolver,
) ([]string, error) {
	paths := make([]string, 0, len(basenames))

	for _, name := range basenames {
		path, apErr := absolutePlannedOutputPath(outDir, name)
		if apErr != nil {
			return nil, apErr
		}

		if valErr := validate.ValidatePath(path, root, resolver); valErr != nil {
			return nil, pathContextError(PathRoleOutputPath, path, fmt.Errorf("late path validation: %w", valErr))
		}

		paths = append(paths, path)
	}

	return paths, nil
}
