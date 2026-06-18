package steps

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/hotchkj/glyph-shift/features/harness"
)

func registerTransformThenEveryLineTerminator(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^every line in "([^"]*)" has (\w+) terminator$`, func(file, ending string) error {
		data, err := readOutputFile(tc, file)
		if err != nil {
			return err
		}

		lines, err := harness.ReadLinesFrom(bytes.NewReader(data))
		if err != nil {
			return err
		}

		wantTerm, err := harness.LineTerminator(strings.ToUpper(ending))
		if err != nil {
			return fmt.Errorf("%w: %w", errUnknownTerminator, err)
		}

		for lineIdx, ln := range lines {
			if !bytes.Equal(ln.Terminator, wantTerm) {
				return fmt.Errorf("%w: line %d want %q got %q", errTerminatorMismatch, lineIdx+1, wantTerm, ln.Terminator)
			}
		}

		return nil
	})
}

func registerTransformThenNoTrailingWS(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^no line in "([^"]*)" has trailing whitespace$`, func(file string) error {
		data, err := readOutputFile(tc, file)
		if err != nil {
			return err
		}

		lines, err := harness.ReadLinesFrom(bytes.NewReader(data))
		if err != nil {
			return err
		}

		for lineIdx, ln := range lines {
			content := ln.Content
			if len(content) == 0 {
				continue
			}

			last := content[len(content)-1]
			if last == ' ' || last == '\t' {
				return fmt.Errorf("%w: line %d", errLineTrailingWS, lineIdx+1)
			}
		}

		return nil
	})
}

func registerTransformThenNewlines(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^"([^"]*)" ends with a newline$`, func(file string) error {
		data, err := readOutputFile(tc, file)
		if err != nil {
			return err
		}

		if len(data) == 0 || data[len(data)-1] != '\n' {
			return fmt.Errorf("%w: file does not end with newline", errSuffixMismatch)
		}

		return nil
	})

	sc.Then(`^"([^"]*)" ends with exactly one newline$`, func(file string) error {
		data, err := readOutputFile(tc, file)
		if err != nil {
			return err
		}

		if len(data) == 0 {
			return fmt.Errorf("%w", errEmptyFileBytes)
		}

		if data[len(data)-1] != '\n' {
			return fmt.Errorf("%w", errMissingFinalNL)
		}

		if len(data) >= 2 && data[len(data)-2] == '\n' {
			return fmt.Errorf("%w", errMultipleFinalNL)
		}

		return nil
	})
}
