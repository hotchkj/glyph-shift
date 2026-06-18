// User vision: unit tests exercise the same session backend contract as production without host filesystem handles.
package testutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/spf13/afero"
)

const memTempHandlePrefix = "memtemp://"

var (
	errMemSeekInvalidWhence     = errors.New("memfs seek: invalid whence")
	errMemSeekNegativePos       = errors.New("memfs seek: negative position")
	errMemWriteAtNegativeOffset = errors.New("memfs writeat: negative offset")
)

// MemSessionBackend implements [fileops.SessionBackend] using only an in-memory afero filesystem.
//
// Not safe for concurrent use.
type MemSessionBackend struct {
	Fs afero.Fs

	mu     sync.Mutex
	temps  map[string][]byte // temp handle name -> staging bytes
	nextID int
}

var _ fileops.SessionBackend = (*MemSessionBackend)(nil)

// StreamWhitespaceSpillBacking implements [fileops.StreamWhitespaceSpillBackingProvider].
func (m *MemSessionBackend) StreamWhitespaceSpillBacking() fileops.WhitespaceSpillBacking {
	return fileops.NewMemWhitespaceSpillBacking()
}

// NewMemSessionBackend returns a MemSessionBackend with an empty in-memory filesystem.
func NewMemSessionBackend() *MemSessionBackend {
	return &MemSessionBackend{
		Fs:    afero.NewMemMapFs(),
		temps: make(map[string][]byte),
	}
}

type memFileInfo struct {
	name string
	size int64
	mode fs.FileMode
}

func (i memFileInfo) Name() string       { return i.name }
func (i memFileInfo) Size() int64        { return i.size }
func (i memFileInfo) Mode() fs.FileMode  { return i.mode }
func (i memFileInfo) ModTime() time.Time { return time.Time{} }
func (i memFileInfo) IsDir() bool        { return i.mode.IsDir() }
func (i memFileInfo) Sys() any           { return nil }

type memReadHandle struct {
	r      *bytes.Reader
	info   memFileInfo
	closed bool
}

func (h *memReadHandle) readClosedErr(op string) error {
	return fmt.Errorf("memfs %s %q: %w", op, h.info.name, fs.ErrClosed)
}

func (h *memReadHandle) Read(buf []byte) (int, error) {
	if h.closed {
		return 0, h.readClosedErr("read")
	}

	return h.r.Read(buf)
}

func (h *memReadHandle) Seek(off int64, whence int) (int64, error) {
	if h.closed {
		return 0, h.readClosedErr("seek")
	}

	return h.r.Seek(off, whence)
}

func (h *memReadHandle) Close() error {
	if h.closed {
		return nil
	}

	h.closed = true

	return nil
}

func (h *memReadHandle) Stat() (fs.FileInfo, error) {
	if h.closed {
		return nil, h.readClosedErr("stat")
	}

	return h.info, nil
}

func (h *memReadHandle) LockShared() error {
	if h.closed {
		return h.readClosedErr("lock shared")
	}

	return nil
}

func (h *memReadHandle) LockExclusive() error {
	if h.closed {
		return h.readClosedErr("lock exclusive")
	}

	return nil
}

func (h *memReadHandle) Unlock() error {
	if h.closed {
		return h.readClosedErr("unlock")
	}

	return nil
}

type memRDWRHandle struct {
	sess   *MemSessionBackend
	path   string
	data   []byte
	off    int64
	info   memFileInfo
	closed bool
	dirty  bool
}

func (h *memRDWRHandle) rdwrClosedErr(op string) error {
	return fmt.Errorf("memfs %s %q: %w", op, h.path, fs.ErrClosed)
}

func (h *memRDWRHandle) Read(buf []byte) (int, error) {
	if h.closed {
		return 0, h.rdwrClosedErr("read")
	}

	if h.off >= int64(len(h.data)) {
		return 0, io.EOF
	}

	n := copy(buf, h.data[h.off:])
	h.off += int64(n)

	return n, nil
}

func (h *memRDWRHandle) WriteAt(buf []byte, off int64) (int, error) {
	if h.closed {
		return 0, h.rdwrClosedErr("writeat")
	}

	if off < 0 {
		return 0, errMemWriteAtNegativeOffset
	}

	end := off + int64(len(buf))
	if end > int64(len(h.data)) {
		grown := make([]byte, end)
		copy(grown, h.data)
		h.data = grown
	}

	copy(h.data[off:], buf)
	h.dirty = true

	return len(buf), nil
}

func (h *memRDWRHandle) Write(buf []byte) (int, error) {
	if h.closed {
		return 0, h.rdwrClosedErr("write")
	}

	end := h.off + int64(len(buf))
	if end > int64(len(h.data)) {
		grown := make([]byte, end)
		copy(grown, h.data)
		h.data = grown
	}

	copy(h.data[h.off:], buf)
	h.off += int64(len(buf))
	h.dirty = true

	return len(buf), nil
}

func (h *memRDWRHandle) Seek(off int64, whence int) (int64, error) {
	if h.closed {
		return 0, h.rdwrClosedErr("seek")
	}

	var base int64

	switch whence {
	case io.SeekStart:
		base = 0
	case io.SeekCurrent:
		base = h.off
	case io.SeekEnd:
		base = int64(len(h.data))
	default:
		return 0, errMemSeekInvalidWhence
	}

	next := base + off
	if next < 0 {
		return 0, errMemSeekNegativePos
	}

	h.off = next

	return h.off, nil
}

func (h *memRDWRHandle) persist() error {
	if !h.dirty || h.sess == nil {
		return nil
	}

	return h.sess.writeLogicalPath(h.path, h.data, h.info.mode)
}

func (h *memRDWRHandle) Close() error {
	if h.closed {
		return nil
	}

	h.closed = true

	return h.persist()
}

func (h *memRDWRHandle) Sync() error {
	if h.closed {
		return h.rdwrClosedErr("sync")
	}

	return h.persist()
}

func (h *memRDWRHandle) Stat() (fs.FileInfo, error) {
	if h.closed {
		return nil, h.rdwrClosedErr("stat")
	}

	h.info.size = int64(len(h.data))

	return h.info, nil
}

func (h *memRDWRHandle) LockShared() error {
	if h.closed {
		return h.rdwrClosedErr("lock shared")
	}

	return nil
}

func (h *memRDWRHandle) LockExclusive() error {
	if h.closed {
		return h.rdwrClosedErr("lock exclusive")
	}

	return nil
}

func (h *memRDWRHandle) Unlock() error {
	if h.closed {
		return h.rdwrClosedErr("unlock")
	}

	return nil
}

type memTempHandle struct {
	sess   *MemSessionBackend
	name   string
	buf    bytes.Buffer
	closed bool
}

func (h *memTempHandle) Write(p []byte) (int, error) {
	if h.closed {
		return 0, fmt.Errorf("memtemp write %q: %w", h.name, fs.ErrClosed)
	}

	return h.buf.Write(p)
}

func (h *memTempHandle) Sync() error {
	if h.closed {
		return fmt.Errorf("memtemp sync %q: %w", h.name, fs.ErrClosed)
	}

	h.sess.commitTempBytes(h.name, h.buf.Bytes())

	return nil
}

func (h *memTempHandle) Close() error {
	if h.closed {
		return nil
	}

	h.closed = true
	h.sess.commitTempBytes(h.name, h.buf.Bytes())

	return nil
}

func (h *memTempHandle) Name() string {
	return h.name
}

func (m *MemSessionBackend) readLogicalBytes(path string) ([]byte, error) {
	return m.readLogicalPath(path)
}

func (m *MemSessionBackend) newReadHandle(path string, content []byte) *memReadHandle {
	return &memReadHandle{
		r: bytes.NewReader(content),
		info: memFileInfo{
			name: path,
			size: int64(len(content)),
			mode: defaultMemFilePerm,
		},
	}
}

// OpenRead opens a seekable in-memory reader for the logical path.
func (m *MemSessionBackend) OpenRead(path string) (fileops.SessionReadHandle, error) {
	content, err := m.readLogicalBytes(path)
	if err != nil {
		return nil, err
	}

	return m.newReadHandle(path, content), nil
}

func (m *MemSessionBackend) newRDWRHandle(path string, content []byte) *memRDWRHandle {
	return &memRDWRHandle{
		sess: m,
		path: path,
		data: append([]byte(nil), content...),
		info: memFileInfo{name: path, size: int64(len(content)), mode: defaultMemFilePerm},
	}
}

// OpenRDWR opens a mutable in-memory buffer seeded from the logical path.
func (m *MemSessionBackend) OpenRDWR(path string) (fileops.SessionRDWRHandle, error) {
	content, err := m.readLogicalBytes(path)
	if err != nil {
		return nil, err
	}

	return m.newRDWRHandle(path, content), nil
}

func (m *MemSessionBackend) allocTempName() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	return fmt.Sprintf("%s%d", memTempHandlePrefix, m.nextID)
}

// CreateTemp registers an in-memory staging buffer; dir is recorded only for diagnostics.
func (m *MemSessionBackend) CreateTemp(dir, pattern string) (fileops.SessionTempHandle, error) {
	_ = dir
	_ = pattern

	name := m.allocTempName()

	m.mu.Lock()
	m.temps[name] = nil
	m.mu.Unlock()

	return &memTempHandle{sess: m, name: name}, nil
}

// Remove deletes a logical path or drops a tracked temp handle.
func (m *MemSessionBackend) Remove(name string) error {
	m.mu.Lock()
	if _, isTemp := m.temps[name]; isTemp {
		delete(m.temps, name)
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	return m.Fs.Remove(name)
}

// Rename commits temp staging bytes to a logical path or copies within the mem filesystem.
func (m *MemSessionBackend) Rename(oldpath, newpath string) error {
	if oldpath == newpath || memfsWriteKey(oldpath) == memfsWriteKey(newpath) {
		return nil
	}

	m.mu.Lock()
	data, isTemp := m.temps[oldpath]
	if isTemp {
		delete(m.temps, oldpath)
		m.mu.Unlock()

		if writeErr := m.writeLogicalPath(newpath, data, defaultMemFilePerm); writeErr != nil {
			return fmt.Errorf("memfs rename write afero %q: %w", newpath, writeErr)
		}

		return nil
	}
	m.mu.Unlock()

	data, err := m.readLogicalPath(oldpath)
	if err != nil {
		return fmt.Errorf("memfs rename read %q: %w", oldpath, err)
	}

	if writeErr := m.writeLogicalPath(newpath, data, defaultMemFilePerm); writeErr != nil {
		return fmt.Errorf("memfs rename write %q: %w", newpath, writeErr)
	}

	for _, candidate := range memfsLookupPaths(oldpath) {
		_ = m.Fs.Remove(candidate)
	}

	return nil
}

// Chmod is a no-op for the in-memory filesystem.
func (m *MemSessionBackend) Chmod(_ string, _ fs.FileMode) error {
	return nil
}

func (m *MemSessionBackend) commitTempBytes(name string, data []byte) {
	m.mu.Lock()
	m.temps[name] = append([]byte(nil), data...)
	m.mu.Unlock()
}

// Files returns a snapshot of logical file contents in the mem filesystem.
func (m *MemSessionBackend) Files() map[string][]byte {
	result := make(map[string][]byte)

	_ = afero.Walk(m.Fs, "/", func(path string, info fs.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		data, readErr := afero.ReadFile(m.Fs, path)
		if readErr == nil {
			key := memWalkKey(path)
			result[key] = data
		}
		return nil
	})

	return result
}

func memWalkKey(path string) string {
	key := filepath.ToSlash(path)
	if key != "" && key != "." && !strings.HasPrefix(key, "/") {
		key = "/" + key
	}

	return key
}

// memfsLookupPaths returns path variants used to find logical files in afero after
// validate.Path applies filepath.Abs on Windows (drive-prefixed) while tests often seed
// POSIX-style absolute paths.
func memfsLookupPaths(path string) []string {
	slash := filepath.ToSlash(path)
	seen := make(map[string]struct{})
	var out []string

	add := func(candidate string) {
		if candidate == "" {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}

	add(path)
	add(slash)

	if vol := filepath.VolumeName(path); vol != "" {
		withoutVol := strings.TrimPrefix(slash, filepath.ToSlash(vol))
		if withoutVol != "" && withoutVol[0] != '/' {
			withoutVol = "/" + withoutVol
		}
		add(withoutVol)
		add(filepath.FromSlash(withoutVol))
	}

	return out
}

func (m *MemSessionBackend) readLogicalPath(path string) ([]byte, error) {
	for _, candidate := range memfsLookupPaths(path) {
		content, err := afero.ReadFile(m.Fs, candidate)
		if err == nil {
			return content, nil
		}
		if !errors.Is(err, afero.ErrFileNotFound) {
			return nil, fmt.Errorf("memfs read %q: %w", candidate, err)
		}
	}

	return nil, fmt.Errorf("memfs open %q: %w", path, fs.ErrNotExist)
}

func (m *MemSessionBackend) writeLogicalPath(path string, data []byte, perm fs.FileMode) error {
	key := memfsWriteKey(path)
	if err := afero.WriteFile(m.Fs, key, data, perm); err != nil {
		return fmt.Errorf("memfs write %q: %w", path, err)
	}

	return nil
}

func memfsWriteKey(path string) string {
	if filepath.VolumeName(path) != "" {
		return filepath.Clean(path)
	}

	slash := filepath.ToSlash(path)
	if slash != "" && slash != "." && !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}

	return slash
}
