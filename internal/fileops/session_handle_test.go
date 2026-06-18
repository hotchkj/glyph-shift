package fileops

import (
	"bytes"
	"io"
	iofs "io/fs"
	"testing"
)

type lockerCalls struct {
	sharedLocked    bool
	exclusiveLocked bool
	unlocked        bool
}

type lockerReadHandle struct {
	*bytes.Reader
	calls *lockerCalls
}

func (h lockerReadHandle) Close() error {
	return nil
}

func (h lockerReadHandle) Stat() (iofs.FileInfo, error) {
	return nil, nil
}

func (h lockerReadHandle) LockShared() error {
	if h.calls != nil {
		h.calls.sharedLocked = true
	}

	return nil
}

func (h lockerReadHandle) LockExclusive() error {
	if h.calls != nil {
		h.calls.exclusiveLocked = true
	}

	return nil
}

func (h lockerReadHandle) Unlock() error {
	if h.calls != nil {
		h.calls.unlocked = true
	}

	return nil
}

func TestLockSharedOn_AdvisoryHandle(t *testing.T) {
	t.Parallel()

	calls := &lockerCalls{}
	handle := lockerReadHandle{Reader: bytes.NewReader(nil), calls: calls}
	if err := lockSharedOn(handle); err != nil {
		t.Fatalf("lockSharedOn: %v", err)
	}
	if !calls.sharedLocked {
		t.Fatal("expected LockShared to be invoked")
	}
}

func TestLockSharedOn_NoLocker(t *testing.T) {
	t.Parallel()

	if err := lockSharedOn(noopReadHandle{}); err != nil {
		t.Fatalf("lockSharedOn without locker: %v", err)
	}
}

type lockerRDWRHandle struct {
	noopReadHandle
	calls *lockerCalls
}

func (h lockerRDWRHandle) Write([]byte) (int, error) {
	return 0, nil
}

func (h lockerRDWRHandle) WriteAt([]byte, int64) (int, error) {
	return 0, nil
}

func (h lockerRDWRHandle) Sync() error {
	return nil
}

func (h lockerRDWRHandle) LockShared() error {
	if h.calls != nil {
		h.calls.sharedLocked = true
	}

	return nil
}

func (h lockerRDWRHandle) LockExclusive() error {
	if h.calls != nil {
		h.calls.exclusiveLocked = true
	}

	return nil
}

func (h lockerRDWRHandle) Unlock() error {
	if h.calls != nil {
		h.calls.unlocked = true
	}

	return nil
}

func TestLockExclusiveOn_AdvisoryHandle(t *testing.T) {
	t.Parallel()

	calls := &lockerCalls{}
	if err := lockExclusiveOn(lockerRDWRHandle{calls: calls}); err != nil {
		t.Fatalf("lockExclusiveOn: %v", err)
	}
	if !calls.exclusiveLocked {
		t.Fatal("expected LockExclusive to be invoked")
	}
}

func TestUnlockReadHandle_AdvisoryHandle(t *testing.T) {
	t.Parallel()

	calls := &lockerCalls{}
	if err := unlockReadHandle(lockerReadHandle{Reader: bytes.NewReader(nil), calls: calls}); err != nil {
		t.Fatalf("unlockReadHandle: %v", err)
	}
	if !calls.unlocked {
		t.Fatal("expected Unlock to be invoked")
	}
}

func TestUnlockReadHandle_NoLocker(t *testing.T) {
	t.Parallel()

	if err := unlockReadHandle(noopReadHandle{}); err != nil {
		t.Fatalf("unlockReadHandle without locker: %v", err)
	}
}

func TestUnlockRDWRHandle_AdvisoryHandle(t *testing.T) {
	t.Parallel()

	calls := &lockerCalls{}
	if err := unlockRDWRHandle(lockerRDWRHandle{calls: calls}); err != nil {
		t.Fatalf("unlockRDWRHandle: %v", err)
	}
	if !calls.unlocked {
		t.Fatal("expected Unlock to be invoked")
	}
}

type noopRDWRHandle struct {
	noopReadHandle
}

func (noopRDWRHandle) Write([]byte) (int, error) {
	return 0, nil
}

func (noopRDWRHandle) WriteAt([]byte, int64) (int, error) {
	return 0, nil
}

func (noopRDWRHandle) Sync() error {
	return nil
}

func TestUnlockRDWRHandle_NoLocker(t *testing.T) {
	t.Parallel()

	if err := unlockRDWRHandle(noopRDWRHandle{}); err != nil {
		t.Fatalf("unlockRDWRHandle without locker: %v", err)
	}
}

type noopReadHandle struct{}

func (noopReadHandle) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (noopReadHandle) Seek(int64, int) (int64, error) {
	return 0, nil
}

func (noopReadHandle) Close() error {
	return nil
}

func (noopReadHandle) Stat() (iofs.FileInfo, error) {
	return nil, nil
}
