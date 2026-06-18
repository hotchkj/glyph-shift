package steps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/cucumber/godog"
	"github.com/hotchkj/glyph-shift/features/harness"
)

func assertOutputFileLinesCRLF(data []byte, entryName string) error {
	wantTerm := []byte{'\r', '\n'}

	lines, err := harness.ReadLinesFrom(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse lines in %q: %w", entryName, err)
	}

	for lineIdx, ln := range lines {
		if !bytes.Equal(ln.Terminator, wantTerm) {
			return fmt.Errorf("%w: file %q line %d: want CRLF got %q",
				errTerminatorMismatch, entryName, lineIdx+1, ln.Terminator)
		}
	}

	return nil
}

func registerEveryOutputFileCRLF(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^every output file has CRLF terminator$`, func() error {
		entries, err := tc.Ws.ReadDir("out")
		if err != nil {
			return fmt.Errorf("read out dir: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir {
				continue
			}

			data, err := tc.Ws.ReadFile(filepath.Join("out", entry.Name))
			if err != nil {
				return fmt.Errorf("read %q: %w", entry.Name, err)
			}

			if crlfErr := assertOutputFileLinesCRLF(data, entry.Name); crlfErr != nil {
				return crlfErr
			}
		}

		return nil
	})
}

func registerAllOutputFilesHaveExt(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^all output files have extension "([^"]*)"$`, func(ext string) error {
		entries, err := tc.Ws.ReadDir("out")
		if err != nil {
			return fmt.Errorf("read out dir: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir {
				continue
			}

			if filepath.Ext(entry.Name) != ext {
				return fmt.Errorf("%w: file %q has extension %q want %q",
					errFileExtensionMismatch, entry.Name, filepath.Ext(entry.Name), ext)
			}
		}

		return nil
	})
}

func registerNFilesWouldBeCreated(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^(\d+) files would be created$`, func(countStr string) error {
		want, err := strconv.Atoi(countStr)
		if err != nil {
			return fmt.Errorf("parse count: %w", err)
		}

		if direct := directSplitBlocksOutputBasenames(tc); direct != nil {
			got := len(direct)
			if got != want {
				return fmt.Errorf(
					"%w: would_create: want %d got %d (direct split/blocks Files)",
					errFileCountMismatch, want, got,
				)
			}

			return nil
		}

		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Stdout), &obj); err != nil {
			return fmt.Errorf("parse stdout JSON: %w", err)
		}

		raw, ok := obj["would_create"]
		if !ok {
			return errStdoutWouldCreateMissing
		}

		arr, ok := raw.([]interface{})
		if !ok {
			return errStdoutWouldCreateNotArray
		}

		if len(arr) != want {
			return fmt.Errorf("%w: would_create: want %d got %d", errFileCountMismatch, want, len(arr))
		}

		return nil
	})
}

func registerThenDirectoryDoesNotExist(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^directory "([^"]*)" does not exist$`, func(dir string) error {
		_, err := tc.Ws.Stat(dir)
		if err == nil {
			return fmt.Errorf("%w: %q", errDirectoryShouldNotExistButDoes, dir)
		}

		if harness.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("stat %q: %w", dir, err)
	})
}

// RegisterSplitExtra registers split-specific Then step definitions.
func RegisterSplitExtra(sc *godog.ScenarioContext, tc *TestContext) {
	registerEveryOutputFileCRLF(sc, tc)
	registerAllOutputFilesHaveExt(sc, tc)
	registerNFilesWouldBeCreated(sc, tc)
	registerThenDirectoryDoesNotExist(sc, tc)
}
