package pipeline_test

import (
	"testing"

	"github.com/spf13/afero"

	"github.com/hotchkj/glyph-shift/internal/testutil"
)

func newMemPublishSession(t *testing.T, fs afero.Fs) *testutil.MemTestSession {
	t.Helper()

	return testutil.NewMemPublishSession(fs)
}

func discardedPublishSession(t *testing.T) *testutil.MemTestSession {
	t.Helper()

	return testutil.NewMemFileSession()
}
