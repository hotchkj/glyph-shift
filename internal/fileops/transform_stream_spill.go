package fileops

import (
	"fmt"
	"io"
)

// WhitespaceSpillFile is scratch storage for bounded trim-trailing whitespace buffering.
type WhitespaceSpillFile interface {
	io.ReadWriteSeeker
	io.Closer
	ScratchName() string
}

// WhitespaceSpillBacking creates and removes spill scratch files for [runTransformStream].
type WhitespaceSpillBacking interface {
	CreateScratch(pattern string) (WhitespaceSpillFile, error)
	RemoveScratch(name string) error
}

type sessionWhitespaceSpillBacking struct {
	fs FileSession
}

// NewWhitespaceSpillBackingFromSession maps [FileSession] temp lifecycle to stream spill storage.
func NewWhitespaceSpillBackingFromSession(fs FileSession) WhitespaceSpillBacking {
	return sessionWhitespaceSpillBacking{fs: fs}
}

func (s sessionWhitespaceSpillBacking) CreateScratch(pattern string) (WhitespaceSpillFile, error) {
	if s.fs == nil {
		return nil, ErrNilFileSession
	}

	if pattern == "" {
		pattern = transformWhitespaceSpillPattern
	}

	handle, err := s.fs.CreateTemp("", pattern)
	if err != nil {
		return nil, fmt.Errorf("transform stream spill create: %w", err)
	}

	spill, convErr := whitespaceSpillFromSessionTemp(handle)
	if convErr != nil {
		_ = handle.Close()
		_ = s.fs.Remove(handle.Name())

		return nil, convErr
	}

	return spill, nil
}

func (s sessionWhitespaceSpillBacking) RemoveScratch(name string) error {
	if s.fs == nil {
		return ErrNilFileSession
	}

	if err := s.fs.Remove(name); err != nil {
		return fmt.Errorf("transform stream spill remove: %w", err)
	}

	return nil
}

func whitespaceSpillFromSessionTemp(handle SessionTempHandle) (WhitespaceSpillFile, error) {
	switch h := handle.(type) {
	case WhitespaceSpillFile:
		return h, nil
	default:
		return nil, fmt.Errorf("transform stream spill: %w (%T)", ErrUnsupportedWhitespaceSpillHandle, handle)
	}
}

// StreamWhitespaceSpillBackingProvider supplies in-memory spill storage for stream tests.
type StreamWhitespaceSpillBackingProvider interface {
	StreamWhitespaceSpillBacking() WhitespaceSpillBacking
}

// ResolveWhitespaceSpillBacking selects spill storage for [runTransformStream] write paths.
func ResolveWhitespaceSpillBacking(fs FileSession) WhitespaceSpillBacking {
	if provider, ok := fs.(StreamWhitespaceSpillBackingProvider); ok {
		return provider.StreamWhitespaceSpillBacking()
	}

	type backendExposer interface {
		Backend() SessionBackend
	}
	if be, ok := fs.(backendExposer); ok {
		if provider, ok2 := be.Backend().(StreamWhitespaceSpillBackingProvider); ok2 {
			return provider.StreamWhitespaceSpillBacking()
		}
	}

	return NewWhitespaceSpillBackingFromSession(fs)
}
