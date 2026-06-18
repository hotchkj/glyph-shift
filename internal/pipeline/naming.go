package pipeline

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

// ErrUnknownNamingStrategy is returned when an unrecognized naming strategy is given.
var ErrUnknownNamingStrategy = errors.New("unknown naming strategy")

// ParseNamingStrategy converts a string naming flag to a NamingStrategy.
// Returns ErrUnknownNamingStrategy for unrecognized values.
// Empty string defaults to Sequential (for MCP callers who omit the field).
func ParseNamingStrategy(raw string) (fileops.NamingStrategy, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "sequential", "":
		return fileops.Sequential, nil
	case "content":
		return fileops.FromContent, nil
	case "match":
		return fileops.FromDelimiter, nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrUnknownNamingStrategy, raw)
	}
}
