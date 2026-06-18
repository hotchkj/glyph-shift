package testutil

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

// BoundednessBinaryCheckReadWindow matches the pipeline Git-style binary guard window; tail-guard
// fixtures must keep delimiter-driven max-files proofs beyond this offset so forbidden tail reads
// are not triggered solely during the binary preamble scan.
const BoundednessBinaryCheckReadWindow = 8000

// ErrBoundednessTailConsumptionForbidden is returned by tail-guard readers when the pipeline
// attempts to read bytes after the bounded prefix (the logical tail). Tests use errors.Is rather
// than string matching on the sentinel.
var ErrBoundednessTailConsumptionForbidden = errors.New(
	"testutil: boundedness tail consumption forbidden beyond max-files-knowable prefix",
)

// ErrTailGuardSeekInvalidWhence is returned by TailGuardReadSeekCloser.Seek for an unsupported whence.
var ErrTailGuardSeekInvalidWhence = errors.New("testutil tail guard seek: invalid whence")

// ErrTailGuardSeekNegativePosition is returned when Seek would position before the prefix start.
var ErrTailGuardSeekNegativePosition = errors.New("testutil tail guard seek: negative position")

// ErrMeasureRunSplitPeakHeapNilDeps is returned when MeasureRunSplitPeakHeap receives a nil
// resolver, source opener, or output opener.
var ErrMeasureRunSplitPeakHeapNilDeps = errors.New(
	"testutil MeasureRunSplitPeakHeap: nil resolver, source, or output opener",
)

// ErrMeasureRunBlocksPeakHeapNilDeps is returned when MeasureRunBlocksPeakHeap receives a nil
// resolver, source opener, or output opener.
var ErrMeasureRunBlocksPeakHeapNilDeps = errors.New(
	"testutil MeasureRunBlocksPeakHeap: nil resolver, source, or output opener",
)

// minBlocksCountForMaxFilesExceededFixture is the minimum closed blocks count for BuildMaxFilesExceededBlocksPrefix.
const minBlocksCountForMaxFilesExceededFixture = 2

// LargeStreamingFixtureLineCount and LargeStreamingFixtureLineLength describe the repeating body
// used by split/blocks large-output residency probes: approximate payload is lineCount*lineLength.
const (
	LargeStreamingFixtureLineCount  = 120_000
	LargeStreamingFixtureLineLength = 48
)

// StreamingResidencyProbeSmallLineCount and StreamingResidencyProbeSmallLineLength size the
// baseline workload paired with LargeStreamingFixture* for primary scaling evidence in pipeline
// residency tests.
const (
	StreamingResidencyProbeSmallLineCount  = 1_000
	StreamingResidencyProbeSmallLineLength = 48
)

// MaxPeakHeapGrowthForStreamingBody is a secondary, process-wide smoke guard on immediate HeapAlloc
// growth (right after RunSplit/RunBlocks returns, before bracket GC stabilization on the residency
// helper) on the large streaming workload only.
//
// Evidence hierarchy for boundedness: (1) primary - paired GC-stabilized retained heap deltas
// (SplitLargeOutputMemMeasurement.RetainedHeapAllocDelta / StreamingBodyResidencyRetainedHeapBudget);
// (2) secondary smoke - PeakHeapAllocDelta immediately after Run* plus this absolute ceiling tied to large
// line-by-line streaming payloads.
//
// Targets full-source/section/block line residency (O(body) retained metadata), not small bounded
// stdlib buffers, regex workspaces, or destination IO scratch (especially when measuring with an
// output opener that discards payloads).
//
// PeakHeap probes use process-wide runtime.MemStats; default `go test ./internal/fileops
// ./internal/pipeline` can see concurrent package goroutines and allocator noise on top of bounded
// runtime buffers. Red tests that materialized full line metadata for LargeStreamingFixture* landed
// well above tens of MiB retained growth; 32MiB absorbs that cross-test variance while remaining
// far below the old full-residency signal.
const MaxPeakHeapGrowthForStreamingBody = 32 * 1024 * 1024

// StreamingBodyRetainedHeapMaxLargeToSmallRatio is the multiplier applied to the small-run
// RetainedHeapAllocDelta when asserting the paired large retained heap stays bounded.
const StreamingBodyRetainedHeapMaxLargeToSmallRatio int64 = 2

// StreamingBodyResidencyRetainedHeapNoiseAllowance is a fixed MemStats slab on top of
// StreamingBodyRetainedHeapMaxLargeToSmallRatio * max(0, small retained delta). It only absorbs jitter
// around bracketed HeapAlloc snapshots and paired-test timing - not proportional body-size slack and
// not line-count normalization.
//
// When the small workload’s retained delta is negative (or zero once clamped), the scaled term contributes
// nothing; the pairing must still tolerate MemStats variance, so brittle 512 KiB-only floors were raised to
// 1 MiB as fixed bracket headroom.
const StreamingBodyResidencyRetainedHeapNoiseAllowance int64 = 1 * 1024 * 1024

// StreamingBodyResidencyRetainedHeapBudget bounds large retained HeapAlloc drift against the paired
// small workload: ratio × nonNegative(smallRetainedHeapDelta) + StreamingBodyResidencyRetainedHeapNoiseAllowance.
func StreamingBodyResidencyRetainedHeapBudget(smallRetainedHeapDelta int64) int64 {
	baseline := smallRetainedHeapDelta
	if baseline < 0 {
		baseline = 0
	}

	return StreamingBodyRetainedHeapMaxLargeToSmallRatio*baseline +
		StreamingBodyResidencyRetainedHeapNoiseAllowance
}

// SplitLargeOutputMemMeasurement captures allocation signals for split boundedness tests.
type SplitLargeOutputMemMeasurement struct {
	SourceBytesRead        int64
	SourceReadCalls        int64
	OutputBytesWritten     int64
	DestinationOpens       int64
	TotalAllocDelta        uint64
	PeakHeapAllocDelta     int64 // immediate HeapAlloc snapshot after RunSplit (secondary smoke)
	RetainedHeapAllocDelta int64 // GC bracket vs baseline HeapAlloc (primary scaled bound)
}

// BlocksLargeOutputMemMeasurement captures allocation signals for blocks boundedness tests.
type BlocksLargeOutputMemMeasurement struct {
	SourceBytesRead        int64
	SourceReadCalls        int64
	OutputBytesWritten     int64
	DestinationOpens       int64
	TotalAllocDelta        uint64
	PeakHeapAllocDelta     int64 // immediate HeapAlloc snapshot after RunBlocks (secondary smoke)
	RetainedHeapAllocDelta int64 // GC bracket vs baseline HeapAlloc (primary scaled bound)
}

var (
	errMeasurePipelineSplitPerfNilResolver = errors.New(
		"testutil MeasurePipelineSplitCountingSrcMem: resolver is nil",
	)

	errMeasurePipelineSplitPerfNilFS = errors.New(
		"testutil MeasurePipelineSplitCountingSrcMem: memFs is nil",
	)

	errMeasurePipelineSplitPerfNilSourceOpener = errors.New(
		"testutil MeasurePipelineSplitCountingSrcMem: source opener is nil",
	)

	errMeasurePipelineBlocksPerfNilResolver = errors.New(
		"testutil MeasurePipelineBlocksCountingSrcMem: resolver is nil",
	)

	errMeasurePipelineBlocksPerfNilFS = errors.New(
		"testutil MeasurePipelineBlocksCountingSrcMem: memFs is nil",
	)

	errMeasurePipelineBlocksPerfNilSourceOpener = errors.New(
		"testutil MeasurePipelineBlocksCountingSrcMem: source opener is nil",
	)
)

// timedPipelineRunSplitBlocksHeapTotalAllocBracket captures HeapAlloc baseline (after one GC),
// brackets TotalAlloc around run(), and returns wall time for layered measurement helpers.
func timedPipelineRunSplitBlocksHeapTotalAllocBracket(run func()) (
	elapsed time.Duration,
	totalAllocDelta uint64,
	msHeapBefore runtime.MemStats,
) {
	runtime.GC()

	var msHeapSnap runtime.MemStats
	runtime.ReadMemStats(&msHeapSnap)

	var msTotalBefore runtime.MemStats
	runtime.ReadMemStats(&msTotalBefore)

	start := time.Now()
	run()
	elapsed = time.Since(start)

	var msTotalAfter runtime.MemStats
	runtime.ReadMemStats(&msTotalAfter)

	totalAllocDelta = msTotalAfter.TotalAlloc - msTotalBefore.TotalAlloc

	return elapsed, totalAllocDelta, msHeapSnap
}

// SplitPipelinePerfMeasurement holds portable deterministic counters plus best-effort wall time for
// split BDD performance contract steps (Layer 1 inline-source measurement shape).
//
// Naming is split-specific so assertions read clearly next to similarly shaped extract thresholds.
type SplitPipelinePerfMeasurement struct {
	SourceBytesRead          int64
	SourceReadCalls          int64
	SourceSeekCalls          int64
	SourceOpens              int64
	DestinationBytesWritten  int64
	DestinationOpens         int64
	DestinationMkdirAllCalls int64
	PlannedOutputCount       int
	Elapsed                  time.Duration
	TotalAllocDelta          uint64
	HeapAllocDelta           int64
}

// BlocksPipelinePerfMeasurement is the blocks analogue of SplitPipelinePerfMeasurement.
type BlocksPipelinePerfMeasurement struct {
	SourceBytesRead          int64
	SourceReadCalls          int64
	SourceSeekCalls          int64
	SourceOpens              int64
	DestinationBytesWritten  int64
	DestinationOpens         int64
	DestinationMkdirAllCalls int64
	PlannedOutputCount       int
	Elapsed                  time.Duration
	TotalAllocDelta          uint64
	HeapAllocDelta           int64
}

// MeasurePipelineSplitCountingSrcMem runs pipeline.RunSplit with CountingSourceOpener and destination
// tallies routed through MeasuringMemOutputOpener/MemFs, aligning BDD thresholds with bench shapes.
//
// Instrumentation captures the first RunSplit attempt only; HeapAllocDelta mirrors the extract
// contract remove-then-bracket(+restore) sequencing for multi-file destinations.
//
//nolint:gocritic // hugeParam: SplitParams grouped with injected openers/resolver mirrors BDD assembly.
func MeasurePipelineSplitCountingSrcMem(
	ctx context.Context,
	src *CountingSourceOpener,
	memFs afero.Fs,
	resolver validate.PathResolver,
	params pipeline.SplitParams,
) (SplitPipelinePerfMeasurement, pipeline.SplitPipelineResult, error) {
	switch {
	case resolver == nil:
		return SplitPipelinePerfMeasurement{}, pipeline.SplitPipelineResult{}, errMeasurePipelineSplitPerfNilResolver
	case src == nil:
		return SplitPipelinePerfMeasurement{}, pipeline.SplitPipelineResult{}, errMeasurePipelineSplitPerfNilSourceOpener
	case memFs == nil:
		return SplitPipelinePerfMeasurement{}, pipeline.SplitPipelineResult{}, errMeasurePipelineSplitPerfNilFS
	}

	out := NewMeasuringMemOutputOpener(memFs)

	var destBytes atomic.Int64
	var tempCreates atomic.Int64

	publishFS := NewCountingMemStagingPublishSession(memFs, func(path string, w io.Writer) io.Writer {
		return &measuringPublishWriter{inner: w, tallyPtr: &destBytes}
	}, &tempCreates)

	var res pipeline.SplitPipelineResult

	var err error

	elapsed, totalAllocDelta, msHeapBefore := timedPipelineRunSplitBlocksHeapTotalAllocBracket(func() {
		res, err = pipeline.RunSplit(ctx, src, out, resolver, publishFS, params)
	})

	heapDelta, heapErr := countingSplitMeasuringHeapDelta(
		&bracketSplitMeasuringHeapCtx{
			ctx:       ctx,
			src:       src,
			memFs:     memFs,
			resolver:  resolver,
			publishFS: publishFS,
			params:    params,
		},
		err,
		&msHeapBefore,
		res,
	)
	if heapErr != nil {
		return SplitPipelinePerfMeasurement{}, pipeline.SplitPipelineResult{}, heapErr
	}

	meas := SplitPipelinePerfMeasurement{
		SourceBytesRead:          src.AggregateSourceBytesRead(),
		SourceReadCalls:          src.AggregateSourceReadCalls(),
		SourceSeekCalls:          src.AggregateSourceSeekCalls(),
		SourceOpens:              src.Opens(),
		DestinationBytesWritten:  destBytes.Load(),
		DestinationOpens:         tempCreates.Load(),
		DestinationMkdirAllCalls: out.mkdirAllCalls.Load(),
		PlannedOutputCount:       len(res.Files),
		Elapsed:                  elapsed,
		TotalAllocDelta:          totalAllocDelta,
		HeapAllocDelta:           heapDelta,
	}

	return meas, res, err
}

// MeasurePipelineBlocksCountingSrcMem is the blocks analogue of MeasurePipelineSplitCountingSrcMem.
//
//nolint:gocritic // hugeParam: BlocksParams grouped with injected openers/resolver mirrors BDD assembly.
func MeasurePipelineBlocksCountingSrcMem(
	ctx context.Context,
	src *CountingSourceOpener,
	memFs afero.Fs,
	resolver validate.PathResolver,
	params pipeline.BlocksParams,
) (BlocksPipelinePerfMeasurement, pipeline.BlocksPipelineResult, error) {
	switch {
	case resolver == nil:
		return BlocksPipelinePerfMeasurement{}, pipeline.BlocksPipelineResult{}, errMeasurePipelineBlocksPerfNilResolver
	case src == nil:
		return BlocksPipelinePerfMeasurement{}, pipeline.BlocksPipelineResult{}, errMeasurePipelineBlocksPerfNilSourceOpener
	case memFs == nil:
		return BlocksPipelinePerfMeasurement{}, pipeline.BlocksPipelineResult{}, errMeasurePipelineBlocksPerfNilFS
	}

	out := NewMeasuringMemOutputOpener(memFs)

	var destBytes atomic.Int64
	var tempCreates atomic.Int64

	publishFS := NewCountingMemStagingPublishSession(memFs, func(path string, w io.Writer) io.Writer {
		return &measuringPublishWriter{inner: w, tallyPtr: &destBytes}
	}, &tempCreates)

	var res pipeline.BlocksPipelineResult

	var err error

	elapsed, totalAllocDelta, msHeapBefore := timedPipelineRunSplitBlocksHeapTotalAllocBracket(func() {
		res, err = pipeline.RunBlocks(ctx, src, out, resolver, publishFS, params)
	})

	heapDelta, heapErr := countingBlocksMeasuringHeapDelta(
		&bracketBlocksMeasuringHeapCtx{
			ctx:       ctx,
			src:       src,
			memFs:     memFs,
			resolver:  resolver,
			publishFS: publishFS,
			params:    params,
		},
		err,
		&msHeapBefore,
		res,
	)
	if heapErr != nil {
		return BlocksPipelinePerfMeasurement{}, pipeline.BlocksPipelineResult{}, heapErr
	}

	meas := BlocksPipelinePerfMeasurement{
		SourceBytesRead:          src.AggregateSourceBytesRead(),
		SourceReadCalls:          src.AggregateSourceReadCalls(),
		SourceSeekCalls:          src.AggregateSourceSeekCalls(),
		SourceOpens:              src.Opens(),
		DestinationBytesWritten:  destBytes.Load(),
		DestinationOpens:         tempCreates.Load(),
		DestinationMkdirAllCalls: out.mkdirAllCalls.Load(),
		PlannedOutputCount:       len(res.Files),
		Elapsed:                  elapsed,
		TotalAllocDelta:          totalAllocDelta,
		HeapAllocDelta:           heapDelta,
	}

	return meas, res, err
}
