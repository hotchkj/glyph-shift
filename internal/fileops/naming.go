package fileops

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/hotchkj/glyph-shift/internal/validate"
)

// NamingStrategy determines how output files are named.
type NamingStrategy int

const (
	// Sequential uses zero-padded 3-digit index: 001, 002, ...
	Sequential NamingStrategy = iota
	// FromContent slugifies the first line of content.
	FromContent
	// FromDelimiter slugifies the delimiter match text.
	FromDelimiter
)

const (
	slugMaxRunes     = 60
	sequentialDigits = 3
)

var osUnsafeReplacer = strings.NewReplacer(
	"<", "",
	">", "",
	":", "",
	`"`, "",
	"'", "",
	"/", "",
	`\`, "",
	"|", "",
	"?", "",
	"*", "",
)

// GenerateFilename produces a filename from the strategy, index, text, and extension.
// Extension should include the leading dot (e.g., ".txt").
func GenerateFilename(strategy NamingStrategy, index int, text, ext string) string {
	switch strategy {
	case Sequential:
		return sequentialFilename(index, ext)
	case FromContent, FromDelimiter:
		slug := slugifyFromText(text)
		slug = strings.TrimRight(slug, " .")
		if slug == "" {
			return sequentialFilename(index, ext)
		}

		candidate := slug + ext
		if err := validate.ValidatePathForOS(candidate); err != nil {
			return sequentialFilename(index, ext)
		}

		return candidate
	default:
		return sequentialFilename(index, ext)
	}
}

func sequentialFilename(index int, ext string) string {
	if index < 0 {
		index = 0
	}

	return fmt.Sprintf("%0*d%s", sequentialDigits, index, ext)
}

func slugifyFromText(text string) string {
	lower := strings.ToLower(text)
	stripped := osUnsafeReplacer.Replace(lower)

	var builder strings.Builder
	lastWasHyphen := false

	for _, r := range stripped {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastWasHyphen = false

			continue
		}

		if !lastWasHyphen && builder.Len() > 0 {
			builder.WriteRune('-')
			lastWasHyphen = true
		}
	}

	slug := builder.String()
	slug = strings.Trim(slug, "-")

	return truncateSlugAtWordBoundary(slug)
}

func truncateSlugAtWordBoundary(sl string) string {
	rr := []rune(sl)
	if len(rr) <= slugMaxRunes {
		return sl
	}

	cut := rr[:slugMaxRunes]
	lastHyp := -1

	for i := len(cut) - 1; i >= 0; i-- {
		if cut[i] == '-' {
			lastHyp = i

			break
		}
	}

	if lastHyp > 0 {
		return string(cut[:lastHyp])
	}

	return string(cut)
}

// DeduplicateFilename appends -2, -3, etc. if name already exists in the set.
// Returns the unique name and updates existing.
func DeduplicateFilename(name string, existing map[string]bool) string {
	if existing == nil {
		return name
	}

	if !existing[name] {
		existing[name] = true

		return name
	}

	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	suffix := 2

	for {
		candidate := fmt.Sprintf("%s-%d%s", base, suffix, ext)
		if !existing[candidate] {
			existing[candidate] = true

			return candidate
		}

		suffix++
	}
}
