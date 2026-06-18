package testutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

const (
	// defaultMemDirPerm is the permission used when creating directories in
	// the in-memory filesystem. Tests do not enforce real permissions.
	defaultMemDirPerm = 0o750
	// defaultMemFilePerm is the permission used when writing files into the
	// in-memory filesystem. Tests do not enforce real permissions, so a
	// conventional value is sufficient.
	defaultMemFilePerm = 0o644
)

// MemFileStater implements the same method signatures as pipeline.FileStater
// using an in-memory afero filesystem. Returns fs.FileInfo from the afero Fs.
//
// Intentionally does NOT import the pipeline package. Go structural typing
// satisfies pipeline.FileStater.
type MemFileStater struct {
	// Fs is the backing in-memory filesystem.
	Fs afero.Fs
}

// NewMemFileStater returns a MemFileStater with an empty in-memory filesystem.
func NewMemFileStater() *MemFileStater {
	return &MemFileStater{Fs: afero.NewMemMapFs()}
}

// NewMemFileStaterWithFS returns a MemFileStater backed by fs.
func NewMemFileStaterWithFS(memFS afero.Fs) *MemFileStater {
	return &MemFileStater{Fs: memFS}
}

// Stat returns FileInfo for the named path from the in-memory filesystem.
// Not-exist errors are wrapped so errors.Is(..., fs.ErrNotExist) succeeds.
func (m *MemFileStater) Stat(path string) (fs.FileInfo, error) {
	info, err := m.Fs.Stat(path)
	if err != nil {
		if errors.Is(err, afero.ErrFileNotFound) {
			return nil, fmt.Errorf("stat %q: %w", path, fs.ErrNotExist)
		}

		return nil, fmt.Errorf("memfs stat %q: %w", path, err)
	}

	return info, nil
}

// MemPathResolver implements validate.PathResolver over an in-memory afero filesystem.
type MemPathResolver struct {
	Fs afero.Fs
}

// NewMemPathResolverWithFS returns a MemPathResolver backed by fs.
func NewMemPathResolverWithFS(memFS afero.Fs) *MemPathResolver {
	return &MemPathResolver{Fs: memFS}
}

func (m *MemPathResolver) Lstat(path string) (fs.FileInfo, error) {
	for _, candidate := range memfsLookupPaths(path) {
		info, err := m.Fs.Stat(candidate)
		if err == nil {
			return info, nil
		}
		if !errors.Is(err, afero.ErrFileNotFound) {
			return nil, fmt.Errorf("memfs lstat %q: %w", candidate, err)
		}
	}

	return nil, fmt.Errorf("memfs lstat %q: %w", path, fs.ErrNotExist)
}

func (m *MemPathResolver) EvalSymlinks(path string) (string, error) {
	return path, nil
}

// NoSymlinkPathResolver treats every path as absent for symlink checks while
// preserving the input path when callers explicitly request symlink resolution.
type NoSymlinkPathResolver struct{}

func (NoSymlinkPathResolver) Lstat(string) (fs.FileInfo, error) {
	return nil, fs.ErrNotExist
}

func (NoSymlinkPathResolver) EvalSymlinks(path string) (string, error) {
	return path, nil
}

// MemSourceOpener implements the same method signatures as pipeline.SourceOpener
// using an in-memory afero filesystem. Open returns an in-memory seekable reader.
//
// MemSourceOpener intentionally does NOT import the pipeline package to avoid
// import cycles. Go structural typing satisfies pipeline.SourceOpener.
type MemSourceOpener struct {
	// Fs is the backing in-memory filesystem. Pre-populate before tests.
	Fs afero.Fs
}

// NewMemSourceOpener returns a MemSourceOpener with an empty in-memory filesystem.
func NewMemSourceOpener() *MemSourceOpener {
	return &MemSourceOpener{Fs: afero.NewMemMapFs()}
}

// NewMemSourceOpenerWithFS returns a MemSourceOpener backed by fs.
func NewMemSourceOpenerWithFS(memFS afero.Fs) *MemSourceOpener {
	return &MemSourceOpener{Fs: memFS}
}

type memReadSeekCloser struct {
	*bytes.Reader
}

func (memReadSeekCloser) Close() error {
	return nil
}

// Open reads the named logical path from the in-memory filesystem and returns
// an in-memory seekable reader.
func (m *MemSourceOpener) Open(path string) (io.ReadSeekCloser, error) {
	content, err := aferoReadFileLookup(m.Fs, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("open %q: %w", path, fs.ErrNotExist)
		}

		return nil, fmt.Errorf("memsrc open %q: %w", path, err)
	}

	return memReadSeekCloser{Reader: bytes.NewReader(content)}, nil
}

// MemOutputOpener implements pipeline.OutputBackend and pipeline.OutputOpener
// using an in-memory afero filesystem. OpenFile buffers writes and commits them
// into the memory filesystem on Close.
//
// Not safe for concurrent use.
type MemOutputOpener struct {
	// Fs is the backing in-memory filesystem. Tests may inspect it after calls.
	Fs afero.Fs
}

var (
	_ pipeline.OutputBackend   = (*MemOutputOpener)(nil)
	_ pipeline.StatBackend     = (*MemFileStater)(nil)
	_ validate.ResolverBackend = (*MemPathResolver)(nil)
	_ validate.ResolverBackend = NoSymlinkPathResolver{}
)

// NewMemOutputOpener returns a MemOutputOpener with an empty in-memory filesystem.
func NewMemOutputOpener() *MemOutputOpener {
	return &MemOutputOpener{
		Fs: afero.NewMemMapFs(),
	}
}

// NewMemOutputOpenerWithFS returns a MemOutputOpener backed by fs.
func NewMemOutputOpenerWithFS(memFS afero.Fs) *MemOutputOpener {
	return &MemOutputOpener{
		Fs: memFS,
	}
}

// NewMemPublishSession returns a MemTestSession backed by the supplied memory filesystem.
// Pipeline extract/split/blocks apply paths publish through publishFS (atomic destination checks
// and final writes); mem-backed tests must align publishFS with the output opener when seeding
// existing destinations or asserting written outputs.
func NewMemPublishSession(memFS afero.Fs) *MemTestSession {
	s := NewMemFileSession()
	s.SetFs(memFS)

	return s
}

// NewMemPublishSessionForOutput returns a MemTestSession whose backing Fs matches out.Fs.
func NewMemPublishSessionForOutput(out *MemOutputOpener) *MemTestSession {
	return NewMemPublishSession(out.Fs)
}

func memOutputReadAppendSeed(memFs afero.Fs, path string, intent pipeline.OutputWriteIntent) ([]byte, error) {
	if intent != pipeline.OutputAppend {
		return nil, nil
	}

	seed, err := aferoReadFileLookup(memFs, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	if len(seed) == 0 {
		return nil, nil
	}

	return seed, nil
}

func memOutputWriteAppendSeed(writer io.Writer, intent pipeline.OutputWriteIntent, seed []byte) error {
	if intent != pipeline.OutputAppend || len(seed) == 0 {
		return nil
	}

	if _, err := writer.Write(seed); err != nil {
		return fmt.Errorf("memout seed append content: %w", err)
	}

	return nil
}

// MkdirAll creates the directory path and all parents in the in-memory filesystem.
func (m *MemOutputOpener) MkdirAll(path string, perm fs.FileMode) error {
	return m.Fs.MkdirAll(path, perm)
}

type memOutputWriteCloser struct {
	fs   afero.Fs
	path string
	perm fs.FileMode
	buf  bytes.Buffer
}

func (w *memOutputWriteCloser) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

func (w *memOutputWriteCloser) Close() error {
	if err := w.fs.MkdirAll(filepath.Dir(w.path), defaultMemDirPerm); err != nil {
		return fmt.Errorf("memout close mkdir %q: %w", filepath.Dir(w.path), err)
	}

	if err := aferoWriteFileKey(w.fs, w.path, w.buf.Bytes(), w.perm); err != nil {
		return fmt.Errorf("memout close write %q: %w", w.path, err)
	}

	return nil
}

// OpenFile opens an in-memory writer for the given logical destination path.
func (m *MemOutputOpener) OpenFile(
	path string,
	intent pipeline.OutputWriteIntent,
	perm fs.FileMode,
) (io.WriteCloser, error) {
	if intent == pipeline.OutputCreateExclusive {
		if aferoFileExistsLookup(m.Fs, path) {
			return nil, fmt.Errorf("open %q: %w", path, fs.ErrExist)
		}
	}

	if intent == pipeline.OutputCreateOrReplace {
		aferoRemoveLookup(m.Fs, path)
	}

	appendSeed, seedErr := memOutputReadAppendSeed(m.Fs, path, intent)
	if seedErr != nil {
		return nil, seedErr
	}

	writer := &memOutputWriteCloser{fs: m.Fs, path: path, perm: perm}

	if seedErr := memOutputWriteAppendSeed(writer, intent, appendSeed); seedErr != nil {
		return nil, seedErr
	}

	return writer, nil
}

// FileExists reports whether the named logical path exists in the in-memory filesystem.
func (m *MemOutputOpener) FileExists(path string) bool {
	return aferoFileExistsLookup(m.Fs, path)
}

// FileContent returns the content written for the named logical path. Returns nil
// if the path does not exist.
func (m *MemOutputOpener) FileContent(path string) []byte {
	data, err := aferoReadFileLookup(m.Fs, path)
	if err != nil {
		return nil
	}

	return data
}

func aferoReadFileLookup(memFs afero.Fs, path string) ([]byte, error) {
	for _, candidate := range memfsLookupPaths(path) {
		data, err := afero.ReadFile(memFs, candidate)
		if err == nil {
			return data, nil
		}
		if !errors.Is(err, afero.ErrFileNotFound) {
			return nil, fmt.Errorf("memfs read %q: %w", candidate, err)
		}
	}

	return nil, fs.ErrNotExist
}

func aferoWriteFileKey(memFs afero.Fs, path string, data []byte, perm fs.FileMode) error {
	return afero.WriteFile(memFs, memfsWriteKey(path), data, perm)
}

func aferoFileExistsLookup(memFs afero.Fs, path string) bool {
	for _, candidate := range memfsLookupPaths(path) {
		exists, err := afero.Exists(memFs, candidate)
		if err == nil && exists {
			return true
		}
	}

	return false
}

func aferoRemoveLookup(memFs afero.Fs, path string) {
	for _, candidate := range memfsLookupPaths(path) {
		_ = memFs.Remove(candidate)
	}
}
