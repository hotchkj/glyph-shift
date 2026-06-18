//go:build integration

package cli_test

import "testing"

func TestCLI_blocks_operations(t *testing.T) {
	t.Parallel()

	for _, c := range blocksOperationCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			runCLICase(t, &c)
		})
	}
}

func TestCLI_blocks_contract(t *testing.T) {
	t.Parallel()

	for _, c := range blocksContractCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			runCLICase(t, &c)
		})
	}
}
