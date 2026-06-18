// User vision: production session backends are thin host filesystem syscall adapters.
package fileops

import (
	iofs "io/fs"
	"os"
)

type osSessionBackend struct{}

var _ SessionBackend = osSessionBackend{}

func (osSessionBackend) OpenRead(path string) (SessionReadHandle, error) {
	f, err := os.Open(path) //nolint:gosec // G304: path is caller-supplied file to read
	if err != nil {
		return nil, err
	}

	return &osReadHandle{f: f}, nil
}

func (osSessionBackend) OpenRDWR(path string) (SessionRDWRHandle, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0) //nolint:gosec // G304: path is caller-supplied file to modify
	if err != nil {
		return nil, err
	}

	return &osRDWRHandle{f: f}, nil
}

func (osSessionBackend) CreateTemp(dir, pattern string) (SessionTempHandle, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}

	return &osTempHandle{f: f}, nil
}

func (osSessionBackend) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (osSessionBackend) Chmod(name string, mode iofs.FileMode) error {
	return os.Chmod(name, os.FileMode(mode))
}

func (osSessionBackend) Remove(name string) error {
	return os.Remove(name)
}
