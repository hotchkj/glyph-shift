// Classification diagnostics preserve operation-error classifier context: sentinel, hint, path slots,
// and variant maps when JSON-edge formatting must degrade (for example MCP tool primary-path
// preparation failure or formatter validation rejecting the primary variant). Diagnostics are encoded
// only as additional prose inside existing non-empty hint strings permitted by the public operation-error
// oneOf shapes; they do not add JSON keys beyond the contracted operation error object.
package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ClassificationDiagnosticKV is a stable diagnostic field for asserting operation error payloads in tests.
type ClassificationDiagnosticKV struct {
	Key   string
	Value string
}

const (
	// Worst-case pair count for StableClassificationDiagnostics (base keys, path slots, encoded maps).
	classificationDiagnosticsStablePairsUpperBound = 14
	classificationSentenceDelimiter                = "; "

	DiagnosticLexicalPrimaryKey = "diagnostic.lexical_primary"
	DiagnosticPrepErrorKey      = "diagnostic.primary_path_prepare_error"

	TagMCPToolPrimaryPathPrepFailure                  = "mcp_tool_primary_path_prepare_failure"
	TagOperationJSONFormatterSuppressedClassification = "operation_error_json_formatter_suppressed_classification"
)

// StableClassificationDiagnostics returns deterministic key/value pairs capturing semantic fields from
// an ErrorOutcome for structured comparisons.
func StableClassificationDiagnostics(out *ErrorOutcome) []ClassificationDiagnosticKV {
	if out == nil {
		return nil
	}

	pairs := make([]ClassificationDiagnosticKV, 0, classificationDiagnosticsStablePairsUpperBound)

	pairs = appendDiagnosticCorePairs(pairs, out)
	pairs = appendDiagnosticPathPairs(pairs, out)
	pairs = appendDiagnosticEncodedMap(pairs, "string_fields", out.StringFields)
	pairs = appendDiagnosticEncodedMap(pairs, "int_fields", out.IntFields)
	pairs = appendDiagnosticEncodedMap(pairs, "string_array_fields", out.StringArrayFields)

	return pairs
}

func appendDiagnosticCorePairs(pairs []ClassificationDiagnosticKV, out *ErrorOutcome) []ClassificationDiagnosticKV {
	pairs = append(pairs, ClassificationDiagnosticKV{Key: "error", Value: out.Error})
	if hint := strings.TrimSpace(out.Hint); hint != "" {
		pairs = append(pairs, ClassificationDiagnosticKV{Key: "hint", Value: hint})
	}

	return pairs
}

func appendDiagnosticPathPairs(pairs []ClassificationDiagnosticKV, out *ErrorOutcome) []ClassificationDiagnosticKV {
	pathPairs := []ClassificationDiagnosticKV{
		{Key: "src", Value: out.Src},
		{Key: "dest", Value: out.Dest},
		{Key: "out_dir", Value: out.OutDir},
		{Key: "output_path", Value: out.OutputPath},
	}

	for _, pair := range pathPairs {
		trimmed := filepath.ToSlash(strings.TrimSpace(pair.Value))
		if trimmed != "" {
			pairs = append(pairs, ClassificationDiagnosticKV{Key: pair.Key, Value: trimmed})
		}
	}

	return pairs
}

func appendDiagnosticEncodedMap(
	pairs []ClassificationDiagnosticKV,
	key string,
	value any,
) []ClassificationDiagnosticKV {
	if diagnosticMapEmpty(value) {
		return pairs
	}

	if encoded := marshalSortedKeysJSON(value); len(encoded) > 0 {
		pairs = append(pairs, ClassificationDiagnosticKV{Key: key, Value: string(encoded)})
	}

	return pairs
}

func diagnosticMapEmpty(value any) bool {
	switch typed := value.(type) {
	case map[string]string:
		return len(typed) == 0
	case map[string]int:
		return len(typed) == 0
	case map[string][]string:
		return len(typed) == 0
	default:
		return value == nil
	}
}

func marshalSortedKeysJSON(value any) []byte {
	b, err := json.Marshal(sortJSONMapKeysStable(value))
	if err != nil {
		return nil
	}

	return b
}

// sortJSONMapKeysStable returns a deterministically keyed map-shaped value for diagnostics JSON.
func sortJSONMapKeysStable(value any) any {
	switch typed := value.(type) {
	case map[string]string:
		return sortedStringKeyMap(typed)
	case map[string]int:
		return sortedStringKeyMap(typed)
	case map[string][]string:
		return sortedStringKeyMap(typed)
	default:
		return value
	}
}

func sortedStringKeyMap[T any](in map[string]T) map[string]T {
	if len(in) == 0 {
		return in
	}

	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	out := make(map[string]T, len(in))
	for _, key := range keys {
		out[key] = in[key]
	}

	return out
}

func formatClassificationDiagnosticSentence(
	tag string,
	prefixExtras []ClassificationDiagnosticKV,
	body *ErrorOutcome,
) string {
	var diagPairs []ClassificationDiagnosticKV

	if body != nil {
		diagPairs = StableClassificationDiagnostics(body)
	}

	parts := make([]string, 0, 4+len(prefixExtras)+len(diagPairs))
	parts = append(parts, "_tag="+strconv.Quote(tag))

	for _, extra := range prefixExtras {
		if strings.TrimSpace(extra.Key) != "" {
			parts = append(parts, extra.Key+"="+strconv.Quote(extra.Value))
		}
	}

	for _, p := range diagPairs {
		parts = append(parts, p.Key+"="+strconv.Quote(p.Value))
	}

	return strings.Join(parts, classificationSentenceDelimiter)
}

var errClassificationSentenceSegment = errors.New("pipeline: classification diagnostic sentence segment invalid")

func parseQuotedPairSegment(segment string) (key, val string, err error) {
	idx := strings.Index(segment, "=")
	if idx <= 0 {
		return "", "", fmt.Errorf("%w: %q", errClassificationSentenceSegment, segment)
	}

	key = strings.TrimSpace(segment[:idx])
	rawVal := strings.TrimSpace(segment[idx+1:])

	unquotedVal, qerr := strconv.Unquote(rawVal)
	if qerr != nil {
		return "", "", fmt.Errorf("%w: unquote failed for %q: %w", errClassificationSentenceSegment, rawVal, qerr)
	}

	return key, unquotedVal, nil
}

// ParseClassificationDiagnosticQuotedPairs splits a '; '-delimited sentence of segments formatted as key="quoted",
// preserving empty quoted values. It is deterministic for diagnostics produced by diagnostic sentence formatters here.
func ParseClassificationDiagnosticQuotedPairs(sentence string) (map[string]string, error) {
	parts := strings.Split(sentence, classificationSentenceDelimiter)
	out := make(map[string]string, len(parts))

	for _, p := range parts {
		key, val, segmentErr := parseQuotedPairSegment(p)
		if segmentErr != nil {
			return nil, segmentErr
		}

		out[key] = val
	}

	return out, nil
}

// DiagnosticLexicalPrimary records the raw MCP lexical primary-path argument alongside diagnostics.
func DiagnosticLexicalPrimary(lexical string) ClassificationDiagnosticKV {
	return ClassificationDiagnosticKV{Key: DiagnosticLexicalPrimaryKey, Value: lexical}
}

// DiagnosticPreparePathFailure records the PreparePath failure for MCP primary-path preparation diagnostics.
func DiagnosticPreparePathFailure(prepErr error) ClassificationDiagnosticKV {
	msg := ""

	if prepErr != nil && strings.TrimSpace(prepErr.Error()) != "" {
		msg = strings.TrimSpace(prepErr.Error())
	}

	return ClassificationDiagnosticKV{Key: DiagnosticPrepErrorKey, Value: msg}
}

func operationOutcomePathSlotsPresent(out *ErrorOutcome) bool {
	if out == nil {
		return false
	}

	return strings.TrimSpace(out.Src) != "" ||
		strings.TrimSpace(out.Dest) != "" ||
		strings.TrimSpace(out.OutDir) != "" ||
		strings.TrimSpace(out.OutputPath) != ""
}

// ReconcileOperationOutcomeWithMCPToolPrimaryPathPrepFailure converts an MCP tool operation-error outcome into
// a JSON-renderable internal_error when MCP tool lexical primary path PreparePath failure makes the original
// classification incompatible with mandatory absolute native path slots.
func ReconcileOperationOutcomeWithMCPToolPrimaryPathPrepFailure(
	out *ErrorOutcome,
	toolPrimaryLexical string,
	prepErr error,
) ErrorOutcome {
	if out == nil || toolPrimaryLexical == "" || prepErr == nil {
		var zero ErrorOutcome

		if out != nil {
			return *out
		}

		return zero
	}

	if operationOutcomePathSlotsPresent(out) {
		return *out
	}

	hintSentence := formatClassificationDiagnosticSentence(TagMCPToolPrimaryPathPrepFailure, []ClassificationDiagnosticKV{
		DiagnosticLexicalPrimary(toolPrimaryLexical),
		DiagnosticPreparePathFailure(prepErr),
	}, out)

	return ErrorOutcome{
		Error:             opErrClassInternalError,
		Hint:              hintSentence,
		ExitCode:          ExitGeneral,
		Src:               "",
		Dest:              "",
		OutDir:            "",
		OutputPath:        "",
		StringFields:      nil,
		IntFields:         nil,
		StringArrayFields: nil,
	}
}

// WriteFailedOutcomeFromFormattingError builds write_failed diagnostics that retain suppressed operation
// classification diagnostics alongside formatter failure details.
func WriteFailedOutcomeFromFormattingError(formatErr error, prior *ErrorOutcome) ErrorOutcome {
	wf := WriteFailedOutcome(formatErr)

	if prior == nil || strings.TrimSpace(wf.Hint) == "" {
		return wf
	}

	supp := formatClassificationDiagnosticSentence(TagOperationJSONFormatterSuppressedClassification, nil, prior)

	wf.Hint = wf.Hint + classificationSentenceDelimiter + supp

	return wf
}
