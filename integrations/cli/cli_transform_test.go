//go:build integration

package cli_test

import "testing"

func TestCLI_transform_operations(t *testing.T) {
	t.Parallel()

	for _, c := range transformOperationCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			runCLICase(t, &c)
		})
	}
}

func TestCLI_transform_contract(t *testing.T) {
	t.Parallel()

	for _, c := range transformContractCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			runCLICase(t, &c)
		})
	}
}
