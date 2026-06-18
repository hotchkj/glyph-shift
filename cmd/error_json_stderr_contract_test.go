package cmd

// errorJSONOutput captures stderr JSON operation envelopes asserted by cmd JSON contract tests.
type errorJSONOutput struct {
	Error            string   `json:"error"`
	Hint             string   `json:"hint,omitempty"`
	Src              string   `json:"src,omitempty"`
	Dest             string   `json:"dest,omitempty"`
	OutDir           string   `json:"out_dir,omitempty"`
	OutputPath       string   `json:"output_path,omitempty"`
	Command          string   `json:"command,omitempty"`
	Argument         string   `json:"argument,omitempty"`
	Field            string   `json:"field,omitempty"`
	Flag             string   `json:"flag,omitempty"`
	MissingFlags     []string `json:"missing_flags,omitempty"`
	MaxFiles         int      `json:"max_files,omitempty"`
	WouldCreateCount int      `json:"would_create_count,omitempty"`
	NamesCount       int      `json:"names_count,omitempty"`
	OutputCount      int      `json:"output_count,omitempty"`
	FileLines        int      `json:"file_lines,omitempty"`
	RangeStart       int      `json:"range_start,omitempty"`
	RangeEnd         int      `json:"range_end,omitempty"`
	StartLine        int      `json:"start_line,omitempty"`
}
