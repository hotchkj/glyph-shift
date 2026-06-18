package steps

import (
	"fmt"
	"path/filepath"

	"github.com/cucumber/godog"
	"github.com/hotchkj/glyph-shift/features/harness"
)

func loadTestdataInput(filename string) ([]byte, error) {
	rel := filepath.Join("testdata", "inputs", filepath.ToSlash(filepath.FromSlash(filename)))

	return readCommittedFileRelativeToFeatures(rel)
}

func registerGivenSourceFromTestdata(sc *godog.ScenarioContext, tc *TestContext) {
	// Given a source file "<wsName>" from testdata "<filename>"
	sc.Given(
		`^a source file "([^"]*)" from testdata "([^"]*)"$`,
		func(wsName, filename string) error {
			data, err := loadTestdataInput(filename)
			if err != nil {
				return err
			}

			return writeSourceFile(tc, wsName, data)
		},
	)
}

func registerGivenFilePrePopulated(sc *godog.ScenarioContext, tc *TestContext) {
	// Given the file "<wsName>" pre-populated from testdata "<filename>"
	sc.Given(
		`^the file "([^"]*)" pre-populated from testdata "([^"]*)"$`,
		func(wsName, filename string) error {
			data, err := loadTestdataInput(filename)
			if err != nil {
				return err
			}

			return writeSourceFile(tc, wsName, data)
		},
	)
}

func registerGivenFileFromTestdata(sc *godog.ScenarioContext, tc *TestContext) {
	// Given a file "<wsName>" from testdata "<filename>"
	sc.Given(
		`^a file "([^"]*)" from testdata "([^"]*)"$`,
		func(wsName, filename string) error {
			data, err := loadTestdataInput(filename)
			if err != nil {
				return err
			}

			return writeSourceFile(tc, wsName, data)
		},
	)
}

func registerGivenDirectoryExists(sc *godog.ScenarioContext, tc *TestContext) {
	// Given a directory "<name>" exists
	sc.Given(`^a directory "([^"]*)" exists$`, func(name string) error {
		if err := tc.Ws.MkdirAll(name, testFixtureDirPerm); err != nil {
			return fmt.Errorf("mkdir %q: %w", name, err)
		}

		return nil
	})
}

func registerGivenFilesExistInDirectory(sc *godog.ScenarioContext, tc *TestContext) {
	// Given files "<a>" and "<b>" exist in directory "<dir>"
	sc.Given(
		`^files "([^"]*)" and "([^"]*)" exist in directory "([^"]*)"$`,
		func(fileA, fileB, dir string) error {
			if err := tc.Ws.MkdirAll(dir, testFixtureDirPerm); err != nil {
				return fmt.Errorf("mkdir %q: %w", dir, err)
			}

			for _, name := range []string{fileA, fileB} {
				rel := filepath.Join(dir, filepath.FromSlash(name))
				if err := writeSourceFile(tc, rel, harness.ThreeLinesSharedContent); err != nil {
					return err
				}
			}

			return nil
		},
	)
}

func registerGivenBinaryFile(sc *godog.ScenarioContext, tc *TestContext) {
	// Given a binary file "<name>"
	sc.Given(`^a binary file "([^"]*)"$`, func(name string) error {
		return writeSourceFile(tc, name, harness.BinaryFileFixture())
	})
}

func registerGivenMixedEndingsComma(sc *godog.ScenarioContext, tc *TestContext) {
	// Given a file "<name>" with lines using mixed CRLF, LF, and CR endings
	// (comma variant — existing step uses "and" separators)
	sc.Given(
		`^a file "([^"]*)" with lines using mixed CRLF, LF, and CR endings$`,
		func(name string) error {
			return writeSourceFile(tc, name, harness.MixedLineEndingsBytes)
		},
	)
}

func registerGivenCRLFWithTrailingAndNoNewlineComma(sc *godog.ScenarioContext, tc *TestContext) {
	// Given a file "<name>" with CRLF endings, trailing whitespace, and no final newline
	// (comma variant — existing step uses "and" separators)
	sc.Given(
		`^a file "([^"]*)" with CRLF endings, trailing whitespace, and no final newline$`,
		func(name string) error {
			return writeSourceFile(tc, name, harness.CRLFTrailingNoFinalNewline)
		},
	)
}

func registerGivenSourceWithNSections(sc *godog.ScenarioContext, tc *TestContext) {
	// Given a source file "<name>" with <N> delimited sections
	sc.Given(
		`^a source file "([^"]*)" with (\d+) delimited sections$`,
		func(name string, n int) error {
			return writeSourceFile(tc, name, harness.DelimitedSectionsContent(n))
		},
	)
}

func registerGivenSourceWithNFencedBlocks(sc *godog.ScenarioContext, tc *TestContext) {
	// Given a source file "<name>" with <N> fenced blocks
	sc.Given(
		`^a source file "([^"]*)" with (\d+) fenced blocks$`,
		func(name string, n int) error {
			return writeSourceFile(tc, name, harness.FencedBlocksContent(n))
		},
	)
}

func registerGivenSourceWithNEmptyFencedBlocks(sc *godog.ScenarioContext, tc *TestContext) {
	// Given a source file "<name>" with <N> empty fenced blocks
	sc.Given(
		`^a source file "([^"]*)" with (\d+) empty fenced blocks$`,
		func(name string, n int) error {
			return writeSourceFile(tc, name, harness.EmptyGoFencedBlocksContent(n))
		},
	)
}

// RegisterTestdataFixtures registers Given steps for loading testdata fixtures.
func RegisterTestdataFixtures(sc *godog.ScenarioContext, tc *TestContext) {
	registerGivenSourceFromTestdata(sc, tc)
	registerGivenSourceFromEscapedTestdata(sc, tc)
	registerGivenFilePrePopulated(sc, tc)
	registerGivenFileFromTestdata(sc, tc)
	registerGivenDirectoryExists(sc, tc)
	registerGivenFilesExistInDirectory(sc, tc)
	registerGivenBinaryFile(sc, tc)
	registerGivenMixedEndingsComma(sc, tc)
	registerGivenCRLFWithTrailingAndNoNewlineComma(sc, tc)
	registerGivenSourceWithNSections(sc, tc)
	registerGivenSourceWithNFencedBlocks(sc, tc)
	registerGivenSourceWithNEmptyFencedBlocks(sc, tc)
}
