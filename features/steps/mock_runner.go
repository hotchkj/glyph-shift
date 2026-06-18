package steps

import (
	"context"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

// MockRunner implements pipeline.Runner with preset results for Layer 2 BDD tests.
// Each operation returns the configured result and error. Configure via fields
// before invoking CLI or MCP paths.
type MockRunner struct {
	extractResult   fileops.ExtractResult
	extractErr      error
	splitResult     pipeline.SplitPipelineResult
	splitErr        error
	blocksResult    pipeline.BlocksPipelineResult
	blocksErr       error
	transformResult pipeline.TransformPipelineResult
	transformErr    error
}

//nolint:gocritic // hugeParam: godog MockRunner matches pipeline.Runner; params are intentionally ignored.
func (m *MockRunner) RunExtract(_ context.Context, _ pipeline.ExtractParams) (fileops.ExtractResult, error) {
	return m.extractResult, m.extractErr
}

//nolint:gocritic // hugeParam: godog MockRunner matches pipeline.Runner; params are intentionally ignored.
func (m *MockRunner) RunSplit(_ context.Context, _ pipeline.SplitParams) (pipeline.SplitPipelineResult, error) {
	return m.splitResult, m.splitErr
}

//nolint:gocritic // hugeParam: godog MockRunner matches pipeline.Runner; params are intentionally ignored.
func (m *MockRunner) RunBlocks(_ context.Context, _ pipeline.BlocksParams) (pipeline.BlocksPipelineResult, error) {
	return m.blocksResult, m.blocksErr
}

//nolint:gocritic // hugeParam: godog MockRunner matches pipeline.Runner; params are intentionally ignored.
func (m *MockRunner) RunTransform(
	_ context.Context, _ pipeline.TransformParams,
) (pipeline.TransformPipelineResult, error) {
	return m.transformResult, m.transformErr
}
