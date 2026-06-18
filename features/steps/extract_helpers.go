package steps

import (
	"fmt"

	"github.com/hotchkj/glyph-shift/features/harness"
)

func expectedRangeBytes(src []byte, start, end int) ([]byte, error) {
	out, err := harness.ExpectedLineRangeBytes(src, start, end)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidRange, err)
	}

	return out, nil
}

func readOutputFile(tc *TestContext, name string) ([]byte, error) {
	data, err := tc.Ws.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("read output: %w", err)
	}

	return data, nil
}
