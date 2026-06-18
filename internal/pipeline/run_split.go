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

// SplitParams configures the RunSplit pipeline.
type SplitParams struct {
	// SrcPath is the source file path (absolute or relative).
	SrcPath string
	// OutDir is the output directory path (absolute or relative).
	OutDir string
	// Root is the workspace root for path validation.
	Root string
	// Delimiter is the compiled regular expression for delimiter lines.
	Delimiter *regexp.Regexp
	// Naming is the naming strategy for output files.
	Naming fileops.NamingStrategy
	// StripDelimiter omits delimiter lines from section output.
	StripDelimiter bool
	// Extension is the declared output file extension (leading dot). Empty means
	// RunSplit derives from filepath.Ext(SrcPath) via ExtensionFromDeclaredOrSource.
	Extension string
	// Force overwrites existing output files.
	Force bool
	// Mkdir creates the output directory if it does not exist.
	Mkdir bool
	// Preview computes planned absolute output paths without writing files or creating the output directory.
	Preview bool
	// MaxFiles caps how many output sections are allowed (uses DefaultMaxFiles when <= 0).
	MaxFiles int
	// Names are optional explicit output basenames (one per section, after split). When non-empty,
	// they override [Naming]; count must equal len(sections).
	Names []string
}

// SplitPipelineResult holds the result of a RunSplit call.
type SplitPipelineResult struct {
	Sections []fileops.SplitSection
	Files    []string
	Warnings []string
}

// RunSplit validates paths, opens source, performs binary guard, scans delimiter sections with
// bounded-cardinality metadata, then previews or streams section byte spans to outputs via publishFS
// (atomic create/replace). Apply mode does not open destination paths for payload writes via out.
//
// Sentinel errors: ErrSourceNotFound, ErrBinarySource, ErrDestinationExists.
//
//nolint:gocritic // hugeParam: grouped config mirrors RunExtract; pointer would not match style
func RunSplit(
	ctx context.Context,
	src SourceOpener,
	out OutputOpener,
	resolver validate.PathResolver,
	publishFS fileops.FileSession,
	params SplitParams,
) (SplitPipelineResult, error) {
	if err := errNilSplitDeps(src, out, resolver, publishFS); err != nil {
		return SplitPipelineResult{}, err
	}

	if valErr := validateSourceAndOutDir(params.SrcPath, params.OutDir, params.Root, resolver); valErr != nil {
		return SplitPipelineResult{}, valErr
	}

	ext, extErr := ExtensionFromDeclaredOrSource(params.Extension, params.SrcPath)
	if extErr != nil {
		return SplitPipelineResult{}, fmt.Errorf("extension: %w", extErr)
	}

	srcFile, openErr := openSourceForPipeline(src, params.SrcPath)
	if openErr != nil {
		return SplitPipelineResult{}, openErr
	}

	defer func() { _ = srcFile.Close() }()

	naming := params.Naming
	if len(params.Names) > 0 {
		// Explicit --names replaces strategy-generated basenames after split.
		naming = fileops.Sequential
	}

	opts := fileops.SplitOptions{
		Source:         srcFile,
		Delimiter:      params.Delimiter,
		Naming:         naming,
		StripDelimiter: params.StripDelimiter,
		Extension:      ext,
	}

	limits := fileops.BoundedScanLimits{MaxFiles: effectiveMaxFiles(params.MaxFiles)}
	scanRes, scanErr := fileops.ScanSplitSectionsMeta(ctx, opts, limits)
	if scanErr != nil {
		return SplitPipelineResult{}, scanErr
	}

	return completeSplitAfterScan(ctx, out, publishFS, srcFile, &params, scanRes, ext, resolver)
}

func errNilSplitDeps(
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

func completeSplitAfterScan(
	ctx context.Context,
	out OutputOpener,
	publishFS fileops.FileSession,
	srcFile io.ReadSeekCloser,
	params *SplitParams,
	scanRes fileops.SplitBoundedScanResult,
	ext string,
	resolver validate.PathResolver,
) (SplitPipelineResult, error) {
	sections := make([]fileops.SplitSection, len(scanRes.Sections))
	for i := range scanRes.Sections {
		sections[i].Name = scanRes.Sections[i].Name
	}

	if applyErr := applyExplicitNamesToSplitSections(sections, params.Names, ext); applyErr != nil {
		return SplitPipelineResult{}, applyErr
	}

	var written []string

	if params.Preview {
		prevWritten, prevErr := splitPreviewFileList(publishFS, params.Force, params.OutDir, params.Root, sections, resolver)
		if prevErr != nil {
			return SplitPipelineResult{}, prevErr
		}
		written = prevWritten
	} else {
		writeWritten, writeErr := writeSplitSectionsAfterScan(
			ctx, out, publishFS, srcFile, params, scanRes, sections, resolver,
		)
		if writeErr != nil {
			return SplitPipelineResult{}, writeErr
		}
		written = writeWritten
	}

	return SplitPipelineResult{
		Sections: sections,
		Files:    written,
		Warnings: nil,
	}, nil
}

func writeSplitSectionsAfterScan(
	ctx context.Context,
	out OutputOpener,
	publishFS fileops.FileSession,
	srcFile io.ReadSeekCloser,
	params *SplitParams,
	scanRes fileops.SplitBoundedScanResult,
	sections []fileops.SplitSection,
	resolver validate.PathResolver,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if mkErr := mkdirOutDirIfRequested(out, params.OutDir, params.Mkdir); mkErr != nil {
		return nil, mkErr
	}

	sw := splitByteSpanWriter{
		ctx:       ctx,
		out:       out,
		publishFS: publishFS,
		outDir:    params.OutDir,
		root:      params.Root,
		src:       srcFile,
		metas:     scanRes.Sections,
		sections:  sections,
		force:     params.Force,
		resolver:  resolver,
	}

	return sw.writeAll()
}

func splitPreviewFileList(
	publishFS fileops.FileSession,
	force bool,
	outDir, root string,
	sections []fileops.SplitSection,
	resolver validate.PathResolver,
) ([]string, error) {
	names := make([]string, len(sections))
	for i := range sections {
		names[i] = sections[i].Name
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

type splitByteSpanWriter struct {
	ctx context.Context

	out       OutputOpener
	publishFS fileops.FileSession
	outDir    string
	root      string
	src       io.ReadSeekCloser
	metas     []fileops.SplitSectionMeta
	sections  []fileops.SplitSection
	force     bool
	resolver  validate.PathResolver
}

//nolint:gocritic // hugeParam: SplitSectionMeta aligns scan metadata with Seek/CopyN application.
func splitSeekAndSpanLength(meta fileops.SplitSectionMeta) (seekOff, byteLen int64, err error) {
	if meta.ByteSpanEndExclusive < meta.ByteSpanStart {
		return 0, 0, errSplitWriteInvalidByteSpan
	}

	span := meta.ByteSpanEndExclusive - meta.ByteSpanStart
	maxOff := uint64(math.MaxInt64)

	if meta.ByteSpanStart > maxOff {
		return 0, 0, errSplitWriteByteSpanStartExceedsMaxSeek
	}

	if span > maxOff {
		return 0, 0, errSplitWriteByteSpanLengthExceedsMaxCopy
	}

	return int64(meta.ByteSpanStart), int64(span), nil
}

//nolint:gocritic // hugeParam: Writer aggregates split scan metas with atomic publication.
func (w splitByteSpanWriter) writeAll() ([]string, error) {
	if len(w.metas) != len(w.sections) {
		return nil, errSplitWriteInternalMetaSectionCountMismatch
	}

	written := make([]string, 0, len(w.sections))

	for i := range w.metas {
		name, err := w.writeOneSection(i)
		if err != nil {
			return nil, err
		}

		written = append(written, name)
	}

	return written, nil
}

//nolint:gocritic // hugeParam: Mirrors blocksByteSpanWriter shape for symmetry with split paths.
func (w splitByteSpanWriter) writeOneSection(index int) (string, error) {
	path, apErr := absolutePlannedOutputPath(w.outDir, w.sections[index].Name)
	if apErr != nil {
		return "", apErr
	}

	if destValErr := validate.ValidatePath(path, w.root, w.resolver); destValErr != nil {
		return "", pathContextError(PathRoleOutputPath, path, fmt.Errorf("validate dest: %w", destValErr))
	}

	seekOff, byteLen, spanErr := splitSeekAndSpanLength(w.metas[index])
	if spanErr != nil {
		return "", pathContextError(PathRoleOutputPath, path, fmt.Errorf("split write span: %w", spanErr))
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
