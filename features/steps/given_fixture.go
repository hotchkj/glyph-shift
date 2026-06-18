package steps

import (
	"fmt"
	"path/filepath"

	"github.com/cucumber/godog"
)

// RegisterFixtures registers fixture loading step definitions.
func RegisterFixtures(sc *godog.ScenarioContext, tc *TestContext) {
	RegisterTestdataFixtures(sc, tc)

	sc.Given(
		`^the source file "([^"]*)" from testdata$`,
		func(name string) error {
			rel := filepath.Join("testdata", filepath.ToSlash(filepath.FromSlash(name)))

			data, err := readCommittedFileRelativeToFeatures(rel)
			if err != nil {
				return fmt.Errorf("read committed testdata source %q: %w", name, err)
			}

			// Write to workspace
			if err := tc.Ws.WriteFile(name, data, workspaceFilePerm); err != nil {
				return fmt.Errorf("write to workspace: %w", err)
			}

			// Store in SourceFiles
			tc.SourceFiles[name] = data

			return nil
		},
	)
}
