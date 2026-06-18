package pipeline

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

// DefaultMaxFiles is the default cap for split/blocks output file counts (see glyph-shift-intent.md).
const DefaultMaxFiles = 50

// effectiveMaxFiles returns params max when positive, otherwise DefaultMaxFiles.
func effectiveMaxFiles(requestedLimit int) int {
	if requestedLimit <= 0 {
		return DefaultMaxFiles
	}

	return requestedLimit
}

// ParseCommaSeparatedNames splits a CLI --names value on commas; trims spaces; rejects empty entries.
func ParseCommaSeparatedNames(s string) ([]string, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}

	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			return nil, ErrEmptyNamesListEntry
		}

		out = append(out, t)
	}

	return out, nil
}

func applyExplicitNamesToBlocks(blocks []fileops.Block, names []string, ext string) error {
	if len(names) == 0 {
		return nil
	}

	if len(names) != len(blocks) {
		return &NamesCountMismatchError{NamesCount: len(names), OutputCount: len(blocks)}
	}

	basenames, err := applyExplicitBasenames(names, ext)
	if err != nil {
		return err
	}

	for i := range blocks {
		blocks[i].Name = basenames[i]
	}

	return nil
}

func applyExplicitNamesToSplitSections(sections []fileops.SplitSection, names []string, ext string) error {
	if len(names) == 0 {
		return nil
	}

	if len(names) != len(sections) {
		return &NamesCountMismatchError{NamesCount: len(names), OutputCount: len(sections)}
	}

	basenames, err := applyExplicitBasenames(names, ext)
	if err != nil {
		return err
	}

	for i := range sections {
		sections[i].Name = basenames[i]
	}

	return nil
}

const asciiSpaceThreshold = 0x20

func rejectExplicitNameControlRunes(s string) error {
	for i, r := range s {
		if r < asciiSpaceThreshold || unicode.IsControl(r) {
			return fmt.Errorf("name contains disallowed control character at rune %d: %w", i, ErrInvalidExplicitName)
		}
	}

	return nil
}

// fragmentLooksAbsolute rejects forms that denote absolute or volume-root paths, not mere basenames.
func fragmentLooksAbsolute(fragment string) bool {
	if fragment == "" {
		return false
	}

	if fragment[0] == '\\' || fragment[0] == '/' {
		return true
	}

	if fragmentHasWindowsVolume(fragment) {
		return true
	}

	return filepath.IsAbs(fragment)
}

func fragmentHasWindowsVolume(fragment string) bool {
	if runtime.GOOS != "windows" {
		return false
	}

	// VolumeName is non-empty for "C:", "C:\path", UNC roots, etc. - reject in a basename fragment.
	return filepath.VolumeName(fragment) != ""
}

func validateExplicitNameFragment(fragment string) error {
	trimmed := strings.TrimSpace(fragment)
	if trimmed == "" {
		return fmt.Errorf("empty name fragment: %w", ErrInvalidExplicitName)
	}

	if fragmentLooksAbsolute(trimmed) {
		return fmt.Errorf("explicit name must be a basename, not an absolute path: %w", ErrInvalidExplicitName)
	}

	if err := rejectExplicitNameControlRunes(trimmed); err != nil {
		return err
	}

	return rejectExplicitNameSpecialFragments(trimmed)
}

func rejectExplicitNameSpecialFragments(trimmed string) error {
	if strings.ContainsAny(trimmed, `/\`) {
		return fmt.Errorf("explicit name must not contain path separators: %w", ErrInvalidExplicitName)
	}

	if strings.Contains(trimmed, "..") {
		return fmt.Errorf("explicit name must not contain '..': %w", ErrInvalidExplicitName)
	}

	switch trimmed {
	case ".", "..":
		return fmt.Errorf("invalid explicit name %q: %w", trimmed, ErrInvalidExplicitName)
	}

	return nil
}

// finalizeExplicitBasename maps one user fragment to an output basename: stems get defaultExt;
// if the fragment already has an extension, it is used as-is.
func finalizeExplicitBasename(fragment, defaultExt string) (string, error) {
	if err := validateExplicitNameFragment(fragment); err != nil {
		return "", err
	}

	trimmed := strings.TrimSpace(fragment)

	if filepath.Ext(trimmed) != "" {
		if err := validate.ValidatePathForOS(trimmed); err != nil {
			return "", errors.Join(ErrInvalidExplicitName, err)
		}

		return trimmed, nil
	}

	full := trimmed + defaultExt
	if err := validate.ValidatePathForOS(full); err != nil {
		return "", errors.Join(ErrInvalidExplicitName, err)
	}

	return full, nil
}

func basenameDedupKey(base string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(base)
	}

	return base
}

// applyExplicitBasenames maps fragments to basenames; rejects duplicates (case-insensitive on Windows).
func applyExplicitBasenames(fragments []string, defaultExt string) ([]string, error) {
	out := make([]string, len(fragments))
	seen := make(map[string]struct{})

	for fragmentIndex, fragment := range fragments {
		base, err := finalizeExplicitBasename(fragment, defaultExt)
		if err != nil {
			return nil, fmt.Errorf("name %d: %w", fragmentIndex+1, err)
		}

		key := basenameDedupKey(base)
		if _, dup := seen[key]; dup {
			return nil, fmt.Errorf("duplicate explicit output basename %q: %w", base, ErrDuplicateExplicitNames)
		}

		seen[key] = struct{}{}
		out[fragmentIndex] = base
	}

	return out, nil
}
