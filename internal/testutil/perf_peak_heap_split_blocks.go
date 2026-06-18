package testutil

import (
	"context"
	"io"
	"runtime"
	"sync/atomic"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

// peakHeapBaselineAfterDoubleGC mirrors the split/blocks residency peak-heap prelude: stabilize with
// two GC cycles before recording HeapAlloc baseline and TotalAlloc start (same snapshot for both uses).
func peakHeapBaselineAfterDoubleGC() runtime.MemStats {
	runtime.GC()
	runtime.GC()

	var msIn runtime.MemStats
	runtime.ReadMemStats(&msIn)

	return msIn
}

// peakHeapSplitBlocksTotals aggregates MemStats deltas shared by MeasureRunSplitPeakHeap and MeasureRunBlocksPeakHeap.
type peakHeapSplitBlocksTotals struct {
	SourceBytesRead        int64
	SourceReadCalls        int64
	OutputBytesWritten     int64
	DestinationOpens       int64
	TotalAllocDelta        uint64
	PeakHeapAllocDelta     int64
	RetainedHeapAllocDelta int64
}

// finalizePeakHeapSplitBlocksTotals records immediate post-run totals and retains the GC-bracket retained delta.
func finalizePeakHeapSplitBlocksTotals(
	src *CountingSourceOpener,
	destBytesWritten int64,
	destinationOpens int64,
	msBaseline *runtime.MemStats,
) peakHeapSplitBlocksTotals {
	var msPeak runtime.MemStats
	runtime.ReadMemStats(&msPeak)

	return peakHeapSplitBlocksTotals{
		SourceBytesRead:        src.AggregateSourceBytesRead(),
		SourceReadCalls:        src.AggregateSourceReadCalls(),
		OutputBytesWritten:     destBytesWritten,
		DestinationOpens:       destinationOpens,
		TotalAllocDelta:        msPeak.TotalAlloc - msBaseline.TotalAlloc,
		PeakHeapAllocDelta:     heapAllocDeltaSigned(msBaseline.HeapAlloc, msPeak.HeapAlloc),
		RetainedHeapAllocDelta: heapAllocDeltaBracketAfterGC(msBaseline),
	}
}

// PeakHeapOutputOpener is the output dependency for MeasureRunSplitPeakHeap and MeasureRunBlocksPeakHeap.
// CountingOutputOpener retains destination bytes for content assertions; NonRetainingOutputOpener
// counts writes and drops payloads for residency probes without retaining output bytes in-process.
type PeakHeapOutputOpener interface {
	pipeline.OutputOpener
	BytesWritten() int64
	DestinationOpens() int64
}

// MeasureRunSplitPeakHeap runs pipeline.RunSplit and records TotalAlloc, an immediate PeakHeapAllocDelta
// (pre-stabilization), and a GC-stabilized RetainedHeapAllocDelta versus the HeapAlloc baseline before RunSplit.
//
// Evidence hierarchy for boundedness: pipeline tests bind large-vs-small workloads using
// RetainedHeapAllocDelta and StreamingBodyResidencyRetainedHeapBudget (primary). PeakHeapAllocDelta on the large
// run with MaxPeakHeapGrowthForStreamingBody is a noisy secondary smoke probe only - not the scaling contract.
//
// Two baseline GC cycles before capturing msIn mirror prior peak-heap noise reduction.
//
//nolint:gocritic // hugeParam: SplitParams carried with openers for peak-heap measurement façade.
func MeasureRunSplitPeakHeap(
	ctx context.Context,
	src *CountingSourceOpener,
	out PeakHeapOutputOpener,
	resolver validate.PathResolver,
	params pipeline.SplitParams,
) (SplitLargeOutputMemMeasurement, pipeline.SplitPipelineResult, error) {
	if resolver == nil || src == nil || out == nil {
		return SplitLargeOutputMemMeasurement{}, pipeline.SplitPipelineResult{}, ErrMeasureRunSplitPeakHeapNilDeps
	}

	msIn := peakHeapBaselineAfterDoubleGC()

	var destBytes atomic.Int64
	var tempCreates atomic.Int64

	publishFS := NewCountingMemStagingPublishSession(afero.NewMemMapFs(), func(path string, w io.Writer) io.Writer {
		return &measuringPublishWriter{inner: w, tallyPtr: &destBytes}
	}, &tempCreates)

	res, err := pipeline.RunSplit(ctx, src, out, resolver, publishFS, params)

	tot := finalizePeakHeapSplitBlocksTotals(src, destBytes.Load(), tempCreates.Load(), &msIn)
	meas := SplitLargeOutputMemMeasurement(tot)

	return meas, res, err
}

// MeasureRunBlocksPeakHeap mirrors MeasureRunSplitPeakHeap: RetainedHeapAllocDelta is primary; immediate
// PeakHeapAllocDelta versus MaxPeakHeapGrowthForStreamingBody on the large run is supplementary smoke coverage.
//
//nolint:gocritic // hugeParam: BlocksParams carried with openers for peak-heap measurement façade.
func MeasureRunBlocksPeakHeap(
	ctx context.Context,
	src *CountingSourceOpener,
	out PeakHeapOutputOpener,
	resolver validate.PathResolver,
	params pipeline.BlocksParams,
) (BlocksLargeOutputMemMeasurement, pipeline.BlocksPipelineResult, error) {
	if resolver == nil || src == nil || out == nil {
		return BlocksLargeOutputMemMeasurement{}, pipeline.BlocksPipelineResult{}, ErrMeasureRunBlocksPeakHeapNilDeps
	}

	msIn := peakHeapBaselineAfterDoubleGC()

	var destBytes atomic.Int64
	var tempCreates atomic.Int64

	publishFS := NewCountingMemStagingPublishSession(afero.NewMemMapFs(), func(path string, w io.Writer) io.Writer {
		return &measuringPublishWriter{inner: w, tallyPtr: &destBytes}
	}, &tempCreates)

	res, err := pipeline.RunBlocks(ctx, src, out, resolver, publishFS, params)

	tot := finalizePeakHeapSplitBlocksTotals(src, destBytes.Load(), tempCreates.Load(), &msIn)
	meas := BlocksLargeOutputMemMeasurement(tot)

	return meas, res, err
}
