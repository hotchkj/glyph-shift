package testutil

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

// errFmtOpenPathWrap is the shared format for wrapping Open failures (%w preserves ErrExist /
// ErrNotExist identity for callers using errors.Is).
const errFmtOpenPathWrap = "open %q: %w"

type outputIntentPreflight struct {
	exists func(path string) bool
	remove func(path string)
}

func preflightOutputWriteIntent(path string, intent pipeline.OutputWriteIntent, gate outputIntentPreflight) error {
	if intent == pipeline.OutputCreateExclusive && gate.exists(path) {
		return fmt.Errorf(errFmtOpenPathWrap, path, fs.ErrExist)
	}

	if intent == pipeline.OutputCreateOrReplace {
		gate.remove(path)
	}

	return nil
}

// CountingSourceOpener satisfies pipeline.SourceOpener with deterministic in-memory Opens.
type CountingSourceOpener struct {
	Immutable []byte
	// AllowedPath unset means any Open path succeeds; when set Open must receive exactly this path (after filepath.Clean).
	AllowedPath string

	opens atomic.Int64
	cntr  sourceCounters
}

func (o *CountingSourceOpener) Open(path string) (io.ReadSeekCloser, error) {
	if o.AllowedPath != "" && filepath.Clean(path) != filepath.Clean(o.AllowedPath) {
		return nil, fmt.Errorf(errFmtOpenPathWrap, path, fs.ErrNotExist)
	}

	if o.Immutable == nil {
		o.Immutable = []byte{}
	}

	o.opens.Add(1)

	return NewCountingReader(o.Immutable, &o.cntr), nil
}

func (o *CountingSourceOpener) Opens() int64 {
	return o.opens.Load()
}

func (o *CountingSourceOpener) AggregateSourceBytesRead() int64 {
	return o.cntr.bytesRead.Load()
}

func (o *CountingSourceOpener) AggregateSourceReadCalls() int64 {
	return o.cntr.readCalls.Load()
}

func (o *CountingSourceOpener) AggregateSourceSeekCalls() int64 {
	return o.cntr.seekCalls.Load()
}

func (o *CountingSourceOpener) AggregateSourceCloseCalls() int64 {
	return o.cntr.closeCalls.Load()
}

// ResetCounters clears Opens and read instrumentation for repeatable benchmark iterations.
//
// Caller must serialize use; shared readers must be closed beforehand.
func (o *CountingSourceOpener) ResetCounters() {
	o.opens.Store(0)
	o.cntr.reset()
}

type countingOutputWriteCloser struct {
	parent *CountingOutputOpener
	path   string
	buf    bytes.Buffer
}

func (w *countingOutputWriteCloser) Write(payload []byte) (int, error) {
	if len(payload) == 0 {
		return 0, nil
	}

	writtenCount, writeErr := w.buf.Write(payload)

	if writeErr == nil {
		w.parent.bytesWritten.Add(int64(writtenCount))
	}

	return writtenCount, writeErr
}

func (w *countingOutputWriteCloser) Close() error {
	payload := append([]byte(nil), w.buf.Bytes()...)
	w.parent.commit(w.path, payload)

	return nil
}

// CountingOutputOpener satisfies pipeline.OutputOpener without persisting anywhere off-heap permanently;
// destinations are keyed by normalized path strings.
type CountingOutputOpener struct {
	mu sync.RWMutex

	mkdirAllCalls    atomic.Int64
	destinationOpens atomic.Int64
	bytesWritten     atomic.Int64

	files map[string][]byte
}

// NewCountingOutputOpener returns an empty output opener with in-memory commit-on-close semantics.
func NewCountingOutputOpener() *CountingOutputOpener {
	return &CountingOutputOpener{files: make(map[string][]byte)}
}

func (o *CountingOutputOpener) pathKey(path string) string {
	return filepath.Clean(path)
}

func (o *CountingOutputOpener) existsLocked(path string) bool {
	_, ok := o.files[o.pathKey(path)]

	return ok
}

func (o *CountingOutputOpener) getCopyLocked(path string) []byte {
	data, ok := o.files[o.pathKey(path)]
	if !ok {
		return nil
	}

	return bytes.Clone(data)
}

func (o *CountingOutputOpener) commit(path string, payload []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()

	key := o.pathKey(path)

	o.files[key] = bytes.Clone(payload)
}

// MkdirAll records calls and succeeds without touching the filesystem.
func (o *CountingOutputOpener) MkdirAll(string, fs.FileMode) error {
	o.mkdirAllCalls.Add(1)

	return nil
}

func (o *CountingOutputOpener) logicalExists(path string) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return o.existsLocked(path)
}

func (o *CountingOutputOpener) logicalRemove(path string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	delete(o.files, o.pathKey(path))
}

// OpenFile mirrors MemOutputOpener intent semantics on logical paths only.
func (o *CountingOutputOpener) OpenFile(
	path string,
	intent pipeline.OutputWriteIntent,
	_ fs.FileMode,
) (io.WriteCloser, error) {
	o.destinationOpens.Add(1)

	if err := preflightOutputWriteIntent(path, intent, outputIntentPreflight{
		exists: o.logicalExists,
		remove: o.logicalRemove,
	}); err != nil {
		return nil, err
	}

	var appendSeed []byte

	if intent == pipeline.OutputAppend {
		o.mu.RLock()
		appendSeed = o.getCopyLocked(path)
		o.mu.RUnlock()
	}

	wc := &countingOutputWriteCloser{parent: o, path: path}

	if len(appendSeed) > 0 {
		seedWritten, werr := wc.buf.Write(appendSeed)
		o.bytesWritten.Add(int64(seedWritten))
		if werr != nil {
			return nil, fmt.Errorf("testutil counting output opener append seed %q: %w", path, werr)
		}
	}

	return wc, nil
}

// OutputBytesSnapshot returns committed bytes for the logical destination path after writers Close.
func (o *CountingOutputOpener) OutputBytesSnapshot(path string) []byte {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return o.getCopyLocked(path)
}

func (o *CountingOutputOpener) MkdirAllCalls() int64 {
	return o.mkdirAllCalls.Load()
}

func (o *CountingOutputOpener) DestinationOpens() int64 {
	return o.destinationOpens.Load()
}

func (o *CountingOutputOpener) BytesWritten() int64 {
	return o.bytesWritten.Load()
}

// Reset clears logical files and counters for benchmark loops; not safe if RunExtract is in flight.
func (o *CountingOutputOpener) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.files = make(map[string][]byte)

	o.mkdirAllCalls.Store(0)
	o.destinationOpens.Store(0)
	o.bytesWritten.Store(0)
}

// nonRetainingOutputWriteCloser discards payloads while counting BytesWritten on the parent opener.
type nonRetainingOutputWriteCloser struct {
	parent *NonRetainingOutputOpener
	path   string
}

func (w *nonRetainingOutputWriteCloser) Write(payload []byte) (int, error) {
	if len(payload) == 0 {
		return 0, nil
	}

	n := len(payload)
	w.parent.bytesWritten.Add(int64(n))

	return n, nil
}

func (w *nonRetainingOutputWriteCloser) Close() error {
	w.parent.commitLogicalClose(w.path)

	return nil
}

// NonRetainingOutputOpener implements pipeline.OutputOpener for boundedness heap probes:
// logical destination existence is tracked by normalized path strings (for O_EXCL / O_TRUNC,
// mirroring CountingOutputOpener), but committed payloads are discarded — nothing is buffered
// for OutputBytesSnapshot.
//
// O_APPEND openings do not prepend prior committed bytes to the writer: previous payload is never
// stored, so append seed semantics from CountingOutputOpener cannot be replayed. Split/blocks
// residency tests open destinations with truncate-or-exhaustive-create flags only.
type NonRetainingOutputOpener struct {
	mu sync.RWMutex

	mkdirAllCalls    atomic.Int64
	destinationOpens atomic.Int64
	bytesWritten     atomic.Int64

	committed map[string]struct{}
}

// NewNonRetainingOutputOpener returns an output opener for boundedness / residency probes (paired
// RetainedHeapAllocDelta / PeakHeapAllocDelta helpers) without retaining committed destination bytes.
func NewNonRetainingOutputOpener() *NonRetainingOutputOpener {
	return &NonRetainingOutputOpener{committed: make(map[string]struct{})}
}

func (o *NonRetainingOutputOpener) pathKey(path string) string {
	return filepath.Clean(path)
}

func (o *NonRetainingOutputOpener) existsLocked(path string) bool {
	_, ok := o.committed[o.pathKey(path)]

	return ok
}

func (o *NonRetainingOutputOpener) commitLogicalClose(path string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.committed[o.pathKey(path)] = struct{}{}
}

func (o *NonRetainingOutputOpener) logicalExists(path string) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return o.existsLocked(path)
}

func (o *NonRetainingOutputOpener) logicalRemove(path string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	delete(o.committed, o.pathKey(path))
}

// MkdirAll records calls and succeeds without touching the filesystem.
func (o *NonRetainingOutputOpener) MkdirAll(string, fs.FileMode) error {
	o.mkdirAllCalls.Add(1)

	return nil
}

// OpenFile mirrors CountingOutputOpener intent semantics on logical paths.
// Payloads written before Close are counted and discarded on Close.
func (o *NonRetainingOutputOpener) OpenFile(
	path string,
	intent pipeline.OutputWriteIntent,
	_ fs.FileMode,
) (io.WriteCloser, error) {
	o.destinationOpens.Add(1)

	if err := preflightOutputWriteIntent(path, intent, outputIntentPreflight{
		exists: o.logicalExists,
		remove: o.logicalRemove,
	}); err != nil {
		return nil, err
	}

	return &nonRetainingOutputWriteCloser{parent: o, path: path}, nil
}

// OutputBytesSnapshot always returns nil: committed payloads are not retained after Close.
func (o *NonRetainingOutputOpener) OutputBytesSnapshot(string) []byte {
	return nil
}

func (o *NonRetainingOutputOpener) MkdirAllCalls() int64 {
	return o.mkdirAllCalls.Load()
}

func (o *NonRetainingOutputOpener) DestinationOpens() int64 {
	return o.destinationOpens.Load()
}

func (o *NonRetainingOutputOpener) BytesWritten() int64 {
	return o.bytesWritten.Load()
}

// Reset clears logical destinations and counters; not safe if RunSplit or RunBlocks is in flight.
func (o *NonRetainingOutputOpener) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.committed = make(map[string]struct{})

	o.mkdirAllCalls.Store(0)
	o.destinationOpens.Store(0)
	o.bytesWritten.Store(0)
}
