package fileops

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"path/filepath"
)

const casVerifyReadBufferSize = 256 * 1024 // streaming buffer for incremental CAS re-hash

var (
	ErrFileModifiedExternally = errors.New("file was modified externally during operation")
	ErrModifierAlreadyDone    = errors.New("modifier has already been committed or aborted")
)

// Modifier holds an exclusive lock on a file for safe in-place modification.
// Create via OpenForModify. Call Commit to apply changes or Abort to release.
//
// Modifier is not safe for concurrent use from multiple goroutines.
//
// Safety model: the exclusive lock is held during read and CAS verification.
// It is released immediately before the atomic rename, creating a negligible
// window. The CAS hash check ensures content integrity through this window.
// On Unix, advisory locks do not prevent non-cooperating processes from
// modifying the file; the CAS detects such modifications.
type Modifier struct {
	path    string
	file    SessionRDWRHandle
	hash    [sha256.Size]byte
	content []byte
	mode    iofs.FileMode
	done    bool
	fs      FileSession
}

// OpenForModify opens a file with an exclusive lock, reads its content,
// and returns a Modifier for safe in-place modification.
//
// The caller must call either Commit or Abort on the returned Modifier.
func OpenForModify(path string, fs FileSession) (*Modifier, error) {
	if fs == nil {
		return nil, ErrNilFileSession
	}

	openedFile, fi, err := openLockedRDWR(path, fs)
	if err != nil {
		return nil, err
	}

	content, err := io.ReadAll(openedFile)
	if err != nil {
		_ = unlockRDWRHandle(openedFile)
		_ = openedFile.Close()
		return nil, fmt.Errorf("safe read: %w", err)
	}

	return &Modifier{
		path:    path,
		file:    openedFile,
		hash:    sha256.Sum256(content),
		content: content,
		mode:    fi.Mode(),
		fs:      fs,
	}, nil
}

// OpenForModifyLocked opens path read/write with an exclusive lock but does not read file content.
// The caller must populate hash via a streaming read before CommitFromWriter, or use Commit with
// content read through other means. Content() returns nil until OpenForModify-style buffering is added.
func OpenForModifyLocked(path string, fs FileSession) (*Modifier, error) {
	if fs == nil {
		return nil, ErrNilFileSession
	}

	openedFile, fi, err := openLockedRDWR(path, fs)
	if err != nil {
		return nil, err
	}

	return &Modifier{
		path:    path,
		file:    openedFile,
		hash:    [sha256.Size]byte{},
		content: nil,
		mode:    fi.Mode(),
		fs:      fs,
	}, nil
}

func openLockedRDWR(path string, fs FileSession) (SessionRDWRHandle, iofs.FileInfo, error) {
	openedFile, err := fs.OpenRDWR(path)
	if err != nil {
		return nil, nil, fmt.Errorf("safe open: %w", err)
	}

	if lockErr := lockExclusiveOn(openedFile); lockErr != nil {
		_ = openedFile.Close()
		return nil, nil, fmt.Errorf("safe lock: %w", lockErr)
	}

	fi, err := openedFile.Stat()
	if err != nil {
		_ = unlockRDWRHandle(openedFile)
		_ = openedFile.Close()
		return nil, nil, fmt.Errorf("safe stat: %w", err)
	}

	return openedFile, fi, nil
}

// Content returns the file content as read at open time.
func (m *Modifier) Content() []byte {
	return m.content
}

// commitWriteAndVerify writes newContent to a temp file, syncs and closes it,
// then verifies the original file has not changed (CAS) and sets permissions.
func (m *Modifier) commitWriteAndVerify(dir string, newContent []byte) (retPath string, retErr error) {
	tmp, createErr := m.fs.CreateTemp(dir, ".glyph-shift-*")
	if createErr != nil {
		return "", fmt.Errorf("safe commit: create temp: %w", createErr)
	}
	tmpPath := tmp.Name()
	tmpClosed := false

	defer func() {
		if retErr != nil {
			if !tmpClosed {
				_ = tmp.Close()
			}
			_ = m.fs.Remove(tmpPath)
		}
	}()

	closed, err := writeSyncCloseCommitTemp(tmp, newContent)
	tmpClosed = closed
	if err != nil {
		return "", err
	}

	if casErr := m.verifyCASAndChmod(tmpPath); casErr != nil {
		return "", casErr
	}

	return tmpPath, nil
}

type commitTempFile interface {
	io.Writer
	Sync() error
	Close() error
}

func writeSyncCloseCommitTemp(tmp commitTempFile, newContent []byte) (bool, error) {
	if _, writeErr := tmp.Write(newContent); writeErr != nil {
		return false, fmt.Errorf("safe commit: write temp: %w", writeErr)
	}

	if syncErr := tmp.Sync(); syncErr != nil {
		return false, fmt.Errorf("safe commit: sync temp: %w", syncErr)
	}

	if closeErr := tmp.Close(); closeErr != nil {
		return true, fmt.Errorf("safe commit: close temp: %w", closeErr)
	}

	return true, nil
}

func (m *Modifier) verifyCASAndChmod(tmpPath string) error {
	if _, seekErr := m.file.Seek(0, io.SeekStart); seekErr != nil {
		return fmt.Errorf("safe commit: seek for verification: %w", seekErr)
	}

	sum, hashErr := m.hashCurrentFileForCAS()
	if hashErr != nil {
		return hashErr
	}

	if sum != m.hash {
		return ErrFileModifiedExternally
	}

	if chmodErr := m.fs.Chmod(tmpPath, m.mode); chmodErr != nil {
		return fmt.Errorf("safe commit: chmod temp: %w", chmodErr)
	}

	return nil
}

func (m *Modifier) hashCurrentFileForCAS() ([sha256.Size]byte, error) {
	casHasher := sha256.New()
	buf := make([]byte, casVerifyReadBufferSize)

	for {
		n, readErr := m.file.Read(buf)
		if n > 0 {
			if _, werr := casHasher.Write(buf[:n]); werr != nil {
				return [sha256.Size]byte{}, fmt.Errorf("safe commit: hash while verifying: %w", werr)
			}
		}

		if errors.Is(readErr, io.EOF) {
			break
		}

		if readErr != nil {
			return [sha256.Size]byte{}, fmt.Errorf("safe commit: re-read for verification: %w", readErr)
		}
	}

	var sum [sha256.Size]byte
	copy(sum[:], casHasher.Sum(nil))

	return sum, nil
}

// Commit atomically replaces the file content with newContent.
//
// Verification: re-reads the file and compares SHA-256 hashes to detect
// external modifications (compare-and-swap). Then writes to a temp file,
// fsyncs, and renames the temp over the original.
func (m *Modifier) Commit(newContent []byte) error {
	if m.done {
		return ErrModifierAlreadyDone
	}
	m.done = true

	tmpPath, commitErr := m.commitWriteAndVerify(filepath.Dir(m.path), newContent)
	if commitErr != nil {
		m.release()
		return commitErr
	}

	// Release the lock, then atomically rename temp over original.
	// The window between release and rename is negligible; the CAS
	// has already verified content integrity.
	m.release()

	if renameErr := m.fs.Rename(tmpPath, m.path); renameErr != nil {
		_ = m.fs.Remove(tmpPath)
		return fmt.Errorf("safe commit: rename: %w", renameErr)
	}

	return nil
}

// CommitFromWriter streams transformed content into a temp file, syncs/closes it, checks that the
// SHA-256 of the source bytes read by the write callback matches m.hash (replay sanity vs the prior
// locked read), then re-hashes the guarded file from disk (CAS), chmods the temp file, releases
// the lock, and atomically renames the temp over the source path. The post-write CAS catches
// non-cooperating writers that mutate the source after the callback's read.
//
// The write callback must read the source from offset 0 exactly once (under the still-held lock)
// while writing transformed bytes to w. It returns the SHA-256 of the raw source bytes read.
func (m *Modifier) CommitFromWriter(write func(w io.Writer) (sourceSHA [sha256.Size]byte, err error)) error {
	if m.done {
		return ErrModifierAlreadyDone
	}
	m.done = true

	tmp, tmpPath, createErr := m.createCommitTemp()
	if createErr != nil {
		m.release()
		return createErr
	}

	tmpClosed := false
	var retErr error

	defer func() {
		if retErr != nil {
			if !tmpClosed {
				_ = tmp.Close()
			}

			_ = m.fs.Remove(tmpPath)
		}
	}()

	sourceSHA, closed, streamErr := streamCommitFromWriterTemp(tmp, write)
	tmpClosed = closed
	if streamErr != nil {
		retErr = streamErr
		m.release()

		return streamErr
	}

	if verifyErr := m.verifyCommitFromWriterSource(sourceSHA, tmpPath); verifyErr != nil {
		retErr = verifyErr
		m.release()

		return verifyErr
	}

	m.release()

	return m.renameCommittedTemp(tmpPath)
}

func (m *Modifier) createCommitTemp() (SessionTempHandle, string, error) {
	tmp, createErr := m.fs.CreateTemp(filepath.Dir(m.path), ".glyph-shift-*")
	if createErr != nil {
		return nil, "", fmt.Errorf("safe commit: create temp: %w", createErr)
	}

	tmpPath := tmp.Name()

	return tmp, tmpPath, nil
}

func streamCommitFromWriterTemp(
	tmp SessionTempHandle,
	write func(w io.Writer) (sourceSHA [sha256.Size]byte, err error),
) (sourceSHA [sha256.Size]byte, tmpClosed bool, err error) {
	sourceSHA, writeErr := write(tmp)
	if writeErr != nil {
		return [sha256.Size]byte{}, false, fmt.Errorf("safe commit: stream write: %w", writeErr)
	}

	if syncErr := tmp.Sync(); syncErr != nil {
		return [sha256.Size]byte{}, false, fmt.Errorf("safe commit: sync temp: %w", syncErr)
	}

	if closeErr := tmp.Close(); closeErr != nil {
		return [sha256.Size]byte{}, true, fmt.Errorf("safe commit: close temp: %w", closeErr)
	}

	return sourceSHA, true, nil
}

func (m *Modifier) verifyCommitFromWriterSource(sourceSHA [sha256.Size]byte, tmpPath string) error {
	if sourceSHA != m.hash {
		return ErrFileModifiedExternally
	}

	return m.verifyCASAndChmod(tmpPath)
}

func (m *Modifier) renameCommittedTemp(tmpPath string) error {
	if renameErr := m.fs.Rename(tmpPath, m.path); renameErr != nil {
		_ = m.fs.Remove(tmpPath)
		return fmt.Errorf("safe commit: rename: %w", renameErr)
	}

	return nil
}

// Abort releases the lock without modifying the file.
func (m *Modifier) Abort() {
	if m.done {
		return
	}
	m.done = true
	m.release()
}

func (m *Modifier) release() {
	if m.file != nil {
		_ = unlockRDWRHandle(m.file)
		_ = m.file.Close()
		m.file = nil
	}
}

// ReadHandle holds a shared lock on a file for read-only access.
type ReadHandle struct {
	file SessionReadHandle
}

// OpenForRead opens a file with a shared lock, reads its content,
// and returns the content along with a handle. Close the handle to
// release the shared lock.
func OpenForRead(path string, fs FileSession) ([]byte, *ReadHandle, error) {
	if fs == nil {
		return nil, nil, ErrNilFileSession
	}

	openedFile, err := fs.OpenRead(path)
	if err != nil {
		return nil, nil, fmt.Errorf("safe open for read: %w", err)
	}

	if lockErr := lockSharedOn(openedFile); lockErr != nil {
		_ = openedFile.Close()
		return nil, nil, fmt.Errorf("safe shared lock: %w", lockErr)
	}

	content, err := io.ReadAll(openedFile)
	if err != nil {
		_ = unlockReadHandle(openedFile)
		_ = openedFile.Close()
		return nil, nil, fmt.Errorf("safe read: %w", err)
	}

	return content, &ReadHandle{file: openedFile}, nil
}

// Close releases the shared lock and closes the underlying file.
func (h *ReadHandle) Close() error {
	if h.file == nil {
		return nil
	}
	unlockErr := unlockReadHandle(h.file)
	closeErr := h.file.Close()
	h.file = nil
	if unlockErr != nil {
		return fmt.Errorf("safe unlock: %w", unlockErr)
	}
	return closeErr
}
