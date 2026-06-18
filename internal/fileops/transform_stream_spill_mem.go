package fileops

import (
	"errors"
	"fmt"
	"io"
	"sync/atomic"
)

var (
	errWhitespaceSpillSeekInvalidWhence = errors.New("mem whitespace spill seek: invalid whence")
	errWhitespaceSpillSeekNegativePos   = errors.New("mem whitespace spill seek: negative position")
)

const memWhitespaceSpillPrefix = "memspill://"

// pendingWhitespaceSpillCreations counts spill scratch files opened (tests reset/load).
var pendingWhitespaceSpillCreations atomic.Uint32

type memWhitespaceSpillBacking struct {
	nextID int
}

// NewMemWhitespaceSpillBacking returns in-memory spill storage for transform stream tests.
func NewMemWhitespaceSpillBacking() WhitespaceSpillBacking {
	return &memWhitespaceSpillBacking{}
}

func (b *memWhitespaceSpillBacking) CreateScratch(pattern string) (WhitespaceSpillFile, error) {
	_ = pattern

	b.nextID++
	name := fmt.Sprintf("%s%d", memWhitespaceSpillPrefix, b.nextID)
	pendingWhitespaceSpillCreations.Add(1)

	return &memWhitespaceSpillFile{name: name}, nil
}

func (*memWhitespaceSpillBacking) RemoveScratch(string) error {
	return nil
}

type memWhitespaceSpillFile struct {
	name string
	data []byte
	off  int64
}

func (f *memWhitespaceSpillFile) Read(buf []byte) (int, error) {
	if f.off >= int64(len(f.data)) {
		return 0, io.EOF
	}

	n := copy(buf, f.data[f.off:])
	f.off += int64(n)

	return n, nil
}

func (f *memWhitespaceSpillFile) Write(buf []byte) (int, error) {
	end := f.off + int64(len(buf))
	if end > int64(len(f.data)) {
		grown := make([]byte, end)
		copy(grown, f.data)
		f.data = grown
	}

	copy(f.data[f.off:], buf)
	f.off += int64(len(buf))

	return len(buf), nil
}

func (f *memWhitespaceSpillFile) Seek(off int64, whence int) (int64, error) {
	var base int64

	switch whence {
	case io.SeekStart:
		base = 0
	case io.SeekCurrent:
		base = f.off
	case io.SeekEnd:
		base = int64(len(f.data))
	default:
		return 0, errWhitespaceSpillSeekInvalidWhence
	}

	next := base + off
	if next < 0 {
		return 0, errWhitespaceSpillSeekNegativePos
	}

	f.off = next

	return f.off, nil
}

func (f *memWhitespaceSpillFile) Sync() error         { return nil }
func (f *memWhitespaceSpillFile) Close() error        { return nil }
func (f *memWhitespaceSpillFile) Name() string        { return f.name }
func (f *memWhitespaceSpillFile) ScratchName() string { return f.name }

var _ SessionTempHandle = (*memWhitespaceSpillFile)(nil)

// memWhitespaceSpillForTests supplies in-memory spill storage when a write path may spill.
func memWhitespaceSpillForTests(out io.Writer, opts TransformOptions) WhitespaceSpillBacking {
	if out == nil || !opts.TrimTrailing {
		return nil
	}

	return NewMemWhitespaceSpillBacking()
}
