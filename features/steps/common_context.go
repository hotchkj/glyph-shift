package steps

import (
	"fmt"
	"strings"

	"github.com/hotchkj/glyph-shift/features/harness"
	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
)

// TestContext holds shared state for BDD scenarios.
type TestContext struct {
	Ws          *harness.Workspace
	Stdout      string
	Stderr      string
	ExitCode    int
	SourceFiles map[string][]byte // logical name -> original file content bytes

	MCPIsError bool
	MCPError   *operationErrorFields

	// Layer 2: when non-nil, CLI and MCP dispatch use this mock instead of real operations
	MockRunner *MockRunner
	// MCPStructuredContent is the last tool call's structuredContent as a JSON object map
	MCPStructuredContent map[string]interface{}
	// MCPContentJSON is the last tool call's text content JSON (decoded object); mirrors structuredContent.
	MCPContentJSON map[string]interface{}

	// Layer 1 extract direct dispatch: last pipeline extract outcome (no CLI stdout/stderr).
	LastExtractResult *fileops.ExtractResult
	// Layer 1 split/blocks/transform direct dispatch pipeline outcomes (no CLI stdout/stderr).
	LastSplitResult     *pipeline.SplitPipelineResult
	LastBlocksResult    *pipeline.BlocksPipelineResult
	LastTransformResult *pipeline.TransformPipelineResult
	LastOperationError  error
	// LastOperationErrorFallbackPath mirrors cmd exitCodeForPipelineErr primary-path fallbacks for
	// ClassifyOperationError alignment in Layer 1 Then steps.
	LastOperationErrorFallbackPath string
	LastPreviewDestPath            string

	// PerfExtractBySource records the last successful pipeline measurement per logical source name.
	PerfExtractBySource map[string]testutil.ExtractMeasurement
	// LastPerfExtract is the measurement from the most recent successful extract/preview When step.
	LastPerfExtract testutil.ExtractMeasurement

	// PerfSplitPipelineBySource records inline-source split pipeline measurements keyed by scenario source name.
	PerfSplitPipelineBySource map[string]testutil.SplitPipelinePerfMeasurement
	// PerfBlocksPipelineBySource records inline-source blocks pipeline measurements keyed by scenario source name.
	PerfBlocksPipelineBySource map[string]testutil.BlocksPipelinePerfMeasurement
	// LastPerfSplitPipeline is the last split inline measurement stored for performance contract steps.
	LastPerfSplitPipeline testutil.SplitPipelinePerfMeasurement
	// LastPerfBlocksPipeline is the last blocks inline measurement stored for performance contract steps.
	LastPerfBlocksPipeline testutil.BlocksPipelinePerfMeasurement

	// PerfTransformBySource records transform pipeline perf measurements keyed by scenario source name.
	PerfTransformBySource map[string]testutil.TransformPipelinePerfMeasurement
	// LastPerfTransform is the measurement from the most recent transform perf When step.
	LastPerfTransform testutil.TransformPipelinePerfMeasurement

	WorkspaceSymlinks map[string]workspaceSymlink
}

// NewTestContext creates a test context with a fresh in-memory workspace.
func NewTestContext() *TestContext {
	ws, err := harness.NewWorkspace()
	if err != nil {
		panic(fmt.Sprintf("create in-memory workspace: %v", err))
	}

	return &TestContext{
		Ws:                         ws,
		SourceFiles:                map[string][]byte{},
		PerfExtractBySource:        map[string]testutil.ExtractMeasurement{},
		PerfSplitPipelineBySource:  map[string]testutil.SplitPipelinePerfMeasurement{},
		PerfBlocksPipelineBySource: map[string]testutil.BlocksPipelinePerfMeasurement{},
		PerfTransformBySource:      map[string]testutil.TransformPipelinePerfMeasurement{},
		WorkspaceSymlinks:          map[string]workspaceSymlink{},
	}
}

// Cleanup releases the in-memory workspace.
func (tc *TestContext) Cleanup() {
	if tc.Ws != nil {
		_ = tc.Ws.Close()
		tc.Ws = nil
	}
}

// resetOperationResult clears Layer 1 direct-operation result fields before a new pipeline-backed When step.
func (tc *TestContext) resetOperationResult() {
	tc.LastExtractResult = nil
	tc.LastSplitResult = nil
	tc.LastBlocksResult = nil
	tc.LastTransformResult = nil
	tc.LastOperationError = nil
	tc.LastOperationErrorFallbackPath = ""
	tc.LastPreviewDestPath = ""
}

func unescapeContent(s string) string {
	s = strings.ReplaceAll(s, `\r`, "\r")
	s = strings.ReplaceAll(s, `\n`, "\n")

	return s
}
