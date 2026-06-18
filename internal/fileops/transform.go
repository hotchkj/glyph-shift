package fileops

import (
	"bytes"
	"errors"
)

var errTransformBinaryOnReplay = errors.New("transform file: binary on replay")

const transformSkipReasonNoTransform = "no transform"

// LineEndingTarget specifies the target line ending for transform.
type LineEndingTarget int

const (
	TargetLF LineEndingTarget = iota
	TargetCRLF
	TargetCR
)

// TransformOptions configures what transforms to apply.
type TransformOptions struct {
	LineEndings  *LineEndingTarget // nil = don't change
	TrimTrailing bool
	FinalNewline bool
}

// TransformFileResult describes changes made to a single file.
type TransformFileResult struct {
	Path string
	// EndingsChanged is the count of line terminators that did not match the target and were rewritten.
	EndingsChanged int
	// Per-type line-ending stats (source inspection). Set when --line-endings is requested.
	LFFound           int
	LFConverted       int
	CRFound           int
	CRConverted       int
	CRLFFound         int
	CRLFConverted     int
	TrailingTrimmed   int
	FinalNewlineAdded bool
	Skipped           bool
	SkipReason        string
	WouldChange       bool
}

func cloneLine(l Line) Line {
	return Line{
		Content:    append([]byte(nil), l.Content...),
		Terminator: append([]byte(nil), l.Terminator...),
	}
}

func terminatorForTarget(t LineEndingTarget) []byte {
	switch t {
	case TargetLF:
		return []byte{'\n'}
	case TargetCRLF:
		return []byte{'\r', '\n'}
	case TargetCR:
		return []byte{'\r'}
	}
	// Invalid values behave like LF (same as pre-explicit-TargetLF default).
	return []byte{'\n'}
}

func defaultFinalTerminator(opts TransformOptions) []byte {
	if opts.LineEndings != nil {
		return terminatorForTarget(*opts.LineEndings)
	}

	return []byte{'\n'}
}

func hasTransformOptions(opts TransformOptions) bool {
	return opts.LineEndings != nil || opts.TrimTrailing || opts.FinalNewline
}

func cloneLines(lines []Line) []Line {
	out := make([]Line, len(lines))
	for i := range lines {
		out[i] = cloneLine(lines[i])
	}

	return out
}

func trimTrailingSpacesTabs(lineBytes []byte) ([]byte, bool) {
	end := len(lineBytes)
	for end > 0 && (lineBytes[end-1] == ' ' || lineBytes[end-1] == '\t') {
		end--
	}

	if end == len(lineBytes) {
		return lineBytes, false
	}

	return append([]byte(nil), lineBytes[:end]...), true
}

type terminatorKind int

const (
	termUnknown terminatorKind = iota
	termLF
	termCR
	termCRLF
)

func classifyTerminator(term []byte) terminatorKind {
	if len(term) == 0 {
		return termUnknown
	}

	if len(term) == 2 && term[0] == '\r' && term[1] == '\n' {
		return termCRLF
	}

	if len(term) != 1 {
		return termUnknown
	}

	return classifySingleByteTerminator(term[0])
}

func classifySingleByteTerminator(ch byte) terminatorKind {
	switch ch {
	case '\n':
		return termLF
	case '\r':
		return termCR
	default:
		return termUnknown
	}
}

// lineEndingScanStats holds per-terminator-type counts from a source scan.
type lineEndingScanStats struct {
	lfFound       int
	lfConverted   int
	crFound       int
	crConverted   int
	crlfFound     int
	crlfConverted int
}

func addLineEndingObservation(stats *lineEndingScanStats, lineTerm, want []byte) {
	switch classifyTerminator(lineTerm) {
	case termUnknown:
		// Unknown terminators are not counted in per-type stats.
	case termCRLF:
		addTermObservation(&stats.crlfFound, &stats.crlfConverted, lineTerm, want)
	case termLF:
		addTermObservation(&stats.lfFound, &stats.lfConverted, lineTerm, want)
	case termCR:
		addTermObservation(&stats.crFound, &stats.crConverted, lineTerm, want)
	}
}

func addTermObservation(found, converted *int, lineTerm, want []byte) {
	*found++
	if !bytes.Equal(lineTerm, want) {
		*converted++
	}
}

func lineEndingStats(lines []Line, target LineEndingTarget) lineEndingScanStats {
	want := terminatorForTarget(target)
	var stats lineEndingScanStats

	for _, ln := range lines {
		addLineEndingObservation(&stats, ln.Terminator, want)
	}

	return stats
}

func applyLineEndings(out []Line, target LineEndingTarget) int {
	want := terminatorForTarget(target)
	changed := 0

	for lineIdx := range out {
		if len(out[lineIdx].Terminator) == 0 {
			continue
		}

		if !bytes.Equal(out[lineIdx].Terminator, want) {
			changed++
		}

		out[lineIdx].Terminator = append([]byte(nil), want...)
	}

	return changed
}

func applyTrimTrailing(out []Line) int {
	trimmed := 0

	for lineIdx := range out {
		newContent, did := trimTrailingSpacesTabs(out[lineIdx].Content)
		if did {
			trimmed++
			out[lineIdx].Content = newContent
		}
	}

	return trimmed
}

func applyFinalNewline(out []Line, opts TransformOptions) bool {
	if len(out) == 0 {
		return false
	}

	last := len(out) - 1
	if len(out[last].Terminator) > 0 {
		return false
	}

	out[last].Terminator = append([]byte(nil), defaultFinalTerminator(opts)...)

	return true
}

// CountTransformChanges returns the total number of individual changes in a transform result.
func CountTransformChanges(res *TransformFileResult) int {
	if res == nil {
		return 0
	}

	n := res.EndingsChanged + res.TrailingTrimmed
	if res.FinalNewlineAdded {
		n++
	}

	return n
}

// TransformLines applies transforms to a slice of Lines and returns a new slice plus stats.
func TransformLines(lines []Line, opts TransformOptions) ([]Line, TransformFileResult) {
	var res TransformFileResult

	if !hasTransformOptions(opts) {
		res.Skipped = true
		res.SkipReason = transformSkipReasonNoTransform

		return lines, res
	}

	out := cloneLines(lines)

	if opts.LineEndings != nil {
		applyLineEndingTransform(lines, out, opts, &res)
	}

	if opts.TrimTrailing {
		res.TrailingTrimmed = applyTrimTrailing(out)
	}

	if opts.FinalNewline {
		res.FinalNewlineAdded = applyFinalNewline(out, opts)
	}

	return out, res
}

func applyLineEndingTransform(lines, out []Line, opts TransformOptions, res *TransformFileResult) {
	scan := lineEndingStats(lines, *opts.LineEndings)
	res.LFFound = scan.lfFound
	res.LFConverted = scan.lfConverted
	res.CRFound = scan.crFound
	res.CRConverted = scan.crConverted
	res.CRLFFound = scan.crlfFound
	res.CRLFConverted = scan.crlfConverted
	res.EndingsChanged = applyLineEndings(out, *opts.LineEndings)
}
