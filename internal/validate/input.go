package validate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hotchkj/glyph-shift/internal/fsnorm"
)

func pathNotExist(err error) bool {
	return os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist)
}

// Sentinel errors for validation failures.
var (
	ErrPathTraversal      = errors.New("path traversal")
	ErrOutsideRoot        = errors.New("outside allowed root")
	ErrReservedName       = errors.New("reserved device name")
	ErrControlChar        = errors.New("control character")
	ErrInvalidExtension   = errors.New("invalid file extension")
	ErrInvalidPattern     = errors.New("invalid regex pattern")
	ErrPatternTooLong     = errors.New("regex pattern too long")
	ErrEmptyRegexpPattern = errors.New("regexp pattern must not be empty")
)

// ValidatePath resolves path to absolute, verifies it is under root after canonicalization.
// Rejects ".." traversal that escapes root. Rejects symlinks resolving outside root.
func ValidatePath(path, root string, resolver PathResolver) error {
	if resolver == nil {
		return ErrNilPathResolver
	}

	path = fsnorm.Canonical(path)
	root = fsnorm.Canonical(root)

	absRoot, err := absoluteCleanPath(root, "root")
	if err != nil {
		return err
	}

	absPath, err := absoluteCleanPath(path, "path")
	if err != nil {
		return err
	}

	if pathErr := validateNativePathInput(absPath); pathErr != nil {
		return pathErr
	}

	effectiveRoot, err := resolveExistingPath(absRoot, resolver, "root")
	if err != nil {
		return err
	}

	candidate, err := resolveCandidatePath(absPath, absRoot, effectiveRoot, resolver)
	if err != nil {
		return err
	}

	return ensureUnderRoot(candidate, effectiveRoot)
}

func absoluteCleanPath(path, role string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("validate path: resolve %s: %w", role, err)
	}

	return filepath.Clean(absPath), nil
}

func validateNativePathInput(absPath string) error {
	if controlErr := rejectPathControlChars(absPath); controlErr != nil {
		return controlErr
	}

	if osPathErr := ValidatePathForOS(absPath); osPathErr != nil {
		return osPathErr
	}

	return nil
}

func ensureUnderRoot(absPath, absRoot string) error {
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return fmt.Errorf("path outside allowed root: %w", errors.Join(ErrOutsideRoot, err))
	}

	if rel == "." {
		return nil
	}

	if relEscapesRoot(rel) {
		return fmt.Errorf("path traversal outside allowed root: %w", ErrPathTraversal)
	}

	return nil
}

func relEscapesRoot(rel string) bool {
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		if part == ".." {
			return true
		}
	}

	return false
}

func resolveExistingPath(path string, resolver PathResolver, role string) (string, error) {
	resolved, evalErr := resolver.EvalSymlinks(path)
	if evalErr != nil {
		return "", fmt.Errorf("validate path: eval %s symlinks: %w", role, evalErr)
	}

	return filepath.Clean(resolved), nil
}

func resolveCandidatePath(absPath, absRoot, effectiveRoot string, resolver PathResolver) (string, error) {
	_, err := resolver.Lstat(absPath)
	if err != nil {
		if pathNotExist(err) {
			return resolveMissingCandidatePath(absPath, absRoot, effectiveRoot, resolver)
		}

		return "", fmt.Errorf("validate path: stat: %w", err)
	}

	return resolveExistingPath(absPath, resolver, "candidate")
}

const asciiSpaceThreshold = 0x20

func rejectPathControlChars(path string) error {
	for i := 0; i < len(path); i++ {
		if path[i] < asciiSpaceThreshold {
			return fmt.Errorf("path contains control character at byte %d: %w", i, ErrControlChar)
		}
	}
	return nil
}

func resolveMissingCandidatePath(absPath, absRoot, effectiveRoot string, resolver PathResolver) (string, error) {
	boundary, resolvedBoundary, ok := candidateBoundary(absPath, absRoot, effectiveRoot)
	if !ok {
		return absPath, nil
	}

	return resolveMissingCandidateWithinBoundary(absPath, boundary, resolvedBoundary, resolver)
}

func resolveMissingCandidateWithinBoundary(
	absPath string,
	boundary string,
	resolvedBoundary string,
	resolver PathResolver,
) (string, error) {
	probe := filepath.Clean(absPath)
	missingParts := []string{}
	for {
		if sameCleanPath(probe, boundary) {
			return filepath.Join(append([]string{resolvedBoundary}, missingParts...)...), nil
		}

		_, statErr := resolver.Lstat(probe)
		if statErr != nil {
			nextProbe, nextMissingParts, err := advanceMissingCandidateProbe(probe, missingParts, statErr)
			if err != nil {
				return "", err
			}
			probe = nextProbe
			missingParts = nextMissingParts
			continue
		}

		return resolveExistingCandidateParent(probe, missingParts, resolver)
	}
}

func advanceMissingCandidateProbe(
	probe string,
	missingParts []string,
	statErr error,
) (
	nextProbe string,
	nextMissingParts []string,
	err error,
) {
	if !pathNotExist(statErr) {
		return "", nil, fmt.Errorf("validate path: stat candidate parent: %w", statErr)
	}

	nextMissingParts = append([]string{filepath.Base(probe)}, missingParts...)

	return filepath.Dir(probe), nextMissingParts, nil
}

func resolveExistingCandidateParent(
	probe string,
	missingParts []string,
	resolver PathResolver,
) (string, error) {
	resolved, evalErr := resolveExistingPath(probe, resolver, "candidate parent")
	if evalErr != nil {
		return "", evalErr
	}

	return filepath.Join(append([]string{resolved}, missingParts...)...), nil
}

func candidateBoundary(absPath, absRoot, effectiveRoot string) (
	boundary string,
	resolvedBoundary string,
	ok bool,
) {
	if pathWithinRoot(absPath, absRoot) {
		return absRoot, effectiveRoot, true
	}
	if pathWithinRoot(absPath, effectiveRoot) {
		return effectiveRoot, effectiveRoot, true
	}

	return "", "", false
}

func pathWithinRoot(path, root string) bool {
	return ensureUnderRoot(path, root) == nil
}

func sameCleanPath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

var reservedBaseNames = buildReservedBaseNames()

func buildReservedBaseNames() map[string]struct{} {
	return map[string]struct{}{
		"AUX":  {},
		"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {},
		"COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
		"CON":  {},
		"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {},
		"LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
		"NUL": {},
		"PRN": {},
	}
}

// ValidatePathForOS rejects Windows reserved device names regardless of platform.
// Reserved names: CON, NUL, PRN, AUX, COM1-COM9, LPT1-LPT9 (case-insensitive, with or without extension).
func ValidatePathForOS(path string) error {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	upper := strings.ToUpper(name)
	if _, ok := reservedBaseNames[upper]; ok {
		return fmt.Errorf("path uses reserved name: %w", ErrReservedName)
	}

	return nil
}

// RejectControlChars rejects bytes below ASCII 0x20 in the string.
// Exception: TAB (0x09) is allowed in general strings.
// NOTE: For paths, ALL control chars including TAB should be rejected — use ValidatePath for paths.
func RejectControlChars(s string) error {
	for idx := 0; idx < len(s); idx++ {
		ch := s[idx]
		if ch < 0x20 && ch != 0x09 {
			return fmt.Errorf("string contains control character: %w", ErrControlChar)
		}
	}

	return nil
}

var validExtensionRe = regexp.MustCompile(`^\.[a-zA-Z0-9]+$`)

// ValidateExtension checks that ext is a leading dot followed by alphanumeric characters only.
// Rejects path separators, traversal sequences, and control characters.
func ValidateExtension(ext string) error {
	if !validExtensionRe.MatchString(ext) {
		return fmt.Errorf("%w: %q (must be a dot followed by alphanumeric characters)", ErrInvalidExtension, ext)
	}
	return nil
}

const MaxPatternLength = 4096

// ValidatePattern compiles pattern as a regexp. Returns compiled regex or error.
func ValidatePattern(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, ErrEmptyRegexpPattern
	}

	if len(pattern) > MaxPatternLength {
		return nil, fmt.Errorf("validate pattern: length %d exceeds limit %d: %w",
			len(pattern), MaxPatternLength, ErrPatternTooLong)
	}

	if err := rejectPatternControlChars(pattern); err != nil {
		return nil, err
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("validate pattern: %w: %w", ErrInvalidPattern, err)
	}

	return re, nil
}

func rejectPatternControlChars(pattern string) error {
	for i := 0; i < len(pattern); i++ {
		if pattern[i] < asciiSpaceThreshold {
			return fmt.Errorf("regex pattern contains control character at byte %d: %w", i, ErrControlChar)
		}
	}

	return nil
}
