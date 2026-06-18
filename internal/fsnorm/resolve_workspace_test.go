package fsnorm

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveUnderWorkspace_JoinsRelativeToRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Join("alpha", "beta")
	rel := Canonical("dir/file.txt")
	got := ResolveUnderWorkspace(rel, root)
	want := filepath.Join(root, "dir", "file.txt")
	if got != want {
		t.Fatalf("ResolveUnderWorkspace = %q, want %q", got, want)
	}
}

func TestResolveUnderWorkspace_EmptyRelReturnsRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Join("r", "s")
	got := ResolveUnderWorkspace("", root)
	want := DirNative(root)
	if got != want {
		t.Fatalf("ResolveUnderWorkspace(\"\") = %q, want %q", got, want)
	}
}

func TestResolveUnderWorkspace_AbsoluteUnchanged(t *testing.T) {
	t.Parallel()

	var abs string
	if runtime.GOOS == "windows" {
		abs = `C:\vol\project\file.go`
	} else {
		abs = "/vol/project/file.go"
	}

	got := ResolveUnderWorkspace(Canonical(abs), filepath.Join("ignored", "root"))
	want := filepath.Clean(filepath.FromSlash(Canonical(abs)))
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("ResolveUnderWorkspace = %q, want %q", got, want)
	}
}
