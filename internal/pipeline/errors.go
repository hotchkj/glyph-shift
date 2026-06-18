package pipeline

import (
	"errors"
	"fmt"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

// Sentinel errors for nil seam dependencies.
var (
	ErrNilSourceOpener = errors.New("pipeline: nil SourceOpener")
	ErrNilOutputOpener = errors.New("pipeline: nil OutputOpener")
	ErrNilFileStater   = errors.New("pipeline: nil FileStater")
)

// ErrSourceNotFound is returned when the source file does not exist.
var ErrSourceNotFound = errors.New("source file not found")

// ErrBinarySource is returned when the source file is detected as binary.
var ErrBinarySource = errors.New("binary source file")

// ErrDestinationExists is returned when the destination already exists
// and neither force nor append was requested.
var ErrDestinationExists = errors.New("destination already exists")

// DestinationExistsError carries the destination path that caused ErrDestinationExists.
type DestinationExistsError struct {
	Path string
}

func (e *DestinationExistsError) Error() string {
	if e.Path == "" {
		return ErrDestinationExists.Error()
	}

	return fmt.Sprintf("%s: %s", ErrDestinationExists, e.Path)
}

func (e *DestinationExistsError) Unwrap() error {
	return ErrDestinationExists
}

func newDestinationExistsError(path string) error {
	return &DestinationExistsError{Path: path}
}

// DestinationPathFromError returns the destination path carried by ErrDestinationExists.
func DestinationPathFromError(err error) (string, bool) {
	var destErr *DestinationExistsError
	if errors.As(err, &destErr) && destErr.Path != "" {
		return destErr.Path, true
	}

	return "", false
}

// ErrNotRegularFile is returned when the source is not a regular file
// (for example a device node or socket).
var ErrNotRegularFile = errors.New("source is not a regular file")

// ErrDirectoryNotFile is returned when the source path names a directory.
var ErrDirectoryNotFile = errors.New("source path is a directory, not a file")

// ErrMaxFilesExceeded is returned when split/blocks would exceed --max-files.
// It references [fileops.ErrMaxFilesExceeded] so scan and pipeline share one sentinel value.
var ErrMaxFilesExceeded = fileops.ErrMaxFilesExceeded

// ErrPathContainsNUL is returned when a path argument contains an embedded NUL byte.
// It aliases [fileops.ErrPathContainsNUL] so callers can use errors.Is against either sentinel.
var ErrPathContainsNUL = fileops.ErrPathContainsNUL

// ErrNamesCountMismatch is returned when --names count does not match output count.
var ErrNamesCountMismatch = errors.New("explicit name count does not match output count")

// PathRole identifies which JSON path vocabulary key should receive a single non-ambiguous path slot.
type PathRole uint8

const (
	PathRoleNone PathRole = iota
	PathRoleSrc
	PathRoleDest
	PathRoleOutDir
	PathRoleOutputPath
)

// PathContext is a typed (role, path) pair for classification; path strings are carried as produced by
// the pipeline (workspace-relative or absolute); absolute native form is applied only at the JSON edge.
type PathContext struct {
	Role PathRole
	Path string
}

// PathContextError wraps an error with explicit path role context for internal_error and path-scoped variants.
type PathContextError struct {
	Context PathContext
	Err     error
}

func (e *PathContextError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}

	return "path context error"
}

func (e *PathContextError) Unwrap() error {
	return e.Err
}

// pathContextError attaches a single path role for error classification and JSON path slots.
// The inner error is preserved for errors.Is/errors.As. path may be relative or absolute as produced by the pipeline.
func pathContextError(role PathRole, path string, err error) error {
	if err == nil {
		return nil
	}

	return &PathContextError{
		Context: PathContext{Role: role, Path: path},
		Err:     err,
	}
}

// WithPathRole wraps err with explicit JSON path-slot role for classification (CLI stderr and MCP payloads).
// The embedded error stays on the unwrap chain so errors.Is and errors.As keep working unchanged.
func WithPathRole(role PathRole, path string, err error) error {
	return pathContextError(role, path, err)
}

// NamesCountMismatchError carries names_count and output_count for the names_count_mismatch JSON variant.
type NamesCountMismatchError struct {
	NamesCount  int
	OutputCount int
}

func (e *NamesCountMismatchError) Error() string {
	return fmt.Sprintf("%v: got %d names for %d outputs", ErrNamesCountMismatch, e.NamesCount, e.OutputCount)
}

func (e *NamesCountMismatchError) Unwrap() error {
	return ErrNamesCountMismatch
}

// PatternFieldError carries logical input field vocabulary for pattern validation failures while
// preserving validate sentinel chains.
type PatternFieldError struct {
	Field string
	Cause error
}

func (e *PatternFieldError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}

	return "pattern field error"
}

func (e *PatternFieldError) Unwrap() error {
	return e.Cause
}

// ErrEmptyNamesListEntry is returned when --names contains an empty comma-separated entry.
var ErrEmptyNamesListEntry = errors.New("empty entry in --names list")

// ErrInvalidExplicitName is returned when a --names fragment is not a safe single basename.
var ErrInvalidExplicitName = errors.New("invalid explicit output name")

// ErrDuplicateExplicitNames is returned when two explicit names resolve to the same basename.
var ErrDuplicateExplicitNames = errors.New("duplicate explicit output names")

// ErrInvalidLineEndings is returned when a transform line-ending target is invalid.
var ErrInvalidLineEndings = errors.New("invalid line-endings value")

// ErrNoTransformSpecified is returned when a transform request has no operation.
var ErrNoTransformSpecified = errors.New("no transform specified")

// ErrMaxFilesAtLeastOne is returned when max-files is less than one.
var ErrMaxFilesAtLeastOne = errors.New("max_files must be at least 1")

// ErrEmptyPreparedPath is returned by PreparePath when rawPath is empty.
var ErrEmptyPreparedPath = errors.New("path must not be empty")

// ErrEmptyRegexpPattern is returned when a split delimiter or blocks start/end
// pattern is an explicitly empty string (before regexp compilation).
var ErrEmptyRegexpPattern = validate.ErrEmptyRegexpPattern

// Internal split/blocks byte-span write path (not contractual for errors.Is unless documented).
var (
	errBlocksWriteInvalidByteSpan                = errors.New("blocks write: invalid byte span (end before start)")
	errBlocksWriteByteSpanStartExceedsMaxSeek    = errors.New("blocks write: byte span start exceeds maximum seek offset")
	errBlocksWriteByteSpanLengthExceedsMaxCopy   = errors.New("blocks write: byte span length exceeds maximum copy size")
	errBlocksWriteInternalMetaBlockCountMismatch = errors.New("blocks write: internal meta/block count mismatch")

	errSplitWriteInvalidByteSpan                  = errors.New("split write: invalid byte span (end before start)")
	errSplitWriteByteSpanStartExceedsMaxSeek      = errors.New("split write: byte span start exceeds maximum seek offset")
	errSplitWriteByteSpanLengthExceedsMaxCopy     = errors.New("split write: byte span length exceeds maximum copy size")
	errSplitWriteInternalMetaSectionCountMismatch = errors.New("split write: internal meta/section count mismatch")
)

// ErrTransformSkippedUnknown is returned when transform skips without a known reason.
var ErrTransformSkippedUnknown = errors.New("transform skipped for unknown reason")
