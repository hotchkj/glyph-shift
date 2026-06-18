package steps

import (
	"fmt"
	"path/filepath"

	"github.com/cucumber/godog"
	"github.com/hotchkj/glyph-shift/features/harness"
)

func registerDirectoryFileCount(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^directory "([^"]*)" contains (\d+) files?$`, func(dir string, want int) error {
		entries, err := tc.Ws.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("read directory: %w", err)
		}

		fileCount := 0
		for _, e := range entries {
			if !e.IsDir {
				fileCount++
			}
		}

		if fileCount != want {
			return fmt.Errorf("%w: directory %q: want %d files got %d", errFileCountMismatch, dir, want, fileCount)
		}

		return nil
	})
}

func registerFileExists(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^"([^"]*)" exists$`, func(rel string) error {
		st, err := tc.Ws.Stat(rel)
		if err != nil {
			if harness.IsNotExist(err) {
				return fmt.Errorf("%w: %q", errMissingFile, rel)
			}

			return fmt.Errorf("stat: %w", err)
		}

		if st.IsDir() {
			return fmt.Errorf("%w: %q is a directory", errExpectedFile, rel)
		}

		return nil
	})
}

func registerDirectoryWithFile(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(`^directory "([^"]*)" exists with file "([^"]*)"$`, func(dir, name string) error {
		if err := tc.Ws.MkdirAll(dir, testFixtureDirPerm); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}

		rel := filepath.Join(dir, filepath.FromSlash(name))
		if err := tc.Ws.WriteFile(rel, []byte("placeholder\n"), testFixtureFilePerm); err != nil {
			return fmt.Errorf("write file: %w", err)
		}

		return nil
	})
}

// RegisterSplit registers split-specific step definitions.
func RegisterSplit(sc *godog.ScenarioContext, tc *TestContext) {
	registerDirectoryFileCount(sc, tc)
	registerFileExists(sc, tc)
	registerDirectoryWithFile(sc, tc)
	RegisterSplitWhen(sc, tc)
	RegisterSplitExtra(sc, tc)
	RegisterSplitBlocksPerformance(sc, tc)
}
