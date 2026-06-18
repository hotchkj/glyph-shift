package fileops

import (
	"context"
	"errors"
	"fmt"
	"io"
)

const (
	defaultLinespanChunkSize = 64 * 1024
	// boundedEarlyExitLinespanChunkSize matches extract span planning: smaller reads avoid tail
	// over-fetch when seekable scans exit early (closed line range, MaxFiles cap).
	boundedEarlyExitLinespanChunkSize = 8 * 1024
	spanContextCheckLines             = 1000
	spanContextCheckBytes             = 8192
	errFmtScanLineSpansWrap           = "scan line spans: %w"
)

var (
	errScanLineSpansNilCallback   = errors.New("fileops.ScanLineSpans: nil onLine callback")
	errScanLineSpansReadZeroNoEOF = errors.New("scan line spans: read returned zero bytes without EOF")
)

// linespanScanOptions configures line-span scanning internals (tests and scanLineSpansWithOptions).
// Zero value means defaults: defaultLinespanChunkSize, spanContextCheckLines / spanContextCheckBytes cadences.
type linespanScanOptions struct {
	chunkSizeBytes    int // if >0, buffer slice capacity per read chunk
	contextCheckLines int // if >0, ctx probes after emitting this many lines
	contextCheckBytes int // if >0, ctx probes after consuming this many bytes within a line stride
}

// LineTerminatorNone means the logical line ends at EOF with no terminating newline bytes on disk.
type LineTerminatorKind int

const (
	LineTerminatorNone LineTerminatorKind = iota
	LineTerminatorLF
	LineTerminatorCRLF
	LineTerminatorCR
)

// Len returns the serialized terminator length encoded by this kind.
func (k LineTerminatorKind) Len() int {
	switch k {
	case LineTerminatorLF, LineTerminatorCR:
		return 1
	case LineTerminatorCRLF:
		return 2 //nolint:mnd // CRLF terminator is two bytes on Windows-style text
	case LineTerminatorNone:
		return 0
	default:
		return 0
	}
}

// LineSpan identifies one logical line within original serialized source bytes (half-open offsets).
// Serialized spans include terminator bytes whenever kind is LF/CRLF/CR; kinds of None omit extra bytes
// beyond ContentEnd.
// ContentStart equals SerializedStart before optional terminator overlap.
// Sequential spans satisfy prev.SerializedEnd == next.SerializedStart.
//
// ScanLineSpans emits metadata only — never full logical-line []byte bodies.
type LineSpan struct {
	LineNum         int
	SerializedStart uint64
	SerializedEnd   uint64
	ContentStart    uint64
	ContentEnd      uint64
	Terminator      LineTerminatorKind
}

// ScanLineSpans reads src from offset zero via fixed-size Reads, parses newlines centrally, and calls
// onLine for each logical line.
// Empty streams yield zero invocations mirroring ReadLinesFromContext empties.
//
// Cancellation: probes ctx.Err() on a byte cadence plus after a line cadence (matching
// spanContextCheckBytes/Lines defaults), analogous to periodic checks in ReadLinesFromContext streaming.
func ScanLineSpans(ctx context.Context, src io.ReadSeeker, onLine func(LineSpan) error) error {
	return scanLineSpansWithOptions(ctx, src, onLine, linespanScanOptions{})
}

func scanLineSpansWithOptions(
	ctx context.Context,
	src io.ReadSeeker,
	onLine func(LineSpan) error,
	opts linespanScanOptions,
) error {
	if onLine == nil {
		return errScanLineSpansNilCallback
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("scan line spans: seek stream start: %w", err)
	}

	sz := defaultLinespanChunkSize
	if opts.chunkSizeBytes > 0 {
		sz = opts.chunkSizeBytes
	}

	st := scannerState{
		ctx: ctx, src: src, onLine: onLine,
		opts: opts,
		buf:  make([]byte, sz),
	}

	return st.run()
}

type scannerState struct {
	ctx context.Context
	src io.ReadSeeker

	opts   linespanScanOptions
	onLine func(LineSpan) error

	buf  []byte
	base uint64 // absolute byte offset mapped to buf[0]
	fill int

	cursor    uint64 // next classifier offset in source coordinates
	lineBegin uint64 // current line serialized/content start offset
	lineNum   int

	bytesSinceCk int
	emittedCk    int

	streamEOF bool
}

func (s *scannerState) finalizeTrailing() error {
	if err := s.ctx.Err(); err != nil {
		return fmt.Errorf(errFmtScanLineSpansWrap, err)
	}

	if !s.needsTrailingIncompleteLineFlush() {
		return nil
	}

	contentEndExclusive := s.cursor
	s.lineNum++
	span := LineSpan{
		LineNum:         s.lineNum,
		SerializedStart: s.lineBegin,
		SerializedEnd:   contentEndExclusive,
		ContentStart:    s.lineBegin,
		ContentEnd:      contentEndExclusive,
		Terminator:      LineTerminatorNone,
	}

	if err := s.onLine(span); err != nil {
		return err
	}

	s.emittedCk++
	s.lineBegin = contentEndExclusive
	if err := s.lineEmitCtxProbe(); err != nil {
		return err
	}

	if err := s.ctx.Err(); err != nil {
		return fmt.Errorf(errFmtScanLineSpansWrap, err)
	}

	return nil
}

func (s *scannerState) needsTrailingIncompleteLineFlush() bool {
	return s.cursor > s.lineBegin
}

func (s *scannerState) emitTerminated(term LineTerminatorKind, terminatorLeadAbs uint64) error {
	if terminatorLeadAbs != s.cursor {
		panic("scannerState.emitTerminated terminator misaligned")
	}

	contentExclusive := terminatorLeadAbs
	serializedEnd := contentExclusive + uint64(term.Len()) //nolint:gosec // G115: terminator length is 0..2

	s.lineNum++
	ln := LineSpan{
		LineNum:         s.lineNum,
		SerializedStart: s.lineBegin,
		SerializedEnd:   serializedEnd,
		ContentStart:    s.lineBegin,
		ContentEnd:      contentExclusive,
		Terminator:      term,
	}

	if err := s.onLine(ln); err != nil {
		return err
	}

	s.emittedCk++
	s.lineBegin = serializedEnd

	return s.lineEmitCtxProbe()
}

func (s *scannerState) lineEmitCtxProbe() error {
	if s.emittedCk < s.lineProbeCadence() {
		return nil
	}

	s.emittedCk = 0
	if err := s.ctx.Err(); err != nil {
		return fmt.Errorf(errFmtScanLineSpansWrap, err)
	}

	return nil
}

func (s *scannerState) lineProbeCadence() int {
	if s.opts.contextCheckLines > 0 {
		return s.opts.contextCheckLines
	}

	return spanContextCheckLines
}

func (s *scannerState) byteProbeCadence() int {
	if s.opts.contextCheckBytes > 0 {
		return s.opts.contextCheckBytes
	}

	return spanContextCheckBytes
}

func (s *scannerState) noteConsumedSerialized(n int) error {
	s.cursor += uint64(n) //nolint:gosec // G115: Read() byte count is non-negative

	s.bytesSinceCk += n
	step := s.byteProbeCadence()
	for s.bytesSinceCk >= step {
		s.bytesSinceCk -= step
		if err := s.ctx.Err(); err != nil {
			return fmt.Errorf(errFmtScanLineSpansWrap, err)
		}
	}

	return nil
}
