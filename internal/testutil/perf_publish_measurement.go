package testutil

import (
	"sync/atomic"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/spf13/afero"
)

// countingMemPublishSession wraps MemTestSession and counts AtomicPublish temp creations.
type countingMemPublishSession struct {
	*MemTestSession
	tempCreates *atomic.Int64
}

func (c *countingMemPublishSession) CreateTemp(dir, pattern string) (fileops.SessionTempHandle, error) {
	if c.tempCreates != nil {
		c.tempCreates.Add(1)
	}

	return c.MemTestSession.CreateTemp(dir, pattern)
}

// NewCountingMemPublishSession returns a fileops.FileSession backed by MemTestSession with shared logical Fs.
func NewCountingMemPublishSession(fs afero.Fs, creates *atomic.Int64) fileops.FileSession {
	inner := NewMemFileSession()
	inner.SetFs(fs)

	return &countingMemPublishSession{MemTestSession: inner, tempCreates: creates}
}
