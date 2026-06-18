package testutil

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"runtime"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

func removeMemFsDestinationsForHeapBracket(memFs afero.Fs, destPaths []string, pipelineVerb string) error {
	for _, outPath := range destPaths {
		remErr := memFs.Remove(outPath)
		if remErr != nil && !errors.Is(remErr, afero.ErrFileNotFound) && !errors.Is(remErr, fs.ErrNotExist) {
			return fmt.Errorf("measure %s heap bracket remove destination %q: %w", pipelineVerb, outPath, remErr)
		}
	}

	return nil
}

// countingSourceOpenerForHeapBracketRestore clones logical source configuration with zero read counters
// for the second Run* pass in heap delta measurement.
func countingSourceOpenerForHeapBracketRestore(src *CountingSourceOpener) *CountingSourceOpener {
	return &CountingSourceOpener{
		Immutable:   src.Immutable,
		AllowedPath: src.AllowedPath,
	}
}

type bracketSplitMeasuringHeapCtx struct {
	ctx       context.Context
	src       *CountingSourceOpener
	memFs     afero.Fs
	resolver  validate.PathResolver
	publishFS fileops.FileSession
	params    pipeline.SplitParams
}

type bracketBlocksMeasuringHeapCtx struct {
	ctx       context.Context
	src       *CountingSourceOpener
	memFs     afero.Fs
	resolver  validate.PathResolver
	publishFS fileops.FileSession
	params    pipeline.BlocksParams
}

func countingSplitMeasuringHeapDelta(
	bracketCtx *bracketSplitMeasuringHeapCtx,
	runErr error,
	msHeapBefore *runtime.MemStats,
	res pipeline.SplitPipelineResult,
) (heapAllocDelta int64, err error) {
	switch {
	case runErr != nil || bracketCtx.params.Preview || len(res.Files) == 0:
		return heapAllocDeltaBracketAfterGC(msHeapBefore), nil

	default:
		if remErr := removeMemFsDestinationsForHeapBracket(bracketCtx.memFs, res.Files, "split"); remErr != nil {
			return 0, remErr
		}

		heapAllocDelta = heapAllocDeltaBracketAfterGC(msHeapBefore)

		restoreSrc := countingSourceOpenerForHeapBracketRestore(bracketCtx.src)

		restoreOut := NewMeasuringMemOutputOpener(bracketCtx.memFs)

		if _, restoreErr := pipeline.RunSplit(
			bracketCtx.ctx, restoreSrc, restoreOut, bracketCtx.resolver, bracketCtx.publishFS, bracketCtx.params,
		); restoreErr != nil {
			return 0, fmt.Errorf("measure split heap bracket restore destinations: %w", restoreErr)
		}

		return heapAllocDelta, nil
	}
}

//nolint:gocritic // hugeParam: BlocksPipelineResult participates in heap-bracket teardown bookkeeping.
func countingBlocksMeasuringHeapDelta(
	bracketCtx *bracketBlocksMeasuringHeapCtx,
	runErr error,
	msHeapBefore *runtime.MemStats,
	res pipeline.BlocksPipelineResult,
) (heapAllocDelta int64, err error) {
	switch {
	case runErr != nil || bracketCtx.params.Preview || len(res.Files) == 0:
		return heapAllocDeltaBracketAfterGC(msHeapBefore), nil

	default:
		if remErr := removeMemFsDestinationsForHeapBracket(bracketCtx.memFs, res.Files, "blocks"); remErr != nil {
			return 0, remErr
		}

		heapAllocDelta = heapAllocDeltaBracketAfterGC(msHeapBefore)

		restoreSrc := countingSourceOpenerForHeapBracketRestore(bracketCtx.src)

		restoreOut := NewMeasuringMemOutputOpener(bracketCtx.memFs)

		if _, restoreErr := pipeline.RunBlocks(
			bracketCtx.ctx, restoreSrc, restoreOut, bracketCtx.resolver, bracketCtx.publishFS, bracketCtx.params,
		); restoreErr != nil {
			return 0, fmt.Errorf("measure blocks heap bracket restore destinations: %w", restoreErr)
		}

		return heapAllocDelta, nil
	}
}
