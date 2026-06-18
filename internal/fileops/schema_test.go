package fileops

import (
	"slices"
	"testing"
)

func TestBuiltinOperationSchemas_Count(t *testing.T) {
	t.Helper()

	schemas := BuiltinOperationSchemas()
	if len(schemas) != 4 {
		t.Fatalf("want 4 operations, got %d", len(schemas))
	}
}

func TestBuiltinOperationSchemas_NonEmptyMetadata(t *testing.T) {
	t.Helper()

	for _, opSchema := range BuiltinOperationSchemas() {
		if opSchema.Name == "" {
			t.Error("empty operation name")
		}

		if opSchema.Description == "" {
			t.Errorf("operation %q: empty description", opSchema.Name)
		}

		if len(opSchema.Parameters) < 1 {
			t.Errorf("operation %q: want at least one parameter", opSchema.Name)
		}

		if len(opSchema.OutputFields) < 1 {
			t.Errorf("operation %q: want at least one output field", opSchema.Name)
		}
	}
}

func TestBuiltinOperationSchemas_UniqueParameterNames(t *testing.T) {
	t.Helper()

	for _, opSchema := range BuiltinOperationSchemas() {
		seen := make(map[string]struct{})

		for _, p := range opSchema.Parameters {
			if _, ok := seen[p.Name]; ok {
				t.Errorf("operation %q: duplicate parameter name %q", opSchema.Name, p.Name)
			}

			seen[p.Name] = struct{}{}
		}
	}
}

func TestBuiltinOperationSchemas_RequiredParameters(t *testing.T) {
	t.Helper()

	cases := []struct {
		op       string
		required []string
	}{
		{op: "extract", required: []string{"source", "lines", "destination"}},
		{op: "split", required: []string{"source", "delimiter", "output-dir"}},
		{op: "blocks", required: []string{"source", "start-line", "end-line", "output-dir"}},
		{op: "transform", required: []string{"source"}},
	}

	schemas := BuiltinOperationSchemas()
	byName := make(map[string]OperationSchema, len(schemas))

	for _, s := range schemas {
		byName[s.Name] = s
	}

	for _, row := range cases {
		opSchema, ok := byName[row.op]
		if !ok {
			t.Fatalf("missing schema for %q", row.op)
		}

		req := requiredNames(opSchema.Parameters)

		for _, want := range row.required {
			if !slices.Contains(req, want) {
				t.Errorf("operation %q: required parameter %q missing (have %v)", row.op, want, req)
			}
		}
	}
}

func requiredNames(params []ParameterDescriptor) []string {
	out := make([]string, 0)

	for _, p := range params {
		if p.Required {
			out = append(out, p.Name)
		}
	}

	return out
}
