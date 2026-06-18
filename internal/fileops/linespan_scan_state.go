package fileops

import "fmt"

func (s *scannerState) run() error {
	for {
		if err := s.ctx.Err(); err != nil {
			return fmt.Errorf(errFmtScanLineSpansWrap, err)
		}

		ok, err := s.haveByteAbs(s.cursor)
		if err != nil {
			return err
		}
		if !ok {
			break
		}

		if err := s.dispatchByte(s.buf[s.relative(s.cursor)]); err != nil {
			return err
		}

		s.dropBufferedBeforeCursor()
	}

	return s.finalizeTrailing()
}

func (s *scannerState) dispatchByte(ch byte) error {
	switch ch {
	case '\n':
		return s.handleLF()
	case '\r':
		return s.handleCR()
	default:
		return s.handleDefaultByte()
	}
}

func (s *scannerState) handleLF() error {
	if err := s.emitTerminated(LineTerminatorLF, s.cursor); err != nil {
		return err
	}

	return s.noteConsumedSerialized(1)
}

func (s *scannerState) handleCR() error {
	nextAbs := s.cursor + 1
	has2nd, err := s.ensureSecondByteForCRLF(nextAbs)
	if err != nil {
		return err
	}
	if !has2nd {
		if err := s.emitTerminated(LineTerminatorCR, s.cursor); err != nil {
			return err
		}

		return s.noteConsumedSerialized(1)
	}

	if s.buf[s.relative(nextAbs)] == '\n' {
		if err := s.emitTerminated(LineTerminatorCRLF, s.cursor); err != nil {
			return err
		}

		return s.noteConsumedSerialized(2) //nolint:mnd // CRLF consumes two serialized bytes
	}

	if err := s.emitTerminated(LineTerminatorCR, s.cursor); err != nil {
		return err
	}

	return s.noteConsumedSerialized(1)
}

// ensureSecondByteForCRLF refills until nextAbs is readable or stream EOF confirms no lookahead byte.
func (s *scannerState) ensureSecondByteForCRLF(nextAbs uint64) (has2nd bool, err error) {
	has2nd, err = s.haveByteAbs(nextAbs)
	if err != nil {
		return false, err
	}
	if has2nd || s.streamEOF {
		return has2nd, nil
	}

	return s.awaitSecondByteForCRLF(nextAbs)
}

func (s *scannerState) awaitSecondByteForCRLF(nextAbs uint64) (bool, error) {
	has2nd := false
	for !has2nd && !s.streamEOF {
		if refillErr := s.refillTail(); refillErr != nil {
			return false, refillErr
		}
		var err error
		has2nd, err = s.haveByteAbs(nextAbs)
		if err != nil {
			return false, err
		}
	}

	return has2nd, nil
}

func (s *scannerState) handleDefaultByte() error {
	return s.noteConsumedSerialized(1)
}
