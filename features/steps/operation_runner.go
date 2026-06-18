package steps

import (
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
)

func newFeatureOperationRunner(tc *TestContext) (pipeline.Runner, error) {
	memFS := tc.Ws.FS()

	runner, err := pipeline.NewDefaultRunner(
		tc.symlinkAwareSourceOpener(testutil.NewMemSourceOpenerWithFS(memFS)),
		testutil.NewMemOutputOpenerWithFS(memFS),
		tc.symlinkAwareFileStater(testutil.NewMemFileStaterWithFS(memFS)),
		tc.symlinkAwareResolver(testutil.NewMemPathResolverWithFS(memFS)),
		tc.symlinkAwareFileSession(testutil.NewMemPublishSession(memFS)),
	)
	if err != nil {
		return nil, err
	}

	return runner, nil
}
