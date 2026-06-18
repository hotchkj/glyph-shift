//go:build windows

package fileops

import (
	"os"

	"golang.org/x/sys/windows"
)

const allBytes = ^uint32(0)

func lockShared(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		0,
		0,
		allBytes,
		allBytes,
		ol,
	)
}

func lockExclusive(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK,
		0,
		allBytes,
		allBytes,
		ol,
	)
}

func unlock(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.UnlockFileEx(
		windows.Handle(f.Fd()),
		0,
		allBytes,
		allBytes,
		ol,
	)
}
