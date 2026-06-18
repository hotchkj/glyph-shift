package steps

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

func parseCommaSeparatedLogicalRelPaths(csv string) ([]string, error) {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			return nil, fmt.Errorf("%w in %q", errEmptyCommaSeparatedPathSegment, csv)
		}

		out = append(out, s)
	}

	if len(out) == 0 {
		return nil, errNoOutputBasenames
	}

	return out, nil
}

func pathsEqualNative(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func assertStdoutJSONObject(tc *TestContext) (map[string]interface{}, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Stdout), &obj); err != nil {
		return nil, fmt.Errorf(parseStdoutJSONFmt, err)
	}

	return obj, nil
}

func assertJSONFieldAbsoluteNativePathArray(
	obj map[string]interface{},
	field string,
	wantWorkspaceRels []string,
	wsJoin func(string) string,
) error {
	raw, ok := obj[field]
	if !ok {
		return fmt.Errorf("%w: %q", errStdoutJSONMissingField, field)
	}

	arr, ok := raw.([]interface{})
	if !ok {
		return fmt.Errorf("%w: field %q", errStdoutJSONFieldTypeMismatch, field)
	}

	if len(arr) != len(wantWorkspaceRels) {
		return fmt.Errorf(
			"%w: field %q want %d elements got %d",
			errStdoutJSONFieldValueMismatch, field, len(wantWorkspaceRels), len(arr),
		)
	}

	for idx := range wantWorkspaceRels {
		gotStr, elemOK := arr[idx].(string)
		if !elemOK {
			return fmt.Errorf("%w: field %q index %d", errStdoutJSONFieldTypeMismatch, field, idx)
		}

		want := wsJoin(wantWorkspaceRels[idx])
		if !pathsEqualNative(gotStr, want) {
			return fmt.Errorf(
				"%w: field %q index %d want absolute path %q got %q",
				errStdoutJSONFieldValueMismatch, field, idx, want, gotStr,
			)
		}

		if !filepath.IsAbs(gotStr) {
			return fmt.Errorf("%w: field %q index %d is not absolute: %q",
				errStdoutJSONFieldValueMismatch, field, idx, gotStr)
		}
	}

	return nil
}

func assertJSONFieldAbsoluteNativePathString(
	obj map[string]interface{},
	field string,
	wantWorkspaceRel string,
	wsJoin func(string) string,
) error {
	raw, ok := obj[field]
	if !ok {
		return fmt.Errorf("%w: %q", errStdoutJSONMissingField, field)
	}

	gotStr, ok := raw.(string)
	if !ok {
		return fmt.Errorf("%w: field %q", errStdoutJSONFieldTypeMismatch, field)
	}

	want := wsJoin(wantWorkspaceRel)
	if !pathsEqualNative(gotStr, want) {
		return fmt.Errorf(
			"%w: field %q want absolute path %q got %q",
			errStdoutJSONFieldValueMismatch, field, want, gotStr,
		)
	}

	if !filepath.IsAbs(gotStr) {
		return fmt.Errorf("%w: field %q is not absolute: %q",
			errStdoutJSONFieldValueMismatch, field, gotStr)
	}

	return nil
}

// RegisterContractPathAssertions registers JSON assertions for absolute native workspace paths (contract docs).
func RegisterContractPathAssertions(sc *godog.ScenarioContext, tc *TestContext) {
	wsJoin := func(rel string) string {
		return tc.Ws.Join(rel)
	}

	//nolint:lll // Regex length for descriptive field/csv wording.
	sc.Then(
		`^stdout JSON field "([^"]*)" is a string array of absolute native paths for workspace-relative paths "([^"]*)"$`,
		func(field, csv string) error {
			rels, err := parseCommaSeparatedLogicalRelPaths(csv)
			if err != nil {
				return err
			}

			obj, err := assertStdoutJSONObject(tc)
			if err != nil {
				return err
			}

			return assertJSONFieldAbsoluteNativePathArray(obj, field, rels, wsJoin)
		},
	)

	sc.Then(
		`^stdout JSON field "([^"]*)" is the absolute native path for workspace-relative path "([^"]*)"$`,
		func(field, rel string) error {
			obj, err := assertStdoutJSONObject(tc)
			if err != nil {
				return err
			}

			return assertJSONFieldAbsoluteNativePathString(obj, field, strings.TrimSpace(rel), wsJoin)
		},
	)

	//nolint:lll // Godog regex length for descriptive MCP structuredContent wording.
	sc.Then(
		`^the MCP structuredContent field "([^"]*)" is a string array of absolute native paths for workspace-relative paths "([^"]*)"$`,
		func(field, csv string) error {
			rels, err := parseCommaSeparatedLogicalRelPaths(csv)
			if err != nil {
				return err
			}

			if tc.MCPStructuredContent == nil {
				return errMCPStructuredContentNil
			}

			return assertJSONFieldAbsoluteNativePathArray(tc.MCPStructuredContent, field, rels, wsJoin)
		},
	)

	sc.Then(
		`^the MCP structuredContent field "([^"]*)" is the absolute native path for workspace-relative path "([^"]*)"$`,
		func(field, rel string) error {
			if tc.MCPStructuredContent == nil {
				return errMCPStructuredContentNil
			}

			return assertJSONFieldAbsoluteNativePathString(tc.MCPStructuredContent, field, strings.TrimSpace(rel), wsJoin)
		},
	)
}
