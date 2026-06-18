package fileops_test

import (
	"bytes"
	"errors"
	"io"
	iofs "io/fs"
	"testing"
	"time"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

var (
	errLockedSourceFake   = errors.New("locked source fake")
	errLockedSourceUnlock = errors.New("locked source unlock")
)

type fakeLockedSourceSession struct {
	handle fileops.SessionReadHandle
}

func (s fakeLockedSourceSession) OpenRead(string) (fileops.SessionReadHandle, error) {
	return s.handle, nil
}

func (fakeLockedSourceSession) OpenRDWR(string) (fileops.SessionRDWRHandle, error) {
	return nil, errLockedSourceFake
}

func (fakeLockedSourceSession) CreateTemp(string, string) (fileops.SessionTempHandle, error) {
	return nil, errLockedSourceFake
}

func (fakeLockedSourceSession) Remove(string) error {
	return errLockedSourceFake
}

func (fakeLockedSourceSession) Rename(string, string) error {
	return errLockedSourceFake
}

func (fakeLockedSourceSession) Chmod(string, iofs.FileMode) error {
	return errLockedSourceFake
}

type fakeReadHandle struct {
	statErr     error
	seekErr     error
	closeErr    error
	closeCalled bool
}

func (h *fakeReadHandle) Read([]byte) (int, error) { return 0, io.EOF }
func (h *fakeReadHandle) Seek(int64, int) (int64, error) {
	if h.seekErr != nil {
		return 0, h.seekErr
	}

	return 0, nil
}

func (h *fakeReadHandle) Close() error {
	h.closeCalled = true
	return h.closeErr
}

func (h *fakeReadHandle) Stat() (iofs.FileInfo, error) {
	if h.statErr != nil {
		return nil, h.statErr
	}
	return fakeLockedSourceInfo{}, nil
}

type fakeLockedSourceInfo struct{}

func (fakeLockedSourceInfo) Name() string        { return "source.txt" }
func (fakeLockedSourceInfo) Size() int64         { return 1 }
func (fakeLockedSourceInfo) Mode() iofs.FileMode { return 0o644 }
func (fakeLockedSourceInfo) ModTime() time.Time  { return time.Time{} }
func (fakeLockedSourceInfo) IsDir() bool         { return false }
func (fakeLockedSourceInfo) Sys() any            { return nil }

func TestOpenLockedSourceRead_NilFileSession_Fake(t *testing.T) {
	t.Parallel()

	_, err := fileops.OpenLockedSourceRead("/any.txt", nil)
	if !errors.Is(err, fileops.ErrNilFileSession) {
		t.Fatalf("want ErrNilFileSession, got %v", err)
	}
}

func TestOpenLockedSourceRead_NonexistentFile_Fake(t *testing.T) {
	t.Parallel()

	_, err := fileops.OpenLockedSourceRead("/missing.txt", testutil.NewMemFileSession())
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestOpenLockedSourceRead_StatErrorClosesHandle_Fake(t *testing.T) {
	t.Parallel()

	handle := &fakeReadHandle{statErr: errLockedSourceFake}
	_, err := fileops.OpenLockedSourceRead("/bad-stat.txt", fakeLockedSourceSession{handle: handle})
	if !errors.Is(err, errLockedSourceFake) {
		t.Fatalf("want stat error, got %v", err)
	}
	if !handle.closeCalled {
		t.Fatal("expected handle close on stat failure")
	}
}

func TestOpenLockedSourceRead_SeekStartError_Fake(t *testing.T) {
	t.Parallel()

	handle := &fakeReadHandle{seekErr: errLockedSourceFake}
	_, err := fileops.OpenLockedSourceRead("/bad-seek.txt", fakeLockedSourceSession{handle: handle})
	if !errors.Is(err, errLockedSourceFake) {
		t.Fatalf("want seek error, got %v", err)
	}
	if !handle.closeCalled {
		t.Fatal("expected handle close on seek failure")
	}
}

func TestOpenLockedSourceRead_StreamingPrefixMemFs_Fake(t *testing.T) {
	t.Parallel()

	mem := testutil.NewMemFileSession()
	payload := bytes.Repeat([]byte("Z"), 512*1024)
	if err := afero.WriteFile(mem.Fs, "/big.bin", payload, 0o600); err != nil {
		t.Fatal(err)
	}

	ls, err := fileops.OpenLockedSourceRead("/big.bin", mem)
	if err != nil {
		t.Fatalf("OpenLockedSourceRead: %v", err)
	}

	var buf [64]byte
	readBytes, readErr := ls.Read(buf[:])
	if readErr != nil {
		t.Fatalf("Read: %v", readErr)
	}

	if readBytes != len(buf) {
		t.Fatalf("Read len: got %d want %d", readBytes, len(buf))
	}

	for _, b := range buf[:] {
		if b != 'Z' {
			t.Fatalf("unexpected payload byte %q", b)
		}
	}

	if ls.State.Size != int64(len(payload)) {
		t.Fatalf("State.Size: got %d want %d", ls.State.Size, len(payload))
	}

	if closeErr := ls.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}
}

func TestOpenLockedSourceRead_SeekAndReadMemFs_Fake(t *testing.T) {
	t.Parallel()

	mem := testutil.NewMemFileSession()
	if err := afero.WriteFile(mem.Fs, "/seek.bin", []byte("abcdefghij"), 0o600); err != nil {
		t.Fatal(err)
	}

	ls, err := fileops.OpenLockedSourceRead("/seek.bin", mem)
	if err != nil {
		t.Fatalf("OpenLockedSourceRead: %v", err)
	}

	defer func() { _ = ls.Close() }()

	if _, err := ls.Seek(3, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}

	var buf [4]byte
	if _, err := io.ReadFull(ls, buf[:]); err != nil {
		t.Fatalf("ReadFull: %v", err)
	}

	if string(buf[:]) != "defg" {
		t.Fatalf("got %q want defg", string(buf[:]))
	}
}

func TestOpenLockedSourceRead_DoubleCloseSafe_Fake(t *testing.T) {
	t.Parallel()

	mem := testutil.NewMemFileSession()
	if err := afero.WriteFile(mem.Fs, "/x.bin", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	ls, err := fileops.OpenLockedSourceRead("/x.bin", mem)
	if err != nil {
		t.Fatalf("OpenLockedSourceRead: %v", err)
	}

	if err := ls.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	if err := ls.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestOpenLockedSourceRead_ReadAfterClose_Fake(t *testing.T) {
	t.Parallel()

	mem := testutil.NewMemFileSession()
	if err := afero.WriteFile(mem.Fs, "/c.bin", []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	ls, err := fileops.OpenLockedSourceRead("/c.bin", mem)
	if err != nil {
		t.Fatalf("OpenLockedSourceRead: %v", err)
	}

	if err := ls.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var one [1]byte
	if _, err := ls.Read(one[:]); !errors.Is(err, iofs.ErrClosed) {
		t.Fatalf("Read after Close: want ErrClosed, got %v", err)
	}
}

func TestOpenLockedSourceRead_SeekAfterClose_ErrClosedFake(t *testing.T) {
	t.Parallel()

	mem := testutil.NewMemFileSession()
	if err := afero.WriteFile(mem.Fs, "/s.bin", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	ls, err := fileops.OpenLockedSourceRead("/s.bin", mem)
	if err != nil {
		t.Fatalf("OpenLockedSourceRead: %v", err)
	}

	if err := ls.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := ls.Seek(0, io.SeekStart); !errors.Is(err, iofs.ErrClosed) {
		t.Fatalf("Seek after Close: want ErrClosed, got %v", err)
	}
}

func TestLockedSourceRead_CloseReturnsHandleCloseError_Fake(t *testing.T) {
	t.Parallel()

	handle := &fakeReadHandle{closeErr: errLockedSourceFake}
	ls, err := fileops.OpenLockedSourceRead("/close-error.txt", fakeLockedSourceSession{handle: handle})
	if err != nil {
		t.Fatalf("OpenLockedSourceRead: %v", err)
	}

	if err := ls.Close(); !errors.Is(err, errLockedSourceFake) {
		t.Fatalf("Close: got %v want fake error", err)
	}
}

type unlockFailReadHandle struct {
	*fakeReadHandle
}

func (unlockFailReadHandle) LockShared() error    { return nil }
func (unlockFailReadHandle) LockExclusive() error { return nil }
func (unlockFailReadHandle) Unlock() error        { return errLockedSourceUnlock }

func TestLockedSourceRead_CloseReturnsCombinedUnlockAndCloseErrors_Fake(t *testing.T) {
	t.Parallel()

	handle := &unlockFailReadHandle{
		fakeReadHandle: &fakeReadHandle{closeErr: errLockedSourceFake},
	}
	ls, err := fileops.OpenLockedSourceRead("/dual-close.txt", fakeLockedSourceSession{handle: handle})
	if err != nil {
		t.Fatalf("OpenLockedSourceRead: %v", err)
	}

	closeErr := ls.Close()
	if !errors.Is(closeErr, errLockedSourceUnlock) || !errors.Is(closeErr, errLockedSourceFake) {
		t.Fatalf("Close: got %v want unlock and close errors", closeErr)
	}
}
