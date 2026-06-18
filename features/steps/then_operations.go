package steps

import (
	"fmt"

	"github.com/cucumber/godog"
)

func registerThenOperationSucceeds(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^the operation succeeds$`, func() error {
		// Any recorded operation failure (e.g. Layer 1 direct extract pipeline/parse) must fail
		// this step even when ExitCode remains 0 and LastExtractResult is nil.
		if tc.LastOperationError != nil {
			return fmt.Errorf("%w: %w", errExpectedExitZero, tc.LastOperationError)
		}

		// Layer 1 direct operations: success populates one of the Last*Result fields; do not infer
		// direct mode from ExitCode alone (non-operation Layer 2 paths may leave exit zero).
		if tc.LastExtractResult != nil || tc.LastSplitResult != nil || tc.LastBlocksResult != nil ||
			tc.LastTransformResult != nil {
			return nil
		}

		if tc.ExitCode != 0 {
			return fmt.Errorf("%w: got %d (stderr=%q stdout=%q)", errExpectedExitZero, tc.ExitCode, tc.Stderr, tc.Stdout)
		}

		return nil
	})
}

func registerThenOperationFails(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^the operation fails$`, func() error {
		if tc.LastOperationError != nil {
			return nil
		}

		if tc.ExitCode == 0 {
			return fmt.Errorf("%w: got 0 (stdout=%q)", errExpectedNonzeroExit, tc.Stdout)
		}

		return nil
	})
}

// RegisterOperations registers the operation succeeds/fails Then step definitions.
func RegisterOperations(sc *godog.ScenarioContext, tc *TestContext) {
	registerThenOperationSucceeds(sc, tc)
	registerThenOperationFails(sc, tc)
}
