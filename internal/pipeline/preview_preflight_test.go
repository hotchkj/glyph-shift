package pipeline

import (
	"errors"
	"io/fs"
	"testing"
)

var errPreviewPreflightPermission = errors.New("preview preflight permission denied")

func TestPreflightVacantPlannedOutputsForceSkipsDestinationProbe(t *testing.T) {
	t.Parallel()

	if err := preflightVacantPlannedOutputsPublishFS(nil, true, []string{"/out.txt"}); err != nil {
		t.Fatalf("preflight force error = %v want nil", err)
	}
}

func TestIsPreviewDestinationNotExist(t *testing.T) {
	t.Parallel()

	for _, err := range []error{fs.ErrNotExist} {
		if !isPreviewDestinationNotExist(err) {
			t.Fatalf("isPreviewDestinationNotExist(%v) = false want true", err)
		}
	}
	if isPreviewDestinationNotExist(errPreviewPreflightPermission) {
		t.Fatal("permission error classified as not-exist")
	}
}
