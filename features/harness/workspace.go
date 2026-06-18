// Package harness provides BDD test workspace I/O and path helpers so feature steps
// avoid importing os directly while keeping behavior byte-faithful.
package harness

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/hotchkj/glyph-shift/internal/fsnorm"
	"github.com/spf13/afero"
)

const parentDirPerm = 0o750

// Workspace is an isolated in-memory BDD working directory.
type Workspace struct {
	root string
	fs   afero.Fs
}

var workspaceSeq atomic.Uint64

// NewWorkspace creates a new isolated in-memory workspace.
func NewWorkspace() (*Workspace, error) {
	root, err := filepath.Abs(filepath.Join(".glyph-shift-bdd-mem", fmt.Sprintf("%d", workspaceSeq.Add(1))))
	if err != nil {
		return nil, fmt.Errorf("resolve in-memory workspace root: %w", err)
	}
	root = fsnorm.DirNative(root)

	memFS := afero.NewMemMapFs()
	if err := memFS.MkdirAll(root, parentDirPerm); err != nil {
		return nil, fmt.Errorf("mkdir in-memory workspace: %w", err)
	}

	return &Workspace{root: root, fs: memFS}, nil
}

// Root returns the absolute, OS-native workspace root.
func (w *Workspace) Root() string {
	return w.root
}

// FS returns the in-memory filesystem backing the BDD workspace.
func (w *Workspace) FS() afero.Fs {
	return w.fs
}

// Join maps a slash-separated logical path under the workspace root.
func (w *Workspace) Join(rel string) string {
	c := fsnorm.Canonical(rel)
	if c == "" {
		return w.root
	}

	return filepath.Join(w.root, filepath.FromSlash(c))
}

// Close releases the in-memory workspace.
func (w *Workspace) Close() error {
	if w.root == "" {
		return nil
	}

	w.root = ""
	w.fs = nil

	return nil
}

// ReadFile reads the file at rel under the workspace.
func (w *Workspace) ReadFile(rel string) ([]byte, error) {
	return afero.ReadFile(w.fs, w.Join(rel))
}

// WriteFile writes data to rel under the workspace, creating parent directories.
func (w *Workspace) WriteFile(rel string, data []byte, perm fs.FileMode) error {
	path := w.Join(rel)
	if err := w.fs.MkdirAll(filepath.Dir(path), parentDirPerm); err != nil {
		return fmt.Errorf("mkdir parents: %w", err)
	}

	return afero.WriteFile(w.fs, path, data, perm)
}

// MkdirAll creates a directory (and parents) at rel under the workspace.
func (w *Workspace) MkdirAll(rel string, perm fs.FileMode) error {
	return w.fs.MkdirAll(w.Join(rel), perm)
}

// RemoveAll removes the path at rel under the workspace.
func (w *Workspace) RemoveAll(rel string) error {
	return w.fs.RemoveAll(w.Join(rel))
}

// Stat returns file info for rel under the workspace.
func (w *Workspace) Stat(rel string) (fs.FileInfo, error) {
	return w.fs.Stat(w.Join(rel))
}

// IsNotExist wraps os.IsNotExist for callers that must not import os.
func IsNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err)
}

// DirEntry is a minimal directory entry for BDD assertions.
type DirEntry struct {
	Name  string
	IsDir bool
}

// ReadDir lists entries in rel under the workspace.
func (w *Workspace) ReadDir(rel string) ([]DirEntry, error) {
	entries, err := afero.ReadDir(w.fs, w.Join(rel))
	if err != nil {
		return nil, err
	}

	out := make([]DirEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, DirEntry{Name: entry.Name(), IsDir: entry.IsDir()})
	}

	return out, nil
}
