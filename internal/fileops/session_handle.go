// User vision: unit tests exercise the same file session contract as production without host filesystem handles.
package fileops

import (
	"io"
	iofs "io/fs"
)

// SessionReadHandle is returned by [FileSession.OpenRead]. Production implementations support
// advisory locking via [AdvisoryLocker]; in-memory fakes use deterministic no-op locks.
type SessionReadHandle interface {
	io.ReadSeekCloser
	Stat() (iofs.FileInfo, error)
}

// SessionRDWRHandle is returned by [FileSession.OpenRDWR] for modifier and transform flows.
type SessionRDWRHandle interface {
	io.Reader
	io.Writer
	io.WriterAt
	io.Seeker
	io.Closer
	Stat() (iofs.FileInfo, error)
	Sync() error
}

// SessionTempHandle is returned by [FileSession.CreateTemp] for atomic publish and safe commit temps.
type SessionTempHandle interface {
	io.Writer
	Sync() error
	Close() error
	Name() string
}

// AdvisoryLocker is implemented by production session handles that map to OS file descriptors.
// In-memory fakes implement the same interface with deterministic no-op locks.
type AdvisoryLocker interface {
	LockShared() error
	LockExclusive() error
	Unlock() error
}

func lockSharedOn(h SessionReadHandle) error {
	if l, ok := h.(AdvisoryLocker); ok {
		return l.LockShared()
	}

	return nil
}

func lockExclusiveOn(h SessionRDWRHandle) error {
	if l, ok := h.(AdvisoryLocker); ok {
		return l.LockExclusive()
	}

	return nil
}

func unlockReadHandle(h SessionReadHandle) error {
	if l, ok := h.(AdvisoryLocker); ok {
		return l.Unlock()
	}

	return nil
}

func unlockRDWRHandle(h SessionRDWRHandle) error {
	if l, ok := h.(AdvisoryLocker); ok {
		return l.Unlock()
	}

	return nil
}
