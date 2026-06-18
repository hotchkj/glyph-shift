package fileops

import (
	"bufio"
	"context"
	"fmt"
	"io"
)

func transformOptsActive(opts TransformOptions) bool {
	return opts.LineEndings != nil || opts.TrimTrailing || opts.FinalNewline
}

// transformStream implements transform-specific streaming line scanning without accumulating
// full logical lines in memory (see lineStream.content in lineio.go). Content bytes are
// written through bw when non-nil; with TrimTrailing and a writer, ambiguous trailing space/tab
// runs use pending with bounded RAM and spill-to-temp to preserve exact byte order when flushed.
// When bw is nil (measure-only pass), trim state is a line-local counter only (no spill).
type transformStream struct {
	ctx  context.Context
	br   *bufio.Reader
	bw   *bufio.Writer
	opts TransformOptions
	want []byte
	scan *lineEndingScanStats
	res  *TransformFileResult

	linesSinceCheck int

	pending pendingWhitespace
	// trimTrailingRun is the length of the current trailing space/tab suffix when bw is nil.
	trimTrailingRun int

	// lineBodyBytes counts every byte belonging to the current logical line body (excluding
	// terminators), matching lineStream.content length for flushTrailing decisions.
	lineBodyBytes int
}

// runTransformStream applies opts by scanning logical lines from src and writing transformed bytes
// to out without retaining each line's full content in memory.
// If out is nil, stats are computed without writing output and without trim-trailing temp spill.
// opts must be non-empty per transformOptsActive.
func runTransformStream(
	ctx context.Context,
	src io.Reader,
	opts TransformOptions,
	out io.Writer,
	spill WhitespaceSpillBacking,
) (TransformFileResult, error) {
	var res TransformFileResult

	var want []byte

	if opts.LineEndings != nil {
		want = terminatorForTarget(*opts.LineEndings)
	}

	var scan lineEndingScanStats

	var bw *bufio.Writer

	if out != nil {
		bw = bufio.NewWriterSize(out, transformStreamWriterBufBytes)
	}

	ts := transformStream{
		ctx:  ctx,
		br:   bufio.NewReader(src),
		bw:   bw,
		opts: opts,
		want: want,
		scan: &scan,
		res:  &res,
	}
	ts.pending.backing = spill
	defer ts.pending.discard()

	runErr := ts.run()
	if runErr != nil {
		return res, runErr
	}

	if bw != nil {
		if flushErr := bw.Flush(); flushErr != nil {
			return res, fmt.Errorf("transform stream flush: %w", flushErr)
		}
	}

	if opts.LineEndings != nil {
		res.LFFound = scan.lfFound
		res.LFConverted = scan.lfConverted
		res.CRFound = scan.crFound
		res.CRConverted = scan.crConverted
		res.CRLFFound = scan.crlfFound
		res.CRLFConverted = scan.crlfConverted
	}

	return res, nil
}
