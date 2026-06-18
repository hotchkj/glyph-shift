package steps

import (
	"bytes"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/hotchkj/glyph-shift/features/harness"
)

func registerExactLinesFromSource(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(
		`^"([^"]*)" contains exactly lines (\d+) through (\d+) from "([^"]*)"$`,
		func(outFile string, start, end int, srcName string) error {
			src, ok := tc.SourceFiles[srcName]
			if !ok {
				return fmt.Errorf("%w: %q", errUnknownSource, srcName)
			}

			want, err := expectedRangeBytes(src, start, end)
			if err != nil {
				return err
			}

			got, err := readOutputFile(tc, outFile)
			if err != nil {
				return err
			}

			if !bytes.Equal(got, want) {
				return fmt.Errorf("%w: want %q got %q", errBytesMismatch, string(want), string(got))
			}

			return nil
		},
	)
}

func registerStartsWith(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^"([^"]*)" starts with "([^"]*)"$`, func(file, prefix string) error {
		got, err := readOutputFile(tc, file)
		if err != nil {
			return err
		}

		want := unescapeContent(prefix)
		if !bytes.HasPrefix(got, []byte(want)) {
			return fmt.Errorf("%w: want %q at start of %q", errPrefixMismatch, want, string(got))
		}

		return nil
	})
}

func registerEndsWithLinesFromSource(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(
		`^"([^"]*)" ends with lines (\d+) through (\d+) from "([^"]*)"$`,
		func(outFile string, start, end int, srcName string) error {
			src, ok := tc.SourceFiles[srcName]
			if !ok {
				return fmt.Errorf("%w: %q", errUnknownSource, srcName)
			}

			want, err := expectedRangeBytes(src, start, end)
			if err != nil {
				return err
			}

			got, err := readOutputFile(tc, outFile)
			if err != nil {
				return err
			}

			if !bytes.HasSuffix(got, want) {
				return fmt.Errorf("%w: want %q at end of %q", errSuffixMismatch, string(want), string(got))
			}

			return nil
		},
	)
}

func registerLineTerminator(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^"([^"]*)" line (\d+) has (\w+) terminator$`, func(file string, lineNum int, ending string) error {
		data, err := readOutputFile(tc, file)
		if err != nil {
			return err
		}

		lines, err := harness.ReadLinesFrom(bytes.NewReader(data))
		if err != nil {
			return err
		}

		if lineNum < 1 || lineNum > len(lines) {
			return fmt.Errorf("%w: %d (have %d lines)", errLineOutOfRange, lineNum, len(lines))
		}

		wantTerm, err := harness.LineTerminator(ending)
		if err != nil {
			return fmt.Errorf("%w: %w", errUnknownTerminator, err)
		}

		ln := lines[lineNum-1]
		if !bytes.Equal(ln.Terminator, wantTerm) {
			return fmt.Errorf("%w: line %d want %q got %q", errTerminatorMismatch, lineNum, wantTerm, ln.Terminator)
		}

		return nil
	})
}

// RegisterExtract registers extract-specific step definitions.
func RegisterExtract(sc *godog.ScenarioContext, tc *TestContext) {
	registerExactLinesFromSource(sc, tc)
	registerStartsWith(sc, tc)
	registerEndsWithLinesFromSource(sc, tc)
	registerLineTerminator(sc, tc)
	RegisterExtractWhen(sc, tc)
	RegisterExtractPerformance(sc, tc)
}
