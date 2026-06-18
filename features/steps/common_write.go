package steps

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/hotchkj/glyph-shift/features/harness"
)

const (
	testFixtureDirPerm  = 0o750
	testFixtureFilePerm = 0o600
)

func writeSourceFile(tc *TestContext, name string, data []byte) error {
	if err := tc.Ws.WriteFile(name, data, testFixtureFilePerm); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	tc.SourceFiles[name] = append([]byte(nil), data...)

	return nil
}

func registerNumberedSource(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(`^a source file "([^"]*)" with (\d+) numbered lines$`, func(name string, lineCount int) error {
		return writeSourceFile(tc, name, harness.LFLineContent(lineCount))
	})
}

func registerDocstringSource(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(`^a source file "([^"]*)" with content:$`, func(name string, doc *godog.DocString) error {
		return writeSourceFile(tc, name, []byte(doc.Content))
	})
}

func registerExistingFiles(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(`^a file "([^"]*)" already exists$`, func(name string) error {
		return writeSourceFile(tc, name, []byte{})
	})

	sc.Given(`^a file "([^"]*)" already exists with content "([^"]*)"$`, func(name, text string) error {
		return writeSourceFile(tc, name, []byte(unescapeContent(text)))
	})

	sc.Given(`^directory "([^"]*)" does not exist$`, func(path string) error {
		if err := tc.Ws.RemoveAll(path); err != nil && !harness.IsNotExist(err) {
			return fmt.Errorf("remove directory: %w", err)
		}

		return nil
	})
}
