package mcpserver

import (
	"encoding/json"
	"testing"
)

func outputSchemaByteSizes(t *testing.T) map[string]int {
	t.Helper()

	tools := []string{toolExtract, toolSplit, toolBlocks, toolTransform}
	sizes := make(map[string]int, len(tools))

	for _, name := range tools {
		schema, err := ToolOutputSchema(name)
		if err != nil {
			t.Fatalf("ToolOutputSchema(%q): %v", name, err)
		}

		raw, err := json.Marshal(schema)
		if err != nil {
			t.Fatalf("marshal output schema %q: %v", name, err)
		}

		sizes[name] = len(raw)
	}

	return sizes
}

func aggregateSchemaBytes(sizes map[string]int) int {
	total := 0
	for _, n := range sizes {
		total += n
	}

	return total
}

// TestOutputSchemaAggregateByteSize guards against unintended growth of MCP outputSchema
// descriptors (issue #12). Cap is lowered as schemas shrink.
func TestOutputSchemaAggregateByteSize(t *testing.T) {
	t.Parallel()

	const baselineAggregate = 24106
	const postOptimizationCap = 14464 // 40% below baseline (issue #12)

	sizes := outputSchemaByteSizes(t)
	got := aggregateSchemaBytes(sizes)

	if got > postOptimizationCap {
		t.Fatalf("aggregate outputSchema bytes %d exceed post-optimization cap %d (baseline %d; per-tool: %v)",
			got, postOptimizationCap, baselineAggregate, sizes)
	}

	t.Logf("per-tool bytes: %v aggregate: %d (baseline %d cap %d)", sizes, got, baselineAggregate, postOptimizationCap)
}
