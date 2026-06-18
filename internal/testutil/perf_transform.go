// Package testutil provides transform pipeline performance and boundedness measurement helpers.
package testutil

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"runtime"
	"sync/atomic"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

var (
	errMeasurePipelineTransformNilDeps = errors.New(
		"testutil MeasurePipelineTransformPeakHeap: nil stater, resolver, or session",
	)
	errMeasurePipelineTransformCountingNilDeps = errors.New(
		"testutil MeasurePipelineTransformCountingSrcMem: nil stater, resolver, or session",
	)
)

// ErrCountingTransformMemSessionNilMem is returned when a CountingTransformMemSession has no inner memory session.
var ErrCountingTransformMemSessionNilMem = errors.New("testutil CountingTransformMemSession: nil mem file session")

// CountingTransformMemSession wraps MemTestSession and counts bytes materialized in OpenRDWR
// (mirrors extract/split counting seams for source-side IO visibility in tests).
type CountingTransformMemSession struct {
	Mem *MemTestSession

	SourceBytesMaterialized atomic.Int64
}

var _ fileops.FileSession = (*CountingTransformMemSession)(nil)

// OpenRead delegates to the inner session.
func (c *CountingTransformMemSession) OpenRead(path string) (fileops.SessionReadHandle, error) {
	if c.Mem == nil {
		return nil, fmt.Errorf("memfs open read: %w", ErrCountingTransformMemSessionNilMem)
	}

	return c.Mem.OpenRead(path)
}

// OpenRDWR materializes logical content like MemFileSession while counting source bytes read
// from the in-memory filesystem snapshot.
func (c *CountingTransformMemSession) OpenRDWR(path string) (fileops.SessionRDWRHandle, error) {
	if c.Mem == nil {
		return nil, fmt.Errorf("memfs open rdwr: %w", ErrCountingTransformMemSessionNilMem)
	}

	content, err := c.Mem.Backend().readLogicalBytes(path)
	if err != nil {
		return nil, fmt.Errorf("memfs open rdwr %q: %w", path, err)
	}

	c.SourceBytesMaterialized.Add(int64(len(content)))

	return c.Mem.Backend().newRDWRHandle(path, content), nil
}

// CreateTemp delegates to the inner session.
func (c *CountingTransformMemSession) CreateTemp(dir, pattern string) (fileops.SessionTempHandle, error) {
	if c.Mem == nil {
		return nil, fmt.Errorf("memfs create temp: %w", ErrCountingTransformMemSessionNilMem)
	}

	return c.Mem.CreateTemp(dir, pattern)
}

// Remove delegates to the inner session.
func (c *CountingTransformMemSession) Remove(name string) error {
	if c.Mem == nil {
		return fmt.Errorf("memfs remove: %w", ErrCountingTransformMemSessionNilMem)
	}

	return c.Mem.Remove(name)
}

// Rename delegates to the inner session.
func (c *CountingTransformMemSession) Rename(oldpath, newpath string) error {
	if c.Mem == nil {
		return fmt.Errorf("memfs rename: %w", ErrCountingTransformMemSessionNilMem)
	}

	return c.Mem.Rename(oldpath, newpath)
}

// Chmod delegates to the inner session.
func (c *CountingTransformMemSession) Chmod(name string, mode fs.FileMode) error {
	if c.Mem == nil {
		return fmt.Errorf("memfs chmod: %w", ErrCountingTransformMemSessionNilMem)
	}

	return c.Mem.Chmod(name, mode)
}

// TransformPipelinePerfMeasurement captures portable counters for transform boundedness BDDs.
type TransformPipelinePerfMeasurement struct {
	SourceBytesMaterialized int64
	TotalAllocDelta         uint64
	PeakHeapAllocDelta      int64
	RetainedHeapAllocDelta  int64
	PreviewTempCreates      int64
	ApplyWritebackBytes     int64
}

type countingCreateTempTransformSession struct {
	inner       fileops.FileSession
	tempCreates *atomic.Int64
}

func (c *countingCreateTempTransformSession) OpenRead(path string) (fileops.SessionReadHandle, error) {
	return c.inner.OpenRead(path)
}

func (c *countingCreateTempTransformSession) OpenRDWR(path string) (fileops.SessionRDWRHandle, error) {
	return c.inner.OpenRDWR(path)
}

func (c *countingCreateTempTransformSession) CreateTemp(dir, pattern string) (fileops.SessionTempHandle, error) {
	tmpFile, err := c.inner.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}

	if c.tempCreates != nil {
		c.tempCreates.Add(1)
	}

	return tmpFile, nil
}

func (c *countingCreateTempTransformSession) Remove(name string) error {
	return c.inner.Remove(name)
}

func (c *countingCreateTempTransformSession) Rename(oldpath, newpath string) error {
	return c.inner.Rename(oldpath, newpath)
}

func (c *countingCreateTempTransformSession) Chmod(name string, mode fs.FileMode) error {
	return c.inner.Chmod(name, mode)
}

var _ fileops.FileSession = (*countingCreateTempTransformSession)(nil)

// MeasurePipelineTransformPeakHeap brackets pipeline.RunTransform with runtime.MemStats like split/blocks peaks.
func MeasurePipelineTransformPeakHeap(
	ctx context.Context,
	st pipeline.FileStater,
	resolver validate.PathResolver,
	session fileops.FileSession,
	params pipeline.TransformParams,
) (TransformPipelinePerfMeasurement, pipeline.TransformPipelineResult, error) {
	if st == nil || resolver == nil || session == nil {
		return TransformPipelinePerfMeasurement{}, pipeline.TransformPipelineResult{}, errMeasurePipelineTransformNilDeps
	}

	runtime.GC()
	runtime.GC()

	var msIn runtime.MemStats
	runtime.ReadMemStats(&msIn)

	var msTotalBefore runtime.MemStats
	runtime.ReadMemStats(&msTotalBefore)

	res, err := pipeline.RunTransform(ctx, st, resolver, session, params)

	var msPeak runtime.MemStats
	runtime.ReadMemStats(&msPeak)

	totalAllocDelta := msPeak.TotalAlloc - msTotalBefore.TotalAlloc

	meas := TransformPipelinePerfMeasurement{
		TotalAllocDelta:        totalAllocDelta,
		PeakHeapAllocDelta:     heapAllocDeltaSigned(msIn.HeapAlloc, msPeak.HeapAlloc),
		RetainedHeapAllocDelta: heapAllocDeltaBracketAfterGC(&msIn),
	}

	return meas, res, err
}

// MeasurePipelineTransformCountingSrcMem records bytes materialized by CountingTransformMemSession during RunTransform.
func MeasurePipelineTransformCountingSrcMem(
	ctx context.Context,
	st pipeline.FileStater,
	resolver validate.PathResolver,
	session *CountingTransformMemSession,
	params pipeline.TransformParams,
) (TransformPipelinePerfMeasurement, pipeline.TransformPipelineResult, error) {
	if st == nil || resolver == nil || session == nil {
		return TransformPipelinePerfMeasurement{},
			pipeline.TransformPipelineResult{},
			errMeasurePipelineTransformCountingNilDeps
	}
	if session.Mem == nil {
		return TransformPipelinePerfMeasurement{},
			pipeline.TransformPipelineResult{},
			ErrCountingTransformMemSessionNilMem
	}

	meas, res, err := MeasurePipelineTransformPeakHeap(ctx, st, resolver, session, params)
	meas.SourceBytesMaterialized = session.SourceBytesMaterialized.Load()

	return meas, res, err
}

// MeasurePipelineTransformRecordsPreviewWithoutWrites asserts preview paths avoid writeback temp creation
// by requiring zero transform applies (caller supplies already-preview params).
func MeasurePipelineTransformRecordsPreviewWithoutWrites(
	ctx context.Context,
	st pipeline.FileStater,
	resolver validate.PathResolver,
	inner fileops.FileSession,
	params pipeline.TransformParams,
) (TransformPipelinePerfMeasurement, pipeline.TransformPipelineResult, error) {
	if st == nil || resolver == nil || inner == nil {
		return TransformPipelinePerfMeasurement{}, pipeline.TransformPipelineResult{},
			errMeasurePipelineTransformCountingNilDeps
	}

	if params.Yes {
		return TransformPipelinePerfMeasurement{}, pipeline.TransformPipelineResult{},
			fmt.Errorf("%w: preview measurement requires params.Yes false", errMeasurePipelineTransformCountingNilDeps)
	}

	var creates atomic.Int64

	wrapped := &countingCreateTempTransformSession{inner: inner, tempCreates: &creates}

	meas, res, err := MeasurePipelineTransformPeakHeap(ctx, st, resolver, wrapped, params)
	meas.PreviewTempCreates = creates.Load()

	return meas, res, err
}

// MeasurePipelineTransformApplyCountsWritebackBytes runs an apply transform and returns measured writeback
// as the post-transform logical file size on memFs (exclusive rename target semantics).
func MeasurePipelineTransformApplyCountsWritebackBytes(
	ctx context.Context,
	st pipeline.FileStater,
	resolver validate.PathResolver,
	session fileops.FileSession,
	params pipeline.TransformParams,
	memFs afero.Fs,
	logicalSrcPath string,
) (TransformPipelinePerfMeasurement, pipeline.TransformPipelineResult, error) {
	if st == nil || resolver == nil || session == nil || memFs == nil {
		return TransformPipelinePerfMeasurement{}, pipeline.TransformPipelineResult{},
			errMeasurePipelineTransformCountingNilDeps
	}

	if !params.Yes {
		return TransformPipelinePerfMeasurement{}, pipeline.TransformPipelineResult{},
			fmt.Errorf("%w: apply measurement requires params.Yes true", errMeasurePipelineTransformCountingNilDeps)
	}

	meas, res, err := MeasurePipelineTransformPeakHeap(ctx, st, resolver, session, params)
	if err != nil {
		return meas, res, err
	}

	fi, statErr := memFs.Stat(logicalSrcPath)
	if statErr != nil {
		return meas, res, fmt.Errorf("writeback measure stat: %w", statErr)
	}

	meas.ApplyWritebackBytes = fi.Size()

	return meas, res, nil
}
