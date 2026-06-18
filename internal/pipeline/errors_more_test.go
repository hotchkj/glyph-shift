package pipeline

import (
	"errors"
	"testing"
)

func TestPipelineErrorUnwrapMethods(t *testing.T) {
	t.Parallel()

	if !errors.Is(pathContextError(PathRoleSrc, "in.txt", errClassifyTestUnexpected), errClassifyTestUnexpected) {
		t.Fatal("path context error should unwrap cause")
	}
}

func TestErrorOutcomeEquivalenceCoversMismatchBranches(t *testing.T) {
	t.Parallel()

	base := &ErrorOutcome{
		Error:             "invalid_input",
		Hint:              "hint",
		ExitCode:          ExitValidation,
		Src:               "src.txt",
		StringFields:      map[string]string{"field": "src"},
		IntFields:         map[string]int{"range_start": 1},
		StringArrayFields: map[string][]string{"missing_flags": {"--src", "--dest"}},
	}
	same := &ErrorOutcome{
		Error:             "invalid_input",
		Hint:              "hint",
		ExitCode:          ExitValidation,
		Src:               "src.txt",
		StringFields:      map[string]string{"field": "src"},
		IntFields:         map[string]int{"range_start": 1},
		StringArrayFields: map[string][]string{"missing_flags": {"--src", "--dest"}},
	}

	if !outcomesEqual(nil, nil) {
		t.Fatal("nil outcomes should compare equal")
	}
	if outcomesEqual(base, nil) {
		t.Fatal("nil and non-nil outcomes should not compare equal")
	}
	if !outcomesEqual(base, same) {
		t.Fatal("equivalent outcomes should compare equal")
	}

	mismatchedCore := *same
	mismatchedCore.Dest = "dest.txt"
	if outcomesEqual(base, &mismatchedCore) {
		t.Fatal("core path mismatch should not compare equal")
	}

	mismatchedString := *same
	mismatchedString.StringFields = map[string]string{"field": "dest"}
	if outcomesEqual(base, &mismatchedString) {
		t.Fatal("string field mismatch should not compare equal")
	}

	mismatchedArrayLen := *same
	mismatchedArrayLen.StringArrayFields = map[string][]string{"missing_flags": {"--src"}}
	if outcomesEqual(base, &mismatchedArrayLen) {
		t.Fatal("array length mismatch should not compare equal")
	}

	mismatchedArrayValue := *same
	mismatchedArrayValue.StringArrayFields = map[string][]string{"missing_flags": {"--src", "--out-dir"}}
	if outcomesEqual(base, &mismatchedArrayValue) {
		t.Fatal("array value mismatch should not compare equal")
	}
}
