package testutil

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/spf13/afero"
)

// ThroughMemOutputOpener writes bytes to the logical destination during Write,
// not only on Close. It is intended for safety tests that must observe partial
// final-path writes when a later write or close fails.
//
// Not safe for concurrent use.
type ThroughMemOutputOpener struct {
	Fs afero.Fs
}

// NewThroughMemOutputOpener returns a ThroughMemOutputOpener with an empty
// in-memory filesystem.
func NewThroughMemOutputOpener() *ThroughMemOutputOpener {
	return &ThroughMemOutputOpener{Fs: afero.NewMemMapFs()}
}

// MkdirAll creates the directory path and all parents in the in-memory filesystem.
func (o *ThroughMemOutputOpener) MkdirAll(path string, perm fs.FileMode) error {
	return o.Fs.MkdirAll(path, perm)
}

// OpenFile opens a write-through logical destination path.
func (o *ThroughMemOutputOpener) OpenFile(
	path string,
	intent pipeline.OutputWriteIntent,
	perm fs.FileMode,
) (io.WriteCloser, error) {
	if intent == pipeline.OutputCreateExclusive {
		if aferoFileExistsLookup(o.Fs, path) {
			return nil, fmt.Errorf("open %q: %w", path, fs.ErrExist)
		}
	}

	if err := o.Fs.MkdirAll(filepath.Dir(path), defaultMemDirPerm); err != nil {
		return nil, fmt.Errorf("through mem mkdir %q: %w", filepath.Dir(path), err)
	}

	initial, seedErr := memOutputReadAppendSeed(o.Fs, path, intent)
	if seedErr != nil {
		return nil, seedErr
	}
	if intent == pipeline.OutputCreateOrReplace {
		initial = nil
	}

	writer := &throughMemWriteCloser{
		fs:      o.Fs,
		path:    path,
		perm:    perm,
		content: append([]byte(nil), initial...),
	}

	if err := writer.persist(); err != nil {
		return nil, err
	}

	return writer, nil
}

// FileExists reports whether the named logical path exists in the in-memory filesystem.
func (o *ThroughMemOutputOpener) FileExists(path string) bool {
	exists, _ := afero.Exists(o.Fs, path)

	return exists
}

// FileContent returns the content written for the named logical path.
func (o *ThroughMemOutputOpener) FileContent(path string) []byte {
	data, err := afero.ReadFile(o.Fs, path)
	if err != nil {
		return nil
	}

	return data
}

type throughMemWriteCloser struct {
	fs      afero.Fs
	path    string
	perm    fs.FileMode
	content []byte
}

func (w *throughMemWriteCloser) Write(data []byte) (int, error) {
	w.content = append(w.content, data...)
	if err := w.persist(); err != nil {
		return 0, err
	}

	return len(data), nil
}

func (w *throughMemWriteCloser) Close() error {
	return nil
}

func (w *throughMemWriteCloser) persist() error {
	if err := afero.WriteFile(w.fs, w.path, w.content, w.perm); err != nil {
		return fmt.Errorf("through mem write %q: %w", w.path, err)
	}

	return nil
}
