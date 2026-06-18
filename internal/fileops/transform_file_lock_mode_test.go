package fileops

import (
	"bytes"
	iofs "io/fs"
	"testing"
	"time"
)

type transformLockModeSession struct {
	data      []byte
	openRead  int
	openRDWR  int
	shared    int
	exclusive int
	unlocks   int
	renames   int
}

func (s *transformLockModeSession) OpenRead(string) (SessionReadHandle, error) {
	s.openRead++
	return &transformLockModeReadHandle{sess: s, reader: bytes.NewReader(s.data)}, nil
}

func (s *transformLockModeSession) OpenRDWR(string) (SessionRDWRHandle, error) {
	s.openRDWR++
	return &transformLockModeRDWRHandle{
		transformLockModeReadHandle: &transformLockModeReadHandle{sess: s, reader: bytes.NewReader(s.data)},
	}, nil
}

func (s *transformLockModeSession) CreateTemp(string, string) (SessionTempHandle, error) {
	return &transformLockModeTempHandle{}, nil
}

func (s *transformLockModeSession) Remove(string) error { return nil }

func (s *transformLockModeSession) Rename(string, string) error {
	s.renames++
	return nil
}

func (s *transformLockModeSession) Chmod(string, iofs.FileMode) error { return nil }

type transformLockModeReadHandle struct {
	sess   *transformLockModeSession
	reader *bytes.Reader
}

func (h *transformLockModeReadHandle) Read(p []byte) (int, error) { return h.reader.Read(p) }
func (h *transformLockModeReadHandle) Seek(off int64, whence int) (int64, error) {
	return h.reader.Seek(off, whence)
}
func (h *transformLockModeReadHandle) Close() error { return nil }
func (h *transformLockModeReadHandle) Stat() (iofs.FileInfo, error) {
	return fakeFileInfo{size: int64(h.reader.Len())}, nil
}

func (h *transformLockModeReadHandle) LockShared() error {
	h.sess.shared++
	return nil
}

func (h *transformLockModeReadHandle) LockExclusive() error {
	h.sess.exclusive++
	return nil
}

func (h *transformLockModeReadHandle) Unlock() error {
	h.sess.unlocks++
	return nil
}

type transformLockModeRDWRHandle struct {
	*transformLockModeReadHandle
}

func (h *transformLockModeRDWRHandle) Write(p []byte) (int, error) { return len(p), nil }
func (h *transformLockModeRDWRHandle) WriteAt(p []byte, _ int64) (int, error) {
	return len(p), nil
}
func (h *transformLockModeRDWRHandle) Sync() error { return nil }

type transformLockModeTempHandle struct {
	bytes.Buffer
}

func (h *transformLockModeTempHandle) Sync() error  { return nil }
func (h *transformLockModeTempHandle) Close() error { return nil }
func (h *transformLockModeTempHandle) Name() string { return "/tmp-transform" }

type fakeFileInfo struct {
	size int64
}

func (f fakeFileInfo) Name() string        { return "source.txt" }
func (f fakeFileInfo) Size() int64         { return f.size }
func (f fakeFileInfo) Mode() iofs.FileMode { return 0o600 }
func (f fakeFileInfo) ModTime() time.Time  { return time.Time{} }
func (f fakeFileInfo) IsDir() bool         { return false }
func (f fakeFileInfo) Sys() any            { return nil }

func TestTransformPreviewUsesSharedReadLock(t *testing.T) {
	t.Parallel()

	session := &transformLockModeSession{data: []byte("line  \n")}
	_, err := TransformFileWithContext(
		t.Context(),
		"/source.txt",
		TransformOptions{TrimTrailing: true},
		false,
		session,
	)
	if err != nil {
		t.Fatalf("TransformFileWithContext preview: %v", err)
	}

	if session.openRead != 1 || session.openRDWR != 0 {
		t.Fatalf("opens read=%d rdwr=%d, want read=1 rdwr=0", session.openRead, session.openRDWR)
	}
	if session.shared != 1 || session.exclusive != 0 {
		t.Fatalf("locks shared=%d exclusive=%d, want shared=1 exclusive=0", session.shared, session.exclusive)
	}
	if session.renames != 0 {
		t.Fatalf("renames=%d, want 0", session.renames)
	}
}

func TestTransformPreviewBinaryUsesSharedReadLock(t *testing.T) {
	t.Parallel()

	session := &transformLockModeSession{data: []byte("text\x00binary")}
	got, err := TransformFileWithContext(
		t.Context(),
		"/source.txt",
		TransformOptions{TrimTrailing: true},
		false,
		session,
	)
	if err != nil {
		t.Fatalf("TransformFileWithContext preview binary: %v", err)
	}

	if !got.Skipped || got.SkipReason != "binary" {
		t.Fatalf("binary preview result = %#v", got)
	}
	if session.openRead != 1 || session.openRDWR != 0 {
		t.Fatalf("opens read=%d rdwr=%d, want read=1 rdwr=0", session.openRead, session.openRDWR)
	}
	if session.shared != 1 || session.exclusive != 0 {
		t.Fatalf("locks shared=%d exclusive=%d, want shared=1 exclusive=0", session.shared, session.exclusive)
	}
}

func TestTransformPreviewNoOptionsUsesSharedReadLock(t *testing.T) {
	t.Parallel()

	session := &transformLockModeSession{data: []byte("line\n")}
	got, err := TransformFileWithContext(
		t.Context(),
		"/source.txt",
		TransformOptions{},
		false,
		session,
	)
	if err != nil {
		t.Fatalf("TransformFileWithContext preview no options: %v", err)
	}

	if !got.Skipped || got.SkipReason != transformSkipReasonNoTransform {
		t.Fatalf("no-options preview result = %#v", got)
	}
	if session.openRead != 1 || session.openRDWR != 0 {
		t.Fatalf("opens read=%d rdwr=%d, want read=1 rdwr=0", session.openRead, session.openRDWR)
	}
	if session.shared != 1 || session.exclusive != 0 {
		t.Fatalf("locks shared=%d exclusive=%d, want shared=1 exclusive=0", session.shared, session.exclusive)
	}
}

func TestTransformApplyUsesExclusiveModifyLock(t *testing.T) {
	t.Parallel()

	session := &transformLockModeSession{data: []byte("line  \n")}
	_, err := TransformFileWithContext(
		t.Context(),
		"/source.txt",
		TransformOptions{TrimTrailing: true},
		true,
		session,
	)
	if err != nil {
		t.Fatalf("TransformFileWithContext apply: %v", err)
	}

	if session.openRead != 0 || session.openRDWR != 1 {
		t.Fatalf("opens read=%d rdwr=%d, want read=0 rdwr=1", session.openRead, session.openRDWR)
	}
	if session.shared != 0 || session.exclusive != 1 {
		t.Fatalf("locks shared=%d exclusive=%d, want shared=0 exclusive=1", session.shared, session.exclusive)
	}
}
