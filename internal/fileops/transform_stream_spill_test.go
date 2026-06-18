package fileops

import (
	"errors"
	"io"
	iofs "io/fs"
	"testing"
)

var errSpillStubNotImplemented = errors.New("fileops: spill stub not implemented")

type unsupportedSessionTempHandle struct{}

func (unsupportedSessionTempHandle) Write([]byte) (int, error) { return 0, nil }
func (unsupportedSessionTempHandle) Sync() error               { return nil }
func (unsupportedSessionTempHandle) Close() error              { return nil }
func (unsupportedSessionTempHandle) Name() string              { return "unsupported" }

type spillStubFileSession struct {
	scratch SessionTempHandle
}

func (s spillStubFileSession) OpenRead(string) (SessionReadHandle, error) {
	return nil, errSpillStubNotImplemented
}

func (s spillStubFileSession) OpenRDWR(string) (SessionRDWRHandle, error) {
	return nil, errSpillStubNotImplemented
}

func (s spillStubFileSession) CreateTemp(_, _ string) (SessionTempHandle, error) {
	return s.scratch, nil
}

func (s spillStubFileSession) Remove(string) error { return nil }
func (s spillStubFileSession) Rename(string, string) error {
	return errSpillStubNotImplemented
}

func (s spillStubFileSession) Chmod(string, iofs.FileMode) error {
	return errSpillStubNotImplemented
}

func TestSessionWhitespaceSpillBacking_NilSessionFails(t *testing.T) {
	t.Parallel()

	backing := sessionWhitespaceSpillBacking{}
	if _, err := backing.CreateScratch(""); !errors.Is(err, ErrNilFileSession) {
		t.Fatalf("CreateScratch nil session: got %v want ErrNilFileSession", err)
	}
	if err := backing.RemoveScratch("scratch"); !errors.Is(err, ErrNilFileSession) {
		t.Fatalf("RemoveScratch nil session: got %v want ErrNilFileSession", err)
	}
}

func TestSessionWhitespaceSpillBacking_UnsupportedTempFails(t *testing.T) {
	t.Parallel()

	sess := spillStubFileSession{scratch: unsupportedSessionTempHandle{}}
	_, err := NewWhitespaceSpillBackingFromSession(sess).CreateScratch("")
	if !errors.Is(err, ErrUnsupportedWhitespaceSpillHandle) {
		t.Fatalf("want ErrUnsupportedWhitespaceSpillHandle, got %v", err)
	}
}

func TestWhitespaceSpillFromSessionTemp_UnsupportedHandle(t *testing.T) {
	t.Parallel()

	_, err := whitespaceSpillFromSessionTemp(unsupportedSessionTempHandle{})
	if !errors.Is(err, ErrUnsupportedWhitespaceSpillHandle) {
		t.Fatalf("want ErrUnsupportedWhitespaceSpillHandle, got %v", err)
	}
}

func assertWhitespaceSpillRoundTrip(t *testing.T, opened WhitespaceSpillFile) {
	t.Helper()

	if _, err := opened.Write([]byte("spill")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if _, err := opened.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}

	readBuf := make([]byte, 8)
	n, readErr := opened.Read(readBuf)
	if readErr != nil || n != 5 || string(readBuf[:n]) != "spill" {
		t.Fatalf("Read: n=%d err=%v buf=%q", n, readErr, readBuf[:n])
	}
}

func TestSessionWhitespaceSpillBacking_MemScratchRoundTrip(t *testing.T) {
	t.Parallel()

	scratch, err := NewMemWhitespaceSpillBacking().CreateScratch(transformWhitespaceSpillPattern)
	if err != nil {
		t.Fatalf("CreateScratch: %v", err)
	}

	scratchHandle, ok := scratch.(SessionTempHandle)
	if !ok {
		t.Fatalf("scratch type %T does not implement SessionTempHandle", scratch)
	}

	backing := NewWhitespaceSpillBackingFromSession(spillStubFileSession{scratch: scratchHandle})

	opened, err := backing.CreateScratch("")
	if err != nil {
		t.Fatalf("backing CreateScratch: %v", err)
	}

	assertWhitespaceSpillRoundTrip(t, opened)

	name := opened.ScratchName()
	if err := opened.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := backing.RemoveScratch(name); err != nil {
		t.Fatalf("RemoveScratch: %v", err)
	}
}

func TestMemSpillProvider_ReturnsBacking(t *testing.T) {
	t.Parallel()

	provider := memSpillProvider{}
	if provider.StreamWhitespaceSpillBacking() == nil {
		t.Fatal("want non-nil backing from provider")
	}
}

func TestResolveWhitespaceSpillBacking_PrefersProvider(t *testing.T) {
	t.Parallel()

	scratch, err := NewMemWhitespaceSpillBacking().CreateScratch(transformWhitespaceSpillPattern)
	if err != nil {
		t.Fatalf("CreateScratch: %v", err)
	}

	handle, ok := scratch.(SessionTempHandle)
	if !ok {
		t.Fatalf("scratch type %T does not implement SessionTempHandle", scratch)
	}

	sess := providerFileSession{
		spillStubFileSession: spillStubFileSession{scratch: handle},
	}

	if ResolveWhitespaceSpillBacking(sess) == nil {
		t.Fatal("want non-nil backing from provider session")
	}
}

func TestMemWhitespaceSpillFile_SeekErrorsAndNames(t *testing.T) {
	t.Parallel()

	scratch, err := NewMemWhitespaceSpillBacking().CreateScratch("")
	if err != nil {
		t.Fatalf("CreateScratch: %v", err)
	}

	if _, err := scratch.Write([]byte("abc")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if _, err := scratch.Seek(-1, io.SeekStart); !errors.Is(err, errWhitespaceSpillSeekNegativePos) {
		t.Fatalf("negative seek: got %v want errWhitespaceSpillSeekNegativePos", err)
	}

	if _, err := scratch.Seek(0, 99); !errors.Is(err, errWhitespaceSpillSeekInvalidWhence) {
		t.Fatalf("invalid whence: got %v want errWhitespaceSpillSeekInvalidWhence", err)
	}

	temp, ok := scratch.(SessionTempHandle)
	if !ok {
		t.Fatalf("scratch type %T does not implement SessionTempHandle", scratch)
	}

	if err := temp.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if temp.Name() != scratch.ScratchName() {
		t.Fatalf("Name %q != ScratchName %q", temp.Name(), scratch.ScratchName())
	}
}

type memSpillProvider struct{}

func (memSpillProvider) StreamWhitespaceSpillBacking() WhitespaceSpillBacking {
	return NewMemWhitespaceSpillBacking()
}

type providerFileSession struct {
	spillStubFileSession
	memSpillProvider
}
