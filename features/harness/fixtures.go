package harness

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/hotchkj/glyph-shift/internal/validate"
)

const (
	// BinaryFixturePaddingSuffix is a small zero-byte padding suffix appended to binary fixtures.
	BinaryFixturePaddingSuffix = 10
	// OutDir is the default CLI/blocks/split output directory in BDD scenarios.
	OutDir = "out"
)

// controlCharSOH is ASCII SOH (0x01), used in regex validation scenarios.
const controlCharSOH = rune(1)

var (
	// MixedLineEndingsBytes is the canonical mixed CRLF/LF/CR source fixture.
	MixedLineEndingsBytes = []byte("aa\r\nbb\ncc\rdd\r\n")
	// TrailingWhitespaceBytes is the canonical trailing-whitespace transform fixture.
	TrailingWhitespaceBytes = []byte("x  \r\ny\t \n")
	// SoloLineNoFinalNewline is a single line without a trailing newline.
	SoloLineNoFinalNewline = []byte("solo line content")
	// CRLFTrailingNoFinalNewline is CRLF lines with trailing whitespace and no final newline.
	CRLFTrailingNoFinalNewline = []byte("line1  \r\nline2\t ")
	// ThreeLinesSharedContent is used when two files in one directory share payload bytes.
	ThreeLinesSharedContent = []byte("line one\nline two\nline three\n")
	// ImageBinaryBytes is the transform binary-source fixture (image.png).
	ImageBinaryBytes = []byte{0x00, 0xff, 0xfe}
)

// BinaryFileFixture returns the "a binary file" BDD Given payload.
func BinaryFileFixture() []byte {
	return append([]byte("binary\x00data\n"), make([]byte, BinaryFixturePaddingSuffix)...)
}

// BinarySourceCLIFixture returns the Layer 3 CLI binary-source payload (null in first 8KB).
func BinarySourceCLIFixture() []byte {
	return append([]byte("binary content\x00with null byte\n"), make([]byte, BinaryFixturePaddingSuffix)...)
}

// LineTerminator maps BDD/CLI names (LF, CRLF, CR) to terminator bytes.
func LineTerminator(name string) ([]byte, error) {
	switch strings.ToUpper(name) {
	case "LF":
		return []byte{'\n'}, nil
	case "CRLF":
		return []byte{'\r', '\n'}, nil
	case "CR":
		return []byte{'\r'}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownLineTerminator, name)
	}
}

// LineSuffix returns the suffix string for numbered-line fixtures (LF, CRLF, CR).
func LineSuffix(terminator string) (string, error) {
	term, err := LineTerminator(terminator)
	if err != nil {
		return "", err
	}

	return string(term), nil
}

// NumberedLineContent builds "line N{suffix}" for N in 1..lineCount.
func NumberedLineContent(lineCount int, terminator string) ([]byte, error) {
	suffix, err := LineSuffix(terminator)
	if err != nil {
		return nil, err
	}

	var buf strings.Builder
	for i := 1; i <= lineCount; i++ {
		_, _ = fmt.Fprintf(&buf, "line %d%s", i, suffix)
	}

	return []byte(buf.String()), nil
}

// CRLFLineContent builds numbered lines with CRLF terminators.
// Panics only if NumberedLineContent returns an unexpected error; the hardcoded
// terminator "CRLF" is guaranteed valid.
func CRLFLineContent(lineCount int) []byte {
	data, err := NumberedLineContent(lineCount, "CRLF")
	if err != nil {
		panic(err)
	}

	return data
}

// LFLineContent builds numbered lines with LF terminators.
// Panics only if NumberedLineContent returns an unexpected error; the hardcoded
// terminator "LF" is guaranteed valid.
func LFLineContent(lineCount int) []byte {
	data, err := NumberedLineContent(lineCount, "LF")
	if err != nil {
		panic(err)
	}

	return data
}

// MixedEndingStatsContent builds L/C/R labeled lines for transform stats scenarios.
func MixedEndingStatsContent(nLF, nCR, nCRLF int) []byte {
	var buf strings.Builder
	for i := 0; i < nLF; i++ {
		_, _ = fmt.Fprintf(&buf, "L%d\n", i)
	}
	for i := 0; i < nCR; i++ {
		_, _ = fmt.Fprintf(&buf, "C%d\r", i)
	}
	for i := 0; i < nCRLF; i++ {
		_, _ = fmt.Fprintf(&buf, "R%d\r\n", i)
	}

	return []byte(buf.String())
}

// DelimitedSectionsContent builds N --- delimited sections.
func DelimitedSectionsContent(n int) []byte {
	var sb strings.Builder
	for i := 1; i <= n; i++ {
		_, _ = fmt.Fprintf(&sb, "---\nsection %d content\n", i)
	}

	return []byte(sb.String())
}

// FencedBlocksContent builds N generic fenced blocks.
func FencedBlocksContent(n int) []byte {
	var sb strings.Builder
	for i := 1; i <= n; i++ {
		_, _ = fmt.Fprintf(&sb, "```\nblock %d\n```\n", i)
	}

	return []byte(sb.String())
}

// EmptyGoFencedBlocksContent builds N empty ```go fenced blocks (BDD blocks scenarios).
func EmptyGoFencedBlocksContent(n int) []byte {
	var sb strings.Builder
	for range n {
		_, _ = fmt.Fprintf(&sb, "```go\n```\n")
	}

	return []byte(sb.String())
}

// DecodeEscapedFixture decodes a quoted .bytes golden/input fixture.
func DecodeEscapedFixture(data []byte) ([]byte, error) {
	encoded := string(data)
	encoded = strings.TrimSuffix(encoded, "\n")
	encoded = strings.TrimSuffix(encoded, "\r")

	decoded, err := strconv.Unquote(`"` + encoded + `"`)
	if err != nil {
		return nil, err
	}

	return []byte(decoded), nil
}

// RegexPatternLongerThanMaximum returns a pattern longer than validate.MaxPatternLength.
func RegexPatternLongerThanMaximum() string {
	return strings.Repeat("a", validate.MaxPatternLength+1)
}

// RegexPatternWithControlCharacter returns a pattern containing a control character.
func RegexPatternWithControlCharacter() string {
	return "prefix" + string(controlCharSOH) + "suffix"
}

// ExpectedLineRangeBytes extracts inclusive line range bytes using lineio semantics.
func ExpectedLineRangeBytes(src []byte, start, end int) ([]byte, error) {
	lines, err := ReadLinesFrom(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}

	if start < 1 || start > end || end > len(lines) {
		return nil, fmt.Errorf("%w: %d-%d (lines=%d)", ErrInvalidLineRange, start, end, len(lines))
	}

	selected := lines[start-1 : end]
	var buf bytes.Buffer
	if writeErr := WriteLinesTo(&buf, selected); writeErr != nil {
		return nil, writeErr
	}

	return buf.Bytes(), nil
}
