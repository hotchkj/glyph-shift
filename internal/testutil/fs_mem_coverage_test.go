package testutil

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/spf13/afero"
)

var (
	errPoisonStat = errors.New("poison stat")
	errPoisonRead = errors.New("poison read")
)

// poisonStatMemMapFs delegates to MemMapFs but returns errPoisonStat for a single logical path.
type poisonStatMemMapFs struct {
	*afero.MemMapFs
	poisonRel string
}

func newPoisonStatMemMapFs(t *testing.T, poisonPath string) *poisonStatMemMapFs {
	t.Helper()

	base := afero.NewMemMapFs()
	raw, ok := base.(*afero.MemMapFs)
	if !ok {
		t.Fatalf("NewMemMapFs concrete type = %T", base)
	}

	return &poisonStatMemMapFs{MemMapFs: raw, poisonRel: filepath.Clean(poisonPath)}
}

func (p *poisonStatMemMapFs) Stat(name string) (fs.FileInfo, error) {
	if filepath.Clean(name) == p.poisonRel {
		return nil, errPoisonStat
	}

	return p.MemMapFs.Stat(name)
}

// poisonReadMemMapFs delegates to MemMapFs but returns errPoisonRead when opening one logical path.
type poisonReadMemMapFs struct {
	*afero.MemMapFs
	poisonRel string
}

func newPoisonReadMemMapFs(t *testing.T, poisonPath string) *poisonReadMemMapFs {
	t.Helper()

	base := afero.NewMemMapFs()
	raw, ok := base.(*afero.MemMapFs)
	if !ok {
		t.Fatalf("NewMemMapFs concrete type = %T", base)
	}

	return &poisonReadMemMapFs{MemMapFs: raw, poisonRel: filepath.Clean(poisonPath)}
}

func (p *poisonReadMemMapFs) Open(name string) (afero.File, error) {
	if filepath.Clean(name) == p.poisonRel {
		return nil, errPoisonRead
	}

	return p.MemMapFs.Open(name)
}

func TestMemFileStater_StatNotExistWrapsFsErrNotExist(t *testing.T) {
	t.Parallel()

	st := NewMemFileStater()
	_, err := st.Stat(filepath.Join(string(filepath.Separator), "missing.txt"))
	if err == nil {
		t.Fatal("Stat: want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Stat: want fs.ErrNotExist, got %v", err)
	}
}

func TestMemFileStater_StatPropagatesNonMissingErrors(t *testing.T) {
	t.Parallel()

	poisonPath := filepath.Join(string(filepath.Separator), "workspace", "poison-stat.txt")
	mem := newPoisonStatMemMapFs(t, poisonPath)
	st := NewMemFileStaterWithFS(mem)

	_, err := st.Stat(poisonPath)
	if err == nil {
		t.Fatal("Stat: want error")
	}
	if !errors.Is(err, errPoisonStat) {
		t.Fatalf("Stat: want errPoisonStat, got %v", err)
	}
}

func TestMemPathResolver_LstatNotExistIsPathErrorWithNotExist(t *testing.T) {
	t.Parallel()

	r := NewMemPathResolverWithFS(afero.NewMemMapFs())
	path := filepath.Join(string(filepath.Separator), "absent", "p.txt")
	_, err := r.Lstat(path)
	if err == nil {
		t.Fatal("Lstat: want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Lstat: want fs.ErrNotExist, got %v", err)
	}
}

func TestMemPathResolver_LstatSuccess(t *testing.T) {
	t.Parallel()

	mem := afero.NewMemMapFs()
	path := filepath.Join(string(filepath.Separator), "ok", "f.txt")
	if werr := afero.WriteFile(mem, path, []byte("z"), 0o644); werr != nil {
		t.Fatalf("WriteFile: %v", werr)
	}

	resolver := NewMemPathResolverWithFS(mem)
	fi, err := resolver.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if fi.IsDir() {
		t.Fatal("Lstat: want file")
	}
	if fi.Size() != 1 {
		t.Fatalf("Size = %d want 1", fi.Size())
	}
}

func TestNoSymlinkPathResolver(t *testing.T) {
	t.Parallel()

	var noSym NoSymlinkPathResolver

	_, err := noSym.Lstat("/any")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Lstat: want fs.ErrNotExist, got %v", err)
	}

	got, err := noSym.EvalSymlinks("/p/q")
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if got != "/p/q" {
		t.Fatalf("EvalSymlinks = %q want /p/q", got)
	}
}

func TestMemSourceOpener_OpenMissingAllowedPath(t *testing.T) {
	t.Parallel()

	open := NewMemSourceOpenerWithFS(afero.NewMemMapFs())
	path := filepath.Join(string(filepath.Separator), "src", "x.txt")
	_, err := open.Open(path)
	if err == nil {
		t.Fatal("Open: want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open: want fs.ErrNotExist, got %v", err)
	}
}

func TestMemFileSession_OpenReadOpenRDWRNotExist(t *testing.T) {
	t.Parallel()

	sess := NewMemFileSession()
	missing := filepath.Join(string(filepath.Separator), "nope.txt")

	_, err := sess.OpenRead(missing)
	if err == nil {
		t.Fatal("OpenRead: want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("OpenRead: want fs.ErrNotExist, got %v", err)
	}

	_, err = sess.OpenRDWR(missing)
	if err == nil {
		t.Fatal("OpenRDWR: want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("OpenRDWR: want fs.ErrNotExist, got %v", err)
	}
}

func TestMemFileSession_RenameLogicalMissingSource(t *testing.T) {
	t.Parallel()

	sess := NewMemFileSession()
	oldPath := filepath.Join(string(filepath.Separator), "old.txt")
	newPath := filepath.Join(string(filepath.Separator), "new.txt")

	err := sess.Rename(oldPath, newPath)
	if err == nil {
		t.Fatal("Rename: want error")
	}
}

func TestMemFileSession_ChmodNoOp(t *testing.T) {
	t.Parallel()

	sess := NewMemFileSession()
	mode := fs.FileMode(0o644)
	path := filepath.Join(string(filepath.Separator), "any")

	if err := sess.Chmod(path, mode); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
}

func TestMemFileSession_FilesSnapshotSkipsDirs(t *testing.T) {
	t.Parallel()

	sess := NewMemFileSession()
	if werr := afero.WriteFile(sess.Fs, "/root/a.txt", []byte("aa"), 0o644); werr != nil {
		t.Fatalf("WriteFile a: %v", werr)
	}
	if werr := afero.WriteFile(sess.Fs, "/root/sub/b.txt", []byte("bb"), 0o644); werr != nil {
		t.Fatalf("WriteFile b: %v", werr)
	}

	got := sess.Files()
	if len(got) < 2 {
		t.Fatalf("Files: want at least 2 entries, got %d", len(got))
	}

	aKey := firstMapKeyAmong(got, "/root/a.txt", "root/a.txt")
	bKey := firstMapKeyAmong(got, "/root/sub/b.txt", "root/sub/b.txt")

	if aKey == "" || bKey == "" {
		t.Fatalf("Files keys = %#v, could not resolve expected paths", got)
	}
	if !bytesEqualString(got[aKey], "aa") {
		t.Fatalf("%s content mismatch", aKey)
	}
	if !bytesEqualString(got[bKey], "bb") {
		t.Fatalf("%s content mismatch", bKey)
	}
}

func firstMapKeyAmong(m map[string][]byte, candidates ...string) string {
	for _, key := range candidates {
		if _, ok := m[key]; ok {
			return key
		}
	}

	return ""
}

func bytesEqualString(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	for i := range b {
		if b[i] != s[i] {
			return false
		}
	}
	return true
}

func TestMemOutputOpener_OpenFileExclDuplicateErrors(t *testing.T) {
	t.Parallel()

	out := NewMemOutputOpener()
	path := filepath.Join(string(filepath.Separator), "out", "excl.txt")

	w1, openErr := out.OpenFile(path, OutputWriteExclusiveCreate, fs.FileMode(0o644))
	if openErr != nil {
		t.Fatalf("first OpenFile: %v", openErr)
	}
	if closeErr := w1.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	_, secondErr := out.OpenFile(path, OutputWriteExclusiveCreate, fs.FileMode(0o644))
	if secondErr == nil {
		t.Fatal("second exclusive OpenFile: want error")
	}
	if !errors.Is(secondErr, fs.ErrExist) {
		t.Fatalf("want fs.ErrExist, got %v", secondErr)
	}
}

func TestMemOutputOpener_TruncThenWriteReplacesContent(t *testing.T) {
	t.Parallel()

	out := NewMemOutputOpener()
	path := filepath.Join(string(filepath.Separator), "out", "trunc.txt")

	w0, truncSeedErr := out.OpenFile(path, OutputWriteExclusiveCreate, fs.FileMode(0o644))
	if truncSeedErr != nil {
		t.Fatalf("seed OpenFile: %v", truncSeedErr)
	}
	if _, seedWriteErr := w0.Write([]byte("old")); seedWriteErr != nil {
		t.Fatalf("Write: %v", seedWriteErr)
	}
	if closeSeedErr := w0.Close(); closeSeedErr != nil {
		t.Fatalf("Close: %v", closeSeedErr)
	}

	w1, truncOpenErr := out.OpenFile(path, OutputWriteTruncCreate, fs.FileMode(0o644))
	if truncOpenErr != nil {
		t.Fatalf("trunc OpenFile: %v", truncOpenErr)
	}
	if _, truncWriteErr := w1.Write([]byte("new")); truncWriteErr != nil {
		t.Fatalf("Write: %v", truncWriteErr)
	}
	if truncCloseErr := w1.Close(); truncCloseErr != nil {
		t.Fatalf("Close: %v", truncCloseErr)
	}

	got := out.FileContent(path)
	if !bytesEqualString(got, "new") {
		t.Fatalf("FileContent = %q want new", string(got))
	}
	if !out.FileExists(path) {
		t.Fatal("FileExists must be true after trunc write")
	}
}

func TestMemOutputOpener_FileContentNilWhenAbsent(t *testing.T) {
	t.Parallel()

	out := NewMemOutputOpener()
	if out.FileContent(filepath.Join(string(filepath.Separator), "missing")) != nil {
		t.Fatal("FileContent(absent): want nil slice")
	}
	if out.FileExists(filepath.Join(string(filepath.Separator), "missing")) {
		t.Fatal("FileExists(absent): want false")
	}
}

func TestMemOutputOpener_OpenFileAppendPrependsExisting(t *testing.T) {
	t.Parallel()

	out := NewMemOutputOpener()
	path := filepath.Join(string(filepath.Separator), "out", "ap.txt")

	w0, appendSeedErr := out.OpenFile(path, OutputWriteExclusiveCreate, fs.FileMode(0o644))
	if appendSeedErr != nil {
		t.Fatalf("seed OpenFile: %v", appendSeedErr)
	}
	if _, baseWriteErr := w0.Write([]byte("BASE")); baseWriteErr != nil {
		t.Fatalf("Write: %v", baseWriteErr)
	}
	if seedCloseErr := w0.Close(); seedCloseErr != nil {
		t.Fatalf("Close: %v", seedCloseErr)
	}

	w1, appendOpenErr := out.OpenFile(path, OutputWriteAppendCreate, fs.FileMode(0o644))
	if appendOpenErr != nil {
		t.Fatalf("append OpenFile: %v", appendOpenErr)
	}
	if _, tailWriteErr := w1.Write([]byte("TAIL")); tailWriteErr != nil {
		t.Fatalf("Write: %v", tailWriteErr)
	}
	if appendCloseErr := w1.Close(); appendCloseErr != nil {
		t.Fatalf("Close: %v", appendCloseErr)
	}

	got := out.FileContent(path)
	if !bytesEqualString(got, "BASETAIL") {
		t.Fatalf("FileContent = %q want BASETAIL", string(got))
	}
}

func TestMemOutputOpener_AppendPropagatesNonNotExistReadErrors(t *testing.T) {
	t.Parallel()

	poisonPath := filepath.Join(string(filepath.Separator), "workspace", "poison-read.txt")
	mem := newPoisonReadMemMapFs(t, poisonPath)
	if err := afero.WriteFile(mem, poisonPath, []byte("seed"), 0o644); err != nil {
		t.Fatalf("WriteFile seed: %v", err)
	}

	out := NewMemOutputOpenerWithFS(mem)
	_, err := out.OpenFile(poisonPath, pipeline.OutputAppend, 0o644)
	if err == nil {
		t.Fatal("OpenFile append: want error")
	}
	if !errors.Is(err, errPoisonRead) {
		t.Fatalf("OpenFile append: want errPoisonRead, got %v", err)
	}
}
