package pipeline

import (
	"context"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

// Runner abstracts the four pipeline operations so callers can inject
// mocked implementations for contract testing without filesystem access.
type Runner interface {
	RunExtract(ctx context.Context, params ExtractParams) (fileops.ExtractResult, error)
	RunSplit(ctx context.Context, params SplitParams) (SplitPipelineResult, error)
	RunBlocks(ctx context.Context, params BlocksParams) (BlocksPipelineResult, error)
	RunTransform(ctx context.Context, params TransformParams) (TransformPipelineResult, error)
}

// DefaultRunner delegates to pipeline.Run* implementations using injected file and path seams.
type DefaultRunner struct {
	src      SourceOpener
	out      OutputOpener
	stater   FileStater
	resolver validate.PathResolver
	fs       fileops.FileSession
}

// NewDefaultRunner validates required seams and returns a Runner wired to src, publish, stat, resolver, fs.
func NewDefaultRunner(
	src SourceOpener, out OutputOpener,
	stater FileStater, resolver validate.PathResolver,
	fs fileops.FileSession,
) (*DefaultRunner, error) {
	switch {
	case src == nil:
		return nil, ErrNilSourceOpener
	case out == nil:
		return nil, ErrNilOutputOpener
	case stater == nil:
		return nil, ErrNilFileStater
	case resolver == nil:
		return nil, validate.ErrNilPathResolver
	case fs == nil:
		return nil, fileops.ErrNilFileSession
	}

	return &DefaultRunner{src: src, out: out, stater: stater, resolver: resolver, fs: fs}, nil
}

//nolint:gocritic // hugeParam: DefaultRunner implements Runner; params mirror pipeline.RunExtract.
func (r *DefaultRunner) RunExtract(ctx context.Context, params ExtractParams) (fileops.ExtractResult, error) {
	return RunExtract(ctx, r.src, r.out, r.resolver, r.fs, params)
}

//nolint:gocritic // hugeParam: DefaultRunner implements Runner; params mirror pipeline.RunSplit.
func (r *DefaultRunner) RunSplit(ctx context.Context, params SplitParams) (SplitPipelineResult, error) {
	return RunSplit(ctx, r.src, r.out, r.resolver, r.fs, params)
}

//nolint:gocritic // hugeParam: DefaultRunner implements Runner; params mirror pipeline.RunBlocks.
func (r *DefaultRunner) RunBlocks(ctx context.Context, params BlocksParams) (BlocksPipelineResult, error) {
	return RunBlocks(ctx, r.src, r.out, r.resolver, r.fs, params)
}

func (r *DefaultRunner) RunTransform(ctx context.Context, params TransformParams) (TransformPipelineResult, error) {
	return RunTransform(ctx, r.stater, r.resolver, r.fs, params)
}
