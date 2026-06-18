package steps

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/hotchkj/glyph-shift/internal/fsnorm"
)

const (
	gotPathTypeFormat  = "%w %q: got %T"
	gotTypeFormat      = "%w: got %T"
	gotValueFormat     = "%w: got %v"
	parseStdoutJSONFmt = "parse stdout json: %w"
	parseStderrJSONFmt = "parse stderr glyph-shift json: %w (stderr=%q)"
)

func dotPathIntoArray(current interface{}, part string, idx int) (interface{}, error) {
	arr, ok := current.([]interface{})
	if !ok {
		return nil, fmt.Errorf(gotPathTypeFormat, errPathTraversalExpectedArray, part, current)
	}
	if idx < 0 || idx >= len(arr) {
		return nil, fmt.Errorf("%w: index %d, length %d", errPathTraversalIndexOutOfRange, idx, len(arr))
	}

	return arr[idx], nil
}

func dotPathIntoMap(current interface{}, part string) (interface{}, error) {
	m, ok := current.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(gotPathTypeFormat, errPathTraversalExpectedMap, part, current)
	}
	val, ok := m[part]
	if !ok {
		return nil, fmt.Errorf("%w %q", errPathTraversalMissingMapKey, part)
	}

	return val, nil
}

func dotPathAdvance(current interface{}, part string) (interface{}, error) {
	if current == nil {
		return nil, fmt.Errorf("%w %q", errPathTraversalNilSegment, part)
	}
	if idx, err := strconv.Atoi(part); err == nil {
		return dotPathIntoArray(current, part, idx)
	}

	return dotPathIntoMap(current, part)
}

// dotPathGet retrieves a value from a nested map/array structure using dot-path notation.
// Numeric path segments index into []interface{}, string segments index into map[string]interface{}.
func dotPathGet(obj interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := obj

	for _, part := range parts {
		next, err := dotPathAdvance(current, part)
		if err != nil {
			return nil, err
		}
		current = next
	}

	return current, nil
}

// nounVerbToField maps (noun, verb) pairs to JSON field paths.
// Returns the field path and whether the result is a count (array length vs direct value).
func nounVerbToField(noun, verb string) (fieldPath string, isCount bool, err error) {
	// Map of (noun, verb) -> (field_path, is_count)
	mappings := map[string]map[string]struct {
		path    string
		isCount bool
	}{
		"lines": {
			"extracted": {path: "lines_extracted", isCount: false},
		},
		"files": {
			"created": {path: "files_created", isCount: true},
		},
		"endings": {
			"changed": {path: "endings_changed", isCount: false},
		},
	}

	verbMap, ok := mappings[noun]
	if !ok {
		return "", false, fmt.Errorf("%w: %q", errUnknownResultNoun, noun)
	}

	mapping, ok := verbMap[verb]
	if !ok {
		return "", false, fmt.Errorf("%w: verb %q noun %q", errUnknownResultVerb, verb, noun)
	}

	return mapping.path, mapping.isCount, nil
}

func assertStdoutNounVerbCount(tc *TestContext, countStr, noun, verb string) error {
	expectedCount, err := strconv.Atoi(countStr)
	if err != nil {
		return fmt.Errorf("parse count: %w", err)
	}

	return assertStdoutFieldCount(tc, expectedCount, noun, verb)
}

func assertStdoutFieldCount(tc *TestContext, expectedCount int, noun, verb string) error {
	var obj map[string]interface{}
	if umErr := json.Unmarshal([]byte(tc.Stdout), &obj); umErr != nil {
		return fmt.Errorf(parseStdoutJSONFmt, umErr)
	}

	fieldPath, isCount, err := nounVerbToField(noun, verb)
	if err != nil {
		return err
	}

	val, err := dotPathGet(obj, fieldPath)
	if err != nil {
		return fmt.Errorf("dot-path lookup for %q: %w", fieldPath, err)
	}

	var gotCount int
	if isCount {
		arr, ok := val.([]interface{})
		if !ok {
			return fmt.Errorf(gotPathTypeFormat, errExpectedJSONArrayAtPath, fieldPath, val)
		}
		gotCount = len(arr)
	} else {
		switch v := val.(type) {
		case float64:
			gotCount = int(v)
		default:
			return fmt.Errorf(gotPathTypeFormat, errExpectedNumberAtPath, fieldPath, val)
		}
	}

	if gotCount != expectedCount {
		return fmt.Errorf(
			"%w: noun %s verb %s expected %d got %d",
			errResultNounVerbCountMismatch, noun, verb, expectedCount, gotCount,
		)
	}

	return nil
}

func assertStdoutBlockContentAndEmptyCounts(tc *TestContext, wantContent, wantEmpty int) error {
	var obj map[string]interface{}
	if umErr := json.Unmarshal([]byte(tc.Stdout), &obj); umErr != nil {
		return fmt.Errorf(parseStdoutJSONFmt, umErr)
	}

	contentRaw, ok := obj["content_blocks_found"].(float64)
	if !ok {
		return fmt.Errorf(gotPathTypeFormat, errExpectedNumberAtPath, "content_blocks_found", obj["content_blocks_found"])
	}

	emptyCount := 0
	if value, exists := obj["empty_blocks_found"]; exists {
		emptyRaw, ok := value.(float64)
		if !ok {
			return fmt.Errorf(gotPathTypeFormat, errExpectedNumberAtPath, "empty_blocks_found", value)
		}

		emptyCount = int(emptyRaw)
	}

	gotContent := int(contentRaw)
	if gotContent != wantContent || emptyCount != wantEmpty {
		return fmt.Errorf(
			"%w: content_blocks_found want %d got %d; empty_blocks_found want %d got %d",
			errResultNounVerbCountMismatch, wantContent, gotContent, wantEmpty, emptyCount,
		)
	}

	return nil
}

func registerThenNounVerbCount(sc *godog.ScenarioContext, tc *TestContext) {
	// Pattern 1: N noun verb (handles both singular "was" and plural "were", singular/plural nouns)
	sc.Then(
		`^(\d+) (line|lines|file|files|block|blocks|ending|endings) (was|were) (extracted|created|found|changed)$`,
		func(
			countStr string,
			noun string,
			_ string,
			verb string,
		) error {
			// Normalise to plural form for mapping lookup
			switch noun {
			case "line":
				noun = "lines"
			case "file":
				noun = "files"
			case "block":
				noun = "blocks"
			case "ending":
				noun = "endings"
			}

			if handled, derr := directNounVerbAssert(tc, countStr, noun, verb); handled {
				return derr
			}

			return assertStdoutNounVerbCount(tc, countStr, noun, verb)
		},
	)
}

func assertStdoutFileChangeStatus(tc *TestContext, status string) error {
	var obj map[string]interface{}
	if umErr := json.Unmarshal([]byte(tc.Stdout), &obj); umErr != nil {
		return fmt.Errorf(parseStdoutJSONFmt, umErr)
	}

	return fileChangeStatusFromObj(obj, status)
}

func fileChangeStatusFromObj(obj map[string]interface{}, status string) error {
	switch status {
	case "changed":
		changed, ok := obj["changed"].(bool)
		if !ok || !changed {
			return fmt.Errorf(gotValueFormat, errExpectedChangedTrue, obj["changed"])
		}
	case "not changed":
		changed, ok := obj["changed"].(bool)
		if !ok || changed {
			return fmt.Errorf(gotValueFormat, errExpectedChangedFalse, obj["changed"])
		}
	case "skipped":
		skipped, ok := obj["skipped"].(bool)
		if !ok || !skipped {
			return fmt.Errorf(gotValueFormat, errExpectedSkippedTrue, obj["skipped"])
		}
	default:
		return fmt.Errorf("%w: %q", errUnknownFileChangeStatus, status)
	}

	return nil
}

func registerThenFileChangeStatus(sc *godog.ScenarioContext, tc *TestContext) {
	// Pattern 2: file change status
	sc.Then(
		`^the file was (changed|not changed|skipped)$`,
		func(status string) error {
			if tc.LastTransformResult != nil {
				res := tc.LastTransformResult.Result

				return assertTransformFileChangeDirect(&res, status)
			}

			return assertStdoutFileChangeStatus(tc, status)
		},
	)
}

func registerThenPreviewExtractLines(sc *godog.ScenarioContext, tc *TestContext) {
	// Pattern 4: preview mode — CLI maps to would_extract_lines; Layer 1 direct uses ExtractResult.LinesExtracted.
	sc.Then(
		`^(\d+) (line|lines) would be extracted$`,
		func(countStr, _ string) error {
			expectedCount, err := strconv.Atoi(countStr)
			if err != nil {
				return fmt.Errorf("parse count: %w", err)
			}

			if tc.LastExtractResult != nil {
				got := tc.LastExtractResult.LinesExtracted
				if got != expectedCount {
					return fmt.Errorf(
						"%w: expected %d got %d (direct preview LinesExtracted)",
						errWouldExtractLineCountMismatch, expectedCount, got,
					)
				}

				return nil
			}

			var obj map[string]interface{}
			if umErr := json.Unmarshal([]byte(tc.Stdout), &obj); umErr != nil {
				return fmt.Errorf(parseStdoutJSONFmt, umErr)
			}

			val, derr := dotPathGet(obj, "would_extract_lines")
			if derr != nil {
				return fmt.Errorf("dot-path lookup for would_extract_lines: %w", derr)
			}

			gotCount, ok := val.(float64)
			if !ok {
				return fmt.Errorf(gotTypeFormat, errExpectedNumberWouldExtract, val)
			}

			if int(gotCount) != expectedCount {
				return fmt.Errorf("%w: expected %d got %d", errWouldExtractLineCountMismatch, expectedCount, int(gotCount))
			}

			return nil
		},
	)
}

func registerThenPreviewWouldCreate(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(
		`^preview would create "([^"]*)"$`,
		func(rel string) error {
			if tc.LastPreviewDestPath != "" {
				want := fsnorm.Canonical(rel)
				got := fsnorm.Canonical(tc.LastPreviewDestPath)
				if got != want {
					return fmt.Errorf("%w: want %q got %q (direct preview LastPreviewDestPath)", errWouldCreatePathMismatch, want, got)
				}

				return nil
			}

			var obj map[string]interface{}
			if umErr := json.Unmarshal([]byte(tc.Stdout), &obj); umErr != nil {
				return fmt.Errorf(parseStdoutJSONFmt, umErr)
			}

			raw, ok := obj["would_create"].(string)
			if !ok {
				return fmt.Errorf(gotTypeFormat, errStdoutWouldCreateNotString, obj["would_create"])
			}

			want := fsnorm.Canonical(rel)
			if raw != want {
				return fmt.Errorf("%w: want %q got %q", errWouldCreatePathMismatch, want, raw)
			}

			return nil
		},
	)
}

func parseCommaSeparatedOutputBasenames(csv string) ([]string, error) {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			return nil, errEmptyOutputFileListEntry
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, errNoOutputBasenames
	}

	return out, nil
}

func assertPreviewWouldCreateOutputFileList(tc *TestContext, csv string) error {
	want, err := parseCommaSeparatedOutputBasenames(csv)
	if err != nil {
		return err
	}

	if direct := directSplitBlocksOutputBasenames(tc); direct != nil {
		return assertBasenameSliceMatchesWant(want, direct)
	}

	var obj map[string]interface{}
	if umErr := json.Unmarshal([]byte(tc.Stdout), &obj); umErr != nil {
		return fmt.Errorf(parseStdoutJSONFmt, umErr)
	}

	raw, ok := obj["would_create"]
	if !ok {
		return errStdoutWouldCreateMissing
	}

	arr, ok := raw.([]interface{})
	if !ok {
		return errStdoutWouldCreateNotArray
	}

	if len(arr) != len(want) {
		return fmt.Errorf("%w: would_create: want %d got %d", errFileCountMismatch, len(want), len(arr))
	}

	parsed := make([]string, len(arr))
	for idx := range arr {
		gotStr, elemOK := arr[idx].(string)
		if !elemOK {
			return fmt.Errorf(gotTypeFormat, errStdoutWouldCreateNotString, arr[idx])
		}
		parsed[idx] = gotStr
	}

	return assertBasenameSliceMatchesWant(want, parsed)
}

func registerThenPreviewWouldCreateOutputFilesCSV(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(
		`^the preview would create output files "([^"]*)"$`,
		func(csv string) error {
			return assertPreviewWouldCreateOutputFileList(tc, csv)
		},
	)
}

func registerThenBlockStdoutCounts(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(
		`^the operation reports (\d+) non-empty blocks? and (\d+) empty blocks?$`,
		func(contentStr, emptyStr string) error {
			wantContent, err := strconv.Atoi(contentStr)
			if err != nil {
				return fmt.Errorf("parse non-empty block count: %w", err)
			}

			wantEmpty, err := strconv.Atoi(emptyStr)
			if err != nil {
				return fmt.Errorf("parse empty block count: %w", err)
			}

			if tc.LastBlocksResult != nil {
				gotContent := len(tc.LastBlocksResult.Files)
				gotEmpty := tc.LastBlocksResult.BlocksFound - gotContent
				if gotEmpty < 0 {
					gotEmpty = 0
				}

				if gotContent != wantContent || gotEmpty != wantEmpty {
					return fmt.Errorf(
						"%w: content_blocks want %d got %d; empty_blocks want %d got %d (direct blocks)",
						errResultNounVerbCountMismatch, wantContent, gotContent, wantEmpty, gotEmpty,
					)
				}

				return nil
			}

			return assertStdoutBlockContentAndEmptyCounts(tc, wantContent, wantEmpty)
		},
	)
}

// RegisterResult registers result assertion step definitions.
func RegisterResult(sc *godog.ScenarioContext, tc *TestContext) {
	registerThenNounVerbCount(sc, tc)
	registerThenBlockStdoutCounts(sc, tc)
	registerThenFileChangeStatus(sc, tc)
	registerOperationErrorJSONSteps(sc, tc)
	registerThenPreviewExtractLines(sc, tc)
	registerThenPreviewWouldCreate(sc, tc)
	registerThenPreviewWouldCreateOutputFilesCSV(sc, tc)
}
