package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

// ExtractParams configures the RunExtract pipeline.
type ExtractParams struct {
	// SrcPath is the source file path (absolute or relative; ValidatePath resolves it).
	SrcPath string
	// DestPath is the destination file path (absolute or relative; ValidatePath resolves it).
	DestPath string
	// Root is the workspace root used for path validation.
	Root string
	// Lines is the 1-based inclusive line range to extract.
	Lines fileops.LineRange
	// Force overwrites an existing destination file.
	Force bool
	// Append appends to an existing destination file instead of failing.
	Append bool
	// Mkdir creates parent directories of the destination if they do not exist.
	Mkdir bool
	// Preview runs extraction to io.Discard to report line count without publishing or writing the destination.
	Preview bool
}

// RunExtract validates paths, opens the source with binary guard, and runs line extraction via
// fileops.Extract. Preview mode validates paths and runs extraction to io.Discard to report line count
// without publishing or writing the destination.
//
// Apply mode publishes extracted bytes atomically: fileops.AtomicPublish writes through publishFS to a
// temporary staging file, then rename/replaces params.DestPath in one finalize step. Extract does not
// open the destination path for payload writes; publication installs the durable file.
//
// Paths must already be resolved (absolute). Root is used for validate.ValidatePath.
// Resolver is used for symlink validation; pass validate.NewOSPathResolver() in production.
//
// publishFS selects the filesystem session used for atomic destination publication (temp create,
// replace rename). Preview mode does not publish but publishFS must still be non-nil.
//
// Sentinel errors: ErrSourceNotFound, ErrBinarySource, ErrDestinationExists.
//
//nolint:gocritic // hugeParam: ExtractParams mirrors the CLI invocation bundle.
func RunExtract(
	ctx context.Context,
	src SourceOpener,
	out OutputOpener,
	resolver validate.PathResolver,
	publishFS fileops.FileSession,
	params ExtractParams,
) (fileops.ExtractResult, error) {
	if err := errNilExtractDeps(src, out, resolver, publishFS); err != nil {
		return fileops.ExtractResult{}, err
	}

	if srcValErr := validate.ValidatePath(params.SrcPath, params.Root, resolver); srcValErr != nil {
		return fileops.ExtractResult{}, pathContextError(
			PathRoleSrc,
			params.SrcPath,
			fmt.Errorf("validate source: %w", srcValErr),
		)
	}

	srcFile, openErr := openSourceForPipeline(src, params.SrcPath)
	if openErr != nil {
		return fileops.ExtractResult{}, openErr
	}

	defer func() { _ = srcFile.Close() }()

	opts := fileops.ExtractOptions{
		Source: srcFile,
		Lines:  params.Lines,
		Append: params.Append,
	}

	res, extErr := extractWithDestWriter(ctx, srcFile, out, publishFS, resolver, params, opts)
	if extErr != nil {
		return fileops.ExtractResult{}, extErr
	}

	return res, nil
}

func errNilExtractDeps(
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

//nolint:gocritic // hugeParam: ExtractParams mirrors the CLI invocation bundle.
func extractPreviewDiscard(
	ctx context.Context,
	publishFS fileops.FileSession,
	params ExtractParams,
	opts fileops.ExtractOptions,
) (fileops.ExtractResult, error) {
	wouldAbs, absErr := canonicalAbsoluteNativePath(params.DestPath)
	if absErr != nil {
		return fileops.ExtractResult{}, pathContextError(
			PathRoleDest,
			params.DestPath,
			fmt.Errorf("preview dest path: %w", absErr),
		)
	}

	if atomicPublishModeForExtract(params.Force, params.Append) == fileops.AtomicPublishCreate {
		exists, chkErr := destinationExistsViaPublishFS(publishFS, wouldAbs)
		if chkErr != nil {
			return fileops.ExtractResult{}, chkErr
		}

		if exists {
			return fileops.ExtractResult{}, newDestinationExistsError(wouldAbs)
		}
	}

	res, extOnlyErr := fileops.Extract(ctx, opts, io.Discard)
	if extOnlyErr != nil {
		return fileops.ExtractResult{}, pathContextError(PathRoleSrc, params.SrcPath, extOnlyErr)
	}

	return fileops.ExtractResult{
		LinesExtracted:  res.LinesExtracted,
		WouldCreatePath: wouldAbs,
	}, nil
}

//nolint:gocritic // hugeParam: ExtractParams mirrors the CLI invocation bundle.
func extractWithDestWriter(
	ctx context.Context,
	srcFile io.ReadSeekCloser,
	out OutputOpener,
	publishFS fileops.FileSession,
	resolver validate.PathResolver,
	params ExtractParams,
	opts fileops.ExtractOptions,
) (fileops.ExtractResult, error) {
	if valErr := validateExtractApplyDestPath(resolver, params); valErr != nil {
		return fileops.ExtractResult{}, valErr
	}

	if params.Preview {
		return extractPreviewDiscard(ctx, publishFS, params, opts)
	}

	wantSHA256, extractPlan, planErr := extractApplyPlanExtractedBytesSHA256(ctx, srcFile, opts.Lines, params.SrcPath)
	if planErr != nil {
		return fileops.ExtractResult{}, planErr
	}

	if _, err := srcFile.Seek(0, io.SeekStart); err != nil {
		return fileops.ExtractResult{}, pathContextError(
			PathRoleSrc,
			params.SrcPath,
			fmt.Errorf("rewind source before extract: %w", err),
		)
	}

	return extractApplyAtomicPublish(ctx, srcFile, out, publishFS, params, extractPlan, wantSHA256)
}

//nolint:gocritic // hugeParam: ExtractParams mirrors the CLI invocation bundle.
func validateExtractApplyDestPath(resolver validate.PathResolver, params ExtractParams) error {
	if destValErr := validate.ValidatePath(params.DestPath, params.Root, resolver); destValErr != nil {
		return pathContextError(PathRoleDest, params.DestPath, fmt.Errorf("validate dest: %w", destValErr))
	}

	return nil
}

//nolint:gocritic // hugeParam: ExtractParams mirrors the CLI invocation bundle.
func mkdirExtractApplyDestParentDirs(out OutputOpener, params ExtractParams) error {
	if !params.Mkdir {
		return nil
	}

	dir := filepath.Dir(params.DestPath)
	if dir == "." || dir == "" {
		return nil
	}

	if mkErr := out.MkdirAll(dir, DirPerm); mkErr != nil {
		return pathContextError(PathRoleDest, params.DestPath, fmt.Errorf("create dest directories: %w", mkErr))
	}

	return nil
}

func extractApplyStageBytesAndVerifyFingerprint(
	ctx context.Context,
	srcFile io.ReadSeekCloser,
	dest io.Writer,
	plan fileops.ExtractSerializedSpanPlan,
	wantSHA256 [32]byte,
	destPath string,
) (fileops.ExtractResult, error) {
	seekOff, byteLen, err := fileops.SerializedSpanSeekAndLength(
		plan.SerializedStart,
		plan.SerializedEndExclusive,
	)
	if err != nil {
		return fileops.ExtractResult{}, pathContextError(PathRoleOutputPath, destPath, err)
	}

	if err := fileops.CopySpanToWriterWithSHA256Verify(ctx, dest, srcFile, seekOff, byteLen, wantSHA256); err != nil {
		return fileops.ExtractResult{}, pathContextError(PathRoleOutputPath, destPath, err)
	}

	return fileops.ExtractResult{LinesExtracted: plan.LinesSelected}, nil
}

func mapExtractAtomicPublishError(pubErr error, destPath string) error {
	if errors.Is(pubErr, fileops.ErrAtomicDestinationExists) {
		absDest, err := canonicalAbsoluteNativePath(destPath)
		if err != nil {
			return newDestinationExistsError(destPath)
		}

		return newDestinationExistsError(absDest)
	}

	var pc *PathContextError
	if errors.As(pubErr, &pc) {
		return pubErr
	}

	return pathContextError(PathRoleDest, destPath, pubErr)
}

//nolint:gocritic // hugeParam: ExtractParams mirrors the CLI invocation bundle.
func extractApplyAtomicPublish(
	ctx context.Context,
	srcFile io.ReadSeekCloser,
	out OutputOpener,
	publishFS fileops.FileSession,
	params ExtractParams,
	extractPlan fileops.ExtractSerializedSpanPlan,
	wantSHA256 [32]byte,
) (fileops.ExtractResult, error) {
	if mkErr := mkdirExtractApplyDestParentDirs(out, params); mkErr != nil {
		return fileops.ExtractResult{}, mkErr
	}

	mode := atomicPublishModeForExtract(params.Force, params.Append)
	pubOpts := fileops.AtomicPublishOptions{
		Path: params.DestPath,
		Perm: fs.FileMode(FilePerm),
		Mode: mode,
	}

	var res fileops.ExtractResult

	pubErr := fileops.AtomicPublish(publishFS, pubOpts, func(w io.Writer) error {
		extRes, extErr := extractApplyStageBytesAndVerifyFingerprint(
			ctx,
			srcFile,
			w,
			extractPlan,
			wantSHA256,
			params.DestPath,
		)
		res = extRes

		return extErr
	})
	if pubErr != nil {
		return fileops.ExtractResult{}, mapExtractAtomicPublishError(pubErr, params.DestPath)
	}

	return res, nil
}

func atomicPublishModeForExtract(force, appendMode bool) fileops.AtomicPublishMode {
	switch {
	case appendMode:
		return fileops.AtomicPublishAppend
	default:
		return atomicPublishModeCreateOrReplace(force)
	}
}

func atomicPublishModeCreateOrReplace(force bool) fileops.AtomicPublishMode {
	if force {
		return fileops.AtomicPublishReplace
	}

	return fileops.AtomicPublishCreate
}

// extractApplyPlanExtractedBytesSHA256 validates lr using the scan metadata path aligned with chunked copy,
// hashes the contiguous serialized-byte span chosen for extraction with fixed-size chunked reads, and
// returns digest plus plan statistics for the replay pass.
func extractApplyPlanExtractedBytesSHA256(
	ctx context.Context,
	srcFile io.ReadSeeker,
	lr fileops.LineRange,
	srcPath string,
) (wantSHA256 [32]byte, extractPlan fileops.ExtractSerializedSpanPlan, err error) {
	var zero fileops.ExtractSerializedSpanPlan

	plan, planErr := fileops.PlanExtractSerializedSpan(ctx, srcFile, lr)
	if planErr != nil {
		return wantSHA256, zero, pathContextError(PathRoleSrc, srcPath, planErr)
	}

	fp, hashErr := fileops.SHA256SerializedByteSpan(ctx, srcFile, plan.SerializedStart, plan.SerializedEndExclusive)
	if hashErr != nil {
		return wantSHA256, zero, pathContextError(PathRoleSrc, srcPath, fmt.Errorf("hash extracted span: %w", hashErr))
	}

	return fp, plan, nil
}

func binaryCheckAndRewind(rs io.ReadSeekCloser, srcPath string) error {
	isBin, binErr := fileops.IsBinary(rs)
	if binErr != nil {
		return pathContextError(PathRoleSrc, srcPath, fmt.Errorf("binary check: %w", binErr))
	}

	if isBin {
		return pathContextError(PathRoleSrc, srcPath, fmt.Errorf("%w", ErrBinarySource))
	}

	if _, seekErr := rs.Seek(0, io.SeekStart); seekErr != nil {
		return pathContextError(PathRoleSrc, srcPath, fmt.Errorf("seek after binary check: %w", seekErr))
	}

	return nil
}
