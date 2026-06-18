package pipeline

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

// destinationExistsViaPublishFS probes whether path names an existing readable destination using the
// same OpenRead existence check atomic publish uses for create-mode collision detection.
// It does not create temp files, directories, or mutate the destination.
func destinationExistsViaPublishFS(session fileops.FileSession, path string) (bool, error) {
	r, err := session.OpenRead(path)
	if err == nil {
		if closeErr := r.Close(); closeErr != nil {
			return true, pathContextError(PathRoleDest, path, fmt.Errorf("preview destination check: close: %w", closeErr))
		}

		return true, nil
	}

	if isPreviewDestinationNotExist(err) {
		return false, nil
	}

	return false, pathContextError(PathRoleDest, path, fmt.Errorf("preview destination check: %w", err))
}

func isPreviewDestinationNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) || os.IsNotExist(err)
}

// preflightVacantPlannedOutputsPublishFS fails with ErrDestinationExists when any planned output path
// already exists and force is false. destPaths must be absolute planned destinations that already satisfy
// validate.ValidatePath.
func preflightVacantPlannedOutputsPublishFS(
	publishFS fileops.FileSession,
	force bool,
	destPaths []string,
) error {
	if force {
		return nil
	}

	for _, full := range destPaths {
		exists, err := destinationExistsViaPublishFS(publishFS, full)
		if err != nil {
			return err
		}

		if exists {
			return newDestinationExistsError(full)
		}
	}

	return nil
}
