package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

// TransformParams configures the RunTransform pipeline.
type TransformParams struct {
	// FilePath is the file path to transform (absolute or relative).
	FilePath string
	// Root is the workspace root for path validation.
	Root string
	// Opts are the transformation options (line endings, trim, newline).
	Opts fileops.TransformOptions
	// Yes applies changes; false means preview only.
	Yes bool
}

// TransformPipelineResult wraps fileops.TransformFileResult with the count of changes.
type TransformPipelineResult struct {
	Result      fileops.TransformFileResult
	ChangeCount int
}

// RunTransform stats the file, verifies it is a regular file, validates the
// path under root, and delegates to fileops.TransformFileWithContext.
//
// Sentinel errors: ErrSourceNotFound, ErrDirectoryNotFile, ErrNotRegularFile.
func RunTransform(
	ctx context.Context,
	stater FileStater,
	resolver validate.PathResolver,
	fileSession fileops.FileSession,
	params TransformParams,
) (TransformPipelineResult, error) {
	if err := ctx.Err(); err != nil {
		return TransformPipelineResult{}, err
	}

	if err := errNilTransformSeams(stater, resolver, fileSession); err != nil {
		return TransformPipelineResult{}, err
	}

	info, statErr := statSourceForTransform(stater, params.FilePath)
	if statErr != nil {
		return TransformPipelineResult{}, statErr
	}

	if err := validateTransformSourceRegular(params.FilePath, info); err != nil {
		return TransformPipelineResult{}, err
	}

	if valErr := validate.ValidatePath(params.FilePath, params.Root, resolver); valErr != nil {
		return TransformPipelineResult{}, pathContextError(
			PathRoleSrc,
			params.FilePath,
			fmt.Errorf("validate source: %w", valErr),
		)
	}

	res, err := fileops.TransformFileWithContext(ctx, params.FilePath, params.Opts, params.Yes, fileSession)
	if err != nil {
		return TransformPipelineResult{}, pathContextError(PathRoleSrc, params.FilePath, err)
	}

	return TransformPipelineResult{
		Result:      res,
		ChangeCount: fileops.CountTransformChanges(&res),
	}, nil
}

func statSourceForTransform(stater FileStater, filePath string) (fs.FileInfo, error) {
	info, statErr := stater.Stat(filePath)
	if statErr == nil {
		return info, nil
	}

	if errors.Is(statErr, fs.ErrNotExist) {
		return nil, pathContextError(PathRoleSrc, filePath, fmt.Errorf("%w", ErrSourceNotFound))
	}

	// Windows rejects Stat on paths that contain glob metacharacters; the CLI treats src as a
	// literal path, matching "no such file" semantics for BDD (e.g. src/*.txt).
	if strings.ContainsAny(filePath, "*?") {
		return nil, pathContextError(PathRoleSrc, filePath, fmt.Errorf("%w", ErrSourceNotFound))
	}

	return nil, pathContextError(PathRoleSrc, filePath, fmt.Errorf("stat source: %w", statErr))
}

func validateTransformSourceRegular(filePath string, info fs.FileInfo) error {
	if info.Mode().IsDir() {
		return pathContextError(PathRoleSrc, filePath, fmt.Errorf("%w", ErrDirectoryNotFile))
	}

	if !info.Mode().IsRegular() {
		return pathContextError(PathRoleSrc, filePath, fmt.Errorf("%w", ErrNotRegularFile))
	}

	return nil
}
