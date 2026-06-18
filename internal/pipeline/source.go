package pipeline

import (
	"io"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

// SourceOpener opens a source file for reading.
// Inject a fake implementation in tests to avoid real filesystem access.
type SourceOpener interface {
	// Open opens the named file for reading.
	Open(path string) (io.ReadSeekCloser, error)
}

// osSourceOpener is the production SourceOpener: source reads and cooperative
// locking go through an injected FileSession (same publication session as the runner).
type osSourceOpener struct {
	fs fileops.FileSession
}

// NewOSSourceOpener returns a SourceOpener that opens sources via LockedSourceRead using fs.
func NewOSSourceOpener(fs fileops.FileSession) (SourceOpener, error) {
	if fs == nil {
		return nil, fileops.ErrNilFileSession
	}

	return osSourceOpener{fs: fs}, nil
}

func (o osSourceOpener) Open(path string) (io.ReadSeekCloser, error) {
	return fileops.OpenLockedSourceRead(path, o.fs)
}
