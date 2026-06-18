package validate

import (
	"errors"
	"io/fs"
	"testing"
	"time"
)

func TestRejectNULByteInPath(t *testing.T) {
	t.Parallel()

	if err := rejectNULByteInPath("ok"); err != nil {
		t.Fatalf("clean path: %v", err)
	}

	if err := rejectNULByteInPath(string([]byte{'a', 0, 'b'})); !errors.Is(err, ErrPathContainsNUL) {
		t.Fatalf("NUL path: got %v want ErrPathContainsNUL", err)
	}
}

var errRejectingResolverBackendCalled = errors.New("rejecting resolver backend called")

type rejectingResolverBackend struct{}

func (rejectingResolverBackend) Lstat(string) (fs.FileInfo, error) {
	return nil, errRejectingResolverBackendCalled
}

func (rejectingResolverBackend) EvalSymlinks(string) (string, error) {
	return "", errRejectingResolverBackendCalled
}

func TestPathResolverRejectsNULBeforeBackend(t *testing.T) {
	t.Parallel()

	invalidPath := string([]byte{0})
	resolver, err := NewPathResolver(rejectingResolverBackend{})
	if err != nil {
		t.Fatalf("NewPathResolver: %v", err)
	}

	if _, err := resolver.Lstat(invalidPath); !errors.Is(err, ErrPathContainsNUL) {
		t.Fatalf("Lstat: got %v want ErrPathContainsNUL", err)
	}

	if _, err := resolver.EvalSymlinks(invalidPath); !errors.Is(err, ErrPathContainsNUL) {
		t.Fatalf("EvalSymlinks: got %v want ErrPathContainsNUL", err)
	}
}

type resolverStubFileInfo struct{}

func (resolverStubFileInfo) Name() string       { return "file.txt" }
func (resolverStubFileInfo) Size() int64        { return 1 }
func (resolverStubFileInfo) Mode() fs.FileMode  { return 0o644 }
func (resolverStubFileInfo) ModTime() time.Time { return time.Time{} }
func (resolverStubFileInfo) IsDir() bool        { return false }
func (resolverStubFileInfo) Sys() any           { return nil }

type recordingResolverBackend struct {
	lstatPath string
	evalPath  string
}

func (b *recordingResolverBackend) Lstat(path string) (fs.FileInfo, error) {
	b.lstatPath = path
	return resolverStubFileInfo{}, nil
}

func (b *recordingResolverBackend) EvalSymlinks(path string) (string, error) {
	b.evalPath = path
	return path, nil
}

func TestPathResolverDelegatesValidPathsToBackend(t *testing.T) {
	t.Parallel()

	backend := &recordingResolverBackend{}
	resolver, err := NewPathResolver(backend)
	if err != nil {
		t.Fatalf("NewPathResolver: %v", err)
	}

	if _, err := resolver.Lstat("/src.txt"); err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if backend.lstatPath != "/src.txt" {
		t.Fatalf("Lstat path = %q", backend.lstatPath)
	}

	if _, err := resolver.EvalSymlinks("/link.txt"); err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if backend.evalPath != "/link.txt" {
		t.Fatalf("EvalSymlinks path = %q", backend.evalPath)
	}
}

func TestNewPathResolverRejectsNilBackend(t *testing.T) {
	t.Parallel()

	_, err := NewPathResolver(nil)
	if !errors.Is(err, ErrNilResolverBackend) {
		t.Fatalf("NewPathResolver nil backend = %v, want ErrNilResolverBackend", err)
	}
}
