// User vision: modifier and read helpers depend on an injectable file session contract, not host syscalls directly.
package fileops

import (
	"errors"
	"io"
	iofs "io/fs"
)

// AtomicPublishStagingDecorator is optionally implemented by concrete [FileSession] types that
// decorate the staging payload [io.Writer] used inside [AtomicPublish] before serialized bytes
// reach the temporary file.
//
// Production [FileSession] implementations do not implement this interface; it is an opt-in seam
// layered on top of the standard session contract.
type AtomicPublishStagingDecorator interface {
	WrapAtomicPublishStagingWriter(finalDestinationPath string, w io.Writer) io.Writer
}

// FileSession abstracts file operations used by Modifier and read helpers.
// Inject a fake implementation in tests to avoid real filesystem access.
type FileSession interface {
	OpenRead(path string) (SessionReadHandle, error)
	OpenRDWR(path string) (SessionRDWRHandle, error)
	CreateTemp(dir, pattern string) (SessionTempHandle, error)
	Remove(name string) error
	Rename(oldpath, newpath string) error
	Chmod(name string, mode iofs.FileMode) error
}

type fileSession struct {
	backend SessionBackend
}

// ErrNilSessionBackend is returned when a FileSession is constructed without a backend.
var ErrNilSessionBackend = errors.New("fileops: nil SessionBackend")

// NewFileSession returns a FileSession backed by the supplied SessionBackend.
// backend must be a non-nil, initialized implementation; typed-nil pointer values
// stored in the interface (for example (*customBackend)(nil)) are not detected here
// and will panic on first delegate call.
func NewFileSession(backend SessionBackend) (FileSession, error) {
	if backend == nil {
		return nil, ErrNilSessionBackend
	}

	return &fileSession{backend: backend}, nil
}

// NewOSFileSession returns a FileSession backed by real os calls.
func NewOSFileSession() FileSession {
	session, err := NewFileSession(osSessionBackend{})
	if err != nil {
		panic(err)
	}

	return session
}

// Backend returns the underlying SessionBackend for optional capability checks.
func (s *fileSession) Backend() SessionBackend {
	return s.backend
}

func (s *fileSession) OpenRead(path string) (SessionReadHandle, error) {
	if err := RejectNULByteInPath(path); err != nil {
		return nil, err
	}

	return s.backend.OpenRead(path)
}

func (s *fileSession) OpenRDWR(path string) (SessionRDWRHandle, error) {
	if err := RejectNULByteInPath(path); err != nil {
		return nil, err
	}

	return s.backend.OpenRDWR(path)
}

func (s *fileSession) CreateTemp(dir, pattern string) (SessionTempHandle, error) {
	if err := RejectNULByteInPath(dir); err != nil {
		return nil, err
	}
	if err := RejectNULByteInPath(pattern); err != nil {
		return nil, err
	}

	return s.backend.CreateTemp(dir, pattern)
}

func (s *fileSession) Remove(name string) error {
	if err := RejectNULByteInPath(name); err != nil {
		return err
	}

	return s.backend.Remove(name)
}

// Rename moves oldpath to newpath through the injected backend seam. Used for modifier temp
// commits and AtomicPublish.
func (s *fileSession) Rename(oldpath, newpath string) error {
	if err := RejectNULByteInPath(oldpath); err != nil {
		return err
	}
	if err := RejectNULByteInPath(newpath); err != nil {
		return err
	}

	return s.backend.Rename(oldpath, newpath)
}

func (s *fileSession) Chmod(name string, mode iofs.FileMode) error {
	if err := RejectNULByteInPath(name); err != nil {
		return err
	}

	return s.backend.Chmod(name, mode)
}
