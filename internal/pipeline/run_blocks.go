package pipeline

import (
	"context"
	"fmt"
	"io"
	"math"
	"regexp"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

// BlocksParams configures the RunBlocks pipeline.
type BlocksParams struct {
	SrcPath           string
	OutDir            string
	Root              string
	StartDelimiter    *regexp.Regexp
	EndDelimiter      *regexp.Regexp
	Naming            fileops.NamingStrategy
	IncludeDelimiters bool
	// Extension is the declared output extension (leading dot). Empty means
	// RunBlocks derives from filepath.Ext(SrcPath) via ExtensionFromDeclaredOrSource.
	Extension string
	Force     bool
	Mkdir     bool
	// Preview computes planned absolute output paths without writing files or creating the output directory.
	Preview bool
	// MaxFiles caps non-empty blocks that produce output files (uses DefaultMaxFiles when <= 0).
	MaxFiles int
	// Names are optional explicit basenames (one per file written). When non-empty they override
	// Naming; count must equal len(Blocks), not BlocksFound.
	Names []string
}

// BlocksPipelineResult holds the result of a RunBlocks call.
type BlocksPipelineResult struct {
	Blocks      []fileops.Block
	BlocksFound int // total matched blocks including empty (see glyph-shift-json-contract.md)
	Files       []string
	Warnings    []string
}

// RunBlocks validates paths, opens source, performs binary guard, scans closed blocks with
// bounded-cardinality metadata, then previews or streams output byte spans via publishFS (atomic
// create/replace). Apply mode does not open destination paths for payload writes via out.
//
// Sentinel errors: ErrSourceNotFound, ErrBinarySource, ErrDestinationExists, fileops.ErrNoBlocksFound.
//
//nolint:gocritic // hugeParam: grouped config mirrors RunExtract; pointer would not match style
func RunBlocks(
	ctx context.Context,
	src SourceOpener,
	out OutputOpener,
	resolver validate.PathResolver,
	publishFS fileops.FileSession,
	params BlocksParams,
) (BlocksPipelineResult, error) {
	if err := errNilBlocksDeps(src, out, resolver, publishFS); err != nil {
		return BlocksPipelineResult{}, err
	}

	if valErr := validateSourceAndOutDir(params.SrcPath, params.OutDir, params.Root, resolver); valErr != nil {
		return BlocksPipelineResult{}, valErr
	}

	ext, extErr := ExtensionFromDeclaredOrSource(params.Extension, params.SrcPath)
	if extErr != nil {
		return BlocksPipelineResult{}, fmt.Errorf("extension: %w", extErr)
	}

	srcFile, openErr := openSourceForPipeline(src, params.SrcPath)
	if openErr != nil {
		return BlocksPipelineResult{}, openErr
	}

	defer func() { _ = srcFile.Close() }()

	naming := params.Naming
	if len(params.Names) > 0 {
		naming = fileops.Sequential
	}

	opts := fileops.BlocksOptions{
		Source:            srcFile,
		StartDelimiter:    params.StartDelimiter,
		EndDelimiter:      params.EndDelimiter,
		Naming:            naming,
		IncludeDelimiters: params.IncludeDelimiters,
		Extension:         ext,
	}

	limits := fileops.BoundedScanLimits{MaxFiles: effectiveMaxFiles(params.MaxFiles)}
	scanRes, scanErr := fileops.ScanBlocksMeta(ctx, opts, limits)
	if scanErr != nil {
		return BlocksPipelineResult{}, scanErr
	}

	return completeBlocksAfterScan(ctx, out, publishFS, srcFile, &params, scanRes, ext, resolver)
}

func errNilBlocksDeps(
	src SourceOpener,
	out OutputOpener,
	resolver validate.PathResolver,
	publishFS fileops.FileSession,
) error {
	if publishFS == nil {
		return fileops.ErrNilFileSession
	}

	return errNilSrcOutResolver(src, out, resolver)
}

func completeBlocksAfterScan(
	ctx context.Context,
	out OutputOpener,
	publishFS fileops.FileSession,
	srcFile io.ReadSeekCloser,
	params *BlocksParams,
	scanRes fileops.BlocksBoundedScanResult,
	ext string,
	resolver validate.PathResolver,
) (BlocksPipelineResult, error) {
	blocks := make([]fileops.Block, len(scanRes.Metas))
	for i := range scanRes.Metas {
		blocks[i] = fileops.Block{Name: scanRes.Metas[i].Name}
	}

	if applyErr := applyExplicitNamesToBlocks(blocks, params.Names, ext); applyErr != nil {
		return BlocksPipelineResult{}, applyErr
	}

	var written []string

	if params.Preview {
		prevWritten, prevErr := blocksPreviewFileList(publishFS, params.Force, params.OutDir, params.Root, blocks, resolver)
		if prevErr != nil {
			return BlocksPipelineResult{}, prevErr
		}
		written = prevWritten
	} else {
		writeWritten, writeErr := writeBlocksAfterScan(ctx, out, publishFS, srcFile, params, scanRes, blocks, resolver)
		if writeErr != nil {
			return BlocksPipelineResult{}, writeErr
		}
		written = writeWritten
	}

	return BlocksPipelineResult{
		Blocks:      blocks,
		BlocksFound: scanRes.BlocksFound,
		Files:       written,
		Warnings:    nil,
	}, nil
}

func writeBlocksAfterScan(
	ctx context.Context,
	out OutputOpener,
	publishFS fileops.FileSession,
	srcFile io.ReadSeekCloser,
	params *BlocksParams,
	scanRes fileops.BlocksBoundedScanResult,
	blocks []fileops.Block,
	resolver validate.PathResolver,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if mkErr := mkdirOutDirIfRequested(out, params.OutDir, params.Mkdir); mkErr != nil {
		return nil, mkErr
	}

	bw := blocksByteSpanWriter{
		ctx:       ctx,
		out:       out,
		publishFS: publishFS,
		outDir:    params.OutDir,
		root:      params.Root,
		src:       srcFile,
		metas:     scanRes.Metas,
		blocks:    blocks,
		force:     params.Force,
		resolver:  resolver,
	}

	return bw.writeAll()
}

func blocksPreviewFileList(
	publishFS fileops.FileSession,
	force bool,
	outDir, root string,
	blocks []fileops.Block,
	resolver validate.PathResolver,
) ([]string, error) {
	names := make([]string, len(blocks))
	for i := range blocks {
		names[i] = blocks[i].Name
	}

	paths, err := validatePlannedOutputPaths(outDir, root, names, resolver)
	if err != nil {
		return nil, err
	}

	if err := preflightVacantPlannedOutputsPublishFS(publishFS, force, paths); err != nil {
		return nil, err
	}

	return paths, nil
}

type blocksByteSpanWriter struct {
	ctx       context.Context
	out       OutputOpener
	publishFS fileops.FileSession
	outDir    string
	root      string
	src       io.ReadSeekCloser
	metas     []fileops.BlockScanMeta
	blocks    []fileops.Block
	force     bool
	resolver  validate.PathResolver
}

//nolint:gocritic // hugeParam: BlockScanMeta sized struct matches scan→write span contract.
func blocksSeekAndSpanLength(meta fileops.BlockScanMeta) (seekOff, byteLen int64, err error) {
	if meta.OutputByteEndExclusive < meta.OutputByteStart {
		return 0, 0, errBlocksWriteInvalidByteSpan
	}

	span := meta.OutputByteEndExclusive - meta.OutputByteStart
	maxOff := uint64(math.MaxInt64)

	if meta.OutputByteStart > maxOff {
		return 0, 0, errBlocksWriteByteSpanStartExceedsMaxSeek
	}

	if span > maxOff {
		return 0, 0, errBlocksWriteByteSpanLengthExceedsMaxCopy
	}

	return int64(meta.OutputByteStart), int64(span), nil
}

//nolint:gocritic // hugeParam: Writer aggregates scan results and output toggles without pointer churn.
func (w blocksByteSpanWriter) writeAll() ([]string, error) {
	if len(w.metas) != len(w.blocks) {
		return nil, errBlocksWriteInternalMetaBlockCountMismatch
	}

	written := make([]string, 0, len(w.blocks))

	for i := range w.metas {
		name, err := w.writeOneBlock(i)
		if err != nil {
			return nil, err
		}

		written = append(written, name)
	}

	return written, nil
}

//nolint:gocritic // hugeParam: Receiver mirrors splitByteSpanWriter field bundle for iterative writes.
func (w blocksByteSpanWriter) writeOneBlock(index int) (string, error) {
	path, apErr := absolutePlannedOutputPath(w.outDir, w.blocks[index].Name)
	if apErr != nil {
		return "", apErr
	}

	if destValErr := validate.ValidatePath(path, w.root, w.resolver); destValErr != nil {
		return "", pathContextError(PathRoleOutputPath, path, fmt.Errorf("validate dest: %w", destValErr))
	}

	seekOff, byteLen, spanErr := blocksSeekAndSpanLength(w.metas[index])
	if spanErr != nil {
		return "", pathContextError(PathRoleOutputPath, path, fmt.Errorf("blocks write span: %w", spanErr))
	}

	pubErr := publishSpanWithFingerprintVerify(
		w.ctx,
		w.publishFS,
		w.src,
		seekOff,
		byteLen,
		w.metas[index].SpanFingerprintSHA256,
		path,
		w.force,
	)
	if pubErr != nil {
		return "", pubErr
	}

	return path, nil
}
