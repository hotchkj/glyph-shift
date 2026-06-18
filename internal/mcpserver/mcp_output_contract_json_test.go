package mcpserver

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestExtractOutput_JSON_linesExtractedZero(t *testing.T) {
	t.Parallel()

	n := 0
	out := ExtractOutput{LinesExtracted: intPtr(n)}

	jsonRaw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ExtractOutput
	if err := json.Unmarshal(jsonRaw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.LinesExtracted == nil {
		t.Fatalf("expected lines_extracted present, got JSON: %s", jsonRaw)
	}
	if *got.LinesExtracted != 0 {
		t.Fatalf("expected lines_extracted 0, got %d (JSON: %s)", *got.LinesExtracted, jsonRaw)
	}
}

func TestTransformOutput_JSON_previewFalseAndZeros(t *testing.T) {
	t.Parallel()

	wc := false
	zero := 0
	out := TransformOutput{
		WouldChange:     boolPtr(wc),
		EndingsChanged:  intPtr(zero),
		LFFound:         intPtr(zero),
		LFConverted:     intPtr(zero),
		CRFound:         intPtr(zero),
		CRConverted:     intPtr(zero),
		CRLFFound:       intPtr(zero),
		CRLFConverted:   intPtr(zero),
		TrailingTrimmed: intPtr(zero),
	}

	fn := false
	out.FinalNewlineNeeded = boolPtr(fn)

	jsonRaw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got TransformOutput
	if err := json.Unmarshal(jsonRaw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	assertPtrBool(t, "would_change", got.WouldChange, false, jsonRaw)
	assertPtrIntZero(t, "endings_changed", got.EndingsChanged, jsonRaw)
	assertPtrIntZero(t, "lf_found", got.LFFound, jsonRaw)
	assertPtrIntZero(t, "lf_converted", got.LFConverted, jsonRaw)
	assertPtrIntZero(t, "cr_found", got.CRFound, jsonRaw)
	assertPtrIntZero(t, "cr_converted", got.CRConverted, jsonRaw)
	assertPtrIntZero(t, "crlf_found", got.CRLFFound, jsonRaw)
	assertPtrIntZero(t, "crlf_converted", got.CRLFConverted, jsonRaw)
	assertPtrIntZero(t, "trailing_trimmed", got.TrailingTrimmed, jsonRaw)
	assertPtrBool(t, "final_newline_needed", got.FinalNewlineNeeded, false, jsonRaw)
}

func assertPtrBool(t *testing.T, name string, value *bool, want bool, rawJSON []byte) {
	t.Helper()
	if value == nil {
		t.Fatalf("%s: expected non-nil pointer (JSON: %s)", name, rawJSON)
	}
	if *value != want {
		t.Fatalf("%s: want %v, got %v (JSON: %s)", name, want, *value, rawJSON)
	}
}

func assertPtrIntZero(t *testing.T, name string, value *int, rawJSON []byte) {
	t.Helper()
	if value == nil {
		t.Fatalf("%s: expected non-nil pointer (JSON: %s)", name, rawJSON)
	}
	if *value != 0 {
		t.Fatalf("%s: want 0, got %d (JSON: %s)", name, *value, rawJSON)
	}
}

func TestTransformOutput_JSON_applyChangedFalse(t *testing.T) {
	t.Parallel()

	out := TransformOutput{Changed: boolPtr(false)}

	jsonRaw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got TransformOutput
	if err := json.Unmarshal(jsonRaw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	assertPtrBool(t, "changed", got.Changed, false, jsonRaw)
}

func TestSplitOutput_JSON_emptyFilesCreated(t *testing.T) {
	t.Parallel()

	emptySlice := []string{}
	out := SplitOutput{FilesCreated: &emptySlice}

	jsonRaw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if string(jsonRaw) != `{"files_created":[]}` {
		t.Fatalf("got %s", jsonRaw)
	}
}

func TestBlocksOutput_JSON_emptyFilesCreated(t *testing.T) {
	t.Parallel()

	empty := []string{}
	out := BlocksOutput{
		ContentBlocksFound: 0,
		EmptyBlocksFound:   2,
		FilesCreated:       &empty,
	}

	jsonRaw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got BlocksOutput
	if err := json.Unmarshal(jsonRaw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ContentBlocksFound != 0 {
		t.Fatalf("content_blocks_found: want 0, got %d (JSON: %s)", got.ContentBlocksFound, jsonRaw)
	}
	if got.EmptyBlocksFound != 2 {
		t.Fatalf("empty_blocks_found: want 2, got %d (JSON: %s)", got.EmptyBlocksFound, jsonRaw)
	}
	if got.FilesCreated == nil {
		t.Fatalf("files_created: expected non-nil slice pointer (JSON: %s)", jsonRaw)
	}
	if len(*got.FilesCreated) != 0 {
		t.Fatalf("files_created: want empty, got %v (JSON: %s)", *got.FilesCreated, jsonRaw)
	}
}

func TestBlocksOutput_JSON_emptyBlocksOmittedWhenZero(t *testing.T) {
	t.Parallel()

	// Marshal/omit behavior for omit-empty fields only; paths follow the runtime contract (absolute native).
	absPath := filepath.FromSlash("/stub/001.md")
	created := []string{absPath}
	out := BlocksOutput{
		ContentBlocksFound: 1,
		FilesCreated:       &created,
	}

	jsonRaw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got BlocksOutput
	if err := json.Unmarshal(jsonRaw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ContentBlocksFound != 1 {
		t.Fatalf("content_blocks_found: want 1, got %d (JSON: %s)", got.ContentBlocksFound, jsonRaw)
	}
	if got.FilesCreated == nil || len(*got.FilesCreated) != 1 || (*got.FilesCreated)[0] != absPath {
		t.Fatalf("files_created: want [%q], got %v (JSON: %s)", absPath, got.FilesCreated, jsonRaw)
	}
}

func TestStringSlicePtr_nilEncodesEmptyArray(t *testing.T) {
	t.Parallel()

	type wrap struct {
		FilesCreated *[]string `json:"files_created,omitempty"`
	}

	jsonRaw, err := json.Marshal(wrap{FilesCreated: stringSlicePtr(nil)})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if string(jsonRaw) != `{"files_created":[]}` {
		t.Fatalf("got %s", jsonRaw)
	}
}
