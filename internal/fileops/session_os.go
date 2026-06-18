// OS file-descriptor handle types for the production SessionBackend.
package fileops

import (
	"io"
	iofs "io/fs"
	"os"
)

// osReadHandle wraps a production read-only OS file descriptor.
type osReadHandle struct {
	f *os.File
}

func (h *osReadHandle) Read(p []byte) (int, error) {
	return h.f.Read(p)
}

func (h *osReadHandle) Seek(offset int64, whence int) (int64, error) {
	return h.f.Seek(offset, whence)
}

func (h *osReadHandle) Close() error {
	return h.f.Close()
}

func (h *osReadHandle) Stat() (iofs.FileInfo, error) {
	return h.f.Stat()
}

func (h *osReadHandle) LockShared() error {
	return lockShared(h.f)
}

func (h *osReadHandle) LockExclusive() error {
	return lockExclusive(h.f)
}

func (h *osReadHandle) Unlock() error {
	return unlock(h.f)
}

// osRDWRHandle wraps a production read/write OS file descriptor.
type osRDWRHandle struct {
	f *os.File
}

func (h *osRDWRHandle) Read(p []byte) (int, error) {
	return h.f.Read(p)
}

func (h *osRDWRHandle) Write(p []byte) (int, error) {
	return h.f.Write(p)
}

func (h *osRDWRHandle) WriteAt(p []byte, off int64) (int, error) {
	return h.f.WriteAt(p, off)
}

func (h *osRDWRHandle) Seek(offset int64, whence int) (int64, error) {
	return h.f.Seek(offset, whence)
}

func (h *osRDWRHandle) Close() error {
	return h.f.Close()
}

func (h *osRDWRHandle) Stat() (iofs.FileInfo, error) {
	return h.f.Stat()
}

func (h *osRDWRHandle) Sync() error {
	return h.f.Sync()
}

func (h *osRDWRHandle) LockShared() error {
	return lockShared(h.f)
}

func (h *osRDWRHandle) LockExclusive() error {
	return lockExclusive(h.f)
}

func (h *osRDWRHandle) Unlock() error {
	return unlock(h.f)
}

var (
	_ SessionReadHandle = (*osReadHandle)(nil)
	_ SessionRDWRHandle = (*osRDWRHandle)(nil)
	_ AdvisoryLocker    = (*osReadHandle)(nil)
	_ AdvisoryLocker    = (*osRDWRHandle)(nil)
	_ io.ReadSeekCloser = (*osReadHandle)(nil)
)
