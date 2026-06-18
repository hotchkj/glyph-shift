package steps

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/cucumber/godog"
)

func compareExpectedOutputFileToWorkspace(tc *TestContext, expectedDirRel, fileName string) error {
	featureRel := filepath.Join(expectedDirRel, filepath.ToSlash(filepath.FromSlash(fileName)))

	expectedBytes, err := readCommittedFileRelativeToFeatures(featureRel)
	if err != nil {
		return fmt.Errorf("read expected file %q: %w", fileName, err)
	}

	actualPath := filepath.Join("out", fileName)

	actualBytes, err := tc.Ws.ReadFile(actualPath)
	if err != nil {
		return fmt.Errorf("read workspace file %q: %w", actualPath, err)
	}

	if !bytes.Equal(actualBytes, expectedBytes) {
		return fmt.Errorf("%w: file %q\nwant %q\ngot  %q",
			errBytesMismatch, fileName, string(expectedBytes), string(actualBytes))
	}

	return nil
}

func compareExpectedDirToWorkspace(
	tc *TestContext,
	expectedDirRel string,
	entries []fs.DirEntry,
) (map[string]struct{}, error) {
	expectedNames := make(map[string]struct{})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		expectedNames[entry.Name()] = struct{}{}

		if cmpErr := compareExpectedOutputFileToWorkspace(tc, expectedDirRel, entry.Name()); cmpErr != nil {
			return nil, cmpErr
		}
	}

	return expectedNames, nil
}

func assertOutDirHasNoExtraFiles(tc *TestContext, expectedNames map[string]struct{}, dirName string) error {
	actualEntries, err := tc.Ws.ReadDir("out")
	if err != nil {
		return fmt.Errorf("read workspace out dir: %w", err)
	}

	for _, entry := range actualEntries {
		if entry.IsDir {
			continue
		}

		if _, ok := expectedNames[entry.Name]; !ok {
			return fmt.Errorf("%w: unexpected file %q in out/ not present in expected set %q",
				errBytesMismatch, entry.Name, dirName)
		}
	}

	return nil
}

func matchOutputFilesToExpected(tc *TestContext, dirName string) error {
	expectedDirRel := filepath.Join("testdata", "expected", filepath.ToSlash(filepath.FromSlash(dirName)))

	entries, err := readCommittedDirRelativeToFeatures(expectedDirRel)
	if err != nil {
		return err
	}

	expectedNames, err := compareExpectedDirToWorkspace(tc, expectedDirRel, entries)
	if err != nil {
		return err
	}

	return assertOutDirHasNoExtraFiles(tc, expectedNames, dirName)
}

func registerThenOutputFilesMatchExpected(sc *godog.ScenarioContext, tc *TestContext) {
	// Then the output files match expected "<dirName>"
	// Compares workspace out/{filename} against testdata/expected/{dirName}/{filename}
	// Also asserts no extra files exist in out/ beyond what the expected set defines.
	sc.Then(`^the output files match expected "([^"]*)"$`, func(dirName string) error {
		return matchOutputFilesToExpected(tc, dirName)
	})
}

func registerThenFileMatchesExpected(sc *godog.ScenarioContext, tc *TestContext) {
	// Then "<wsFile>" matches expected "<relPath>"
	// Compares a single workspace file against testdata/expected/{relPath}
	sc.Then(`^"([^"]*)" matches expected "([^"]*)"$`, func(wsFile, relPath string) error {
		featureRel := filepath.Join(
			"testdata", "expected", filepath.ToSlash(filepath.FromSlash(relPath)))

		expectedBytes, err := readCommittedFileRelativeToFeatures(featureRel)
		if err != nil {
			return fmt.Errorf("read expected %q: %w", relPath, err)
		}

		actualBytes, err := readOutputFile(tc, wsFile)
		if err != nil {
			return err
		}

		if !bytes.Equal(actualBytes, expectedBytes) {
			return fmt.Errorf("%w: file %q\nwant %q\ngot  %q",
				errBytesMismatch, wsFile, string(expectedBytes), string(actualBytes))
		}

		return nil
	})
}

func registerThenBeginsWithTestdata(sc *godog.ScenarioContext, tc *TestContext) {
	// Then "<wsFile>" begins with the content of testdata "<filename>"
	sc.Then(
		`^"([^"]*)" begins with the content of testdata "([^"]*)"$`,
		func(wsFile, tdFile string) error {
			featureRel := filepath.Join("testdata", "inputs", filepath.ToSlash(filepath.FromSlash(tdFile)))

			expected, err := readCommittedFileRelativeToFeatures(featureRel)
			if err != nil {
				return fmt.Errorf("read committed testdata %q: %w", tdFile, err)
			}

			actual, err := readOutputFile(tc, wsFile)
			if err != nil {
				return err
			}

			if !bytes.HasPrefix(actual, expected) {
				return fmt.Errorf("%w: %q does not begin with content of %q", errPrefixMismatch, wsFile, tdFile)
			}

			return nil
		},
	)
}

// RegisterGolden registers golden file comparison Then step definitions.
func RegisterGolden(sc *godog.ScenarioContext, tc *TestContext) {
	registerThenOutputFilesMatchExpected(sc, tc)
	registerThenFileMatchesExpected(sc, tc)
	registerThenFileMatchesEscapedExpected(sc, tc)
	registerThenOutputFilesMatchEscapedExpected(sc, tc)
	registerThenBeginsWithTestdata(sc, tc)
}
