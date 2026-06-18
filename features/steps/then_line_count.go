package steps

import (
	"bytes"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/hotchkj/glyph-shift/features/harness"
)

func registerThenLineCount(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^"([^"]*)" exists and contains (\d+) lines$`, func(file string, lineCount int) error {
		data, err := tc.Ws.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		lines, err := harness.ReadLinesFrom(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("read lines: %w", err)
		}

		if len(lines) != lineCount {
			return fmt.Errorf("%w: want %d got %d", errLineCountMismatch, lineCount, len(lines))
		}

		return nil
	})
}
