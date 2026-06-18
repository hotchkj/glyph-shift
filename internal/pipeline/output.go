package pipeline

import (
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

const (
	// DirPerm is the permission used when creating destination directories.
	DirPerm = 0o750
	// FilePerm is the permission used when creating destination files.
	FilePerm = 0o644
)

// OutputOpener opens or creates a destination file for writing.
// Inject a fake implementation in tests to avoid real filesystem access.
type OutputOpener interface {
	MkdirAll(path string, perm fs.FileMode) error
	OpenFile(path string, intent OutputWriteIntent, perm fs.FileMode) (io.WriteCloser, error)
}

// OutputBackend is the injection seam for OutputOpener — platform syscalls for directory creation and file opening.
type OutputBackend interface {
	MkdirAll(path string, perm fs.FileMode) error
	OpenFile(path string, intent OutputWriteIntent, perm fs.FileMode) (io.WriteCloser, error)
}

// osOutputBackend is the production OutputBackend backed by real OS syscalls.
type osOutputBackend struct{}

var _ OutputBackend = osOutputBackend{}

func (osOutputBackend) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, os.FileMode(perm))
}

func (osOutputBackend) OpenFile(
	path string,
	intent OutputWriteIntent,
	perm fs.FileMode,
) (io.WriteCloser, error) {
	return openFileOS(path, intent, perm)
}

// outputOpener delegates OutputOpener calls to an injected OutputBackend.
type outputOpener struct {
	backend OutputBackend
}

// ErrNilOutputBackend is returned when an OutputOpener is constructed without a backend.
var ErrNilOutputBackend = errors.New("pipeline: nil OutputBackend")

// NewOutputOpener wraps a backend as an OutputOpener.
func NewOutputOpener(backend OutputBackend) (OutputOpener, error) {
	if backend == nil {
		return nil, ErrNilOutputBackend
	}

	return &outputOpener{backend: backend}, nil
}

// NewOSOutputOpener returns an OutputOpener backed by real os calls.
func NewOSOutputOpener() OutputOpener {
	opener, err := NewOutputOpener(osOutputBackend{})
	if err != nil {
		panic(err)
	}

	return opener
}

func (o *outputOpener) MkdirAll(path string, perm fs.FileMode) error {
	if err := fileops.RejectNULByteInPath(path); err != nil {
		return err
	}

	return o.backend.MkdirAll(path, perm)
}

func (o *outputOpener) OpenFile(path string, intent OutputWriteIntent, perm fs.FileMode) (io.WriteCloser, error) {
	if err := fileops.RejectNULByteInPath(path); err != nil {
		return nil, err
	}

	return o.backend.OpenFile(path, intent, perm)
}

// intentToOSFlags maps known [OutputWriteIntent] values to OS open flags.
// Unknown intents fall through to create-or-replace (O_TRUNC): callers outside this
// package should only pass the three defined constants; the default is defensive
// against corrupt or future enum values without silently appending.
func intentToOSFlags(intent OutputWriteIntent) int {
	switch intent {
	case OutputCreateExclusive:
		return os.O_WRONLY | os.O_CREATE | os.O_EXCL
	case OutputCreateOrReplace:
		return os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	case OutputAppend:
		return os.O_WRONLY | os.O_CREATE | os.O_APPEND
	default:
		return os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
}

func openFileOS(path string, intent OutputWriteIntent, perm fs.FileMode) (io.WriteCloser, error) {
	flags := intentToOSFlags(intent)
	f, err := os.OpenFile(path, flags, os.FileMode(perm)) //nolint:gosec // G304: path is caller-validated before OpenFile
	if err != nil {
		return nil, err
	}

	return f, nil
}
