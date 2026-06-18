package goldenreader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// RepoRoot returns the directory that contains go.mod, discovered by walking up
// from this package's source file location.
func RepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", ErrRuntimeCaller
	}

	dir := filepath.Dir(file)
	for {
		_, statErr := os.Stat(filepath.Join(dir, "go.mod"))
		if statErr == nil {
			return dir, nil
		}
		if !os.IsNotExist(statErr) {
			return "", fmt.Errorf("stat go.mod under %q: %w", dir, statErr)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrRepoRoot
		}
		dir = parent
	}
}

// FeaturesRoot returns the features/ directory under the repository root.
func FeaturesRoot() (string, error) {
	root, err := RepoRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, "features"), nil
}

func absUnderFeatures(featuresRel string) (joined string, err error) {
	root, err := FeaturesRoot()
	if err != nil {
		return "", err
	}

	clean := filepath.Clean(filepath.FromSlash(featuresRel))
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("%w: %q", ErrFixturePath, featuresRel)
	}

	joined = filepath.Join(root, clean)

	relOut, relErr := filepath.Rel(root, joined)
	if relErr != nil {
		return "", fmt.Errorf("%w: %w", ErrFixturePath, relErr)
	}

	if relOut == ".." || strings.HasPrefix(relOut, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrFixturePath, featuresRel)
	}

	return joined, nil
}

// ReadGolden reads a committed file using a path expressed relative to the
// features/ directory (for example testdata/inputs/three-lines.txt).
func ReadGolden(featuresRel string) ([]byte, error) {
	abs, err := absUnderFeatures(featuresRel)
	if err != nil {
		return nil, err
	}

	parent := filepath.Dir(abs)
	leaf := filepath.Base(abs)

	data, err := fs.ReadFile(os.DirFS(parent), filepath.ToSlash(leaf))
	if err != nil {
		return nil, fmt.Errorf("read committed fixture %q: %w", featuresRel, err)
	}

	return data, nil
}

// ReadGoldenDir lists a committed directory using a path relative to features/.
func ReadGoldenDir(featuresRel string) ([]fs.DirEntry, error) {
	abs, err := absUnderFeatures(featuresRel)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("read committed dir %q: %w", featuresRel, err)
	}

	return entries, nil
}
