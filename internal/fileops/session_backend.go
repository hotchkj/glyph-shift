// User vision: FileSession logic stays platform-agnostic by injecting syscall-level backends for production and tests.
//
// SessionBackend abstracts the syscall-level operations behind FileSession.
package fileops

import iofs "io/fs"

// SessionBackend is the injection seam between FileSession logic and platform syscalls.
type SessionBackend interface {
	OpenRead(path string) (SessionReadHandle, error)
	OpenRDWR(path string) (SessionRDWRHandle, error)
	CreateTemp(dir, pattern string) (SessionTempHandle, error)
	Remove(name string) error
	Rename(oldpath, newpath string) error
	Chmod(name string, mode iofs.FileMode) error
}
