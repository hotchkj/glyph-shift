package pipeline

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// openSourceForPipeline opens path via src, maps ErrNotExist to ErrSourceNotFound,
// runs binary guard (may return ErrBinarySource), and leaves the reader at offset 0.
// Caller must Close the returned ReadSeekCloser.
func openSourceForPipeline(src SourceOpener, path string) (io.ReadSeekCloser, error) {
	srcFile, err := src.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, pathContextError(PathRoleSrc, path, fmt.Errorf("%w", ErrSourceNotFound))
		}

		return nil, pathContextError(PathRoleSrc, path, fmt.Errorf("open source: %w", err))
	}

	if binErr := binaryCheckAndRewind(srcFile, path); binErr != nil {
		_ = srcFile.Close()

		return nil, binErr
	}

	return srcFile, nil
}
