package pipeline

import (
	"errors"
	"io"
	"io/fs"
	"testing"
	"time"
)

var errRejectingPipelineBackendCalled = errors.New("rejecting pipeline backend called")

type rejectingOutputBackend struct{}

func (rejectingOutputBackend) MkdirAll(string, fs.FileMode) error {
	return errRejectingPipelineBackendCalled
}

func (rejectingOutputBackend) OpenFile(string, OutputWriteIntent, fs.FileMode) (io.WriteCloser, error) {
	return nil, errRejectingPipelineBackendCalled
}

type rejectingStatBackend struct{}

func (rejectingStatBackend) Stat(string) (fs.FileInfo, error) {
	return nil, errRejectingPipelineBackendCalled
}

func TestPipelineAdaptersRejectNULBeforeBackend(t *testing.T) {
	t.Parallel()

	invalidPath := string([]byte{0})
	out, outErr := NewOutputOpener(rejectingOutputBackend{})
	if outErr != nil {
		t.Fatalf("NewOutputOpener: %v", outErr)
	}
	stater, staterErr := NewFileStater(rejectingStatBackend{})
	if staterErr != nil {
		t.Fatalf("NewFileStater: %v", staterErr)
	}

	if err := out.MkdirAll(invalidPath, DirPerm); !errors.Is(err, ErrPathContainsNUL) {
		t.Fatalf("MkdirAll: got %v want ErrPathContainsNUL", err)
	}

	if _, err := out.OpenFile(invalidPath, OutputCreateExclusive, FilePerm); !errors.Is(err, ErrPathContainsNUL) {
		t.Fatalf("OpenFile: got %v want ErrPathContainsNUL", err)
	}

	if _, err := stater.Stat(invalidPath); !errors.Is(err, ErrPathContainsNUL) {
		t.Fatalf("Stat: got %v want ErrPathContainsNUL", err)
	}
}

type recordingOutputBackend struct {
	mkdirPath string
	openPath  string
}

func (b *recordingOutputBackend) MkdirAll(path string, _ fs.FileMode) error {
	b.mkdirPath = path
	return nil
}

func (b *recordingOutputBackend) OpenFile(path string, _ OutputWriteIntent, _ fs.FileMode) (io.WriteCloser, error) {
	b.openPath = path
	return nopWriteCloser{}, nil
}

type nopWriteCloser struct{}

func (nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (nopWriteCloser) Close() error                { return nil }

func TestOutputOpenerDelegatesValidPathsToBackend(t *testing.T) {
	t.Parallel()

	backend := &recordingOutputBackend{}
	opener, err := NewOutputOpener(backend)
	if err != nil {
		t.Fatalf("NewOutputOpener: %v", err)
	}

	if err := opener.MkdirAll("/out", DirPerm); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if backend.mkdirPath != "/out" {
		t.Fatalf("MkdirAll path = %q", backend.mkdirPath)
	}

	if _, err := opener.OpenFile("/out/file.txt", OutputCreateOrReplace, FilePerm); err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if backend.openPath != "/out/file.txt" {
		t.Fatalf("OpenFile path = %q", backend.openPath)
	}
}

type stubFileInfo struct{}

func (stubFileInfo) Name() string       { return "file.txt" }
func (stubFileInfo) Size() int64        { return 1 }
func (stubFileInfo) Mode() fs.FileMode  { return 0o644 }
func (stubFileInfo) ModTime() time.Time { return time.Time{} }
func (stubFileInfo) IsDir() bool        { return false }
func (stubFileInfo) Sys() any           { return nil }

type recordingStatBackend struct {
	path string
}

func (b *recordingStatBackend) Stat(path string) (fs.FileInfo, error) {
	b.path = path
	return stubFileInfo{}, nil
}

func TestFileStaterDelegatesValidPathToBackend(t *testing.T) {
	t.Parallel()

	backend := &recordingStatBackend{}
	stater, err := NewFileStater(backend)
	if err != nil {
		t.Fatalf("NewFileStater: %v", err)
	}

	if _, err := stater.Stat("/src.txt"); err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if backend.path != "/src.txt" {
		t.Fatalf("Stat path = %q", backend.path)
	}
}

func TestPipelineAdapterConstructorsRejectNilBackend(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func() error
		want error
	}{
		{name: "output_opener", run: func() error {
			_, err := NewOutputOpener(nil)
			return err
		}, want: ErrNilOutputBackend},
		{name: "file_stater", run: func() error {
			_, err := NewFileStater(nil)
			return err
		}, want: ErrNilStatBackend},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.run(); !errors.Is(err, tc.want) {
				t.Fatalf("constructor error = %v, want %v", err, tc.want)
			}
		})
	}
}
