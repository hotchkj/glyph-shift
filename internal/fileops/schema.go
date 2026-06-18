package fileops

// ParameterDescriptor describes a CLI flag for schema introspection.
type ParameterDescriptor struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     string `json:"default"`
	Description string `json:"description"`
}

// OutputFieldDescriptor describes a JSON output field for an operation.
type OutputFieldDescriptor struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// OperationSchema is the runtime JSON schema for one glyph-shift operation.
type OperationSchema struct {
	Name         string                  `json:"name"`
	Description  string                  `json:"description"`
	Parameters   []ParameterDescriptor   `json:"parameters"`
	OutputFields []OutputFieldDescriptor `json:"output_fields"`
}

// BuiltinOperationSchemas returns self-description metadata for all built-in operations.
func BuiltinOperationSchemas() []OperationSchema {
	return []OperationSchema{
		extractOperationSchema(),
		splitOperationSchema(),
		blocksOperationSchema(),
		transformOperationSchema(),
	}
}

func extractOperationSchema() OperationSchema {
	return OperationSchema{
		Name:        "extract",
		Description: "Extract a 1-based inclusive line range from the source file to the destination.",
		Parameters: []ParameterDescriptor{
			{
				Name: "source", Type: "string", Required: true, Default: "",
				Description: "Source file path.",
			},
			{
				Name: "lines", Type: "string", Required: true, Default: "",
				Description: "Line range (e.g. 45-55, 95-, -10).",
			},
			{
				Name: "destination", Type: "string", Required: true, Default: "",
				Description: "Destination file path.",
			},
			{
				Name: "force", Type: "boolean", Required: false, Default: "false",
				Description: "Overwrite existing destination.",
			},
			{
				Name: "append", Type: "boolean", Required: false, Default: "false",
				Description: "Append to existing destination.",
			},
			{
				Name: "mkdir", Type: "boolean", Required: false, Default: "false",
				Description: "Create destination parent directories.",
			},
			{
				Name: "preview", Type: "boolean", Required: false, Default: "false",
				Description: "Validate and report line count without writing the destination.",
			},
		},
		OutputFields: []OutputFieldDescriptor{
			{Name: "lines_extracted", Type: "integer", Description: "Apply: lines written to destination."},
			{Name: "would_extract_lines", Type: "integer", Description: "Preview: lines that would be written."},
			{Name: "would_create", Type: "string", Description: "Preview: destination path that would be created."},
		},
	}
}

func splitOperationSchema() OperationSchema {
	return OperationSchema{
		Name:        "split",
		Description: "Split a file into multiple files at each line matching the delimiter pattern.",
		Parameters: []ParameterDescriptor{
			{
				Name: "source", Type: "string", Required: true, Default: "",
				Description: "Source file path.",
			},
			{
				Name: "delimiter", Type: "string", Required: true, Default: "",
				Description: "Regular expression for delimiter lines.",
			},
			{
				Name: "output-dir", Type: "string", Required: true, Default: "",
				Description: "Output directory.",
			},
			{
				Name: "extension", Type: "string", Required: false, Default: "",
				Description: `Output filename extension (include leading "."); default from source file.`,
			},
			{
				Name: "names", Type: "string", Required: false, Default: "",
				Description: "Comma-separated explicit output basenames (single path segment each).",
			},
			{
				Name: "max-files", Type: "integer", Required: false, Default: "50",
				Description: "Maximum number of output sections.",
			},
			{
				Name: "strip-delimiter", Type: "boolean", Required: false, Default: "false",
				Description: "Omit delimiter line from each section output.",
			},
			{
				Name: "force", Type: "boolean", Required: false, Default: "false",
				Description: "Overwrite existing output files.",
			},
			{
				Name: "mkdir", Type: "boolean", Required: false, Default: "false",
				Description: "Create output directory if missing.",
			},
			{
				Name: "preview", Type: "boolean", Required: false, Default: "false",
				Description: "Report output basenames without writes or creating the output directory.",
			},
		},
		OutputFields: []OutputFieldDescriptor{
			{Name: "files_created", Type: "array", Description: "Apply: absolute native paths of files created."},
			{Name: "would_create", Type: "array", Description: "Preview: absolute native paths that would be created."},
		},
	}
}

func blocksOperationSchema() OperationSchema {
	return OperationSchema{
		Name:        "blocks",
		Description: "Extract lines between start and end delimiter patterns into separate files.",
		Parameters: []ParameterDescriptor{
			{
				Name: "source", Type: "string", Required: true, Default: "",
				Description: "Source file path.",
			},
			{
				Name: "start-line", Type: "string", Required: true, Default: "",
				Description: "Regular expression for start delimiter lines.",
			},
			{
				Name: "end-line", Type: "string", Required: true, Default: "",
				Description: "Regular expression for end delimiter lines.",
			},
			{
				Name: "output-dir", Type: "string", Required: true, Default: "",
				Description: "Output directory.",
			},
			{
				Name: "extension", Type: "string", Required: false, Default: "",
				Description: `Output filename extension (include leading "."); default from source file.`,
			},
			{
				Name: "names", Type: "string", Required: false, Default: "",
				Description: "Comma-separated basenames for non-empty blocks (single path segment each).",
			},
			{
				Name: "max-files", Type: "integer", Required: false, Default: "50",
				Description: "Maximum matched blocks (including empty blocks).",
			},
			{
				Name: "include-delimiters", Type: "boolean", Required: false, Default: "false",
				Description: "Include delimiter lines in each block output.",
			},
			{
				Name: "force", Type: "boolean", Required: false, Default: "false",
				Description: "Overwrite existing output files.",
			},
			{
				Name: "mkdir", Type: "boolean", Required: false, Default: "false",
				Description: "Create output directory if missing.",
			},
			{
				Name: "preview", Type: "boolean", Required: false, Default: "false",
				Description: "Report block metadata and basenames without writes or creating the output directory.",
			},
		},
		OutputFields: []OutputFieldDescriptor{
			{Name: "content_blocks_found", Type: "integer", Description: "Non-empty blocks that produce output files."},
			{Name: "empty_blocks_found", Type: "integer", Description: "Empty matched blocks, present only when non-zero."},
			{Name: "files_created", Type: "array", Description: "Apply: absolute native paths of files created."},
			{Name: "would_create", Type: "array", Description: "Preview: absolute native paths that would be created."},
		},
	}
}

func transformOperationSchema() OperationSchema {
	return OperationSchema{
		Name: "transform",
		Description: "Mechanical line-ending and whitespace transforms in-place. " +
			"Executes by default; use preview to inspect without writing.",
		Parameters: []ParameterDescriptor{
			{
				Name: "source", Type: "string", Required: true, Default: "",
				Description: "Source file path.",
			},
			{
				Name: "line-endings", Type: "string", Required: false, Default: "",
				Description: `Target line endings: "lf", "crlf", or "cr".`,
			},
			{
				Name: "trim-trailing", Type: "boolean", Required: false, Default: "false",
				Description: "Trim trailing spaces and tabs on each line.",
			},
			{
				Name: "final-newline", Type: "boolean", Required: false, Default: "false",
				Description: "Ensure the file ends with a newline.",
			},
			{
				Name: "preview", Type: "boolean", Required: false, Default: "false",
				Description: "Inspect and report what would change without modifying the file.",
			},
		},
		OutputFields: []OutputFieldDescriptor{
			{Name: "changed", Type: "boolean", Description: "Apply: whether the file was modified."},
			{Name: "would_change", Type: "boolean", Description: "Preview: whether applying would modify the file."},
			{
				Name: "endings_changed", Type: "integer",
				Description: "When line-endings requested: total terminators converted to the target.",
			},
			{
				Name: "lf_found", Type: "integer",
				Description: "When line-endings requested: lines whose source terminator was LF (\\n only).",
			},
			{
				Name: "lf_converted", Type: "integer",
				Description: "When line-endings requested: LF terminators rewritten to reach the target.",
			},
			{
				Name: "cr_found", Type: "integer",
				Description: "When line-endings requested: lines whose source terminator was standalone CR.",
			},
			{
				Name: "cr_converted", Type: "integer",
				Description: "When line-endings requested: CR terminators rewritten to reach the target.",
			},
			{
				Name: "crlf_found", Type: "integer",
				Description: "When line-endings requested: lines whose source terminator was CRLF.",
			},
			{
				Name: "crlf_converted", Type: "integer",
				Description: "When line-endings requested: CRLF terminators rewritten to reach the target.",
			},
			{
				Name: "trailing_trimmed", Type: "integer",
				Description: "When trim-trailing requested: lines trimmed (apply) or that would be trimmed (preview).",
			},
			{
				Name: "final_newline_added", Type: "boolean",
				Description: "Apply with final-newline: whether a final newline was added.",
			},
			{
				Name: "final_newline_needed", Type: "boolean",
				Description: "Preview with final-newline: whether a final newline would be needed.",
			},
		},
	}
}
