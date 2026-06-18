// perf_extract provides deterministic helpers for benchmarking and profiling extract throughput.
//
// Lower-level helpers (for example CountingSourceOpener, CountingOutputOpener, and
// NewSyntheticAbsentPathResolver) support focused tests and MeasurePipelineExtract wiring
// without routing through MemSourceOpener/MemOutputOpener on a shared mem filesystem.
//
// Contract-level pipeline performance measurement aligns with Gherkin and pipeline benchmarks:
// MeasurePipelineExtractCountingSrcMem in perf_extract_measurement.go runs pipeline.RunExtract
// with a counting source opener, MeasuringMemOutputOpener, and validate.PathResolver over the
// same afero mem FS (typically NewMemPathResolverWithFS) so absent/exists semantics match
// production MemOutput behavior.
//
// Callers still must satisfy validate.ValidatePath (absolute workspace-rooted paths).
// Helpers here use in-memory openers and [pipeline.OutputWriteIntent]; they do not import os.

package testutil

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"sync"
	"sync/atomic"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

// ExtractLineTerminator selects terminator bytes replicated on every synthetic line including the last.
type ExtractLineTerminator int

const (
	// ExtractLineTerminatorLF terminates each synthetic line with '\n'.
	ExtractLineTerminatorLF ExtractLineTerminator = iota + 1
	// ExtractLineTerminatorCRLF terminates each synthetic line with "\r\n".
	ExtractLineTerminatorCRLF
)

func (t ExtractLineTerminator) bytes() ([]byte, error) {
	switch t {
	case ExtractLineTerminatorLF:
		return []byte{'\n'}, nil
	case ExtractLineTerminatorCRLF:
		return []byte{'\r', '\n'}, nil
	default:
		return nil, fmt.Errorf("%w: value %d", errExtractUnknownLineTerminator, t)
	}
}

// ExtractFixture describes deterministic synthetic extract inputs.
type ExtractFixture struct {
	LineCount  int
	LineLength int
	Terminator ExtractLineTerminator
	Lines      fileops.LineRange
}

func validateExtractFixture(fx ExtractFixture) error {
	if fx.LineCount < 1 {
		return fmt.Errorf("%w, got %d", errExtractLineCountBelowMin, fx.LineCount)
	}

	if fx.LineLength < 0 {
		return fmt.Errorf("%w, got %d", errExtractLineLengthNegative, fx.LineLength)
	}

	return nil
}

func buildFixtureSourceBytes(fx ExtractFixture, term []byte) ([]byte, error) {
	linePayload := bytes.Repeat([]byte{'x'}, fx.LineLength)
	var bb bytes.Buffer
	bb.Grow(fx.LineCount * (fx.LineLength + len(term)))
	for range fx.LineCount {
		if _, werr := bb.Write(linePayload); werr != nil {
			return nil, fmt.Errorf("testutil perf extract: build source: %w", werr)
		}

		if _, werr := bb.Write(term); werr != nil {
			return nil, fmt.Errorf("testutil perf extract: build source terminator: %w", werr)
		}
	}

	return bytes.Clone(bb.Bytes()), nil
}

func verifyGoldTerminatorsMatchExtract(
	ctx context.Context,
	gold []byte,
	term []byte,
	linesExtracted int,
) error {
	if linesExtracted <= 0 || len(term) == 0 {
		return nil
	}

	expLines, rlErr := fileops.ReadLinesFromContext(ctx, bytes.NewReader(gold))
	if rlErr != nil {
		return rlErr
	}

	for _, ln := range expLines {
		if len(ln.Terminator) > 0 && !bytes.Equal(ln.Terminator, term) {
			return fmt.Errorf(
				"%w (%q vs %q); adjust fx.Terminator",
				errExtractGoldenTerminatorMismatch,
				string(ln.Terminator),
				string(term),
			)
		}
	}

	return nil
}

// BuildExtractFixture returns immutable source bytes and golden output bytes for fixture.Lines.
// Payload bytes per line repeat 'x' count times for stable lengths; terminator bytes follow each line,
// including the final line.
//
// The expected slice is computed by invoking fileops.Extract so fixtures stay aligned with production extract.
func BuildExtractFixture(ctx context.Context, fx ExtractFixture) (source, expected []byte, err error) {
	if vErr := validateExtractFixture(fx); vErr != nil {
		return nil, nil, vErr
	}

	term, tErr := fx.Terminator.bytes()
	if tErr != nil {
		return nil, nil, tErr
	}

	src, sErr := buildFixtureSourceBytes(fx, term)
	if sErr != nil {
		return nil, nil, sErr
	}

	var gold bytes.Buffer

	res, exErr := fileops.Extract(ctx, fileops.ExtractOptions{
		Source: bytes.NewReader(src),
		Lines:  fx.Lines,
	}, &gold)
	if exErr != nil {
		return nil, nil, exErr
	}

	goldBytes := gold.Bytes()

	if gErr := verifyGoldTerminatorsMatchExtract(ctx, goldBytes, term, res.LinesExtracted); gErr != nil {
		return nil, nil, gErr
	}

	return src, bytes.Clone(goldBytes), nil
}

// SyntheticAbsentPathResolver is a validator-only stub: Lstat always reports absent paths and
// EvalSymlinks is identity. It mirrors stub PathResolver implementations used by pipeline.RunExtract
// tests and never touches the filesystem.
type SyntheticAbsentPathResolver struct{}

// NewSyntheticAbsentPathResolver returns SyntheticAbsentPathResolver as validate.PathResolver.
func NewSyntheticAbsentPathResolver() validate.PathResolver {
	return SyntheticAbsentPathResolver{}
}

func (SyntheticAbsentPathResolver) Lstat(string) (fs.FileInfo, error) {
	return nil, fs.ErrNotExist
}

func (SyntheticAbsentPathResolver) EvalSymlinks(path string) (string, error) {
	return path, nil
}

// sourceCounters holds instrumentation shared across one or more counting readers tied to one logical source opener.
type sourceCounters struct {
	bytesRead  atomic.Int64
	readCalls  atomic.Int64
	seekCalls  atomic.Int64
	closeCalls atomic.Int64
}

func (sc *sourceCounters) reset() {
	if sc == nil {
		return
	}

	sc.bytesRead.Store(0)
	sc.readCalls.Store(0)
	sc.seekCalls.Store(0)
	sc.closeCalls.Store(0)
}

// CountingReader wraps bytes.Reader while recording reads, seeks, and closes onto shared counters.
type CountingReader struct {
	mu       sync.Mutex
	inner    *bytes.Reader
	counters *sourceCounters
}

// NewCountingReader returns an io.ReadSeekCloser over immutable src bytes recording into counters.
// The caller must keep src unchanged for the lifetime of the reader (bytes.Reader does not copy).
func NewCountingReader(src []byte, counters *sourceCounters) *CountingReader {
	b := bytes.NewReader(src)

	return &CountingReader{inner: b, counters: counters}
}

// Unwrap exposes the backing *bytes.Reader for tests that inspect seek offsets.
func (c *CountingReader) Unwrap() *bytes.Reader {
	if c == nil {
		return nil
	}

	return c.inner
}

func (c *CountingReader) safeCountersLocked() *sourceCounters {
	if c.counters == nil {
		return nil
	}

	return c.counters
}

func (c *CountingReader) Read(buf []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sc := c.safeCountersLocked()
	if sc != nil {
		sc.readCalls.Add(1)
	}

	readCount, readErr := c.inner.Read(buf)

	if sc != nil {
		sc.bytesRead.Add(int64(readCount))
	}

	return readCount, readErr
}

func (c *CountingReader) Seek(offset int64, whence int) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sc := c.safeCountersLocked()
	if sc != nil {
		sc.seekCalls.Add(1)
	}

	return c.inner.Seek(offset, whence)
}

// Close increments close instrumentation; backing bytes.Reader has no teardown.
func (c *CountingReader) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	sc := c.safeCountersLocked()

	if sc != nil {
		sc.closeCalls.Add(1)
	}

	return nil
}

// CountingWriter accumulates writes in memory without touching filesystem state.
type CountingWriter struct {
	buf          bytes.Buffer
	bytesWritten atomic.Int64
	writeOps     atomic.Int64
}

func (w *CountingWriter) Write(payload []byte) (int, error) {
	if w == nil {
		return 0, fmt.Errorf("%w", errNilCountingWriter)
	}

	w.writeOps.Add(1)

	writtenCount, writeErr := w.buf.Write(payload)
	if writeErr == nil {
		w.bytesWritten.Add(int64(writtenCount))
	}

	return writtenCount, writeErr
}

func (w *CountingWriter) Bytes() []byte {
	if w == nil {
		return nil
	}

	return w.buf.Bytes()
}

// BytesWritten returns total payload bytes appended through Write excluding internal accounting errors.
func (w *CountingWriter) BytesWritten() int64 {
	if w == nil {
		return 0
	}

	return w.bytesWritten.Load()
}

// WriteOps counts Write calls invoked.
func (w *CountingWriter) WriteOps() int64 {
	if w == nil {
		return 0
	}

	return w.writeOps.Load()
}

// Reset clears instrumentation and buffer for reuse inside the same goroutine-centric benchmark loop.
func (w *CountingWriter) Reset() {
	if w == nil {
		return
	}

	w.buf.Reset()

	w.bytesWritten.Store(0)
	w.writeOps.Store(0)
}
