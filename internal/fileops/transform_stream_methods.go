package fileops

import (
	"bytes"
	"fmt"
	"io"
)

func (s *transformStream) checkContextAfterLine() error {
	s.linesSinceCheck++
	if s.linesSinceCheck >= contextCheckInterval {
		s.linesSinceCheck = 0

		if err := s.ctx.Err(); err != nil {
			return fmt.Errorf(errReadLinesFormat, err)
		}
	}

	return nil
}

func (s *transformStream) emitContentByte(chr byte) error {
	if !s.opts.TrimTrailing {
		return s.emitUntrimmedContentByte(chr)
	}

	if s.bw == nil {
		return s.observeTrimmedContentByte(chr)
	}

	switch chr {
	case ' ', '\t':
		s.lineBodyBytes++

		return s.pending.push(chr)
	default:
		if err := s.pending.flushTo(s.bw); err != nil {
			return err
		}

		s.lineBodyBytes++

		if err := s.bw.WriteByte(chr); err != nil {
			return fmt.Errorf("write lines: %w", err)
		}

		return nil
	}
}

func (s *transformStream) emitUntrimmedContentByte(chr byte) error {
	s.lineBodyBytes++

	if s.bw == nil {
		return nil
	}

	if err := s.bw.WriteByte(chr); err != nil {
		return fmt.Errorf("write lines: %w", err)
	}

	return nil
}

func (s *transformStream) observeTrimmedContentByte(chr byte) error {
	switch chr {
	case ' ', '\t':
		s.trimTrailingRun++
	default:
		s.trimTrailingRun = 0
	}

	s.lineBodyBytes++

	return nil
}

func (s *transformStream) handleCR() error {
	next, err := s.br.ReadByte()
	if err != nil {
		if err == io.EOF {
			return s.completeLine([]byte{'\r'}, false)
		}

		return fmt.Errorf(errReadLinesFormat, err)
	}

	if next == '\n' {
		return s.completeLine([]byte{'\r', '\n'}, false)
	}

	if err := s.completeLine([]byte{'\r'}, false); err != nil {
		return err
	}

	return s.emitContentByte(next)
}

func (s *transformStream) writeLineTerminatorOutOnlyStats(srcTerm []byte, eof bool) {
	switch {
	case eof:
		if s.opts.FinalNewline && len(srcTerm) == 0 {
			s.res.FinalNewlineAdded = true
		}
	case s.opts.LineEndings != nil && len(srcTerm) > 0:
	case len(srcTerm) > 0:
	}
}

func (s *transformStream) writeLineTerminatorOutWrite(srcTerm []byte, eof bool) error {
	if eof {
		return s.writeEOFLineTerminator(srcTerm)
	}

	return s.writeNonEOFLineTerminator(srcTerm)
}

func (s *transformStream) writeEOFLineTerminator(srcTerm []byte) error {
	if !s.opts.FinalNewline || len(srcTerm) != 0 {
		return nil
	}

	s.res.FinalNewlineAdded = true

	return WriteFull(s.bw, defaultFinalTerminator(s.opts))
}

func (s *transformStream) writeNonEOFLineTerminator(srcTerm []byte) error {
	if len(srcTerm) == 0 {
		return nil
	}

	if s.opts.LineEndings != nil {
		return WriteFull(s.bw, s.want)
	}

	return WriteFull(s.bw, srcTerm)
}

func (s *transformStream) writeLineTerminatorOut(srcTerm []byte, eof bool) error {
	if s.bw == nil {
		s.writeLineTerminatorOutOnlyStats(srcTerm, eof)

		return nil
	}

	return s.writeLineTerminatorOutWrite(srcTerm, eof)
}

// completeLine finishes the current logical line. srcTerm is nil only for an incomplete last
// line at EOF; eof must be true when srcTerm is nil.
func (s *transformStream) completeLine(srcTerm []byte, eof bool) error {
	s.completeTrimTrailing()
	s.observeLineEnding(srcTerm)

	if err := s.writeLineTerminatorOut(srcTerm, eof); err != nil {
		return err
	}

	s.lineBodyBytes = 0

	return s.checkContextAfterLine()
}

func (s *transformStream) completeTrimTrailing() {
	if !s.opts.TrimTrailing {
		return
	}

	if s.bw != nil {
		s.completeTrimTrailingWrite()

		return
	}

	if s.trimTrailingRun > 0 {
		s.res.TrailingTrimmed++
		s.trimTrailingRun = 0
	}
}

func (s *transformStream) completeTrimTrailingWrite() {
	if !s.pending.nonEmpty() {
		return
	}

	s.res.TrailingTrimmed++
	s.pending.discard()
}

func (s *transformStream) observeLineEnding(srcTerm []byte) {
	if s.opts.LineEndings == nil {
		return
	}

	addLineEndingObservation(s.scan, srcTerm, s.want)

	if len(srcTerm) > 0 && !bytes.Equal(srcTerm, s.want) {
		s.res.EndingsChanged++
	}
}

func (s *transformStream) flushAtEOF() error {
	if s.lineBodyBytes == 0 {
		if s.bw == nil || !s.pending.nonEmpty() {
			return nil
		}
	}

	return s.completeLine(nil, true)
}

func (s *transformStream) dispatchByte(bt byte) error {
	switch bt {
	case '\n':
		return s.completeLine([]byte{'\n'}, false)
	case '\r':
		return s.handleCR()
	default:
		return s.emitContentByte(bt)
	}
}

func (s *transformStream) run() error {
	for {
		bt, err := s.br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return s.flushAtEOF()
			}

			return fmt.Errorf(errReadLinesFormat, err)
		}

		if err := s.dispatchByte(bt); err != nil {
			return err
		}
	}
}
