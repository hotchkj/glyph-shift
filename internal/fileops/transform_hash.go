package fileops

import (
	"crypto/sha256"
	"hash"
	"io"
)

// sha256Reader hashes every byte read from r while passing reads through.
type sha256Reader struct {
	r io.Reader
	h hash.Hash
	n int64
}

func newSHA256Reader(r io.Reader) *sha256Reader {
	return &sha256Reader{r: r, h: sha256.New()}
}

func (s *sha256Reader) Read(p []byte) (int, error) {
	readCount, err := s.r.Read(p)
	if readCount > 0 {
		_, _ = s.h.Write(p[:readCount])
		s.n += int64(readCount)
	}

	return readCount, err
}

func (s *sha256Reader) Digest() [sha256.Size]byte {
	var out [sha256.Size]byte
	copy(out[:], s.h.Sum(nil))

	return out
}

// sha256CountWriter hashes and counts everything written to it.
type sha256CountWriter struct {
	h hash.Hash
	n int64
}

func newSHA256CountWriter() *sha256CountWriter {
	return &sha256CountWriter{h: sha256.New()}
}

func (w *sha256CountWriter) Write(p []byte) (int, error) {
	_, _ = w.h.Write(p)
	w.n += int64(len(p))

	return len(p), nil
}

func (w *sha256CountWriter) Digest() [sha256.Size]byte {
	var out [sha256.Size]byte
	copy(out[:], w.h.Sum(nil))

	return out
}
