// User vision: feature tests model filesystem indirection without leaking OS-shaped test doubles
// into BDD step glue.
package testutil

import (
	"io/fs"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

// PathRewritingFileSession rewrites logical file paths before delegating reads and final publication.
type PathRewritingFileSession struct {
	inner   fileops.FileSession
	rewrite func(string) string
}

// NewPathRewritingFileSession returns a FileSession wrapper for tests that need logical path aliases.
func NewPathRewritingFileSession(inner fileops.FileSession, rewrite func(string) string) fileops.FileSession {
	return PathRewritingFileSession{inner: inner, rewrite: rewrite}
}

func (s PathRewritingFileSession) rewritten(path string) string {
	if s.rewrite == nil {
		return path
	}

	return s.rewrite(path)
}

func (s PathRewritingFileSession) OpenRead(path string) (fileops.SessionReadHandle, error) {
	if s.inner == nil {
		return nil, fileops.ErrNilFileSession
	}

	return s.inner.OpenRead(s.rewritten(path))
}

func (s PathRewritingFileSession) OpenRDWR(path string) (fileops.SessionRDWRHandle, error) {
	if s.inner == nil {
		return nil, fileops.ErrNilFileSession
	}

	return s.inner.OpenRDWR(s.rewritten(path))
}

func (s PathRewritingFileSession) CreateTemp(dir, pattern string) (fileops.SessionTempHandle, error) {
	if s.inner == nil {
		return nil, fileops.ErrNilFileSession
	}

	return s.inner.CreateTemp(dir, pattern)
}

func (s PathRewritingFileSession) Remove(name string) error {
	if s.inner == nil {
		return fileops.ErrNilFileSession
	}

	return s.inner.Remove(name)
}

func (s PathRewritingFileSession) Rename(oldpath, newpath string) error {
	if s.inner == nil {
		return fileops.ErrNilFileSession
	}

	return s.inner.Rename(oldpath, s.rewritten(newpath))
}

func (s PathRewritingFileSession) Chmod(name string, mode fs.FileMode) error {
	if s.inner == nil {
		return fileops.ErrNilFileSession
	}

	return s.inner.Chmod(name, mode)
}
