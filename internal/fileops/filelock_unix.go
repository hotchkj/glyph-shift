//go:build unix

package fileops

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

const maxFileDescriptorInt = int(^uint(0) >> 1)

var errFileDescriptorOverflow = errors.New("file descriptor exceeds int range")

func fileDescriptorInt(f *os.File) (int, error) {
	fd := f.Fd()
	if fd > uintptr(maxFileDescriptorInt) {
		return 0, errFileDescriptorOverflow
	}
	return int(fd), nil
}

func lockFile(f *os.File, how int) error {
	fd, err := fileDescriptorInt(f)
	if err != nil {
		return err
	}
	return unix.Flock(fd, how)
}

func lockShared(f *os.File) error {
	return lockFile(f, unix.LOCK_SH)
}

func lockExclusive(f *os.File) error {
	return lockFile(f, unix.LOCK_EX)
}

func unlock(f *os.File) error {
	return lockFile(f, unix.LOCK_UN)
}
