package fileops

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
)

const contextCheckInterval = 1000

const (
	// binaryCheckSize matches Git's FIRST_FEW_BYTES (xdiff-interface.c).
	// Git considers a file binary if any null byte appears in this window.
	binaryCheckSize = 8000
	// maxConsecutiveZeroReads caps spin when a Reader returns (0, nil) repeatedly.
	maxConsecutiveZeroReads = 10
)

// errWriteFullInvalidCount is returned when io.Write reports a negative byte count.
var errWriteFullInvalidCount = errors.New("write lines: invalid Write count")

// errReadLinesFormat is the fmt.Errorf format for read-path failures; kept as one literal for Sonar.
const errReadLinesFormat = "read lines: %w"

// Line represents a single line with its exact terminator preserved.
type Line struct {
	Content    []byte // Line content without terminator
	Terminator []byte // Exact terminator: \n, \r\n, \r, or nil (last line without terminator)
}

// ReadLinesFrom reads all lines from r, preserving exact terminators per line.
func ReadLinesFrom(r io.Reader) ([]Line, error) {
	return ReadLinesFromContext(context.Background(), r)
}

// ForEachLineFromContext streams callbacks over r, invoking onLine for each logical line.
// Terminators and trailing content without a terminator match ReadLinesFromContext semantics.
// Context cancellation is checked every contextCheckInterval completed lines.
// Before each callback, one full logical Line is materialized: Line.Content holds the entire logical
// line body for that call (not a sliding window). The callback layer does not retain prior lines, but
// this is still not a bounded-memory primitive for pathologically large single lines.
func ForEachLineFromContext(ctx context.Context, r io.Reader, onLine func(Line) error) error {
	stream := &lineStream{ctx: ctx, br: bufio.NewReader(r), onLine: onLine}

	return stream.run()
}

type lineStream struct {
	ctx             context.Context
	br              *bufio.Reader
	content         []byte
	linesSinceCheck int
	onLine          func(Line) error
}

func (s *lineStream) run() error {
	for {
		done, err := s.readNextLineByte()
		if done || err != nil {
			return err
		}
	}
}

func (s *lineStream) checkContext() error {
	s.linesSinceCheck++
	if s.linesSinceCheck >= contextCheckInterval {
		s.linesSinceCheck = 0

		if err := s.ctx.Err(); err != nil {
			return fmt.Errorf(errReadLinesFormat, err)
		}
	}

	return nil
}

func (s *lineStream) readNextLineByte() (bool, error) {
	byt, err := s.br.ReadByte()
	if err != nil {
		if err == io.EOF {
			return true, s.flushTrailing()
		}

		return true, fmt.Errorf(errReadLinesFormat, err)
	}

	switch byt {
	case '\n':
		return false, s.commitLine([]byte{'\n'})
	case '\r':
		return false, s.handleCarriageReturn()
	default:
		s.content = append(s.content, byt)

		return false, nil
	}
}

func (s *lineStream) commitLine(terminator []byte) error {
	if err := s.onLine(Line{Content: s.content, Terminator: terminator}); err != nil {
		return err
	}

	s.content = nil

	return s.checkContext()
}

func (s *lineStream) handleCarriageReturn() error {
	next, err := s.br.ReadByte()
	if err != nil {
		if err == io.EOF {
			return s.commitLine([]byte{'\r'})
		}

		return fmt.Errorf(errReadLinesFormat, err)
	}

	if next == '\n' {
		return s.commitLine([]byte{'\r', '\n'})
	}

	if err := s.commitLine([]byte{'\r'}); err != nil {
		return err
	}

	s.content = append([]byte(nil), next)

	return nil
}

func (s *lineStream) flushTrailing() error {
	if len(s.content) == 0 {
		return nil
	}

	return s.onLine(Line{Content: s.content, Terminator: nil})
}

// ReadLinesFromContext reads every logical line from r and materializes all of them into a []Line in
// memory; it is not a bounded-memory primitive for inputs with huge logical lines or enormous line counts.
// Checks ctx.Err() every 1000 lines to balance responsiveness with throughput.
func ReadLinesFromContext(ctx context.Context, r io.Reader) ([]Line, error) {
	var lines []Line

	err := ForEachLineFromContext(ctx, r, func(ln Line) error {
		lines = append(lines, ln)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return lines, nil
}

// WriteFull writes every byte of data to w, retrying short writes until completion.
func WriteFull(w io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := w.Write(data)
		if written < 0 {
			return fmt.Errorf("%w", errWriteFullInvalidCount)
		}

		data = data[written:]

		if err != nil {
			return fmt.Errorf("write lines: %w", err)
		}
	}

	return nil
}

// WriteLinesTo writes lines to w, writing each Content followed by its Terminator.
func WriteLinesTo(writer io.Writer, lines []Line) error {
	for _, ln := range lines {
		if err := WriteFull(writer, ln.Content); err != nil {
			return err
		}

		if len(ln.Terminator) > 0 {
			if err := WriteFull(writer, ln.Terminator); err != nil {
				return err
			}
		}
	}

	return nil
}

func scanForNull(buf []byte) bool {
	for _, b := range buf {
		if b == 0 {
			return true
		}
	}

	return false
}

// IsBinary uses the same heuristic as Git's buffer_is_binary (xdiff-interface.c):
// scan the first 8000 bytes for any null byte (0x00). Returns true if found.
func IsBinary(reader io.Reader) (bool, error) {
	buf := make([]byte, binaryCheckSize)
	total := 0
	var zeroReads int

	for total < binaryCheckSize {
		numRead, err := reader.Read(buf[total:])
		var stop bool
		zeroReads, stop = nextZeroReadCount(zeroReads, numRead, err)
		if stop {
			return false, nil
		}
		if scanForNull(buf[total : total+numRead]) {
			return true, nil
		}

		total += numRead
		if err == io.EOF {
			return false, nil
		}

		if err != nil {
			return false, fmt.Errorf("is binary: %w", err)
		}
	}

	return false, nil
}

func nextZeroReadCount(zeroReads, numRead int, err error) (int, bool) {
	if numRead > 0 {
		return 0, false
	}
	if err != nil {
		return zeroReads, false
	}

	zeroReads++

	return zeroReads, zeroReads >= maxConsecutiveZeroReads
}
