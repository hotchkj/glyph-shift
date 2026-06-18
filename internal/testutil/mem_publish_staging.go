// User vision: in-memory filesystem tests occasionally need deterministic writer decoration at AtomicPublish
// staging boundaries without injecting optional fields into pipeline invocation bundles.
//
// MemStagingPublishSession layers [fileops.AtomicPublishStagingDecorator] on top of [MemTestSession].
package testutil

import (
	"io"
	"sync/atomic"

	"github.com/spf13/afero"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

// measuringPublishWriter counts bytes written through the wrapped writer.
type measuringPublishWriter struct {
	inner    io.Writer
	tallyPtr *atomic.Int64
}

func (m *measuringPublishWriter) Write(p []byte) (int, error) {
	payloadWritten, err := m.inner.Write(p)
	if payloadWritten > 0 && m.tallyPtr != nil {
		m.tallyPtr.Add(int64(payloadWritten))
	}

	return payloadWritten, err
}

// MemStagingPublishSession embeds MemTestSession and optionally decorates AtomicPublish staging writers.
type MemStagingPublishSession struct {
	*MemTestSession
	stagingWrap func(destPath string, w io.Writer) io.Writer
}

// NewMemStagingPublishSession returns a MemStagingPublishSession whose logical filesystem is fs.
// When stagingWrap is nil, staging writes pass through unchanged.
func NewMemStagingPublishSession(fs afero.Fs, stagingWrap func(destPath string, w io.Writer) io.Writer,
) *MemStagingPublishSession {
	inner := NewMemFileSession()
	inner.SetFs(fs)

	return &MemStagingPublishSession{MemTestSession: inner, stagingWrap: stagingWrap}
}

// WrapAtomicPublishStagingWriter implements [fileops.AtomicPublishStagingDecorator].
func (m *MemStagingPublishSession) WrapAtomicPublishStagingWriter(path string, w io.Writer) io.Writer {
	if m == nil || m.stagingWrap == nil {
		return w
	}

	return m.stagingWrap(path, w)
}

// countAtomicPublishStagingSession counts CreateTemp invocations without changing staging behavior.
type countAtomicPublishStagingSession struct {
	*MemStagingPublishSession
	tempCreates *atomic.Int64
}

func (c *countAtomicPublishStagingSession) CreateTemp(dir, pattern string) (fileops.SessionTempHandle, error) {
	if c.tempCreates != nil {
		c.tempCreates.Add(1)
	}

	return c.MemTestSession.CreateTemp(dir, pattern)
}

// NewCountingMemStagingPublishSession returns a [fileops.FileSession] that aliases fs, applies stagingWrap
// inside [fileops.AtomicPublish], and increments tempCreates on each CreateTemp.
func NewCountingMemStagingPublishSession(fs afero.Fs, stagingWrap func(destPath string, w io.Writer) io.Writer,
	tempCreates *atomic.Int64,
) fileops.FileSession {
	return &countAtomicPublishStagingSession{
		MemStagingPublishSession: NewMemStagingPublishSession(fs, stagingWrap),
		tempCreates:              tempCreates,
	}
}

// ChainAtomicPublishStagingWrap composes an optional outer wrap with byte tallying for AtomicPublish staging.
func ChainAtomicPublishStagingWrap(
	prev func(destPath string, w io.Writer) io.Writer,
	tally *atomic.Int64,
) func(destPath string, w io.Writer) io.Writer {
	return func(path string, w io.Writer) io.Writer {
		if prev != nil {
			w = prev(path, w)
		}

		return &measuringPublishWriter{inner: w, tallyPtr: tally}
	}
}

var _ fileops.AtomicPublishStagingDecorator = (*MemStagingPublishSession)(nil)
