//go:build integration

package cli_test

import "testing"

func TestCLI_surface(t *testing.T) {
	t.Parallel()

	for _, c := range cliSurfaceCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			runCLICase(t, &c)
		})
	}
}
