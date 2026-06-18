// OS temporary-file handle for atomic publish and whitespace spill storage.
package fileops

import "os"

// osTempHandle wraps a production temporary OS file used for atomic publish and commits.
type osTempHandle struct {
	f *os.File
}

func (h *osTempHandle) Read(p []byte) (int, error)  { return h.f.Read(p) }
func (h *osTempHandle) Write(p []byte) (int, error) { return h.f.Write(p) }
func (h *osTempHandle) Seek(off int64, whence int) (int64, error) {
	return h.f.Seek(off, whence)
}

func (h *osTempHandle) Sync() error {
	return h.f.Sync()
}

func (h *osTempHandle) Close() error {
	return h.f.Close()
}

func (h *osTempHandle) Name() string {
	return h.f.Name()
}

// ScratchName implements [WhitespaceSpillFile] for transform stream spill storage.
func (h *osTempHandle) ScratchName() string {
	return h.f.Name()
}

var (
	_ SessionTempHandle   = (*osTempHandle)(nil)
	_ WhitespaceSpillFile = (*osTempHandle)(nil)
)
