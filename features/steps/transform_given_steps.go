package steps

import (
	"fmt"
	"path/filepath"

	"github.com/cucumber/godog"
	"github.com/hotchkj/glyph-shift/features/harness"
)

func registerTransformGivenLineEndings(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(`^a file "([^"]*)" with (\d+) lines using CRLF endings$`, func(name string, lineCount int) error {
		return writeSourceFile(tc, name, harness.CRLFLineContent(lineCount))
	})

	sc.Given(`^a file "([^"]*)" with (\d+) lines using LF endings$`, func(name string, lineCount int) error {
		return writeSourceFile(tc, name, harness.LFLineContent(lineCount))
	})

	// Given a file "<name>" with <N> LF, <M> CR, and <K> CRLF line endings
	sc.Given(
		`^a file "([^"]*)" with (\d+) LF, (\d+) CR, and (\d+) CRLF line endings$`,
		func(name string, nLF, nCR, nCRLF int) error {
			return writeSourceFile(tc, name, harness.MixedEndingStatsContent(nLF, nCR, nCRLF))
		},
	)

	sc.Given(`^a file "([^"]*)" with trailing whitespace on lines$`, func(name string) error {
		return writeSourceFile(tc, name, harness.TrailingWhitespaceBytes)
	})

	sc.Given(`^a file "([^"]*)" that ends without a newline$`, func(name string) error {
		return writeSourceFile(tc, name, harness.SoloLineNoFinalNewline)
	})

	sc.Given(`^a binary file "([^"]*)" in the test directory$`, func(name string) error {
		return writeSourceFile(tc, name, harness.ImageBinaryBytes)
	})
}

func registerTransformGivenDirs(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(
		`^files "([^"]*)" and "([^"]*)" with CRLF endings in directory "([^"]*)"$`,
		func(fileA, fileB, dir string) error {
			if err := tc.Ws.MkdirAll(dir, testFixtureDirPerm); err != nil {
				return fmt.Errorf("mkdir: %w", err)
			}

			lines := harness.CRLFLineContent(globScenarioLineCount)

			for _, rel := range []string{fileA, fileB} {
				key := filepath.Join(dir, filepath.FromSlash(rel))
				if err := tc.Ws.WriteFile(key, lines, testFixtureFilePerm); err != nil {
					return fmt.Errorf("write file: %w", err)
				}

				tc.SourceFiles[key] = append([]byte(nil), lines...)
			}

			return nil
		},
	)

	sc.Given(
		`^text files with CRLF endings in "([^"]*)" and "([^"]*)"$`,
		func(dir, sub string) error {
			if err := tc.Ws.MkdirAll(dir, testFixtureDirPerm); err != nil {
				return fmt.Errorf("mkdir: %w", err)
			}

			if err := tc.Ws.MkdirAll(sub, testFixtureDirPerm); err != nil {
				return fmt.Errorf("mkdir sub: %w", err)
			}

			lines := harness.CRLFLineContent(directoryScenarioLineCount)

			aKey := filepath.Join(dir, "a.txt")
			if err := tc.Ws.WriteFile(aKey, lines, testFixtureFilePerm); err != nil {
				return fmt.Errorf("write a: %w", err)
			}

			tc.SourceFiles[filepath.Join(dir, "a.txt")] = append([]byte(nil), lines...)

			bKey := filepath.Join(sub, "b.txt")
			if err := tc.Ws.WriteFile(bKey, lines, testFixtureFilePerm); err != nil {
				return fmt.Errorf("write b: %w", err)
			}

			tc.SourceFiles[filepath.Join(sub, "b.txt")] = append([]byte(nil), lines...)

			return nil
		},
	)
}

func registerTransformGivenContent(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(`^a file "([^"]*)" with content "([^"]*)"$`, func(name, text string) error {
		return writeSourceFile(tc, name, []byte(unescapeContent(text)))
	})
}
