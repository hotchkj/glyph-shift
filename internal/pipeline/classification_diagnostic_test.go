package pipeline

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

var (
	errTestSimulatedOutcomePreparePathMismatch = errors.New(
		"pipeline classification test: simulated outcome prepare-path mismatch",
	)
	errTestSimulatedMCPPrimaryPrepareFailure = errors.New(
		"pipeline classification test: simulated MCP primary PreparePath failure",
	)
	errTestFormatContractViolation = errors.New(
		"pipeline classification test: format contract violation",
	)
)

func quotedPairMapFromSentence(t *testing.T, sentence string) map[string]string {
	t.Helper()

	pairMap, err := ParseClassificationDiagnosticQuotedPairs(sentence)
	if err != nil {
		t.Fatalf("ParseClassificationDiagnosticQuotedPairs: %v", err)
	}

	return pairMap
}

func TestParseClassificationDiagnosticQuotedPairs_roundtrip(t *testing.T) {
	t.Parallel()

	sentence := `_tag=` + strconv.Quote("t") + `; ` +
		DiagnosticLexicalPrimaryKey + `=` + strconv.Quote("../x") + `; ` +
		`error=` + strconv.Quote(`one"tick`)

	got, err := ParseClassificationDiagnosticQuotedPairs(sentence)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	if len(got) != 3 || got["_tag"] != "t" || got[DiagnosticLexicalPrimaryKey] != "../x" || got["error"] != `one"tick` {
		t.Fatalf("unexpected map %#v", got)
	}
}

func TestReconcileOperationOutcomeWithMCPToolPrimaryPathPrepFailure_preserves_when_path_slots_present(t *testing.T) {
	t.Parallel()

	out := ErrorOutcome{
		Error:    "binary_source",
		Hint:     HintBinarySource,
		ExitCode: ExitBinarySource,
		Src:      "doc.txt",
	}

	got := ReconcileOperationOutcomeWithMCPToolPrimaryPathPrepFailure(
		&out,
		`..\bad`,
		errTestSimulatedOutcomePreparePathMismatch,
	)

	if !reflect.DeepEqual(got, out) {
		t.Fatalf("reconcile mutated typed path outcome: %+v", got)
	}
}

func TestReconcileOperationOutcomeWithMCPToolPrimaryPathPrepFailure_internalWhenNoPathSlots(t *testing.T) {
	t.Parallel()

	before := ClassifyOperationError(ErrBinarySource, "")

	prepFail := errTestSimulatedMCPPrimaryPrepareFailure
	after := ReconcileOperationOutcomeWithMCPToolPrimaryPathPrepFailure(&before, "lex-primary", prepFail)

	if after.Error != opErrClassInternalError {
		t.Fatalf("want internal_error sentinel, got %q", after.Error)
	}

	payload, ferr := FormatOperationErrorJSON(`/workspace/repo`, after)
	if ferr != nil {
		t.Fatalf("FormatOperationErrorJSON: %v", ferr)
	}

	if err := ValidateOperationErrorPayload(payload); err != nil {
		t.Fatalf("ValidateOperationErrorPayload: %v", err)
	}

	sentencePairs := quotedPairMapFromSentence(t, after.Hint)

	if got := sentencePairs["_tag"]; got != TagMCPToolPrimaryPathPrepFailure {
		t.Fatalf("_tag=%q want %q", got, TagMCPToolPrimaryPathPrepFailure)
	}

	if got := sentencePairs["error"]; got != before.Error {
		t.Fatalf(`preserved "error" want %q got %q`, before.Error, got)
	}

	if got := sentencePairs[DiagnosticLexicalPrimaryKey]; got != "lex-primary" {
		t.Fatalf(`lexical key: got %q`, got)
	}

	if got := sentencePairs[DiagnosticPrepErrorKey]; got != prepFail.Error() {
		t.Fatalf(`prep error mismatch: got %q want %q`, got, prepFail.Error())
	}
}

func TestReconcileOperationOutcomeWithMCPToolPrimaryPathPrepFailure_noopBranches(t *testing.T) {
	t.Parallel()

	before := ClassifyOperationError(ErrBinarySource, "")

	cases := map[string]struct {
		out     *ErrorOutcome
		lexical string
		prepErr error
		want    ErrorOutcome
	}{
		"nil outcome": {
			out:     nil,
			lexical: "lex",
			prepErr: errTestSimulatedMCPPrimaryPrepareFailure,
			want:    ErrorOutcome{},
		},
		"empty lexical": {
			out:     &before,
			lexical: "",
			prepErr: errTestSimulatedMCPPrimaryPrepareFailure,
			want:    before,
		},
		"nil prep error": {
			out:     &before,
			lexical: "lex",
			prepErr: nil,
			want:    before,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := ReconcileOperationOutcomeWithMCPToolPrimaryPathPrepFailure(tc.out, tc.lexical, tc.prepErr)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}

func TestWriteFailedOutcomeFromFormattingError_keeps_classifier_digest(t *testing.T) {
	t.Parallel()

	prior := ErrorOutcome{
		Error:             opErrClassPatternTooLong,
		Hint:              "boom",
		ExitCode:          ExitValidation,
		StringFields:      map[string]string{"field": "x"},
		Src:               "",
		IntFields:         nil,
		StringArrayFields: nil,
	}

	formatErr := errTestFormatContractViolation
	got := WriteFailedOutcomeFromFormattingError(formatErr, &prior)

	wantPrefix := WriteFailedOutcome(formatErr).Hint
	wantSuffix := formatClassificationDiagnosticSentence(TagOperationJSONFormatterSuppressedClassification, nil, &prior)

	if got.Hint != wantPrefix+classificationSentenceDelimiter+wantSuffix {
		t.Fatalf("hint mismatch:\ngot:  %q\nwant: %q", got.Hint, wantPrefix+classificationSentenceDelimiter+wantSuffix)
	}

	if got.Error != opErrClassWriteFailed {
		t.Fatalf("error sentinel: got %q", got.Error)
	}

	if err := ValidateOperationErrorPayload(operationErrorPayloadFromWriteFailed(&got)); err != nil {
		t.Fatalf("payload contract: %v", err)
	}

	sentencePairs := quotedPairMapFromSentence(t, wantSuffix)
	if g := sentencePairs["_tag"]; g != TagOperationJSONFormatterSuppressedClassification {
		t.Fatalf("_tag=%q", g)
	}

	if g := sentencePairs["error"]; g != prior.Error {
		t.Fatalf("prior error echoed: got %q want %q", g, prior.Error)
	}

	if _, ok := sentencePairs["field"]; ok {
		t.Fatalf("sentence map must encode variant fields via stable string_fields, saw bare \"field\" key")
	}

	rawSF := sentencePairs["string_fields"]

	var dec map[string]string

	if jsonErr := json.Unmarshal([]byte(rawSF), &dec); jsonErr != nil {
		t.Fatalf("string_fields JSON decode: %v (%q)", jsonErr, rawSF)
	}

	if dec["field"] != "x" {
		t.Fatalf("string_fields.field: got %#v want %q", dec["field"], "x")
	}
}

func TestStableClassificationDiagnostics_encodesAllStructuredMaps(t *testing.T) {
	t.Parallel()

	outcome := ErrorOutcome{
		Error:             opErrClassMaxFilesExceeded,
		Hint:              "trimmed hint",
		Src:               filepath.Join("root", "src.txt"),
		Dest:              filepath.Join("root", "dest.txt"),
		OutDir:            filepath.Join("root", "out"),
		OutputPath:        filepath.Join("root", "out", "001.txt"),
		StringFields:      map[string]string{"z": "last", "a": "first"},
		IntFields:         map[string]int{"would_create_count": 3, "max_files": 2},
		StringArrayFields: map[string][]string{"missing_flags": {"src", "dest"}},
	}

	pairs := StableClassificationDiagnostics(&outcome)
	got := map[string]string{}
	for _, pair := range pairs {
		got[pair.Key] = pair.Value
	}

	if got["string_fields"] != `{"a":"first","z":"last"}` {
		t.Fatalf("string_fields = %q", got["string_fields"])
	}
	if got["int_fields"] != `{"max_files":2,"would_create_count":3}` {
		t.Fatalf("int_fields = %q", got["int_fields"])
	}
	if got["string_array_fields"] != `{"missing_flags":["src","dest"]}` {
		t.Fatalf("string_array_fields = %q", got["string_array_fields"])
	}
}

func TestIsOperationOutcomeRenderableAtJSONEdge_smoke(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string([]rune{filepath.Separator}), "RenderableRoot")

	prepared, prepErr := PreparePath("a.bin", root)
	if prepErr != nil {
		t.Fatalf("PreparePath: %v", prepErr)
	}

	renderable := ClassifyOperationError(ErrBinarySource, prepared)

	if !IsOperationOutcomeRenderableAtJSONEdge(root, &renderable) {
		t.Fatalf("classified binary_source with prepared src should render")
	}

	unrender := ClassifyOperationError(ErrBinarySource, "")
	if IsOperationOutcomeRenderableAtJSONEdge(root, &unrender) {
		t.Fatalf("binary outcome without resolved src slots should fail JSON-edge render")
	}
}
