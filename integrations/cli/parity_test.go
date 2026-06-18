//go:build integration

package cli_test

import "testing"

func TestCliCase_countMatchesTables(t *testing.T) {
	t.Parallel()

	got := len(extractOperationCases()) +
		len(extractContractCases()) +
		len(splitOperationCases()) +
		len(splitContractCases()) +
		len(blocksOperationCases()) +
		len(blocksContractCases()) +
		len(transformOperationCases()) +
		len(transformContractCases()) +
		len(cliSurfaceCases())

	const want = 151
	if got != want {
		t.Fatalf("cliCase count: got %d want %d (update want in parity_test.go if intentional)", got, want)
	}
}
