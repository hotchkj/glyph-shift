// User vision: path resolution depends on an injectable backend, not host syscalls directly.
package validate

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// ResolverBackend is the low-level filesystem seam injected into a PathResolver.
// Implementations provide raw Lstat and EvalSymlinks without NUL-byte guarding;
// guarding is applied by pathResolver before delegating.
type ResolverBackend interface {
	Lstat(path string) (fs.FileInfo, error)
	EvalSymlinks(path string) (string, error)
}

// PathResolver abstracts OS filesystem calls used during path validation.
// Inject a fake in tests to avoid real filesystem access.
type PathResolver interface {
	Lstat(path string) (fs.FileInfo, error)
	EvalSymlinks(path string) (string, error)
}

// pathResolver wraps a ResolverBackend and adds NUL-byte guarding.
type pathResolver struct {
	backend ResolverBackend
}

// ErrNilResolverBackend is returned when a PathResolver is constructed without a backend.
var ErrNilResolverBackend = errors.New("validate: nil ResolverBackend")

// NewPathResolver returns a PathResolver that guards calls with NUL-byte
// rejection before delegating to backend. backend must be a non-nil,
// initialized implementation; typed-nil pointer values stored in the
// interface are not detected here and will panic on first delegate call.
func NewPathResolver(backend ResolverBackend) (PathResolver, error) {
	if backend == nil {
		return nil, ErrNilResolverBackend
	}

	return pathResolver{backend: backend}, nil
}

func (r pathResolver) Lstat(path string) (fs.FileInfo, error) {
	if err := rejectNULByteInPath(path); err != nil {
		return nil, err
	}

	return r.backend.Lstat(path)
}

func (r pathResolver) EvalSymlinks(path string) (string, error) {
	if err := rejectNULByteInPath(path); err != nil {
		return "", err
	}

	return r.backend.EvalSymlinks(path)
}

// osResolverBackend is the production ResolverBackend backed by real os calls.
type osResolverBackend struct{}

func (osResolverBackend) Lstat(path string) (fs.FileInfo, error) {
	return os.Lstat(path)
}

func (osResolverBackend) EvalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

// NewOSPathResolver returns a PathResolver backed by real os calls.
func NewOSPathResolver() PathResolver {
	resolver, err := NewPathResolver(osResolverBackend{})
	if err != nil {
		panic(err)
	}

	return resolver
}
