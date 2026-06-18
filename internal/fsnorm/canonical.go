// Package fsnorm provides lexical path-string normalization at trust boundaries
// (CLI flags, JSON tool paths, cwd strings). Canonical does not open files or
// enforce root containment.
package fsnorm

import (
	"path/filepath"
	"strings"
)

// Canonical returns one forward-slash, filepath.Clean lexical form for comparing or
// matching path-like strings from any GOOS. Backslashes are mapped to '/' first;
// filepath.Clean then filepath.ToSlash so Windows does not reintroduce '\' via Clean.
// Applying Canonical again is a no-op on its output (idempotent); callers may
// canonicalize at both a boundary and an inner layer without changing meaning.
func Canonical(p string) string {
	if p == "" {
		return ""
	}
	slashed := strings.ReplaceAll(p, `\`, `/`)

	return filepath.ToSlash(filepath.Clean(slashed))
}

// DirNative returns a filepath.Clean OS-native directory string. It applies
// Canonical first so mixed separators normalize consistently, then
// filepath.FromSlash so Windows receives backslashes where expected.
// Empty input becomes ".".
func DirNative(dir string) string {
	c := Canonical(dir)
	if c == "" {
		c = "."
	}

	return filepath.Clean(filepath.FromSlash(c))
}

// ResolveUnderWorkspace resolves relOrAbs against root when relOrAbs is not an absolute
// path for the current OS. relOrAbs is typically the output of Canonical; root should be
// a DirNative workspace directory (the CLI working directory / workspace root).
func ResolveUnderWorkspace(relOrAbs, root string) string {
	root = DirNative(root)
	if relOrAbs == "" {
		return root
	}

	p := filepath.FromSlash(relOrAbs)
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}

	return filepath.Clean(filepath.Join(root, p))
}
