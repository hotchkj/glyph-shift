package validate

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeResolver struct {
	lstatFn func(string) (fs.FileInfo, error)
	evalFn  func(string) (string, error)
}

func (f fakeResolver) Lstat(path string) (fs.FileInfo, error) {
	return f.lstatFn(path)
}

func (f fakeResolver) EvalSymlinks(path string) (string, error) {
	return f.evalFn(path)
}

type fakeFileInfo struct{ mode fs.FileMode }

func (f fakeFileInfo) Name() string {
	return ""
}

func (f fakeFileInfo) Size() int64 {
	return 0
}

func (f fakeFileInfo) Mode() fs.FileMode {
	return f.mode
}

func (f fakeFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (f fakeFileInfo) IsDir() bool {
	return f.mode.IsDir()
}

func (f fakeFileInfo) Sys() any {
	return nil
}

func TestValidatePathForOS_ReservedCon(t *testing.T) {
	t.Parallel()

	err := ValidatePathForOS("CON")
	if err == nil {
		t.Fatal("want error for reserved name CON")
	}

	if !errors.Is(err, ErrReservedName) {
		t.Fatalf("expected ErrReservedName, got %v", err)
	}
}

func TestValidatePathForOS_ReservedCom1WithExtension(t *testing.T) {
	t.Parallel()

	err := ValidatePathForOS("COM1.txt")
	if err == nil {
		t.Fatal("want error for reserved name COM1.txt")
	}

	if !errors.Is(err, ErrReservedName) {
		t.Fatalf("expected ErrReservedName, got %v", err)
	}
}

func TestRejectControlChars_RejectsSOH(t *testing.T) {
	t.Parallel()

	s := string([]byte{0x01})
	err := RejectControlChars(s)
	if err == nil {
		t.Fatal("want error for control character SOH")
	}

	if !errors.Is(err, ErrControlChar) {
		t.Fatalf("expected ErrControlChar, got %v", err)
	}
}

func TestRejectControlChars_TabAllowed(t *testing.T) {
	t.Parallel()

	s := "\tallowed\t"
	err := RejectControlChars(s)
	if err != nil {
		t.Fatalf("want nil for TAB, got %v", err)
	}
}

func TestValidatePattern_ValidRegex(t *testing.T) {
	t.Parallel()

	re, err := ValidatePattern(`^## Feature:`)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if re == nil {
		t.Fatal("want non-nil compiled regexp")
	}
}

func TestValidatePattern_InvalidRegex(t *testing.T) {
	t.Parallel()

	_, err := ValidatePattern(`[invalid`)
	if err == nil {
		t.Fatal("want error for invalid pattern")
	}
	if !errors.Is(err, ErrInvalidPattern) {
		t.Fatalf("expected ErrInvalidPattern, got %v", err)
	}
}

func TestValidatePattern_EmptyPatternRejected(t *testing.T) {
	t.Parallel()

	_, err := ValidatePattern("")
	if err == nil {
		t.Fatal("want error for empty pattern")
	}
	if !errors.Is(err, ErrEmptyRegexpPattern) {
		t.Fatalf("expected ErrEmptyRegexpPattern, got %v", err)
	}
}

func TestValidatePattern_ControlCharsInPatternRejected(t *testing.T) {
	t.Parallel()

	pattern := "a\x01b"
	_, err := ValidatePattern(pattern)
	if err == nil {
		t.Fatal("want error for control byte in pattern")
	}

	if !errors.Is(err, ErrControlChar) {
		t.Fatalf("expected ErrControlChar, got %v", err)
	}
}

func TestValidatePattern_TooLong(t *testing.T) {
	t.Parallel()

	pattern := strings.Repeat("a", MaxPatternLength+1)
	_, err := ValidatePattern(pattern)
	if err == nil {
		t.Fatal("want error for overlong pattern")
	}

	if !errors.Is(err, ErrPatternTooLong) {
		t.Fatalf("expected ErrPatternTooLong, got %v", err)
	}
}

func TestValidateExtension(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		ext     string
		wantErr bool
	}{
		{name: "valid txt", ext: ".txt", wantErr: false},
		{name: "valid go", ext: ".go", wantErr: false},
		{name: "valid md", ext: ".md", wantErr: false},
		{name: "valid numbers", ext: ".7z", wantErr: false},
		{name: "no dot", ext: "txt", wantErr: true},
		{name: "empty", ext: "", wantErr: true},
		{name: "path separator forward", ext: ".t/xt", wantErr: true},
		{name: "path separator back", ext: ".t\\xt", wantErr: true},
		{name: "dot dot", ext: "..", wantErr: true},
		{name: "dot dot slash", ext: "../etc", wantErr: true},
		{name: "control char", ext: ".t\x01xt", wantErr: true},
		{name: "space", ext: ".t xt", wantErr: true},
		{name: "dash", ext: ".t-xt", wantErr: true},
		{name: "underscore", ext: ".t_xt", wantErr: true},
		{name: "dot only", ext: ".", wantErr: true},
		{name: "double extension", ext: ".tar.gz", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateExtension(tt.ext)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for %q", tt.ext)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.ext, err)
			}
			if tt.wantErr && err != nil && !errors.Is(err, ErrInvalidExtension) {
				t.Fatalf("expected ErrInvalidExtension, got %v", err)
			}
		})
	}
}

func TestValidatePath_FakeResolverTraversalOutsideRoot(t *testing.T) {
	t.Parallel()

	sep := string(filepath.Separator)
	root := sep + "testroot"
	path := sep + "outside" + sep + "file.txt"
	resolver := fakeResolver{
		lstatFn: func(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist },
		evalFn:  func(s string) (string, error) { return s, nil },
	}
	err := ValidatePath(path, root, resolver)
	if err == nil {
		t.Fatal("want error for traversal")
	}
}

func TestValidatePath_SymlinkEscapesRoot(t *testing.T) {
	t.Parallel()

	root := absTestPath(t, "testroot")
	path := filepath.Join(root, "link")
	outside := absTestPath(t, "outside")
	resolver := mapPathResolver(
		map[string]fs.FileInfo{
			root: fakeFileInfo{mode: fs.ModeDir},
			path: fakeFileInfo{mode: fs.ModeSymlink},
		},
		map[string]string{
			root: root,
			path: outside,
		},
		nil,
	)
	err := ValidatePath(path, root, resolver)
	if err == nil {
		t.Fatal("want error: symlink resolves outside root")
	}
}

func TestValidatePath_NonexistentPathUnderRoot(t *testing.T) {
	t.Parallel()

	sep := string(filepath.Separator)
	root := sep + "testroot"
	path := sep + "testroot" + sep + "newfile.txt"
	resolver := fakeResolver{
		lstatFn: func(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist },
		evalFn:  func(s string) (string, error) { return s, nil },
	}
	err := ValidatePath(path, root, resolver)
	if err != nil {
		t.Fatalf("want nil (path under root, not existent), got %v", err)
	}
}

func TestValidatePath_RejectsReservedNamePurely(t *testing.T) {
	t.Parallel()

	err := ValidatePathForOS("NUL")
	if !errors.Is(err, ErrReservedName) {
		t.Fatalf("expected ErrReservedName, got %v", err)
	}
}

func TestValidatePath_ResolvedRootDefinesBoundary(t *testing.T) {
	t.Parallel()

	root := absTestPath(t, "link-root")
	realRoot := absTestPath(t, "real-root")
	path := filepath.Join(root, "file.txt")
	realPath := filepath.Join(realRoot, "file.txt")

	resolver := mapPathResolver(
		map[string]fs.FileInfo{
			root: fakeFileInfo{mode: fs.ModeDir},
			path: fakeFileInfo{},
		},
		map[string]string{
			root: realRoot,
			path: realPath,
		},
		nil,
	)

	if err := ValidatePath(path, root, resolver); err != nil {
		t.Fatalf("resolved root boundary error = %v", err)
	}
}

func TestValidatePath_DoesNotInspectParentSymlinkAboveRoot(t *testing.T) {
	t.Parallel()

	ancestor := absTestPath(t, "ancestor")
	root := filepath.Join(ancestor, "workspace")
	path := filepath.Join(root, "missing", "file.txt")
	outside := absTestPath(t, "outside")
	var lstatCalls []string

	resolver := mapPathResolver(
		map[string]fs.FileInfo{
			ancestor: fakeFileInfo{mode: fs.ModeSymlink},
			root:     fakeFileInfo{mode: fs.ModeDir},
		},
		map[string]string{
			ancestor: outside,
			root:     root,
		},
		&lstatCalls,
	)

	if err := ValidatePath(path, root, resolver); err != nil {
		t.Fatalf("parent symlink above root error = %v", err)
	}

	for _, call := range lstatCalls {
		if call == ancestor {
			t.Fatalf("lstat inspected ancestor above root: calls=%v", lstatCalls)
		}
	}
}

func TestValidatePath_RootSymlinkResolvesToEffectiveBoundary(t *testing.T) {
	t.Parallel()

	root := absTestPath(t, "link-root")
	realRoot := absTestPath(t, "real-root")
	path := filepath.Join(root, "new.txt")

	resolver := mapPathResolver(
		map[string]fs.FileInfo{
			root: fakeFileInfo{mode: fs.ModeSymlink},
		},
		map[string]string{
			root: realRoot,
		},
		nil,
	)

	if err := ValidatePath(path, root, resolver); err != nil {
		t.Fatalf("root symlink boundary error = %v", err)
	}
}

func TestValidatePath_SymlinkBelowRootResolvingOutsideIsRejected(t *testing.T) {
	t.Parallel()

	root := absTestPath(t, "root")
	link := filepath.Join(root, "link")
	path := filepath.Join(link, "file.txt")
	outside := absTestPath(t, "outside")

	resolver := mapPathResolver(
		map[string]fs.FileInfo{
			root: fakeFileInfo{mode: fs.ModeDir},
			link: fakeFileInfo{mode: fs.ModeSymlink},
		},
		map[string]string{
			root: root,
			link: outside,
		},
		nil,
	)

	err := ValidatePath(path, root, resolver)
	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("escaping symlink error = %v, want ErrPathTraversal", err)
	}
}

func TestValidatePath_MissingDestinationUnderInsideParentIsAllowed(t *testing.T) {
	t.Parallel()

	root := absTestPath(t, "root")
	parent := filepath.Join(root, "existing")
	path := filepath.Join(parent, "new.txt")

	resolver := mapPathResolver(
		map[string]fs.FileInfo{
			root:   fakeFileInfo{mode: fs.ModeDir},
			parent: fakeFileInfo{mode: fs.ModeDir},
		},
		map[string]string{
			root:   root,
			parent: parent,
		},
		nil,
	)

	if err := ValidatePath(path, root, resolver); err != nil {
		t.Fatalf("missing destination under inside parent error = %v", err)
	}
}

func TestValidatePath_MissingDestinationUnderResolvedParentIsAllowed(t *testing.T) {
	t.Parallel()

	root := absTestPath(t, "link-root")
	realRoot := absTestPath(t, "real-root")
	parent := filepath.Join(root, "existing")
	realParent := filepath.Join(realRoot, "existing")
	path := filepath.Join(parent, "new.txt")

	resolver := mapPathResolver(
		map[string]fs.FileInfo{
			root:   fakeFileInfo{mode: fs.ModeDir},
			parent: fakeFileInfo{mode: fs.ModeDir},
		},
		map[string]string{
			root:   realRoot,
			parent: realParent,
		},
		nil,
	)

	if err := ValidatePath(path, root, resolver); err != nil {
		t.Fatalf("missing destination under resolved parent error = %v", err)
	}
}

func TestValidatePath_MissingDestinationUnderOutsideSymlinkParentIsRejected(t *testing.T) {
	t.Parallel()

	root := absTestPath(t, "root")
	link := filepath.Join(root, "escape")
	path := filepath.Join(link, "new.txt")
	outside := absTestPath(t, "outside")

	resolver := mapPathResolver(
		map[string]fs.FileInfo{
			root: fakeFileInfo{mode: fs.ModeDir},
			link: fakeFileInfo{mode: fs.ModeSymlink},
		},
		map[string]string{
			root: root,
			link: outside,
		},
		nil,
	)

	err := ValidatePath(path, root, resolver)
	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("missing destination outside symlink error = %v, want ErrPathTraversal", err)
	}
}

func absTestPath(t *testing.T, parts ...string) string {
	t.Helper()

	path := filepath.Join(append([]string{string(filepath.Separator)}, parts...)...)
	absPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("abs test path: %v", err)
	}

	return filepath.Clean(absPath)
}

func mapPathResolver(
	lstats map[string]fs.FileInfo,
	evals map[string]string,
	lstatCalls *[]string,
) fakeResolver {
	return fakeResolver{
		lstatFn: func(path string) (fs.FileInfo, error) {
			path = filepath.Clean(path)
			if lstatCalls != nil {
				*lstatCalls = append(*lstatCalls, path)
			}
			if info, ok := lstats[path]; ok {
				return info, nil
			}

			return nil, fs.ErrNotExist
		},
		evalFn: func(path string) (string, error) {
			path = filepath.Clean(path)
			if resolved, ok := evals[path]; ok {
				return filepath.Clean(resolved), nil
			}

			return path, nil
		},
	}
}
