package steps

import (
	"fmt"

	"github.com/cucumber/godog"
)

func registerThenExitCode(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^the exit code is (\d+)$`, func(code int) error {
		if tc.ExitCode != code {
			return fmt.Errorf("%w: want %d got %d (stdout=%q stderr=%q)",
				errExitCodeMismatch, code, tc.ExitCode, tc.Stdout, tc.Stderr)
		}

		return nil
	})
}
