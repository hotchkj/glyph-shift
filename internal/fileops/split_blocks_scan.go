package fileops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
)

// ErrSeekableSourceRequired reports bounded scan metadata calls that cannot preserve byte-span replay.
var ErrSeekableSourceRequired = errors.New("seekable source is required for bounded scan metadata")

// linespanScanOptsForSeekableBoundedLimits uses a smaller line-span read chunk when MaxFiles may force
// early exit, reducing seekable read-ahead past the cap failure point (mirrors extract planning reads).
func linespanScanOptsForSeekableBoundedLimits(limits BoundedScanLimits) linespanScanOptions {
	if limits.MaxFiles <= 0 {
		return linespanScanOptions{}
	}

	return linespanScanOptions{chunkSizeBytes: boundedEarlyExitLinespanChunkSize}
}

// SplitSectionMeta is bounded metadata for one split output file.
//
// Production pipeline applies byte spans verified by SpanFingerprintSHA256 over raw
// [ByteSpanStart, ByteSpanEndExclusive). Logical line-window fields annotate those spans alongside a
// 1-based line framing model. Library helpers (notably fileops.Split) replay into []SplitSection using
// ForEachLineFromContext and still materialize one logical Line value at a time during that replay phase.
type SplitSectionMeta struct {
	Name string

	OutputStartLineNum int
	OutputEndLineNum   int
	ContentLineCount   int

	ByteSpanStart        uint64
	ByteSpanEndExclusive uint64

	OrdinalSeq int

	IsPreambleSection bool

	StripDelimiterInOutput bool

	DelimiterLineSourceNum int

	// SpanFingerprintSHA256 is the SHA-256 digest of raw bytes [ByteSpanStart, ByteSpanEndExclusive).
	SpanFingerprintSHA256 [32]byte
}

// SplitBoundedScanResult aggregates split scan output.
type SplitBoundedScanResult struct {
	Sections           []SplitSectionMeta
	OutputSectionCount int
	DelimiterLineCount int
}

// ScanSplitSectionsMeta records bounded split section metadata.
//
// When opts.Source satisfies io.ReadSeeker, delimiter matching runs via ScanLineSpans plus
// MatchLineSpan within each logical line CONTENT span — no scan-phase materialization into []Line.
//
// Pipeline apply paths derive output from published byte spans and verify those bytes with fingerprints;
// this seekable meta scan aligns with that model. By contrast, the fileops.Split library helper replays the
// same metadata into populated []SplitSection values via ForEachLineFromContext, which materializes logical
// Line payloads line-by-line after seekable metadata has already been planned.
//
// Memory model: BoundedScanLimits caps output section cardinality and related scan state, and retained
// metadata is bounded by those counts. Seekable scans retain only span metadata plus per-line naming
// strings built from readSerializedSpanContentPrefixUTF8 (chunked, capped by
// NamingMaterializationMaxBytes per logical line content span), not full line bodies, while
// fingerprinting streams full output byte ranges.
//
// Pipeline / production previews use seekable opens; tests may pass bytes.Reader buffers.
func ScanSplitSectionsMeta(
	ctx context.Context,
	opts SplitOptions,
	limits BoundedScanLimits,
) (SplitBoundedScanResult, error) {
	if opts.Delimiter == nil {
		return SplitBoundedScanResult{}, fmt.Errorf(splitErrorFormat, errSplitNilDelimiter)
	}

	if err := ctx.Err(); err != nil {
		return SplitBoundedScanResult{}, fmt.Errorf(splitErrorFormat, err)
	}

	if rs, ok := opts.Source.(io.ReadSeeker); ok {
		return scanSplitSectionsMetaSeekable(ctx, rs, opts, limits, linespanScanOptsForSeekableBoundedLimits(limits))
	}

	return SplitBoundedScanResult{}, fmt.Errorf(splitErrorFormat, ErrSeekableSourceRequired)
}

// BlockScanMeta is bounded metadata for one emitted non-empty blocks output file.
//
// Pipeline apply verifies emitted bytes via SpanFingerprintSHA256 over raw output ranges below; line-window
// fields pair with IncludeDelimiters for interpreting those spans on the logical line ladder.
type BlockScanMeta struct {
	Name string

	IncludeDelimiters bool

	StartDelimLineNum int
	InnerStartLineNum int // zero when inner empty (no emission path)
	InnerEndLineNum   int // inclusive inner tail; zero empty
	EndDelimLineNum   int

	InnerContentLineCount int
	EmittedOrdinal        int // 1-based index among emitted block files only

	BlockFoundOrdinal int // 1-based index among delimiter-closed blocks (includes empty)

	InnerByteStart         uint64
	InnerByteEndExclusive  uint64
	OutputByteStart        uint64
	OutputByteEndExclusive uint64

	// SpanFingerprintSHA256 is the SHA-256 digest of raw bytes [OutputByteStart, OutputByteEndExclusive).
	SpanFingerprintSHA256 [32]byte
}

// BlocksBoundedScanResult aggregates blocks scan counters and emission metadata.
type BlocksBoundedScanResult struct {
	Metas []BlockScanMeta

	BlocksFound          int // includes delimiter-closed empty blocks
	OutputBlockFileCount int // len(non-empty emits)
	EmptyBlocksDiscarded int // closed blocks omitted from disk (empty inner body)
}

// ScanBlocksMeta records bounded block metadata.
//
// When opts.Source satisfies io.ReadSeeker, start/end delimiter matching runs via ScanLineSpans plus
// MatchLineSpan on each line CONTENT span without scan-phase []Line materialization, matching pipeline
// preview/descriptor paths that fingerprint byte spans rather than buffering whole lines during the scan.
func ScanBlocksMeta(
	ctx context.Context,
	opts BlocksOptions,
	limits BoundedScanLimits,
) (BlocksBoundedScanResult, error) {
	if opts.StartDelimiter == nil {
		return BlocksBoundedScanResult{}, fmt.Errorf(blocksErrorFormat, errBlocksNilStart)
	}

	if opts.EndDelimiter == nil {
		return BlocksBoundedScanResult{}, fmt.Errorf(blocksErrorFormat, errBlocksNilEnd)
	}

	if err := ctx.Err(); err != nil {
		return BlocksBoundedScanResult{}, fmt.Errorf(blocksErrorFormat, err)
	}

	if rs, ok := opts.Source.(io.ReadSeeker); ok {
		return scanBlocksMetaSeekable(ctx, rs, opts, limits, linespanScanOptsForSeekableBoundedLimits(limits))
	}

	return BlocksBoundedScanResult{}, fmt.Errorf(blocksErrorFormat, ErrSeekableSourceRequired)
}

// isolateMatchLineSpanOnSharedSeeker runs fn for one ScanLineSpans line callback while temporarily
// snapshotting a shared ReadSeeker used by both the line scanner and all auxiliary I/O in this line's
// callback (delimiter matching, section/block finalization, bounded UTF-8 naming prefix extraction,
// streaming span SHA-256). Every such helper seeks the same rs; fn must not leave the stream offset
// inconsistent with ScanLineSpans' sequential read state.
//
// Deferred restore sets the stream offset to max(entryCursor, span.SerializedEnd): entry capture is
// the physical seek offset after the scanner's read-ahead, and span.SerializedEnd is the minimum
// serialized byte past which the scanner has already consumed this line.
//
// Snapshotting Seek alone is insufficient: MatchLineSpan can leave the cursor before
// span.SerializedEnd while the scanner has already advanced past the terminator in its buffer —
// restoring only SeekCurrent strands the underlying reader mid-line for the sequential scan reader.
func isolateMatchLineSpanOnSharedSeeker(rs io.ReadSeeker, span LineSpan, fn func() error) (err error) {
	saved, err := rs.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	if span.SerializedEnd > uint64(math.MaxInt64) {
		return ErrLineSpanOffsetTooLarge
	}

	minAfterLine := int64(span.SerializedEnd)

	defer func() {
		next := saved
		if minAfterLine > next {
			next = minAfterLine
		}

		_, seekErr := rs.Seek(next, io.SeekStart)
		if seekErr != nil && err == nil {
			err = seekErr
		}
	}()

	return fn()
}
