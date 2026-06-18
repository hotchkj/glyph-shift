// stat.go defines the FileStater seam used by the pipeline to query file
// metadata. Production code is backed by os.Stat; tests inject an in-memory
// implementation to avoid real filesystem access.
package pipeline

import (
	"errors"
	"io/fs"
	"os"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

// FileStater stats a file. It follows symlinks (like os.Stat, not os.Lstat).
// Inject a fake implementation in tests to avoid real filesystem access.
type FileStater interface {
	// Stat returns the FileInfo for the named file.
	Stat(path string) (fs.FileInfo, error)
}

// StatBackend is the low-level Stat primitive injected into fileStater.
// Implement this interface to provide an alternative filesystem backend.
type StatBackend interface {
	// Stat returns the FileInfo for the named file.
	Stat(path string) (fs.FileInfo, error)
}

// osStatBackend is the production StatBackend backed by the real OS.
type osStatBackend struct{}

func (osStatBackend) Stat(path string) (fs.FileInfo, error) {
	return statOS(path)
}

// fileStater wraps any StatBackend to produce a FileStater.
type fileStater struct {
	backend StatBackend
}

// ErrNilStatBackend is returned when a FileStater is constructed without a backend.
var ErrNilStatBackend = errors.New("pipeline: nil StatBackend")

// NewFileStater returns a FileStater backed by the given StatBackend.
// backend must be a non-nil, initialized implementation; typed-nil pointer values
// stored in the interface are not detected here and will panic on first delegate call.
func NewFileStater(backend StatBackend) (FileStater, error) {
	if backend == nil {
		return nil, ErrNilStatBackend
	}

	return fileStater{backend: backend}, nil
}

func (f fileStater) Stat(path string) (fs.FileInfo, error) {
	if err := fileops.RejectNULByteInPath(path); err != nil {
		return nil, err
	}

	return f.backend.Stat(path)
}

// NewOSFileStater returns a FileStater backed by real os.Stat calls.
func NewOSFileStater() FileStater {
	stater, err := NewFileStater(osStatBackend{})
	if err != nil {
		panic(err)
	}

	return stater
}

func statOS(path string) (fs.FileInfo, error) {
	info, err := os.Stat(path) //nolint:gosec // G304: path is caller-validated before Stat
	if err != nil {
		return nil, err
	}

	return info, nil
}
