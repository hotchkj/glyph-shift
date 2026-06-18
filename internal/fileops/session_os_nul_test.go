package fileops

import (
	"errors"
	iofs "io/fs"
	"testing"
)

var errRejectingSessionBackendCalled = errors.New("rejecting session backend called")

type rejectingSessionBackend struct{}

func (rejectingSessionBackend) OpenRead(string) (SessionReadHandle, error) {
	return nil, errRejectingSessionBackendCalled
}

func (rejectingSessionBackend) OpenRDWR(string) (SessionRDWRHandle, error) {
	return nil, errRejectingSessionBackendCalled
}

func (rejectingSessionBackend) CreateTemp(string, string) (SessionTempHandle, error) {
	return nil, errRejectingSessionBackendCalled
}

func (rejectingSessionBackend) Remove(string) error {
	return errRejectingSessionBackendCalled
}

func (rejectingSessionBackend) Rename(string, string) error {
	return errRejectingSessionBackendCalled
}

func (rejectingSessionBackend) Chmod(string, iofs.FileMode) error {
	return errRejectingSessionBackendCalled
}

func TestFileSessionRejectsNULBeforeBackend(t *testing.T) {
	t.Parallel()

	invalidPath := string([]byte{0})
	session, err := NewFileSession(rejectingSessionBackend{})
	if err != nil {
		t.Fatalf("NewFileSession: %v", err)
	}

	cases := []struct {
		name string
		run  func() error
	}{
		{name: "open_read", run: func() error {
			_, err := session.OpenRead(invalidPath)
			return err
		}},
		{name: "open_rdwr", run: func() error {
			_, err := session.OpenRDWR(invalidPath)
			return err
		}},
		{name: "create_temp_dir", run: func() error {
			_, err := session.CreateTemp(invalidPath, "glyph-shift-*")
			return err
		}},
		{name: "create_temp_pattern", run: func() error {
			_, err := session.CreateTemp("", string([]byte{'g', 0}))
			return err
		}},
		{name: "rename_old", run: func() error {
			return session.Rename(invalidPath, "ok")
		}},
		{name: "rename_new", run: func() error {
			return session.Rename("ok", invalidPath)
		}},
		{name: "remove", run: func() error {
			return session.Remove(invalidPath)
		}},
		{name: "chmod", run: func() error {
			return session.Chmod(invalidPath, 0o644)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.run(); !errors.Is(err, ErrPathContainsNUL) {
				t.Fatalf("got %v want ErrPathContainsNUL", err)
			}
		})
	}
}

type recordingSessionBackend struct {
	openReadPath string
	openRDWRPath string
	tempDir      string
	tempPattern  string
	removePath   string
	renameOld    string
	renameNew    string
	chmodPath    string
	chmodMode    iofs.FileMode
}

func (b *recordingSessionBackend) OpenRead(path string) (SessionReadHandle, error) {
	b.openReadPath = path
	return nil, nil
}

func (b *recordingSessionBackend) OpenRDWR(path string) (SessionRDWRHandle, error) {
	b.openRDWRPath = path
	return nil, nil
}

func (b *recordingSessionBackend) CreateTemp(dir, pattern string) (SessionTempHandle, error) {
	b.tempDir = dir
	b.tempPattern = pattern
	return nil, nil
}

func (b *recordingSessionBackend) Remove(name string) error {
	b.removePath = name
	return nil
}

func (b *recordingSessionBackend) Rename(oldpath, newpath string) error {
	b.renameOld = oldpath
	b.renameNew = newpath
	return nil
}

func (b *recordingSessionBackend) Chmod(name string, mode iofs.FileMode) error {
	b.chmodPath = name
	b.chmodMode = mode
	return nil
}

func assertRecordingSessionDelegation(t *testing.T, backend *recordingSessionBackend) {
	t.Helper()

	if backend.openReadPath != "/read.txt" ||
		backend.openRDWRPath != "/write.txt" ||
		backend.tempDir != "/tmp" ||
		backend.tempPattern != "glyph-*" ||
		backend.removePath != "/remove.txt" ||
		backend.renameOld != "/old.txt" ||
		backend.renameNew != "/new.txt" ||
		backend.chmodPath != "/chmod.txt" ||
		backend.chmodMode != 0o644 {
		t.Fatalf("backend delegation mismatch: %#v", backend)
	}
}

func TestFileSessionDelegatesValidPathsToBackend(t *testing.T) {
	t.Parallel()

	backend := &recordingSessionBackend{}
	session, err := NewFileSession(backend)
	if err != nil {
		t.Fatalf("NewFileSession: %v", err)
	}

	if _, err := session.OpenRead("/read.txt"); err != nil {
		t.Fatalf("OpenRead: %v", err)
	}
	if _, err := session.OpenRDWR("/write.txt"); err != nil {
		t.Fatalf("OpenRDWR: %v", err)
	}
	if _, err := session.CreateTemp("/tmp", "glyph-*"); err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if err := session.Remove("/remove.txt"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := session.Rename("/old.txt", "/new.txt"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if err := session.Chmod("/chmod.txt", 0o644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	assertRecordingSessionDelegation(t, backend)
}

func TestNewFileSessionRejectsNilBackend(t *testing.T) {
	t.Parallel()

	_, err := NewFileSession(nil)
	if !errors.Is(err, ErrNilSessionBackend) {
		t.Fatalf("NewFileSession nil backend = %v, want ErrNilSessionBackend", err)
	}
}
