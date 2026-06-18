package steps

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
	"github.com/hotchkj/glyph-shift/features/harness"
)

const escapedFixtureSuffix = ".bytes"

func decodeEscapedFixture(data []byte) ([]byte, error) {
	decoded, err := harness.DecodeEscapedFixture(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errEscapedFixtureDecode, err)
	}

	return decoded, nil
}

func loadEscapedTestdataInput(filename string) ([]byte, error) {
	data, err := loadTestdataInput(filename)
	if err != nil {
		return nil, err
	}

	return decodeEscapedFixture(data)
}

func registerGivenSourceFromEscapedTestdata(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(
		`^a source file "([^"]*)" from escaped testdata "([^"]*)"$`,
		func(wsName, filename string) error {
			data, err := loadEscapedTestdataInput(filename)
			if err != nil {
				return err
			}

			return writeSourceFile(tc, wsName, data)
		},
	)
}

func loadEscapedExpected(relPath string) ([]byte, error) {
	featureRel := filepath.Join("testdata", "expected", filepath.ToSlash(filepath.FromSlash(relPath)))

	data, err := readCommittedFileRelativeToFeatures(featureRel)
	if err != nil {
		return nil, err
	}

	return decodeEscapedFixture(data)
}

func registerThenFileMatchesEscapedExpected(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^"([^"]*)" matches escaped expected "([^"]*)"$`, func(wsFile, relPath string) error {
		expectedBytes, err := loadEscapedExpected(relPath)
		if err != nil {
			return err
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

func compareEscapedExpectedDirToWorkspace(
	tc *TestContext, expectedDirName string, entries []fs.DirEntry,
) (map[string]struct{}, error) {
	expectedNames := make(map[string]struct{})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), escapedFixtureSuffix) {
			continue
		}

		actualName := strings.TrimSuffix(entry.Name(), escapedFixtureSuffix)
		expectedNames[actualName] = struct{}{}

		decodedRel := filepath.ToSlash(filepath.Join(expectedDirName, entry.Name()))

		expectedBytes, err := loadEscapedExpected(decodedRel)
		if err != nil {
			return nil, err
		}

		actualBytes, err := tc.Ws.ReadFile(filepath.Join("out", actualName))
		if err != nil {
			return nil, fmt.Errorf("read workspace file %q: %w", actualName, err)
		}

		if !bytes.Equal(actualBytes, expectedBytes) {
			return nil, fmt.Errorf("%w: file %q\nwant %q\ngot  %q",
				errBytesMismatch, actualName, string(expectedBytes), string(actualBytes))
		}
	}

	return expectedNames, nil
}

func registerThenOutputFilesMatchEscapedExpected(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^the output files match escaped expected "([^"]*)"$`, func(dirName string) error {
		expectedDirRel := filepath.Join("testdata", "expected", filepath.ToSlash(filepath.FromSlash(dirName)))

		entries, err := readCommittedDirRelativeToFeatures(expectedDirRel)
		if err != nil {
			return err
		}

		expectedNames, err := compareEscapedExpectedDirToWorkspace(tc, dirName, entries)
		if err != nil {
			return err
		}

		return assertOutDirHasNoExtraFiles(tc, expectedNames, dirName)
	})
}
