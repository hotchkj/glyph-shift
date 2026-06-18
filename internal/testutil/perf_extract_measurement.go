package testutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

// errChainWrap is the standard fmt.Errorf pattern for wrapping two errors (%w: %w).
const errChainWrap = "%w: %w"

// heapAllocDeltaSigned returns after-before for runtime.MemStats.HeapAlloc as int64 without
// casting each absolute uint64 to int64 (gosec G115).
func heapAllocDeltaSigned(before, after uint64) int64 {
	const maxInt64 = uint64(math.MaxInt64)

	if after >= before {
		diff := after - before
		if diff > maxInt64 {
			return math.MaxInt64
		}

		return int64(diff)
	}

	diff := before - after
	if diff > maxInt64 {
		return math.MinInt64
	}

	return -int64(diff)
}

// ExtractMeasurement captures deterministic portable extract counters plus incidental wall time.
type ExtractMeasurement struct {
	SourceBytesRead          int64
	SourceReadCalls          int64
	SourceSeekCalls          int64
	SourceOpens              int64
	DestinationBytesWritten  int64
	DestinationOpens         int64
	DestinationMkdirAllCalls int64
	OutputBytes              []byte
	LinesExtracted           int
	Elapsed                  time.Duration

	// TotalAllocDelta is runtime.MemStats.TotalAlloc delta bracketing pipeline.RunExtract
	// (aggregate process allocations; best-effort, not deterministic under load).
	TotalAllocDelta uint64
	// HeapAllocDelta is a best-effort retained-heap surrogate for BDD thresholds (signed,
	// saturating arithmetic; avoids casting absolute uint64 HeapAlloc values to int64).
	// MeasurePipelineExtractCountingSrcMem excludes successful non-append destination payload
	// in the afero mem map during the GC/ReadMemStats bracket (see that function). Append
	// mode is narrower: an exact post-first-run snapshot may stay live across that bracket
	// because restoring via a second RunExtract on an empty file is not equivalent to append.
	// MeasureFileopsExtract and MeasurePipelineExtract bracket HeapAlloc alongside
	// TotalAllocDelta for their respective calls.
	HeapAllocDelta int64
}

// MeasureFileopsExtract runs fileops.Extract with CountingReader/CountingWriter instrumentation.
func MeasureFileopsExtract(
	ctx context.Context,
	source []byte,
	lines fileops.LineRange,
	appendMode bool,
) (ExtractMeasurement, fileops.ExtractResult, error) {
	counters := &sourceCounters{}

	src := NewCountingReader(source, counters)

	dest := &CountingWriter{}

	var ms0, ms1 runtime.MemStats
	runtime.ReadMemStats(&ms0)

	start := time.Now()

	res, err := fileops.Extract(ctx, fileops.ExtractOptions{
		Source: src,
		Lines:  lines,
		Append: appendMode,
	}, dest)

	elapsed := time.Since(start)

	runtime.ReadMemStats(&ms1)

	meas := ExtractMeasurement{
		SourceBytesRead:          counters.bytesRead.Load(),
		SourceReadCalls:          counters.readCalls.Load(),
		SourceSeekCalls:          counters.seekCalls.Load(),
		SourceOpens:              0,
		DestinationBytesWritten:  dest.BytesWritten(),
		DestinationOpens:         0,
		DestinationMkdirAllCalls: 0,
		OutputBytes:              bytes.Clone(dest.Bytes()),
		LinesExtracted:           res.LinesExtracted,
		Elapsed:                  elapsed,
		TotalAllocDelta:          ms1.TotalAlloc - ms0.TotalAlloc,
		HeapAllocDelta:           heapAllocDeltaSigned(ms0.HeapAlloc, ms1.HeapAlloc),
	}

	return meas, res, err
}

// MeasuringMemOutputOpener forwards to MemOutputOpener on a shared filesystem while tallying
// destination counters for ExtractMeasurement without losing MemOutput semantics (existence,
// append seeding, truncation).
type MeasuringMemOutputOpener struct {
	mem              *MemOutputOpener
	mkdirAllCalls    atomic.Int64
	destinationOpens atomic.Int64
	bytesWritten     atomic.Int64
}

// NewMeasuringMemOutputOpener returns a pipeline.OutputOpener that writes through MemOutputOpener.
func NewMeasuringMemOutputOpener(memFs afero.Fs) *MeasuringMemOutputOpener {
	return &MeasuringMemOutputOpener{mem: NewMemOutputOpenerWithFS(memFs)}
}

// MkdirAll delegates and records mkdir calls into the tally.
func (o *MeasuringMemOutputOpener) MkdirAll(path string, perm fs.FileMode) error {
	o.mkdirAllCalls.Add(1)

	return o.mem.MkdirAll(path, perm)
}

type measuringAppendWriteCloser struct {
	delegate io.WriteCloser
	bytes    *atomic.Int64
}

func (w *measuringAppendWriteCloser) Write(payload []byte) (int, error) {
	writtenCount, writeErr := w.delegate.Write(payload)

	if writeErr == nil && writtenCount > 0 {
		w.bytes.Add(int64(writtenCount))
	}

	return writtenCount, writeErr
}

func (w *measuringAppendWriteCloser) Close() error {
	return w.delegate.Close()
}

// OpenFile delegates to MemOutputOpener and counts payload bytes appended through Write calls.
func (o *MeasuringMemOutputOpener) OpenFile(
	path string,
	intent pipeline.OutputWriteIntent,
	perm fs.FileMode,
) (io.WriteCloser, error) {
	o.destinationOpens.Add(1)

	inner, err := o.mem.OpenFile(path, intent, perm)
	if err != nil {
		return nil, err //nolint:wrapcheck // pipeline maps open errors distinctly
	}

	return &measuringAppendWriteCloser{delegate: inner, bytes: &o.bytesWritten}, nil
}

func readDestinationSnapshot(memFs afero.Fs, preview bool, destPath string) []byte {
	if preview {
		return nil
	}

	data, err := afero.ReadFile(memFs, destPath)
	if err != nil {
		return nil
	}

	return bytes.Clone(data)
}

func heapAllocDeltaBracketAfterGC(msHeapBefore *runtime.MemStats) int64 {
	runtime.GC()

	var msHeapAfter runtime.MemStats
	runtime.ReadMemStats(&msHeapAfter)

	return heapAllocDeltaSigned(msHeapBefore.HeapAlloc, msHeapAfter.HeapAlloc)
}

// readAppendDestSnapshotForHeapRestore clones the on-disk destination after a successful
// append-mode extract so it can be written back after HeapAllocDelta measurement. The snapshot
// intentionally may remain allocated during runtime.GC for the append path only.
func readAppendDestSnapshotForHeapRestore(memFs afero.Fs, destPath string) ([]byte, error) {
	raw, readErr := afero.ReadFile(memFs, destPath)
	if readErr != nil {
		return nil, fmt.Errorf(errChainWrap, errMeasureCountingSrcReadAppendDestSnapshot, readErr)
	}

	return bytes.Clone(raw), nil
}

// countingSrcMemHeapMeasure carries pipeline dependencies for heap delta bracketing and
// destination restore (second RunExtract uses fresh openers; see countingSrcMemHeapDelta).
type countingSrcMemHeapMeasure struct {
	ctx      context.Context
	src      *CountingSourceOpener
	memFs    afero.Fs
	resolver validate.PathResolver
	params   pipeline.ExtractParams
}

func countingSrcMemHeapDelta(
	bracket *countingSrcMemHeapMeasure,
	runErr error,
	msHeapBefore *runtime.MemStats,
	appendDestSnapshot []byte,
) (heapAllocDelta int64, err error) {
	params := bracket.params

	switch {
	case runErr == nil && !params.Preview && params.DestPath != "":
		remErr := bracket.memFs.Remove(params.DestPath)
		if remErr == nil {
			heapAllocDelta = heapAllocDeltaBracketAfterGC(msHeapBefore)
			switch {
			case params.Append:
				if werr := afero.WriteFile(bracket.memFs, params.DestPath, appendDestSnapshot, defaultMemFilePerm); werr != nil {
					return 0, fmt.Errorf(errChainWrap, errMeasureCountingSrcRestoreDestination, werr)
				}
			default:
				restoreSrc := &CountingSourceOpener{
					Immutable:   bracket.src.Immutable,
					AllowedPath: bracket.src.AllowedPath,
				}
				restoreOut := NewMeasuringMemOutputOpener(bracket.memFs)
				restorePublish := NewMemFileSession()
				restorePublish.SetFs(bracket.memFs)
				if _, restoreErr := pipeline.RunExtract(
					bracket.ctx, restoreSrc, restoreOut, bracket.resolver, restorePublish, params,
				); restoreErr != nil {
					return 0, fmt.Errorf(errChainWrap, errMeasureCountingSrcRestoreDestination, restoreErr)
				}
			}

			return heapAllocDelta, nil
		}

		return 0, fmt.Errorf(errChainWrap, errMeasureCountingSrcRemoveDestination, remErr)
	default:
		return heapAllocDeltaBracketAfterGC(msHeapBefore), nil
	}
}

//nolint:gocritic // hugeParam: mirrors pipeline.ExtractParams bundle.
func countingSrcMemAdjustPublishInstrumentation(
	memFs afero.Fs,
	runErr error,
	params pipeline.ExtractParams,
	destOpensCount int64,
	destBytesWritten int64,
) (opens, bytesWritten int64) {
	if runErr == nil && !params.Preview && params.DestPath != "" {
		published, readErr := afero.ReadFile(memFs, params.DestPath)
		if readErr == nil {
			return 1, int64(len(published))
		}

		return destOpensCount, destBytesWritten
	}

	if runErr != nil && errors.Is(runErr, pipeline.ErrDestinationExists) {
		return 1, destBytesWritten
	}

	return destOpensCount, destBytesWritten
}

//nolint:gocritic // hugeParam: mirrors pipeline.ExtractParams bundle.
func measurePipelineExtractPublishInstrumentation(
	runErr error,
	params pipeline.ExtractParams,
	publishFS fileops.FileSession,
	out *CountingOutputOpener,
) (outputBytes []byte, destOpens, destBytes int64) {
	outputBytes = bytes.Clone(out.OutputBytesSnapshot(params.DestPath))
	destBytes = out.BytesWritten()
	destOpens = out.DestinationOpens()

	if runErr == nil && !params.Preview && params.DestPath != "" {
		if mf, ok := publishFS.(*MemTestSession); ok {
			published, readErr := afero.ReadFile(mf.Fs, params.DestPath)
			if readErr == nil {
				outputBytes = bytes.Clone(published)
				destOpens = 1
				destBytes = int64(len(published))
			}
		}
	}

	if runErr != nil && errors.Is(runErr, pipeline.ErrDestinationExists) {
		destOpens = 1
	}

	return outputBytes, destOpens, destBytes
}

// MeasurePipelineExtractCountingSrcMem is the contract-level pipeline extract measurement helper.
//
// Gherkin direct extract steps and internal/pipeline extract benchmarks call this helper so
// performance thresholds observe the same pipeline shape: counting source instrumentation,
// MemOutputOpener-backed destination writes on a shared afero.Fs (existence, EXCL/TRUNC/APPEND),
// and runtime memory stats. TotalAllocDelta and Elapsed bracket the first pipeline.RunExtract only.
//
// HeapAllocDelta compares GC-stabilized HeapAlloc before that run vs after removing the written
// destination from memFs so afero destination storage is not retained during the bracket. For
// successful non-preview, non-append runs with DestPath, no destination-sized byte clone from the
// helper is held across that GC/ReadMemStats pair; the file is restored with a second
// pipeline.RunExtract using a fresh CountingSourceOpener (same Immutable and AllowedPath as the
// measured opener) and a fresh MeasuringMemOutputOpener, which does not mutate the first-run
// instrumentation already captured on the original opener. Append mode instead snapshots
// post-first-run destination bytes before removal and restores with WriteFile after measurement,
// because re-running append against an empty destination would drop the prior prefix; that
// snapshot may remain live during the HeapAllocDelta bracket (append is outside bounded-memory
// perf scenarios).
//
// Instrumentation fields reflect the first RunExtract only. The returned ExtractMeasurement is
// what BDD perf assertions store on the test context.
//
//nolint:funlen,gocritic // Measurement orchestrates MemStats brackets, heap restore, and pipeline.RunExtract.
func MeasurePipelineExtractCountingSrcMem(
	ctx context.Context,
	src *CountingSourceOpener,
	memFs afero.Fs,
	resolver validate.PathResolver,
	params pipeline.ExtractParams,
) (ExtractMeasurement, fileops.ExtractResult, error) {
	if err := validateCountingSrcMemExtractDeps(src, memFs, resolver); err != nil {
		return ExtractMeasurement{}, fileops.ExtractResult{}, err
	}

	out := NewMeasuringMemOutputOpener(memFs)

	publishSess := NewMemFileSession()
	publishSess.SetFs(memFs)

	runtime.GC()

	var msHeapBefore runtime.MemStats
	runtime.ReadMemStats(&msHeapBefore)

	var msTotalBefore runtime.MemStats
	runtime.ReadMemStats(&msTotalBefore)

	start := time.Now()

	res, err := pipeline.RunExtract(ctx, src, out, resolver, publishSess, params)

	elapsed := time.Since(start)

	var msTotalAfter runtime.MemStats
	runtime.ReadMemStats(&msTotalAfter)

	totalAllocDelta := msTotalAfter.TotalAlloc - msTotalBefore.TotalAlloc

	srcBytesRead := src.AggregateSourceBytesRead()
	srcReadCalls := src.AggregateSourceReadCalls()
	srcSeekCalls := src.AggregateSourceSeekCalls()
	srcOpensCount := src.Opens()
	destBytesWritten := out.bytesWritten.Load()
	destOpensCount := out.destinationOpens.Load()
	destMkdirCalls := out.mkdirAllCalls.Load()

	destOpensCount, destBytesWritten = countingSrcMemAdjustPublishInstrumentation(
		memFs, err, params, destOpensCount, destBytesWritten,
	)

	appendDestSnapshot, snapshotErr := appendDestSnapshotForHeapRestore(memFs, err, &params)
	if snapshotErr != nil {
		return ExtractMeasurement{}, fileops.ExtractResult{}, snapshotErr
	}

	heapAllocDelta, heapErr := countingSrcMemHeapDelta(
		&countingSrcMemHeapMeasure{ctx: ctx, src: src, memFs: memFs, resolver: resolver, params: params},
		err,
		&msHeapBefore,
		appendDestSnapshot,
	)
	if heapErr != nil {
		return ExtractMeasurement{}, fileops.ExtractResult{}, heapErr
	}

	outputBytes := readDestinationSnapshot(memFs, params.Preview, params.DestPath)

	meas := ExtractMeasurement{
		SourceBytesRead:          srcBytesRead,
		SourceReadCalls:          srcReadCalls,
		SourceSeekCalls:          srcSeekCalls,
		SourceOpens:              srcOpensCount,
		DestinationBytesWritten:  destBytesWritten,
		DestinationOpens:         destOpensCount,
		DestinationMkdirAllCalls: destMkdirCalls,
		OutputBytes:              outputBytes,
		LinesExtracted:           res.LinesExtracted,
		Elapsed:                  elapsed,
		TotalAllocDelta:          totalAllocDelta,
		HeapAllocDelta:           heapAllocDelta,
	}

	return meas, res, err
}

func validateCountingSrcMemExtractDeps(
	src *CountingSourceOpener,
	memFs afero.Fs,
	resolver validate.PathResolver,
) error {
	if resolver == nil {
		return fmt.Errorf("%w", errMeasureCountingSrcNilResolver)
	}

	if src == nil || memFs == nil {
		return fmt.Errorf("%w", errMeasureCountingSrcNilDeps)
	}

	return nil
}

func appendDestSnapshotForHeapRestore(
	memFs afero.Fs,
	runErr error,
	params *pipeline.ExtractParams,
) ([]byte, error) {
	if runErr != nil || params.Preview || params.DestPath == "" || !params.Append {
		return nil, nil
	}

	return readAppendDestSnapshotForHeapRestore(memFs, params.DestPath)
}

// MeasurePipelineExtract is a lower-level helper for focused testutil coverage: it runs
// pipeline.RunExtract with an explicit CountingSourceOpener and CountingOutputOpener pair.
//
// It is not the pipeline performance contract path. Prefer MeasurePipelineExtractCountingSrcMem
// with a shared mem Fs and NewMemPathResolverWithFS when matching BDD or benchmark measurement.
//
// Resolver must be non-nil (use NewSyntheticAbsentPathResolver for the common purely logical
// output-opener pattern that does not consult the filesystem).
//
// publishFS must match the logical destination backing store when atomic publish writes through
// MemFileSession (typically publishFS.Fs shared with an empty seed memFs used only for snapshots).
//
//nolint:gocritic // hugeParam: ExtractParams mirrors the CLI invocation bundle.
func MeasurePipelineExtract(
	ctx context.Context,
	src *CountingSourceOpener,
	out *CountingOutputOpener,
	publishFS fileops.FileSession,
	resolver validate.PathResolver,
	params pipeline.ExtractParams,
) (ExtractMeasurement, fileops.ExtractResult, error) {
	if resolver == nil {
		return ExtractMeasurement{}, fileops.ExtractResult{}, fmt.Errorf("%w", errMeasurePipelineNilResolver)
	}

	if src == nil || out == nil {
		return ExtractMeasurement{}, fileops.ExtractResult{}, fmt.Errorf("%w", errMeasurePipelineNilOpeners)
	}

	if publishFS == nil {
		return ExtractMeasurement{}, fileops.ExtractResult{}, fileops.ErrNilFileSession
	}

	var ms0, ms1 runtime.MemStats
	runtime.ReadMemStats(&ms0)

	start := time.Now()

	res, err := pipeline.RunExtract(ctx, src, out, resolver, publishFS, params)

	elapsed := time.Since(start)

	runtime.ReadMemStats(&ms1)

	outputBytes, destOpensCount, destBytesWritten := measurePipelineExtractPublishInstrumentation(
		err, params, publishFS, out,
	)

	meas := ExtractMeasurement{
		SourceBytesRead:          src.AggregateSourceBytesRead(),
		SourceReadCalls:          src.AggregateSourceReadCalls(),
		SourceSeekCalls:          src.AggregateSourceSeekCalls(),
		SourceOpens:              src.Opens(),
		DestinationBytesWritten:  destBytesWritten,
		DestinationOpens:         destOpensCount,
		DestinationMkdirAllCalls: out.MkdirAllCalls(),
		OutputBytes:              outputBytes,
		LinesExtracted:           res.LinesExtracted,
		Elapsed:                  elapsed,
		TotalAllocDelta:          ms1.TotalAlloc - ms0.TotalAlloc,
		HeapAllocDelta:           heapAllocDeltaSigned(ms0.HeapAlloc, ms1.HeapAlloc),
	}

	return meas, res, err
}
