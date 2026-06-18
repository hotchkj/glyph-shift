package mcpserver

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// structuredContentToMap re-encodes MCP structured payloads to a generic map for field assertions.
func structuredContentToMap(t *testing.T, structured any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(structured)
	if err != nil {
		t.Fatalf("marshal structuredContent: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal structuredContent map: %v", err)
	}

	return decoded
}

func assertDecodedJSONObjectIsFlatForTOONTransport(t *testing.T, decoded map[string]any, caseName string) {
	t.Helper()

	for key, raw := range decoded {
		switch rawTyped := raw.(type) {
		case string:
		case float64:
		case bool:
		case nil:
		case []any:
			for pathIdx, elt := range rawTyped {
				if _, ok := elt.(string); !ok {
					t.Fatalf("%s: array %s[%d] must be primitive string entries for TOON-ish flat JSON,"+
						" got %#v (%T)",
						caseName, key, pathIdx, elt, elt)
				}
			}
		default:
			t.Fatalf("%s: forbidden nested JSON aggregate at %s (want flat primitive + uniform string "+
				"slices only), got %#v (%T)",
				caseName, key, rawTyped, rawTyped)
		}
	}
}

// assertWouldCreateAbsoluteNativeDirectory asserts preview/apply paths are rooted on the filesystem
// and live directly under resolved outDirLogical (semantic "absolute native", not basename-only paths).
//
//nolint:unparam // Fixtures use "out" today; keeping the logical dir explicit preserves call-site readability.
func assertStringSlicePathsAbsoluteUnderResolvedDir(
	t *testing.T,
	srv *GlyphShiftServer,
	outDirLogical string,
	structured any,
	field string,
) {
	t.Helper()

	m := structuredContentToMap(t, structured)
	rawSlice, ok := m[field].([]any)
	if !ok {
		t.Fatalf("%s: missing or not array in %#v", field, m)
	}

	wantDir := filepath.Clean(mustToolPath(t, srv, outDirLogical))

	for pathIdx, elt := range rawSlice {
		pathNative, ok := elt.(string)
		if !ok {
			t.Fatalf("%s[%d]: not string %#v", field, pathIdx, elt)
		}
		if !filepath.IsAbs(pathNative) {
			t.Fatalf("%s[%d]: must be absolute native path, got %q", field, pathIdx, pathNative)
		}
		if filepath.Clean(filepath.Dir(filepath.Clean(pathNative))) != wantDir {
			t.Fatalf("%s[%d]: must be child of resolved out dir.\ngot dir=%q\ngot path=%q\nwant dir=%q",
				field, pathIdx, filepath.Dir(pathNative), pathNative, wantDir)
		}
		if filepath.Base(pathNative) == pathNative {
			t.Fatalf("%s[%d]: path appears to be basename only: %q", field, pathIdx, pathNative)
		}
	}
}
