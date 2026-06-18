package fileops

import (
	"fmt"
	"io"
)

const (
	transformStreamWriterBufBytes = 64 << 10
	defaultPendingWSMaxBuf        = 64 << 10
	pendingWSSpillHalfDivisor     = 2
)

// pendingWhitespace holds a suffix run of space/tab bytes that might be trailing until a line
// boundary or a non-whitespace byte disambiguates. Memory stays bounded by spilling the oldest
// prefix to scratch storage while preserving exact FIFO byte order on flush.
type pendingWhitespace struct {
	buf []byte

	spill       WhitespaceSpillFile
	spillPath   string
	spillBytes  int64
	maxBufBytes int
	backing     WhitespaceSpillBacking
}

func (p *pendingWhitespace) nonEmpty() bool {
	return p.spillBytes > 0 || len(p.buf) > 0
}

func (p *pendingWhitespace) discard() {
	p.buf = p.buf[:0]
	p.closeSpill()
}

func (p *pendingWhitespace) closeSpill() {
	if p.spill != nil {
		_ = p.spill.Close()
		p.spill = nil
	}

	if p.spillPath != "" && p.backing != nil {
		_ = p.backing.RemoveScratch(p.spillPath)
		p.spillPath = ""
	}

	p.spillBytes = 0
}

func (p *pendingWhitespace) ensureOpenSpill() error {
	if p.spill != nil {
		return nil
	}

	if p.backing == nil {
		return fmt.Errorf("transform stream pending whitespace spill: %w", ErrNilWhitespaceSpillBacking)
	}

	spillFile, err := p.backing.CreateScratch(transformWhitespaceSpillPattern)
	if err != nil {
		return err
	}

	p.spill = spillFile
	p.spillPath = spillFile.ScratchName()

	return nil
}

func (p *pendingWhitespace) spillOlderHalf() error {
	if len(p.buf) == 0 {
		return nil
	}

	bytesToSpill := len(p.buf) / pendingWSSpillHalfDivisor
	if bytesToSpill == 0 {
		bytesToSpill = 1
	}

	if err := p.ensureOpenSpill(); err != nil {
		return err
	}

	if _, err := p.spill.Write(p.buf[:bytesToSpill]); err != nil {
		return fmt.Errorf("transform stream pending whitespace spill write: %w", err)
	}

	p.spillBytes += int64(bytesToSpill)
	copy(p.buf, p.buf[bytesToSpill:])
	p.buf = p.buf[:len(p.buf)-bytesToSpill]

	return nil
}

func (p *pendingWhitespace) push(ch byte) error {
	if p.maxBufBytes <= 0 {
		p.maxBufBytes = defaultPendingWSMaxBuf
	}

	for len(p.buf) >= p.maxBufBytes {
		if err := p.spillOlderHalf(); err != nil {
			return err
		}
	}

	p.buf = append(p.buf, ch)

	return nil
}

func (p *pendingWhitespace) flushTo(dst io.Writer) error {
	if p.spill != nil {
		if _, err := p.spill.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("transform stream pending whitespace spill seek: %w", err)
		}

		if _, err := io.Copy(dst, p.spill); err != nil {
			return fmt.Errorf("transform stream pending whitespace spill copy: %w", err)
		}

		p.closeSpill()
	}

	if len(p.buf) > 0 {
		if err := WriteFull(dst, p.buf); err != nil {
			return err
		}

		p.buf = p.buf[:0]
	}

	return nil
}
