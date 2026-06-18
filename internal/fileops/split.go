package fileops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
)

const splitErrorFormat = "split: %w"

var errSplitNilDelimiter = errors.New("delimiter is required")

var errSplitScanEmptySegment = errors.New("split scan: empty segment")

// ErrSplitReplayComplete ends line iteration once all output line spans are consumed.
// ForEachLineFromContext returns it wrapped; callers should treat errors.Is(err, ErrSplitReplayComplete) as success.
var ErrSplitReplayComplete = errors.New("split replay complete")

// SplitOptions configures the split operation.
type SplitOptions struct {
	Source         io.Reader
	Delimiter      *regexp.Regexp
	Naming         NamingStrategy
	StripDelimiter bool
	Extension      string // includes leading dot, e.g. ".txt"
}

// SplitSection is one output section with its filename and lines.
type SplitSection struct {
	Name  string
	Lines []Line
}

// SplitResult holds split sections and optional warnings.
type SplitResult struct {
	Sections []SplitSection
	Warnings []string
}

// Split reads lines from Source and splits at each line matching Delimiter. It is a helper API returning
// []SplitSection with []Line payloads; it is not the production byte-span output path.
//
// Seekable inputs use ScanSplitSectionsMeta (line-span scan + MatchLineSpan, no scan-phase []Line retention)
// for metadata—a shape aligned with production pipeline descriptors that fingerprint byte spans—then replay
// into helper SplitSection slices via ForEachLineFromContext, retaining one materialized logical Line at a
// time. Non-seekable inputs buffer the full source, then use the same replay path (full-line materialization
// before seek-like replay on an in-memory reader).
func Split(ctx context.Context, opts SplitOptions) (SplitResult, error) {
	if opts.Delimiter == nil {
		return SplitResult{}, fmt.Errorf(splitErrorFormat, errSplitNilDelimiter)
	}

	if err := ctx.Err(); err != nil {
		return SplitResult{}, fmt.Errorf(splitErrorFormat, err)
	}

	src := opts.Source
	if _, ok := src.(io.ReadSeeker); ok {
		return splitViaScanAndReplay(ctx, opts)
	}

	data, err := io.ReadAll(src)
	if err != nil {
		return SplitResult{}, fmt.Errorf(splitErrorFormat, err)
	}

	opts2 := opts
	opts2.Source = bytes.NewReader(data)

	return splitViaScanAndReplay(ctx, opts2)
}

// splitViaScanAndReplay replays scan metadata from ScanSplitSectionsMeta into materialized helper results:
// []SplitSection with []Line built via ForEachLineFromContext (dupLine per logical line), after rewinding the
// seekable source.
func splitViaScanAndReplay(ctx context.Context, opts SplitOptions) (SplitResult, error) {
	rs := opts.Source.(io.ReadSeeker)

	scan, err := ScanSplitSectionsMeta(ctx, opts, BoundedScanLimits{})
	if err != nil {
		return SplitResult{}, err
	}

	if _, seekErr := rs.Seek(0, io.SeekStart); seekErr != nil {
		return SplitResult{}, fmt.Errorf(splitErrorFormat, seekErr)
	}

	sections, err := replaySplitMetasToSections(ctx, opts.Source, scan.Sections)
	if err != nil {
		return SplitResult{}, fmt.Errorf(splitErrorFormat, err)
	}

	return SplitResult{Sections: sections}, nil
}

type splitSectionReplay struct {
	metas    []SplitSectionMeta
	sections []SplitSection
	lastSec  int
	hint     int
	lineNum  int
}

func newSplitSectionReplay(metas []SplitSectionMeta) *splitSectionReplay {
	sections := make([]SplitSection, len(metas))
	for i := range metas {
		sections[i].Name = metas[i].Name
		if metas[i].ContentLineCount > 0 {
			sections[i].Lines = make([]Line, 0, metas[i].ContentLineCount)
		}
	}

	return &splitSectionReplay{
		metas:    metas,
		sections: sections,
		lastSec:  len(metas) - 1,
	}
}

func (r *splitSectionReplay) advanceHint() {
	for r.hint < len(r.metas) && r.lineNum > r.metas[r.hint].OutputEndLineNum {
		r.hint++
	}
}

func (r *splitSectionReplay) onLine(ln Line) error {
	r.lineNum++
	r.advanceHint()

	if r.hint >= len(r.metas) {
		return fmt.Errorf("%w", ErrSplitReplayComplete)
	}

	m := r.metas[r.hint]
	if r.lineNum < m.OutputStartLineNum {
		return nil
	}

	r.sections[r.hint].Lines = append(r.sections[r.hint].Lines, dupLine(ln))

	if r.hint == r.lastSec && r.lineNum == r.metas[r.lastSec].OutputEndLineNum {
		return fmt.Errorf("%w", ErrSplitReplayComplete)
	}

	return nil
}

func replaySplitMetasToSections(
	ctx context.Context,
	src io.Reader,
	metas []SplitSectionMeta,
) ([]SplitSection, error) {
	if len(metas) == 0 {
		return []SplitSection{}, nil
	}

	rep := newSplitSectionReplay(metas)

	err := ForEachLineFromContext(ctx, src, rep.onLine)
	if err == nil {
		return rep.sections, nil
	}

	if errors.Is(err, ErrSplitReplayComplete) {
		return rep.sections, nil
	}

	return nil, err
}
