package steps

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/cucumber/godog"
)

func parseStdoutAsJSONObject(tc *TestContext) (map[string]interface{}, error) {
	var raw interface{}
	if err := json.Unmarshal([]byte(tc.Stdout), &raw); err != nil {
		return nil, fmt.Errorf(parseStdoutJSONFmt, err)
	}
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return nil, errStdoutJSONNotObject
	}

	return obj, nil
}

func assertStdoutJSONBoolEqual(key string, exp bool, got interface{}) error {
	gotBool, ok := got.(bool)
	if !ok {
		return fmt.Errorf("%w: field %q", errStdoutJSONFieldTypeMismatch, key)
	}
	if gotBool != exp {
		return fmt.Errorf("%w: field %q want %v got %v", errStdoutJSONFieldValueMismatch, key, exp, gotBool)
	}

	return nil
}

func assertStdoutJSONNumberEqual(key string, exp float64, got interface{}) error {
	switch value := got.(type) {
	case float64:
		if value != exp {
			return fmt.Errorf("%w: field %q want %v got %v", errStdoutJSONFieldValueMismatch, key, exp, value)
		}
	case json.Number:
		gotFloat, err := value.Float64()
		if err != nil || gotFloat != exp {
			return fmt.Errorf("%w: field %q want %v got %v", errStdoutJSONFieldValueMismatch, key, exp, got)
		}
	default:
		return fmt.Errorf("%w: field %q", errStdoutJSONFieldTypeMismatch, key)
	}

	return nil
}

func assertStdoutJSONStringEqual(key, exp string, got interface{}) error {
	gotStr, ok := got.(string)
	if !ok {
		return fmt.Errorf("%w: field %q", errStdoutJSONFieldTypeMismatch, key)
	}
	if gotStr != exp {
		return fmt.Errorf("%w: field %q want %q got %q", errStdoutJSONFieldValueMismatch, key, exp, gotStr)
	}

	return nil
}

func assertStdoutJSONFieldValueEqual(key string, expected, got interface{}) error {
	switch exp := expected.(type) {
	case bool:
		return assertStdoutJSONBoolEqual(key, exp, got)
	case float64:
		return assertStdoutJSONNumberEqual(key, exp, got)
	case string:
		return assertStdoutJSONStringEqual(key, exp, got)
	case []interface{}:
		return assertStdoutJSONStringArrayMatches(key, exp, got)
	default:
		return fmt.Errorf("%w: field %q unsupported spec type %T", errStdoutJSONFieldTypeMismatch, key, expected)
	}
}

func assertStdoutJSONStringArrayMatches(key string, expArr []interface{}, got interface{}) error {
	gotArr, ok := got.([]interface{})
	if !ok {
		return fmt.Errorf("%w: field %q", errStdoutJSONFieldTypeMismatch, key)
	}
	if len(gotArr) != len(expArr) {
		return fmt.Errorf(
			"%w: field %q want %d elements got %d",
			errStdoutJSONFieldValueMismatch, key, len(expArr), len(gotArr),
		)
	}

	for idx := range expArr {
		expElem, elemOK := expArr[idx].(string)
		if !elemOK {
			return fmt.Errorf("%w: field %q spec index %d", errStdoutJSONFieldTypeMismatch, key, idx)
		}
		gotElem, gotElemOK := gotArr[idx].(string)
		if !gotElemOK {
			return fmt.Errorf("%w: field %q index %d", errStdoutJSONFieldTypeMismatch, key, idx)
		}
		if gotElem != expElem {
			return fmt.Errorf(
				"%w: field %q index %d want %q got %q",
				errStdoutJSONFieldValueMismatch, key, idx, expElem, gotElem,
			)
		}
	}

	return nil
}

// assertJSONSpecExactMatch checks that obj has exactly the fields in the docstring spec:
// spec null values require absence; other values require equality. No extra keys in obj.
func assertJSONSpecExactMatch(obj map[string]interface{}, doc *godog.DocString) error {
	var spec map[string]interface{}
	if err := json.Unmarshal([]byte(doc.Content), &spec); err != nil {
		return fmt.Errorf("parse JSON shape docstring: %w", err)
	}

	for field := range obj {
		if _, ok := spec[field]; !ok {
			return fmt.Errorf("%w: %q", errStdoutJSONExtraField, field)
		}
	}

	for specField, expVal := range spec {
		if expVal == nil {
			if _, exists := obj[specField]; exists {
				return fmt.Errorf("%w: field %q must be absent", errStdoutJSONFieldValueMismatch, specField)
			}

			continue
		}

		gotVal, exists := obj[specField]
		if !exists {
			return fmt.Errorf("%w: %q", errStdoutJSONMissingField, specField)
		}

		if err := assertStdoutJSONFieldValueEqual(specField, expVal, gotVal); err != nil {
			return err
		}
	}

	return nil
}

func assertStdoutJSONExactly(tc *TestContext, doc *godog.DocString) error {
	stdout, err := parseStdoutAsJSONObject(tc)
	if err != nil {
		return err
	}

	return assertJSONSpecExactMatch(stdout, doc)
}

func assertStdoutJSONFieldAbsent(tc *TestContext, field string) error {
	stdout, err := parseStdoutAsJSONObject(tc)
	if err != nil {
		return err
	}
	if _, exists := stdout[field]; exists {
		return fmt.Errorf("%w: field %q is present", errStdoutJSONExtraField, field)
	}

	return nil
}

func assertStdoutJSONFieldInt(tc *TestContext, field string, want int) error {
	stdout, err := parseStdoutAsJSONObject(tc)
	if err != nil {
		return err
	}

	raw, exists := stdout[field]
	if !exists {
		return fmt.Errorf("%w: %q", errStdoutJSONMissingField, field)
	}

	if !jsonInt64MatchingExactInt(raw, int64(want)) {
		return fmt.Errorf("%w: field %q want %d got %v", errStdoutJSONFieldValueMismatch, field, want, raw)
	}

	return nil
}

func assertStdoutJSONFieldBool(tc *TestContext, field string, want bool) error {
	stdout, err := parseStdoutAsJSONObject(tc)
	if err != nil {
		return err
	}

	raw, exists := stdout[field]
	if !exists {
		return fmt.Errorf("%w: %q", errStdoutJSONMissingField, field)
	}

	got, ok := raw.(bool)
	if !ok {
		return fmt.Errorf("%w: field %q", errStdoutJSONFieldTypeMismatch, field)
	}
	if got != want {
		return fmt.Errorf("%w: field %q want %v got %v", errStdoutJSONFieldValueMismatch, field, want, got)
	}

	return nil
}

func assertStdoutJSONFieldStringArrayLen(tc *TestContext, field string, wantLen int) error {
	stdout, err := parseStdoutAsJSONObject(tc)
	if err != nil {
		return err
	}

	raw, exists := stdout[field]
	if !exists {
		return fmt.Errorf("%w: %q", errStdoutJSONMissingField, field)
	}

	arr, ok := raw.([]interface{})
	if !ok {
		return fmt.Errorf("%w: field %q", errStdoutJSONFieldTypeMismatch, field)
	}
	for i, el := range arr {
		if _, ok := el.(string); !ok {
			return fmt.Errorf("%w: field %q index %d", errStdoutJSONFieldTypeMismatch, field, i)
		}
	}
	if len(arr) != wantLen {
		return fmt.Errorf(
			"%w: field %q want %d elements got %d",
			errStdoutJSONFieldValueMismatch, field, wantLen, len(arr),
		)
	}

	return nil
}

func registerThenStdoutJSONExactly(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^stdout JSON is exactly:$`, func(doc *godog.DocString) error {
		return assertStdoutJSONExactly(tc, doc)
	})
}

func registerThenStdoutJSONFieldAbsent(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^stdout JSON does not have field "([^"]*)"$`, func(field string) error {
		return assertStdoutJSONFieldAbsent(tc, field)
	})
}

func registerThenStdoutJSONFieldInt(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^stdout JSON field "([^"]*)" is (\d+)$`, func(field, nStr string) error {
		want, err := strconv.Atoi(nStr)
		if err != nil {
			return fmt.Errorf("parse integer field expectation: %w", err)
		}

		return assertStdoutJSONFieldInt(tc, field, want)
	})
}

func registerThenStdoutJSONFieldBool(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^stdout JSON field "([^"]*)" is (true|false)$`, func(field, bStr string) error {
		want, err := strconv.ParseBool(bStr)
		if err != nil {
			return fmt.Errorf("parse boolean field expectation: %w", err)
		}

		return assertStdoutJSONFieldBool(tc, field, want)
	})
}

func registerThenStdoutJSONFieldStringArrayLen(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^stdout JSON field "([^"]*)" is a string array with (\d+) elements$`, func(field, nStr string) error {
		wantLen, err := strconv.Atoi(nStr)
		if err != nil {
			return fmt.Errorf("parse string array length: %w", err)
		}

		return assertStdoutJSONFieldStringArrayLen(tc, field, wantLen)
	})
}

// RegisterStdoutJSON registers stdout JSON contract shape assertion steps (Layer 2).
func RegisterStdoutJSON(sc *godog.ScenarioContext, tc *TestContext) {
	registerThenStdoutJSONExactly(sc, tc)
	registerThenStdoutJSONFieldAbsent(sc, tc)
	registerThenStdoutJSONFieldInt(sc, tc)
	registerThenStdoutJSONFieldBool(sc, tc)
	registerThenStdoutJSONFieldStringArrayLen(sc, tc)
}
