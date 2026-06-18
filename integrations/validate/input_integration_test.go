//go:build integration

// Integration tests for validate.ValidatePath — requires real OS filesystem
// for symlink resolution and actual directory structures.
// Run: mage integration. Diagnostic: go test -tags integration ./integrations/...
package validate_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/validate"
)

func TestValidatePath_ValidRelativeUnderCwd(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	path := filepath.Join("subdir", "file.txt")
	err = validate.ValidatePath(path, cwd, validate.NewOSPathResolver())
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
}

func TestValidatePath_TraversalOutsideRoot(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	path := filepath.Join("..", "..", "etc", "passwd")
	err = validate.ValidatePath(path, cwd, validate.NewOSPathResolver())
	if err == nil {
		t.Fatal("want error for traversal outside root")
	}

	if !errors.Is(err, validate.ErrPathTraversal) && !errors.Is(err, validate.ErrOutsideRoot) {
		t.Fatalf("want ErrPathTraversal or ErrOutsideRoot, got %v", err)
	}
}

func TestValidatePath_AbsoluteWithinRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	subPath := filepath.Join(root, "sub", "file.txt")

	err := validate.ValidatePath(subPath, root, validate.NewOSPathResolver())
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
}

func TestValidatePath_AbsoluteOutsideRoot(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	root := filepath.Join(base, "allowed")
	outside := filepath.Join(base, "outside")

	err := os.MkdirAll(root, 0o750)
	if err != nil {
		t.Fatalf("mkdir root: %v", err)
	}

	err = os.MkdirAll(outside, 0o750)
	if err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}

	evil := filepath.Join(outside, "evil.txt")
	err = validate.ValidatePath(evil, root, validate.NewOSPathResolver())
	if err == nil {
		t.Fatal("want error for path outside root")
	}
}

func TestValidatePathRejectsReservedName(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Windows resolves CON to \\\\.\\CON before ValidatePath sees basename; " +
			"reserved-name logic is covered by unit tests")
	}

	tmpDir := t.TempDir()
	badPath := filepath.Join(tmpDir, "CON")
	err := validate.ValidatePath(badPath, tmpDir, validate.NewOSPathResolver())
	if err == nil {
		t.Fatal("expected error for reserved name CON")
	}

	if !errors.Is(err, validate.ErrReservedName) {
		t.Fatalf("expected ErrReservedName, got %v", err)
	}
}

func TestValidatePath_RejectsTabInPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	pathWithTab := tmpDir + string(filepath.Separator) + "file\twith\ttab.txt"
	err := validate.ValidatePath(pathWithTab, tmpDir, validate.NewOSPathResolver())
	if err == nil {
		t.Fatal("expected error for TAB in path")
	}
	if !errors.Is(err, validate.ErrControlChar) {
		t.Fatalf("expected ErrControlChar, got %v", err)
	}
}

func TestValidatePath_RejectsNullInPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	pathWithNull := tmpDir + string(filepath.Separator) + "file\x00name.txt"
	err := validate.ValidatePath(pathWithNull, tmpDir, validate.NewOSPathResolver())
	if err == nil {
		t.Fatal("expected error for null byte in path")
	}
	// On Windows, filepath.Abs may error on null bytes before our check runs,
	// so we accept any error here as correct behavior
}

func TestProductionPathResolverRejectsNULPathBeforeSyscalls(t *testing.T) {
	t.Parallel()

	invalidPath := string([]byte{0})
	resolver := validate.NewOSPathResolver()

	if _, err := resolver.Lstat(invalidPath); !errors.Is(err, validate.ErrPathContainsNUL) {
		t.Fatalf("Lstat: got %v want ErrPathContainsNUL", err)
	}
	if _, err := resolver.EvalSymlinks(invalidPath); !errors.Is(err, validate.ErrPathContainsNUL) {
		t.Fatalf("EvalSymlinks: got %v want ErrPathContainsNUL", err)
	}
}

func TestValidatePath_SymlinkParentOutsideRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()

	linkPath := filepath.Join(root, "escape-link")
	err := os.Symlink(outside, linkPath)
	if err != nil {
		t.Skipf("cannot create symlink (may need elevated privileges): %v", err)
	}

	targetPath := filepath.Join(linkPath, "nonexistent.txt")
	err = validate.ValidatePath(targetPath, root, validate.NewOSPathResolver())
	if err == nil {
		t.Fatal("expected error for non-existent file under symlink pointing outside root")
	}
}

func TestValidatePath_SymlinkedAncestorAboveRootIsAllowed(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	realAncestor := filepath.Join(base, "real-ancestor")
	linkAncestor := filepath.Join(base, "link-ancestor")
	realRoot := filepath.Join(realAncestor, "workspace")
	root := filepath.Join(linkAncestor, "workspace")

	err := os.MkdirAll(realRoot, 0o750)
	if err != nil {
		t.Fatalf("mkdir real root: %v", err)
	}

	err = os.Symlink(realAncestor, linkAncestor)
	if err != nil {
		t.Skipf("cannot create symlink (may need elevated privileges): %v", err)
	}

	path := filepath.Join(root, "new-file.txt")
	err = validate.ValidatePath(path, root, validate.NewOSPathResolver())
	if err != nil {
		t.Fatalf("want nil for symlinked ancestor above root, got %v", err)
	}
}
