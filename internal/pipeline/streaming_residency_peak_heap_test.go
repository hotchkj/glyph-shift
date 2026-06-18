package pipeline_test

import (
	"context"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
)

func streamingResidencySplitParams(
	root, srcLeaf, outLeaf string,
) (srcPath, outDir string, params pipeline.SplitParams) {
	srcPath = filepath.Join(root, srcLeaf)
	outDir = filepath.Join(root, outLeaf)
	delimiter := regexp.MustCompile(`^---$`)

	params = pipeline.SplitParams{
		SrcPath:   srcPath,
		OutDir:    outDir,
		Root:      root,
		Delimiter: delimiter,
		Naming:    fileops.Sequential,
		Extension: ".txt",
		Mkdir:     true,
	}

	return srcPath, outDir, params
}

func mustMeasureRunSplitPeakHeap(
	t *testing.T,
	ctx context.Context,
	params *pipeline.SplitParams,
	srcBytes []byte,
	srcPathForOpen string,
) (testutil.SplitLargeOutputMemMeasurement, pipeline.SplitPipelineResult) {
	t.Helper()

	meas, pres, runErr := testutil.MeasureRunSplitPeakHeap(
		ctx,
		&testutil.CountingSourceOpener{
			Immutable:   srcBytes,
			AllowedPath: filepath.Clean(srcPathForOpen),
		},
		testutil.NewNonRetainingOutputOpener(),
		testutil.NoSymlinkPathResolver{},
		*params,
	)
	if runErr != nil {
		t.Fatalf("RunSplit measurement: %v", runErr)
	}

	return meas, pres
}

func requireOneOutputBasenameSlice(t *testing.T, phase string, files []string) {
	t.Helper()

	if len(files) != 1 {
		t.Fatalf("%s: want exactly 1 output file, got %#v", phase, files)
	}
}

func requireSplitDestinationAndReadSanity(
	t *testing.T,
	phase string,
	meas testutil.SplitLargeOutputMemMeasurement,
	lineCount, lineLen int,
) {
	t.Helper()

	minLogical := int64(lineCount * (lineLen + 1))
	if meas.SourceBytesRead < minLogical/4 {
		t.Fatalf(
			"sanity (%s): expected substantial source reads (>= ~%d), got %d",
			phase,
			minLogical/4,
			meas.SourceBytesRead,
		)
	}

	if meas.OutputBytesWritten < minLogical/4 {
		t.Fatalf(
			"sanity (%s): expected substantial destination writes (>= ~%d logical body bytes), got %d",
			phase,
			minLogical/4,
			meas.OutputBytesWritten,
		)
	}

	if meas.DestinationOpens != 1 {
		t.Fatalf("sanity (%s): expected one destination open, got %d", phase, meas.DestinationOpens)
	}
}

func requireRetainedHeapBudgetSplit(
	t *testing.T,
	largeMeas, smallMeas testutil.SplitLargeOutputMemMeasurement,
) {
	t.Helper()

	budget := testutil.StreamingBodyResidencyRetainedHeapBudget(smallMeas.RetainedHeapAllocDelta)

	if largeMeas.RetainedHeapAllocDelta > budget {
		t.Fatalf(
			"boundedness (primary, retained GC-stabilized heap): large retained heap %d exceeds "+
				"small-vs-large budget %d "+
				"(small retained baseline %d; ratio cap %d; fixed noise allowance %d)",
			largeMeas.RetainedHeapAllocDelta,
			budget,
			smallMeas.RetainedHeapAllocDelta,
			testutil.StreamingBodyRetainedHeapMaxLargeToSmallRatio,
			testutil.StreamingBodyResidencyRetainedHeapNoiseAllowance,
		)
	}
}

func TestRunSplitStreamsLargeSectionWithoutFullResidency(t *testing.T) {
	t.Skip("MemStats residency gate flaky on CI; see issue #2")

	ctx := context.Background()
	root := testRoot()

	smallLines := testutil.StreamingResidencyProbeSmallLineCount
	smallLineLen := testutil.StreamingResidencyProbeSmallLineLength
	largeLines := testutil.LargeStreamingFixtureLineCount
	largeLineLen := testutil.LargeStreamingFixtureLineLength
	delim := []byte("---\n")

	srcPath, _, splitParams := streamingResidencySplitParams(root, "large-split-in.txt", "large-split-out")

	smallSrc := testutil.BuildLargeSplitSingleSectionSource(smallLines, smallLineLen, delim)
	smallMeas, smallPres := mustMeasureRunSplitPeakHeap(t, ctx, &splitParams, smallSrc, srcPath)
	requireOneOutputBasenameSlice(t, "small", smallPres.Files)
	requireSplitDestinationAndReadSanity(t, "small", smallMeas, smallLines, smallLineLen)

	largeSrc := testutil.BuildLargeSplitSingleSectionSource(largeLines, largeLineLen, delim)
	largeMeas, largePres := mustMeasureRunSplitPeakHeap(t, ctx, &splitParams, largeSrc, srcPath)
	requireOneOutputBasenameSlice(t, "large", largePres.Files)
	requireSplitDestinationAndReadSanity(t, "large", largeMeas, largeLines, largeLineLen)

	requireRetainedHeapBudgetSplit(t, largeMeas, smallMeas)

	minLogicalLarge := int64(largeLines * (largeLineLen + 1))
	if largeMeas.PeakHeapAllocDelta > testutil.MaxPeakHeapGrowthForStreamingBody {
		t.Fatalf(
			"boundedness (secondary smoke, immediate peak): peak HeapAlloc delta %d exceeds streaming "+
				"budget %d "+
				"for ~%d byte single-section body (hypothesis: full body/metadata residency in heap; "+
				"destination bytes are not retained by the test opener)",
			largeMeas.PeakHeapAllocDelta,
			testutil.MaxPeakHeapGrowthForStreamingBody,
			minLogicalLarge,
		)
	}
}

func streamingResidencyBlocksParams(root string) (srcPath, outDir string, params pipeline.BlocksParams) {
	srcPath = filepath.Join(root, "large-blocks-in.txt")
	outDir = filepath.Join(root, "large-blocks-out")

	params = pipeline.BlocksParams{
		SrcPath:        srcPath,
		OutDir:         outDir,
		Root:           root,
		StartDelimiter: regexp.MustCompile(`^<<BEGIN>>$`),
		EndDelimiter:   regexp.MustCompile(`^<<END>>$`),
		Naming:         fileops.Sequential,
		Extension:      ".txt",
		Mkdir:          true,
	}

	return srcPath, outDir, params
}

// blocksResidencyPeakFixture groups newline-delimited block markers and repeating body sizing for paired
// RunBlocks residency heap measurements (_test-only).
type blocksResidencyPeakFixture struct {
	header, beginMarker, endMarker []byte
	lineCount, lineLen             int
}

func mustMeasureRunBlocksPeakHeap(
	t *testing.T,
	ctx context.Context,
	params *pipeline.BlocksParams,
	fix *blocksResidencyPeakFixture,
	srcPathForOpen string,
) (testutil.BlocksLargeOutputMemMeasurement, pipeline.BlocksPipelineResult) {
	t.Helper()

	srcBytes := testutil.BuildLargeBlocksSingleBodySource(
		fix.header, fix.beginMarker, fix.endMarker, fix.lineCount, fix.lineLen,
	)

	meas, pres, runErr := testutil.MeasureRunBlocksPeakHeap(
		ctx,
		&testutil.CountingSourceOpener{
			Immutable:   srcBytes,
			AllowedPath: filepath.Clean(srcPathForOpen),
		},
		testutil.NewNonRetainingOutputOpener(),
		testutil.NoSymlinkPathResolver{},
		*params,
	)
	if runErr != nil {
		t.Fatalf("RunBlocks measurement: %v", runErr)
	}

	return meas, pres
}

func requireBlocksDestinationAndReadSanity(
	t *testing.T,
	phase string,
	meas testutil.BlocksLargeOutputMemMeasurement,
	lineCount, lineLen int,
) {
	t.Helper()

	minLogical := int64(lineCount * (lineLen + 1))
	if meas.SourceBytesRead < minLogical/4 {
		t.Fatalf(
			"sanity (%s): expected substantial source reads (>= ~%d), got %d",
			phase,
			minLogical/4,
			meas.SourceBytesRead,
		)
	}

	if meas.OutputBytesWritten < minLogical/4 {
		t.Fatalf(
			"sanity (%s): expected substantial destination writes (>= ~%d logical body bytes), got %d",
			phase,
			minLogical/4,
			meas.OutputBytesWritten,
		)
	}

	if meas.DestinationOpens != 1 {
		t.Fatalf("sanity (%s): expected one destination open, got %d", phase, meas.DestinationOpens)
	}
}

func requireRetainedHeapBudgetBlocks(
	t *testing.T,
	largeMeas, smallMeas testutil.BlocksLargeOutputMemMeasurement,
) {
	t.Helper()

	budget := testutil.StreamingBodyResidencyRetainedHeapBudget(smallMeas.RetainedHeapAllocDelta)

	if largeMeas.RetainedHeapAllocDelta > budget {
		t.Fatalf(
			"boundedness (primary, retained GC-stabilized heap): large retained heap %d exceeds "+
				"small-vs-large budget %d "+
				"(small retained baseline %d; ratio cap %d; fixed noise allowance %d)",
			largeMeas.RetainedHeapAllocDelta,
			budget,
			smallMeas.RetainedHeapAllocDelta,
			testutil.StreamingBodyRetainedHeapMaxLargeToSmallRatio,
			testutil.StreamingBodyResidencyRetainedHeapNoiseAllowance,
		)
	}
}

func TestRunBlocksStreamsLargeBodyWithoutFullResidency(t *testing.T) {
	t.Skip("MemStats residency gate flaky on CI; see issue #2")

	ctx := context.Background()
	root := testRoot()

	smallLines := testutil.StreamingResidencyProbeSmallLineCount
	smallLineLen := testutil.StreamingResidencyProbeSmallLineLength
	largeLines := testutil.LargeStreamingFixtureLineCount
	largeLineLen := testutil.LargeStreamingFixtureLineLength

	hdr := []byte("banner\n")
	beginLine := []byte("<<BEGIN>>\n")
	endLine := []byte("<<END>>\n")

	srcPath, _, blocksParams := streamingResidencyBlocksParams(root)

	sMeas, sPres := mustMeasureRunBlocksPeakHeap(t, ctx, &blocksParams, &blocksResidencyPeakFixture{
		header:      hdr,
		beginMarker: beginLine,
		endMarker:   endLine,
		lineCount:   smallLines,
		lineLen:     smallLineLen,
	}, srcPath)

	if sPres.BlocksFound != 1 {
		t.Fatalf("sanity (small): want BlocksFound 1, got %d", sPres.BlocksFound)
	}

	requireOneOutputBasenameSlice(t, "small", sPres.Files)
	requireBlocksDestinationAndReadSanity(t, "small", sMeas, smallLines, smallLineLen)

	lMeas, lPres := mustMeasureRunBlocksPeakHeap(t, ctx, &blocksParams, &blocksResidencyPeakFixture{
		header:      hdr,
		beginMarker: beginLine,
		endMarker:   endLine,
		lineCount:   largeLines,
		lineLen:     largeLineLen,
	}, srcPath)

	if lPres.BlocksFound != 1 {
		t.Fatalf("sanity (large): want BlocksFound 1, got %d", lPres.BlocksFound)
	}

	requireOneOutputBasenameSlice(t, "large", lPres.Files)
	requireBlocksDestinationAndReadSanity(t, "large", lMeas, largeLines, largeLineLen)

	requireRetainedHeapBudgetBlocks(t, lMeas, sMeas)

	minLogicalLarge := int64(largeLines * (largeLineLen + 1))
	if lMeas.PeakHeapAllocDelta > testutil.MaxPeakHeapGrowthForStreamingBody {
		t.Fatalf(
			"boundedness (secondary smoke, immediate peak): peak HeapAlloc delta %d exceeds streaming "+
				"budget %d "+
				"for ~%d byte block body (hypothesis: full body/metadata residency in heap; "+
				"destination bytes are not retained by the test opener)",
			lMeas.PeakHeapAllocDelta,
			testutil.MaxPeakHeapGrowthForStreamingBody,
			minLogicalLarge,
		)
	}
}
