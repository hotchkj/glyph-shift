package cmd

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
)

func intPtr(v int) *int {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

func TestBuildTransformApplyOutput_emitsZeroEndingsChangedWhenLineEndingsRequested(t *testing.T) {
	t.Parallel()

	flags := &transformFlagValues{lineEndings: "lf"}
	res := fileops.TransformFileResult{
		WouldChange:    false,
		EndingsChanged: 0,
	}

	got := buildTransformApplyOutput(flags, res)
	want := transformApplyOutput{
		Changed:        false,
		EndingsChanged: intPtr(0),
		LFFound:        intPtr(0),
		LFConverted:    intPtr(0),
		CRFound:        intPtr(0),
		CRConverted:    intPtr(0),
		CRLFFound:      intPtr(0),
		CRLFConverted:  intPtr(0),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("apply output mismatch:\ngot:  %#v\nwant: %#v", got, want)
	}

	assertTransformJSONRoundtrip[transformApplyOutput](t, got)
}

func TestBuildTransformApplyOutput_emitsZeroTrailingWhenTrimRequested(t *testing.T) {
	t.Parallel()

	flags := &transformFlagValues{trimTrailing: true}
	res := fileops.TransformFileResult{
		WouldChange:     false,
		TrailingTrimmed: 0,
	}

	got := buildTransformApplyOutput(flags, res)
	want := transformApplyOutput{
		Changed:         false,
		TrailingTrimmed: intPtr(0),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("apply output mismatch:\ngot:  %#v\nwant: %#v", got, want)
	}

	assertTransformJSONRoundtrip[transformApplyOutput](t, got)
}

func TestBuildTransformApplyOutput_emitsFalseFinalNewlineWhenRequested(t *testing.T) {
	t.Parallel()

	flags := &transformFlagValues{finalNewline: true}
	res := fileops.TransformFileResult{
		WouldChange:       false,
		FinalNewlineAdded: false,
	}

	got := buildTransformApplyOutput(flags, res)
	want := transformApplyOutput{
		Changed:           false,
		FinalNewlineAdded: boolPtr(false),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("apply output mismatch:\ngot:  %#v\nwant: %#v", got, want)
	}

	assertTransformJSONRoundtrip[transformApplyOutput](t, got)
}

func TestBuildTransformPreviewOutput_emitsZerosAndFalseWhenRequested(t *testing.T) {
	t.Parallel()

	flags := &transformFlagValues{
		lineEndings: "lf", trimTrailing: true, finalNewline: true,
	}
	res := fileops.TransformFileResult{
		WouldChange:       false,
		EndingsChanged:    0,
		TrailingTrimmed:   0,
		FinalNewlineAdded: false,
	}

	got := buildTransformPreviewOutput(flags, res)
	want := transformPreviewOutput{
		WouldChange:        false,
		EndingsChanged:     intPtr(0),
		LFFound:            intPtr(0),
		LFConverted:        intPtr(0),
		CRFound:            intPtr(0),
		CRConverted:        intPtr(0),
		CRLFFound:          intPtr(0),
		CRLFConverted:      intPtr(0),
		TrailingTrimmed:    intPtr(0),
		FinalNewlineNeeded: boolPtr(false),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("preview output mismatch:\ngot:  %#v\nwant: %#v", got, want)
	}

	assertTransformJSONRoundtrip[transformPreviewOutput](t, got)
}

func assertTransformJSONRoundtrip[T any](t *testing.T, payload T) {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal roundtrip: %v", err)
	}

	var decoded T
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}

	if !reflect.DeepEqual(payload, decoded) {
		t.Fatalf("JSON round-trip changed shape: %#v vs %#v", payload, decoded)
	}
}
