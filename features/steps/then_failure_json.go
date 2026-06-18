package steps

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cucumber/godog"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

// parseStrictGlyphShiftStderrJSON requires that strings.TrimSpace(stderr) is exactly one JSON object
// emitted for operation errors: non-empty string field "error", optional "hint" and typed context fields.
// No other non-whitespace may appear before or after the value (encoder newlines are covered by TrimSpace).
func parseStrictGlyphShiftStderrJSON(stderr string) (map[string]interface{}, error) {
	trimmed := strings.TrimSpace(stderr)
	if trimmed == "" {
		return nil, errGlyphShiftErrorJSONObjectNotFound
	}

	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()

	var obj map[string]interface{}

	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("stderr must be a JSON object: %w", err)
	}

	rest := trimmed[dec.InputOffset():]
	if strings.TrimSpace(rest) != "" {
		return nil, fmt.Errorf("%w: remainder=%q", errStderrExtraContentAfterJSON, rest)
	}

	asAny := make(map[string]any, len(obj))
	for k, v := range obj {
		asAny[k] = v
	}

	if err := pipeline.ValidateOperationErrorPayload(asAny); err != nil {
		return nil, fmt.Errorf("stderr operation error payload: %w", err)
	}

	return obj, nil
}

func registerThenStdoutEmpty(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^stdout is empty$`, func() error {
		if tc.Stdout != "" {
			return fmt.Errorf("%w: want empty string, got %q", errStdoutMismatch, tc.Stdout)
		}

		return nil
	})
}

func registerThenStderrIsGlyphShiftJSONError(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^stderr is a JSON error object$`, func() error {
		_, err := parseStrictGlyphShiftStderrJSON(tc.Stderr)
		if err != nil {
			return fmt.Errorf("%w (stderr=%q)", err, tc.Stderr)
		}

		return nil
	})
}

// RegisterFailureStreamProof registers stdout/stderr contract steps for failed invocations.
func RegisterFailureStreamProof(sc *godog.ScenarioContext, tc *TestContext) {
	registerThenStdoutEmpty(sc, tc)
	registerThenStderrIsGlyphShiftJSONError(sc, tc)
}
