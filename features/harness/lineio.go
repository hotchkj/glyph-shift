package harness

import (
	"bufio"
	"context"
	"fmt"
	"io"
)

const contextCheckInterval = 1000

// Line represents a single line with its exact terminator preserved (byte-faithful,
// aligned with internal/fileops semantics for BDD assertions).
type Line struct {
	Content    []byte // Line content without terminator
	Terminator []byte // Exact terminator: \n, \r\n, \r, or nil (last line without terminator)
}

func flushTrailingLine(lines []Line, content []byte) []Line {
	if len(content) > 0 {
		return append(lines, Line{Content: content, Terminator: nil})
	}

	return lines
}

func processCarriageReturn(br *bufio.Reader, lines []Line, content []byte) ([]Line, []byte, error) {
	next, err := br.ReadByte()
	if err != nil {
		if err == io.EOF {
			return append(lines, Line{Content: content, Terminator: []byte{'\r'}}), nil, nil
		}

		return lines, content, err
	}

	if next == '\n' {
		return append(lines, Line{Content: content, Terminator: []byte{'\r', '\n'}}), nil, nil
	}

	return append(lines, Line{Content: content, Terminator: []byte{'\r'}}), append([]byte(nil), next), nil
}

// ReadLinesFrom reads all lines from r, preserving exact terminators per line.
func ReadLinesFrom(r io.Reader) ([]Line, error) {
	return ReadLinesFromContext(context.Background(), r)
}

type lineReader struct {
	ctx             context.Context
	br              *bufio.Reader
	lines           []Line
	content         []byte
	linesSinceCheck int
}

func (lr *lineReader) checkContext() error {
	lr.linesSinceCheck++
	if lr.linesSinceCheck >= contextCheckInterval {
		lr.linesSinceCheck = 0

		if err := lr.ctx.Err(); err != nil {
			return fmt.Errorf("read lines: %w", err)
		}
	}

	return nil
}

func (lr *lineReader) commitLine(terminator []byte) error {
	lr.lines = append(lr.lines, Line{Content: lr.content, Terminator: terminator})
	lr.content = nil

	return lr.checkContext()
}

func (lr *lineReader) handleCarriageReturn() error {
	var procErr error

	lr.lines, lr.content, procErr = processCarriageReturn(lr.br, lr.lines, lr.content)
	if procErr != nil {
		return fmt.Errorf("read lines: %w", procErr)
	}

	return lr.checkContext()
}

// ReadLinesFromContext reads all lines from r with periodic context cancellation checks.
func ReadLinesFromContext(ctx context.Context, r io.Reader) ([]Line, error) {
	lr := &lineReader{ctx: ctx, br: bufio.NewReader(r)}

	for {
		byt, err := lr.br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return flushTrailingLine(lr.lines, lr.content), nil
			}

			return nil, fmt.Errorf("read lines: %w", err)
		}

		switch byt {
		case '\n':
			if ctxErr := lr.commitLine([]byte{'\n'}); ctxErr != nil {
				return nil, ctxErr
			}
		case '\r':
			if crErr := lr.handleCarriageReturn(); crErr != nil {
				return nil, crErr
			}
		default:
			lr.content = append(lr.content, byt)
		}
	}
}

// WriteLinesTo writes lines to w, writing each Content followed by its Terminator.
func WriteLinesTo(writer io.Writer, lines []Line) error {
	for _, ln := range lines {
		if _, err := writer.Write(ln.Content); err != nil {
			return fmt.Errorf("write lines: %w", err)
		}

		if len(ln.Terminator) > 0 {
			if _, err := writer.Write(ln.Terminator); err != nil {
				return fmt.Errorf("write lines: %w", err)
			}
		}
	}

	return nil
}
