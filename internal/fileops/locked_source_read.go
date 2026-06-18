package fileops

import (
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
)

// LockedSourceState captures immutable metadata observed while a shared lock is held.
// Callers may stash this across passes within one cooperative locking session.
type LockedSourceState struct {
	Size int64
}

// LockedSourceRead holds a shared advisory lock on a source file and exposes streaming reads.
// It avoids buffering the whole source into heap slices owned by fileops.
//
// Acquire ordering mirrors OpenForRead: OpenRead → shared advisory lock → Stat captures
// LockedSourceState while cooperative locking guarantees consistency vs renaming peers.
// The underlying cursor begins at offset 0 unless OpenRead leaves another offset —
// callers should not assume EOF positioning beyond SeekStart semantics documented by io.Reader.
//
// Close releases the advisory shared lock and closes the underlying handle. Until Close completes,
// cooperating writers attempting exclusive locks block — identical cooperative semantics as ReadHandle
// from OpenForRead.
//
// On Unix, advisory locks do not prevent non-cooperating processes from modifying the file;
// later pipeline phases must layer CAS/hash verification when correctness demands detecting hostile
// mutation — locking alone cannot enforce integrity across unrelated writers.
//
// LockedSourceRead must not be used concurrently from multiple goroutines unless external
// synchronization mirrors ordinary file handle constraints for serialized reads + Seek coordination.
type LockedSourceRead struct {
	handle SessionReadHandle

	State LockedSourceState
}

var _ io.ReadSeekCloser = (*LockedSourceRead)(nil)

// OpenLockedSourceRead opens path via FileSession.OpenRead, acquires lockShared, records
// LockedSourceState (Stat while locked), then returns a streaming handle implementing
// io.ReadSeekCloser without invoking io.ReadAll on the source.
//
// Use Close to release the shared lock — unlocking earlier would violate cooperative locking
// contracts exercised by pipeline orchestration.
func OpenLockedSourceRead(path string, fs FileSession) (*LockedSourceRead, error) {
	if fs == nil {
		return nil, ErrNilFileSession
	}

	opened, err := fs.OpenRead(path)
	if err != nil {
		return nil, fmt.Errorf("locked source open read: %w", err)
	}

	if lockErr := lockSharedOn(opened); lockErr != nil {
		_ = opened.Close()
		return nil, fmt.Errorf("locked source shared lock: %w", lockErr)
	}

	state, stateErr := lockedSourceStateAtStart(opened)
	if stateErr != nil {
		_ = unlockReadHandle(opened)
		_ = opened.Close()
		return nil, stateErr
	}

	return &LockedSourceRead{
		handle: opened,
		State:  state,
	}, nil
}

func lockedSourceStateAtStart(opened SessionReadHandle) (LockedSourceState, error) {
	fi, statErr := opened.Stat()
	if statErr != nil {
		return LockedSourceState{}, fmt.Errorf("locked source stat: %w", statErr)
	}

	state := LockedSourceState{
		Size: fi.Size(),
	}

	if _, seekErr := opened.Seek(0, io.SeekStart); seekErr != nil {
		return LockedSourceState{}, fmt.Errorf("locked source seek start: %w", seekErr)
	}

	return state, nil
}

// Read streams bytes from the locked source without buffering the entire file through fileops-managed slices.
func (l *LockedSourceRead) Read(p []byte) (int, error) {
	if l.handle == nil {
		return 0, iofs.ErrClosed
	}

	return l.handle.Read(p)
}

// Seek sets the position for the next Read relative to the locked handle.
func (l *LockedSourceRead) Seek(offset int64, whence int) (int64, error) {
	if l.handle == nil {
		return 0, iofs.ErrClosed
	}

	return l.handle.Seek(offset, whence)
}

// Close releases the advisory shared lock then closes the underlying handle.
func (l *LockedSourceRead) Close() error {
	if l.handle == nil {
		return nil
	}

	unlockErr := unlockReadHandle(l.handle)
	closeErr := l.handle.Close()
	l.handle = nil

	if unlockErr != nil && closeErr != nil {
		return errors.Join(
			fmt.Errorf("locked source unlock: %w", unlockErr),
			fmt.Errorf("locked source close: %w", closeErr),
		)
	}

	if unlockErr != nil {
		return fmt.Errorf("locked source unlock: %w", unlockErr)
	}

	return closeErr
}
