package pipeline

import (
	"context"
	"errors"
	"io"
	"io/fs"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

func publishSpanWithFingerprintVerify(
	ctx context.Context,
	publishFS fileops.FileSession,
	src io.ReadSeekCloser,
	seekOff, byteLen int64,
	wantFP [32]byte,
	destPath string,
	force bool,
) error {
	mode := atomicPublishModeCreateOrReplace(force)
	pubOpts := fileops.AtomicPublishOptions{
		Path: destPath,
		Perm: fs.FileMode(FilePerm),
		Mode: mode,
	}

	pubErr := fileops.AtomicPublish(publishFS, pubOpts, func(dw io.Writer) error {
		if err := fileops.CopySpanToWriterWithSHA256Verify(ctx, dw, src, seekOff, byteLen, wantFP); err != nil {
			return pathContextError(PathRoleOutputPath, destPath, err)
		}

		return nil
	})
	if pubErr != nil {
		if errors.Is(pubErr, fileops.ErrAtomicDestinationExists) {
			absDest, absErr := canonicalAbsoluteNativePath(destPath)
			if absErr != nil {
				return newDestinationExistsError(destPath)
			}

			return newDestinationExistsError(absDest)
		}

		var pc *PathContextError
		if errors.As(pubErr, &pc) {
			return pubErr
		}

		return pathContextError(PathRoleOutputPath, destPath, pubErr)
	}

	return nil
}
