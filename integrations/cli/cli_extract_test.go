//go:build integration

package cli_test

import (
	"path/filepath"
	"testing"
)

func TestCLI_extract_operations(t *testing.T) {
	t.Parallel()

	for _, c := range extractOperationCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			runCLICase(t, &c)
		})
	}
}

func TestCLI_extract_contract(t *testing.T) {
	t.Parallel()

	for _, c := range extractContractCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			runCLICase(t, &c)
		})
	}
}

func TestCLI_extract_expected_paths_resolve(t *testing.T) {
	t.Parallel()
	// Guard that a representative expected path exists before the full matrix runs.
	matches, err := filepath.Glob(cliGoldenPath("extract/stdout/*.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no golden files found for extract/stdout")
	}
}
