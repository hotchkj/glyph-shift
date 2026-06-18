package goldenreader

import "errors"

var (
	// ErrRuntimeCaller is returned when runtime.Caller cannot resolve a source file.
	ErrRuntimeCaller = errors.New("runtime.Caller failed")

	// ErrFixturePath is returned when a features-relative path escapes features/.
	ErrFixturePath = errors.New("committed fixture path must stay relative under features/")

	// ErrRepoRoot is returned when no directory containing go.mod can be found.
	ErrRepoRoot = errors.New("repository root (go.mod) not found")
)
