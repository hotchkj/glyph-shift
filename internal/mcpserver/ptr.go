package mcpserver

// intPtr returns a stable pointer copy of v (for JSON output fields that must encode 0).
func intPtr(v int) *int {
	return &v
}

// boolPtr returns a stable pointer copy of v (for JSON output fields that must encode false).
func boolPtr(v bool) *bool {
	return &v
}

// stringPtr returns a pointer to s.
func stringPtr(s string) *string {
	return &s
}

// stringSlicePtr returns a pointer to a slice with non-nil backing so JSON encodes [] not null.
// (A pointer to a nil slice body marshals as null.)
// mcpMaxFilesForPipeline maps optional MCP max_files to pipeline (0 means use default when omitted).
func mcpMaxFilesForPipeline(p *int) int {
	if p == nil {
		return 0
	}

	return *p
}

func stringSlicePtr(slice []string) *[]string {
	if len(slice) == 0 {
		empty := []string{}

		return &empty
	}

	cp := append([]string(nil), slice...)

	return &cp
}
