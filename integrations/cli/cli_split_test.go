//go:build integration

package cli_test

import "testing"

func TestCLI_split_operations(t *testing.T) {
	t.Parallel()

	for _, c := range splitOperationCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			runCLICase(t, &c)
		})
	}
}

func TestCLI_split_contract(t *testing.T) {
	t.Parallel()

	for _, c := range splitContractCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			runCLICase(t, &c)
		})
	}
}
