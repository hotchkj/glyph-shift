package testutil

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sync/atomic"
)

// TailGuardSourceOpener opens a ReadSeekCloser that serves only an in-memory prefix, then forbids
// further reads via ErrBoundednessTailConsumptionForbidden so pipelines that scan the remainder of
// the logical file fail deterministically once the guarded boundary is crossed.
//
// Separate source read and destination write instrumentation match CountingSourceOpener patterns:
// Aggregate* methods belong to the opener; destination metrics must be read from the chosen
// output instrumentation opener (including CountingOutputOpener).
type TailGuardSourceOpener struct {
	Prefix      []byte
	AllowedPath string

	opens atomic.Int64
	ctr   sourceCounters
}

// NewTailGuardSourceOpener returns a TailGuardSourceOpener; prefix is captured by reference and
// must not be mutated for the opener's lifetime.
func NewTailGuardSourceOpener(prefix []byte, allowedPath string) *TailGuardSourceOpener {
	return &TailGuardSourceOpener{Prefix: prefix, AllowedPath: allowedPath}
}

func (o *TailGuardSourceOpener) Open(path string) (io.ReadSeekCloser, error) {
	if o.AllowedPath != "" && filepath.Clean(path) != filepath.Clean(o.AllowedPath) {
		return nil, fmt.Errorf("open %q: %w", path, fs.ErrNotExist)
	}

	o.opens.Add(1)

	return newTailGuardReadSeekCloser(o.Prefix, &o.ctr), nil
}

func (o *TailGuardSourceOpener) Opens() int64 {
	return o.opens.Load()
}

func (o *TailGuardSourceOpener) AggregateSourceBytesRead() int64 {
	return o.ctr.bytesRead.Load()
}

func (o *TailGuardSourceOpener) AggregateSourceReadCalls() int64 {
	return o.ctr.readCalls.Load()
}

func (o *TailGuardSourceOpener) AggregateSourceSeekCalls() int64 {
	return o.ctr.seekCalls.Load()
}

func (o *TailGuardSourceOpener) AggregateSourceCloseCalls() int64 {
	return o.ctr.closeCalls.Load()
}

type tailGuardReadSeekCloser struct {
	prefix []byte
	pos    int64
	ctr    *sourceCounters
}

func newTailGuardReadSeekCloser(prefix []byte, ctr *sourceCounters) *tailGuardReadSeekCloser {
	return &tailGuardReadSeekCloser{prefix: prefix, ctr: ctr}
}

//nolint:varnamelen // io.Reader convention uses `p` for the scratch slice.
func (r *tailGuardReadSeekCloser) Read(p []byte) (int, error) {
	if r.ctr != nil {
		r.ctr.readCalls.Add(1)
	}

	if r.pos >= int64(len(r.prefix)) {
		if len(p) == 0 {
			return 0, nil
		}

		return 0, ErrBoundednessTailConsumptionForbidden
	}

	avail := int64(len(r.prefix)) - r.pos
	want := int64(len(p))

	chunk := avail
	if want < avail {
		chunk = want
	}

	copy(p, r.prefix[r.pos:r.pos+chunk])
	r.pos += chunk

	if r.ctr != nil {
		r.ctr.bytesRead.Add(chunk)
	}

	return int(chunk), nil
}

func (r *tailGuardReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	if r.ctr != nil {
		r.ctr.seekCalls.Add(1)
	}

	var abs int64

	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = r.pos + offset
	case io.SeekEnd:
		abs = int64(len(r.prefix)) + offset
	default:
		return 0, ErrTailGuardSeekInvalidWhence
	}

	if abs < 0 {
		return 0, ErrTailGuardSeekNegativePosition
	}

	if abs > int64(len(r.prefix)) {
		abs = int64(len(r.prefix))
	}

	r.pos = abs

	return r.pos, nil
}

func (r *tailGuardReadSeekCloser) Close() error {
	if r.ctr != nil {
		r.ctr.closeCalls.Add(1)
	}

	return nil
}
