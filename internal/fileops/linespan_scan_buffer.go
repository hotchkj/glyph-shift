package fileops

import (
	"errors"
	"fmt"
	"io"
)

func (s *scannerState) relative(abs uint64) int {
	if abs < s.base {
		panic("scan relative with abs before window base")
	}

	return int(abs - s.base) //nolint:gosec // G115: window slice indexing keeps delta within int range for buffers
}

func (s *scannerState) haveByteAbs(abs uint64) (bool, error) {
	for abs >= s.base+uint64(s.fill) { //nolint:gosec // G115: buffer fill is non-negative slice length
		if s.streamEOF {
			return false, nil
		}
		if err := s.refillTail(); err != nil {
			return false, err
		}
	}

	return abs < s.base+uint64(s.fill), nil //nolint:gosec // G115: buffer fill is non-negative slice length
}

func (s *scannerState) dropBufferedBeforeCursor() {
	if s.cursor <= s.base {
		return
	}

	delta := int(s.cursor - s.base) //nolint:gosec // G115: cursor and base track a sliding window within the buffer
	if delta > s.fill {
		panic("scannerState.dropBufferedBeforeCursor overreach")
	}

	copy(s.buf[:s.fill-delta], s.buf[delta:s.fill])
	s.fill -= delta
	s.base += uint64(delta) //nolint:gosec // G115: delta bounded by prior fill and copy semantics
}

func (s *scannerState) growBuf() {
	nsz := len(s.buf) * 2 //nolint:mnd // standard slice growth doubling
	if nsz < defaultLinespanChunkSize {
		nsz = defaultLinespanChunkSize
	}

	nb := make([]byte, nsz)
	copy(nb, s.buf[:s.fill])
	s.buf = nb
}

func (s *scannerState) refillTail() error {
	if s.streamEOF {
		return nil
	}

	free := s.ensureTailCapacity()
	n, err := s.src.Read(s.buf[s.fill : s.fill+free])
	s.fill += n

	return s.recordTailReadResult(n, err)
}

func (s *scannerState) ensureTailCapacity() int {
	free := len(s.buf) - s.fill
	if free <= 0 {
		s.dropBufferedBeforeCursor()
		free = len(s.buf) - s.fill
	}
	if free <= 0 {
		s.growBuf()
		free = len(s.buf) - s.fill
		if free <= 0 {
			panic("scan line spans: growBuf produced no tail capacity")
		}
	}

	return free
}

func (s *scannerState) recordTailReadResult(n int, err error) error {
	if n == 0 && err == nil {
		return errScanLineSpansReadZeroNoEOF
	}

	if errors.Is(err, io.EOF) {
		s.streamEOF = true
	}

	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("scan line spans read: %w", err)
	}

	return nil
}
