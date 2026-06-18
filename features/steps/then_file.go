package steps

import (
	"bytes"
	"fmt"

	"github.com/cucumber/godog"
)

func registerThenFileContent(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^"([^"]*)" content is "([^"]*)"$`, func(file, want string) error {
		data, err := tc.Ws.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		if string(data) != unescapeContent(want) {
			return fmt.Errorf("%w: want %q got %q", errContentMismatch, unescapeContent(want), string(data))
		}

		return nil
	})

	sc.Then(`^"([^"]*)" is unchanged$`, func(file string) error {
		want, ok := tc.SourceFiles[file]
		if !ok {
			return fmt.Errorf("%w: %q", errNoStoredSource, file)
		}

		data, err := tc.Ws.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		if !bytes.Equal(data, want) {
			return fmt.Errorf("%w: want %q got %q", errFileChanged, string(want), string(data))
		}

		return nil
	})

	sc.Then(`^"([^"]*)" does not exist$`, func(file string) error {
		_, err := tc.Ws.ReadFile(file)
		if err == nil {
			return fmt.Errorf("%w: %q should not exist", errMissingFile, file)
		}

		// Accept both "not found" and path-rejection errors: both confirm the file
		// is not accessible from the workspace, satisfying the security scenario intent.
		return nil
	})
}
