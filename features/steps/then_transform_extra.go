package steps

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/hotchkj/glyph-shift/features/harness"
)

func jsonIntField(obj map[string]interface{}, key string) (int, error) {
	rawValue, ok := obj[key]
	if !ok {
		return 0, fmt.Errorf("%w: key=%q", errStdoutJSONMissingNumericField, key)
	}

	switch typed := rawValue.(type) {
	case float64:
		return int(typed), nil
	default:
		return 0, fmt.Errorf("%w: key=%q got=%T", errStdoutJSONFieldWantNumberGotKind, key, rawValue)
	}
}

const (
	errFmtParseStdoutJSON         = "parse stdout JSON: %w"
	errFmtLineEndingStatsMismatch = "%w: want_found=%d want_converted=%d got_found=%d got_converted=%d"
)

func registerNoChangesMade(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^no changes were made$`, func() error {
		if tc.LastTransformResult != nil {
			if tc.LastTransformResult.Result.WouldChange {
				return errStdoutChangedExpectedFalse
			}

			return nil
		}

		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Stdout), &obj); err != nil {
			return fmt.Errorf(errFmtParseStdoutJSON, err)
		}

		changed, ok := obj["changed"].(bool)
		if !ok {
			return errStdoutChangedMissingBool
		}

		if changed {
			return errStdoutChangedExpectedFalse
		}

		return nil
	})
}

func registerReportsChangesWouldOccur(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^the result reports changes would occur$`, func() error {
		if tc.LastTransformResult != nil {
			if !tc.LastTransformResult.Result.WouldChange {
				return errWouldChangeExpectedTrue
			}

			return nil
		}

		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Stdout), &obj); err != nil {
			return fmt.Errorf(errFmtParseStdoutJSON, err)
		}

		wouldChange, ok := obj["would_change"].(bool)
		if !ok {
			return errStdoutWouldChangeMissingBool
		}

		if !wouldChange {
			return errWouldChangeExpectedTrue
		}

		return nil
	})
}

func registerReportsNoChangesWouldOccur(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^the result reports no changes would occur$`, func() error {
		if tc.LastTransformResult != nil {
			if tc.LastTransformResult.Result.WouldChange {
				return errWouldChangeExpectedFalse
			}

			return nil
		}

		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Stdout), &obj); err != nil {
			return fmt.Errorf(errFmtParseStdoutJSON, err)
		}

		wouldChange, ok := obj["would_change"].(bool)
		if !ok {
			return errStdoutWouldChangeMissingBool
		}

		if wouldChange {
			return errWouldChangeExpectedFalse
		}

		return nil
	})
}

func registerStillHasTrailingWS(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^"([^"]*)" still has trailing whitespace$`, func(file string) error {
		data, err := readOutputFile(tc, file)
		if err != nil {
			return err
		}

		lines, err := harness.ReadLinesFrom(bytes.NewReader(data))
		if err != nil {
			return err
		}

		for _, ln := range lines {
			content := ln.Content
			if len(content) == 0 {
				continue
			}

			last := content[len(content)-1]
			if last == ' ' || last == '\t' {
				return nil
			}
		}

		return fmt.Errorf("%w: no line in %q has trailing whitespace", errLineTrailingWS, file)
	})
}

func registerStillEndsWithoutNewline(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^"([^"]*)" still ends without a newline$`, func(file string) error {
		data, err := readOutputFile(tc, file)
		if err != nil {
			return err
		}

		if len(data) == 0 {
			return nil
		}

		if data[len(data)-1] == '\n' {
			return fmt.Errorf("%w: %q", errFileShouldNotEndWithNewline, file)
		}

		return nil
	})
}

func registerReportsLineEndingLFStats(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^the result reports (\d+) LF endings found and (\d+) converted$`, func(wantFound, wantConv int) error {
		if tc.LastTransformResult != nil {
			res := tc.LastTransformResult.Result
			if res.LFFound != wantFound || res.LFConverted != wantConv {
				return fmt.Errorf(errFmtLineEndingStatsMismatch,
					errTransformLFEndingStatsMismatch, wantFound, wantConv, res.LFFound, res.LFConverted)
			}

			return nil
		}

		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Stdout), &obj); err != nil {
			return fmt.Errorf(errFmtParseStdoutJSON, err)
		}

		gotFound, err := jsonIntField(obj, "lf_found")
		if err != nil {
			return err
		}

		gotConv, err := jsonIntField(obj, "lf_converted")
		if err != nil {
			return err
		}

		if gotFound != wantFound || gotConv != wantConv {
			return fmt.Errorf(errFmtLineEndingStatsMismatch,
				errTransformLFEndingStatsMismatch, wantFound, wantConv, gotFound, gotConv)
		}

		return nil
	})
}

func registerReportsLineEndingCRStats(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^the result reports (\d+) CR endings found and (\d+) converted$`, func(wantFound, wantConv int) error {
		if tc.LastTransformResult != nil {
			res := tc.LastTransformResult.Result
			if res.CRFound != wantFound || res.CRConverted != wantConv {
				return fmt.Errorf(errFmtLineEndingStatsMismatch,
					errTransformCRLineEndingStatsMismatch, wantFound, wantConv, res.CRFound, res.CRConverted)
			}

			return nil
		}

		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Stdout), &obj); err != nil {
			return fmt.Errorf(errFmtParseStdoutJSON, err)
		}

		gotFound, err := jsonIntField(obj, "cr_found")
		if err != nil {
			return err
		}

		gotConv, err := jsonIntField(obj, "cr_converted")
		if err != nil {
			return err
		}

		if gotFound != wantFound || gotConv != wantConv {
			return fmt.Errorf(errFmtLineEndingStatsMismatch,
				errTransformCRLineEndingStatsMismatch, wantFound, wantConv, gotFound, gotConv)
		}

		return nil
	})
}

func registerReportsLineEndingCRLFStats(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^the result reports (\d+) CRLF endings? found and (\d+) converted$`, func(wantFound, wantConv int) error {
		if tc.LastTransformResult != nil {
			res := tc.LastTransformResult.Result
			if res.CRLFFound != wantFound || res.CRLFConverted != wantConv {
				return fmt.Errorf(errFmtLineEndingStatsMismatch,
					errTransformCRLFEndingStatsMismatch, wantFound, wantConv, res.CRLFFound, res.CRLFConverted)
			}

			return nil
		}

		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Stdout), &obj); err != nil {
			return fmt.Errorf(errFmtParseStdoutJSON, err)
		}

		gotFound, err := jsonIntField(obj, "crlf_found")
		if err != nil {
			return err
		}

		gotConv, err := jsonIntField(obj, "crlf_converted")
		if err != nil {
			return err
		}

		if gotFound != wantFound || gotConv != wantConv {
			return fmt.Errorf(errFmtLineEndingStatsMismatch,
				errTransformCRLFEndingStatsMismatch, wantFound, wantConv, gotFound, gotConv)
		}

		return nil
	})
}

// RegisterTransformExtra registers transform-specific Then step definitions.
func RegisterTransformExtra(sc *godog.ScenarioContext, tc *TestContext) {
	registerNoChangesMade(sc, tc)
	registerReportsChangesWouldOccur(sc, tc)
	registerReportsNoChangesWouldOccur(sc, tc)
	registerStillHasTrailingWS(sc, tc)
	registerStillEndsWithoutNewline(sc, tc)
	registerReportsLineEndingLFStats(sc, tc)
	registerReportsLineEndingCRStats(sc, tc)
	registerReportsLineEndingCRLFStats(sc, tc)
}
