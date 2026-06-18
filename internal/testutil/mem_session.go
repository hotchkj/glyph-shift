// User vision: tests seed and assert against in-memory filesystem state through a thin FileSession wrapper.
package testutil

import (
	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/spf13/afero"
)

// MemTestSession wraps a FileSession backed by an in-memory MemSessionBackend.
// It exposes .Fs for test seeding and satisfies fileops.FileSession via embedding.
type MemTestSession struct {
	fileops.FileSession
	Fs      afero.Fs
	backend *MemSessionBackend
}

// NewMemFileSession returns a MemTestSession with an empty in-memory filesystem.
func NewMemFileSession() *MemTestSession {
	backend := NewMemSessionBackend()
	session, err := fileops.NewFileSession(backend)
	if err != nil {
		panic(err)
	}

	return &MemTestSession{
		FileSession: session,
		Fs:          backend.Fs,
		backend:     backend,
	}
}

// SetFs aligns the wrapper and backend filesystem references for shared memFs test wiring.
func (m *MemTestSession) SetFs(fs afero.Fs) {
	m.Fs = fs
	if m.backend != nil {
		m.backend.Fs = fs
	}
}

// Files returns a snapshot of logical file contents in the mem filesystem.
func (m *MemTestSession) Files() map[string][]byte {
	return m.backend.Files()
}

// Backend exposes the underlying MemSessionBackend for direct access in tests.
func (m *MemTestSession) Backend() *MemSessionBackend {
	return m.backend
}
