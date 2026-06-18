package steps

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/fsnorm"
)

const resultVerbChanged = "changed"

func directNounVerbLinesExtracted(tc *TestContext, expectedCount int, noun, verb string) (bool, error) {
	if noun != "lines" || verb != "extracted" || tc.LastExtractResult == nil {
		return false, nil
	}

	got := tc.LastExtractResult.LinesExtracted
	if got != expectedCount {
		return true, fmt.Errorf(
			"%w: noun lines verb extracted expected %d got %d (direct extract LinesExtracted)",
			errResultNounVerbCountMismatch, expectedCount, got,
		)
	}

	return true, nil
}

func directNounVerbFilesCreated(tc *TestContext, expectedCount int, noun, verb string) (bool, error) {
	if noun != "files" || verb != "created" {
		return false, nil
	}

	switch {
	case tc.LastSplitResult != nil:
		got := len(tc.LastSplitResult.Files)
		if got != expectedCount {
			return true, fmt.Errorf(
				"%w: noun files verb created expected %d got %d (direct split Files)",
				errResultNounVerbCountMismatch, expectedCount, got,
			)
		}

		return true, nil
	case tc.LastBlocksResult != nil:
		got := len(tc.LastBlocksResult.Files)
		if got != expectedCount {
			return true, fmt.Errorf(
				"%w: noun files verb created expected %d got %d (direct blocks Files)",
				errResultNounVerbCountMismatch, expectedCount, got,
			)
		}

		return true, nil
	default:
		return false, nil
	}
}

func directNounVerbEndingsChanged(tc *TestContext, expectedCount int, noun, verb string) (bool, error) {
	if noun != "endings" || verb != resultVerbChanged || tc.LastTransformResult == nil {
		return false, nil
	}

	got := tc.LastTransformResult.Result.EndingsChanged
	if got != expectedCount {
		return true, fmt.Errorf(
			"%w: noun endings verb changed expected %d got %d (direct transform EndingsChanged)",
			errResultNounVerbCountMismatch, expectedCount, got,
		)
	}

	return true, nil
}

// directNounVerbAssert handles Layer 1 pipeline-backed noun/verb counts before stdout JSON fallback.
func directNounVerbAssert(tc *TestContext, countStr, noun, verb string) (handled bool, err error) {
	expectedCount, aerr := strconv.Atoi(countStr)
	if aerr != nil {
		return false, fmt.Errorf("parse count: %w", aerr)
	}

	if ok, e := directNounVerbLinesExtracted(tc, expectedCount, noun, verb); ok {
		return true, e
	}

	if ok, e := directNounVerbFilesCreated(tc, expectedCount, noun, verb); ok {
		return true, e
	}

	if ok, e := directNounVerbEndingsChanged(tc, expectedCount, noun, verb); ok {
		return true, e
	}

	return false, nil
}

func assertTransformFileChangeDirect(res *fileops.TransformFileResult, status string) error {
	switch status {
	case "changed":
		if !res.WouldChange {
			return fmt.Errorf(gotValueFormat, errExpectedChangedTrue, res.WouldChange)
		}
	case "not changed":
		if res.WouldChange {
			return fmt.Errorf(gotValueFormat, errExpectedChangedFalse, res.WouldChange)
		}
	case "skipped":
		if !res.Skipped {
			return fmt.Errorf(gotValueFormat, errExpectedSkippedTrue, res.Skipped)
		}
	default:
		return fmt.Errorf("%w: %q", errUnknownFileChangeStatus, status)
	}

	return nil
}

func directSplitBlocksOutputBasenames(tc *TestContext) []string {
	switch {
	case tc.LastSplitResult != nil:
		files := tc.LastSplitResult.Files
		if files == nil {
			return []string{}
		}

		return files
	case tc.LastBlocksResult != nil:
		files := tc.LastBlocksResult.Files
		if files == nil {
			return []string{}
		}

		return files
	default:
		return nil
	}
}

func assertBasenameSliceMatchesWant(want, arr []string) error {
	if len(arr) != len(want) {
		return fmt.Errorf("%w: would_create: want %d got %d", errFileCountMismatch, len(want), len(arr))
	}

	for wantIdx := range want {
		gotStr := arr[wantIdx]
		gotBase := filepath.Base(gotStr)
		if fsnorm.Canonical(gotBase) != fsnorm.Canonical(want[wantIdx]) {
			return fmt.Errorf(
				"%w: would_create index %d want basename %q got %q (full path %q)",
				errWouldCreatePathMismatch, wantIdx, want[wantIdx], gotBase, gotStr,
			)
		}
	}

	return nil
}
