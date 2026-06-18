# glyph-shift - JSON Output Contract

Attendant to [glyph-shift-intent.md](glyph-shift-intent.md). Defines JSON structures for operation subcommands (`extract`, `split`, `blocks`, `transform`): success payloads on stdout, error objects on stderr. BDD features assert against these field names and types for those commands.

**Out of scope here:** `help`, `--help`, `version`, and other plain-text operational output. Those paths do not participate in this JSON contract.

## Principles

- Stdout carries success JSON. Stderr carries error JSON. An invocation either succeeds or fails - the streams never mix.
- JSON field names are `snake_case`.
- Return only new information. Do not repeat request inputs just because the agent passed them. Omit paths, flags, and operation names unless the tool generated, normalized, validated, or selected them in a way the agent could not rely on without the response. Include computed counts, derived basenames, normalized or validated planned output paths, boolean outcomes of inspection, and any field materially produced by the tool. If a field duplicates the request verbatim with no added semantics, drop it.
- JSON values that report tool-generated, validated, or error-context paths are **absolute paths in native form for the host OS** (roots, drive letters where applicable, and native directory separators). They are neither project-relative paths nor bare basenames.
- With `--preview`, the tool performs the same validation that would reject apply. Preview writes no files; stderr carries the operation error JSON, stdout is empty, and exit code reflects the sentinel category.
- Apply uses past tense. Something happened: counts and flags such as `lines_extracted`, `files_created`, `changed`, `trailing_trimmed`, `final_newline_added`.
- Preview uses prospective naming. Nothing has been written yet: `would_extract_lines`, `would_create`, `would_change`, `final_newline_needed`, and analogous fields defined in each operation section below. Preview objects are not the same shape as apply - mutually exclusive schemas, not one schema with a toggle.
- TOON-compatible. All output structures are flat objects with primitive values and uniform string arrays - no nested objects. This ensures lossless, token-efficient encoding via [TOON](https://github.com/toon-format/toon) for agents that use it as a transport format.

---

## extract

### extract apply (default)

```json
{
  "lines_extracted": 76
}
```

| Field | Type | Description |
|---|---|---|
| `lines_extracted` | integer | Lines written to destination |

### extract preview (`--preview`)

```json
{
  "would_extract_lines": 76,
  "would_create": "D:\\workspace\\repo\\out\\fragment.go"
}
```

(On Unix, the same field uses a rooted native path such as `/workspace/repo/out/fragment.go`.)

| Field | Type | Description |
|---|---|---|
| `would_extract_lines` | integer | Lines that would be written |
| `would_create` | string | Absolute native path to the destination that would be created after workspace resolution and validation |

---

## split

### split apply (default)

```json
{
  "files_created": [
    "D:\\workspace\\repo\\out\\001.md",
    "D:\\workspace\\repo\\out\\002.md",
    "D:\\workspace\\repo\\out\\003.md",
    "D:\\workspace\\repo\\out\\004.md"
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `files_created` | string array | Absolute native paths of files created under the output directory, ordered by position in source |

### split preview (`--preview`)

```json
{
  "would_create": [
    "D:\\workspace\\repo\\out\\001.md",
    "D:\\workspace\\repo\\out\\002.md",
    "D:\\workspace\\repo\\out\\003.md",
    "D:\\workspace\\repo\\out\\004.md"
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `would_create` | string array | Absolute native paths of files that would be created |

### Delimiter matches no lines

When `--delimiter` matches no line in the source, the operation **fails**. Stdout is empty. Stderr contains the standard error object with `error` set to `no_delimiter_match` (category: validation / input, exit code `6`). This is a required contract alignment target: success-shaped output must not disguise a no-match split.

---

## blocks

### blocks apply (default)

```json
{
  "content_blocks_found": 2,
  "empty_blocks_found": 1,
  "files_created": [
    "D:\\workspace\\repo\\out\\001.md",
    "D:\\workspace\\repo\\out\\002.md"
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `content_blocks_found` | integer | Non-empty blocks matched; equals the number of output files created |
| `empty_blocks_found` | integer | Empty blocks matched. Present only when non-zero. |
| `files_created` | string array | Absolute native paths of files created for non-empty blocks, ordered by position in source. |

### blocks preview (`--preview`)

```json
{
  "content_blocks_found": 2,
  "empty_blocks_found": 1,
  "would_create": [
    "D:\\workspace\\repo\\out\\001.md",
    "D:\\workspace\\repo\\out\\002.md"
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `content_blocks_found` | integer | Non-empty blocks that would be matched; equals the number of files that would be created |
| `empty_blocks_found` | integer | Empty blocks that would be matched. Present only when non-zero. |
| `would_create` | string array | Absolute native paths that would be created for non-empty blocks. |

`files_created` and `would_create` list one absolute native path per content block output. When `--names` is supplied, the name count must equal `content_blocks_found`. Empty means no content between the start and end delimiters; this metadata definition is unchanged by `--include-delimiters`.

### Start/end patterns match no complete blocks

When the start/end patterns match no complete block in the source, the operation **fails**. Stdout is empty. Stderr contains the standard error object with `error` set to `no_blocks_found` (category: validation / input, exit code `6`). A complete empty block is still a valid match: it increments `empty_blocks_found`, produces no output file, and succeeds unless another validation error applies.

---

## transform

Fields are present only for transforms that were requested. Per-type line-ending fields are included whenever `--line-endings` is set (apply or preview). Each `_found` count records how many lines had that source terminator shape (standalone `\n`, standalone `\r`, or `\r\n`). Each `_converted` count records how many of those terminators required rewriting to reach the explicit target (`lf`, `crlf`, or `cr`). The aggregate `endings_changed` equals the sum of `lf_converted`, `cr_converted`, and `crlf_converted`. Boolean and integer fields encode `false` and `0` when those are the computed values - they are not omitted.

### transform apply (default)

When invoked with `--line-endings crlf --trim-trailing`:

```json
{
  "changed": true,
  "endings_changed": 87,
  "lf_found": 12,
  "lf_converted": 12,
  "cr_found": 0,
  "cr_converted": 0,
  "crlf_found": 75,
  "crlf_converted": 75,
  "trailing_trimmed": 3
}
```

When invoked with `--final-newline` only:

```json
{
  "changed": true,
  "final_newline_added": true
}
```

| Field | Type | Present | Description |
|---|---|---|---|
| `changed` | boolean | always | Whether the file was modified |
| `endings_changed` | integer | `--line-endings` | Total terminators converted to the target |
| `lf_found` | integer | `--line-endings` | Lines whose source terminator was LF (`\n` only) |
| `lf_converted` | integer | `--line-endings` | LF terminators rewritten to reach the target |
| `cr_found` | integer | `--line-endings` | Lines whose source terminator was standalone CR |
| `cr_converted` | integer | `--line-endings` | CR terminators rewritten to reach the target |
| `crlf_found` | integer | `--line-endings` | Lines whose source terminator was CRLF |
| `crlf_converted` | integer | `--line-endings` | CRLF terminators rewritten to reach the target |
| `trailing_trimmed` | integer | `--trim-trailing` | Lines trimmed of trailing whitespace |
| `final_newline_added` | boolean | `--final-newline` | Whether a final newline was added |

### transform preview (`--preview`)

When invoked with `--preview --line-endings crlf --trim-trailing`:

```json
{
  "would_change": true,
  "endings_changed": 87,
  "lf_found": 12,
  "lf_converted": 12,
  "cr_found": 0,
  "cr_converted": 0,
  "crlf_found": 75,
  "crlf_converted": 75,
  "trailing_trimmed": 3
}
```

When invoked with `--preview --final-newline` only:

```json
{
  "would_change": false,
  "final_newline_needed": false
}
```

| Field | Type | Present | Description |
|---|---|---|---|
| `would_change` | boolean | always | Whether applying would modify the file |
| `endings_changed` | integer | `--line-endings` | Total terminators that would be converted to the target |
| `lf_found` | integer | `--line-endings` | Lines whose source terminator is LF (`\n` only) |
| `lf_converted` | integer | `--line-endings` | LF terminators that would be rewritten |
| `cr_found` | integer | `--line-endings` | Lines whose source terminator is standalone CR |
| `cr_converted` | integer | `--line-endings` | CR terminators that would be rewritten |
| `crlf_found` | integer | `--line-endings` | Lines whose source terminator is CRLF |
| `crlf_converted` | integer | `--line-endings` | CRLF terminators that would be rewritten |
| `trailing_trimmed` | integer | `--trim-trailing` | Lines that would be trimmed |
| `final_newline_needed` | boolean | `--final-newline` | Whether the file would need a final newline added |

---

## Error response

When an operation fails, stderr contains a JSON error object and the process exits with a non-zero code. Stdout is empty on failure.

```json
{
  "error": "range_exceeds_file",
  "src": "D:\\workspace\\repo\\src\\main.go",
  "file_lines": 50,
  "range_start": 45,
  "range_end": 120,
  "hint": "File has 50 lines but range 45-120 was requested. Adjust --lines to match the actual file length."
}
```

### Requirements

The error object has to satisfy these constraints:

- It must remain losslessly convertible through TOON
- It must preserve one stable branch key: `error`
- It must identify recovery context only when the tool has selected, validated, normalized, generated, or rejected something material; the contract should not echo request inputs just to fill a field
- It must make context field names meaningful to consumers; concepts should not be hidden behind one generic value slot
- It must emit fields only when non-null, non-empty content exists. Strings must be non-empty; arrays must contain at least one value; numbers must be emitted only when the value is known and semantically meaningful. No placeholder empty strings, empty arrays, nulls, or `"none"` values.
- It must render all emitted path fields to the same standard as the success schema: absolute native host paths at the emitted JSON edge. Internal pipeline classification should carry canonical path context; conversion to native absolute strings belongs in the CLI/MCP JSON formation layer, not in lower-level error construction.
- It must keep CLI and MCP parity. Both transports should serialize the same operation error payload from the same classifier.

### Error Schemas

A strict flat `oneOf` of error variants. Every variant is still TOON-compatible because every value is a primitive scalar or an explicitly allowed uniform string array, and no variant contains nested objects.

All variants require:

```text
error: string const or enum
hint: string
```

Context fields are required only on variants where they are useful to the consumer.

In each variant, a field listed under `required` is a producer obligation: the variant may only be emitted when every required value exists and passes the non-empty rule. If a context value is missing, the producer must choose a different valid variant or treat that as a classifier/serialization bug.

Variants are discriminated by `error` plus the required context field set. Producers and validators must not parse `hint` to choose a variant or synthesize structured fields.

Required integer fields are emitted when their variant is chosen, including `0` when `0` is semantically valid.

Field names must align with existing input and success-output vocabulary:

- Use input field names for normalized input-path context: `src`, `dest`, and `out_dir`.
- Use `field` when rejected context is the logical input field. Valid `field` values include `lines`, `delimiter`, `start_line`, `end_line`, `line_endings`, `max_files`, and `names`.
- Use top-level rejected-token fields only where an exact variant requires them: `argument`, `command`, `flag`, and `missing_flags`.
- Use success-output vocabulary when the context is generated output metadata: `would_create`, `files_created`, `content_blocks_found`, `empty_blocks_found`, and transform count names where applicable.
- Use `output_path` only for a single generated/planned output path that has no existing singular success-output field without changing the type of an existing field.

### Operation Applicability

Every error variant is documented with the operation or transport surface that can emit it. Applicability tags:

- `extract`: operation subcommand and MCP extract tool.
- `split`: operation subcommand and MCP split tool.
- `blocks`: operation subcommand and MCP blocks tool.
- `transform`: operation subcommand and MCP transform tool.
- `cli_dispatch`: CLI command selection before an operation runs.
- `cli_flags`: CLI flag parsing or required flag validation before an operation runs.
- `cli_json`: CLI JSON serialization of an operation error.
- `mcp_decode`: MCP argument decoding before an operation runs.
- `mcp_json`: MCP JSON serialization of an operation error into `structuredContent` and mirrored `content`.
- `mcp_tools`: MCP tool execution result wrapping for operation failures.

For `write_failed`, operation tags identify the operation whose error response was being serialized; `cli_json` and `mcp_json` identify the serialization surface that emits the sentinel.

Sentinel coverage by operation or surface:

- `extract`: `source_not_found`, `binary_source`, `directory_not_file`, `not_regular_file`, `empty_range`, `range_exceeds_file`, `destination_exists`, `source_fingerprint_mismatch`, path-scoped/base `invalid_input`, `write_failed`, and `internal_error`.
- `split`: `source_not_found`, `binary_source`, `directory_not_file`, `not_regular_file`, `no_delimiter_match`, `invalid_pattern`, `pattern_too_long`, `control_chars_in_input`, `destination_exists`, `max_files_exceeded`, `names_count_mismatch`, path-scoped/base `invalid_input`, `write_failed`, and `internal_error`.
- `blocks`: `source_not_found`, `binary_source`, `directory_not_file`, `not_regular_file`, `no_blocks_found`, `unclosed_block`, `invalid_pattern`, `pattern_too_long`, `control_chars_in_input`, `destination_exists`, `max_files_exceeded`, `names_count_mismatch`, path-scoped/base `invalid_input`, `write_failed`, and `internal_error`.
- `transform`: `source_not_found`, `binary_source`, `directory_not_file`, `not_regular_file`, `no_transform_specified`, `invalid_line_endings`, path-scoped/base `invalid_input`, `write_failed`, and `internal_error`.
- `cli_dispatch`: `unknown_command` and CLI `unexpected_argument`.
- `cli_flags`: `invalid_flag` and `missing_required_flag`.
- `cli_json`: `write_failed` when CLI operation error serialization fails under the shared JSON-edge formatter model.
- `mcp_decode`: MCP `unexpected_argument` with `field`.
- `mcp_json`: `write_failed` when MCP operation error serialization fails under the shared JSON-edge formatter model.
- `mcp_tools`: operation failures from `extract`, `split`, `blocks`, and `transform` when returned through MCP tool results.

### Exact Error Variants

Source-path operation failures:

Applies to:

- `source_not_found`, `binary_source`, `directory_not_file`, `not_regular_file`: `extract`, `split`, `blocks`, `transform`, and `mcp_tools`.
- `no_delimiter_match`: `split` and `mcp_tools`.
- `no_blocks_found`: `blocks` and `mcp_tools`.

```text
required: [error, src, hint]
properties:
  error: enum [
    source_not_found,
    binary_source,
    directory_not_file,
    not_regular_file,
    no_delimiter_match,
    no_blocks_found
  ]
  src: string
  hint: string
additionalProperties: false
```

Line/range source failures should expose the source path and structured range details where available.

Applies to:

- `empty_range`, `range_exceeds_file`: `extract` and `mcp_tools`.
- `unclosed_block`: `blocks` and `mcp_tools`.

```text
required: [error, src, range_start, range_end, hint]
properties:
  error: const empty_range
  src: string
  range_start: integer
  range_end: integer
  hint: string
additionalProperties: false
```

```text
required: [error, src, file_lines, range_start, range_end, hint]
properties:
  error: const range_exceeds_file
  src: string
  file_lines: integer
  range_start: integer
  range_end: integer
  hint: string
additionalProperties: false
```

```text
required: [error, src, start_line, hint]
properties:
  error: const unclosed_block
  src: string
  start_line: integer
  hint: string
additionalProperties: false
```

Destination and output path failures:

Applies to:

- `destination_exists`: `extract`, `split`, `blocks`, and `mcp_tools`.
- `source_fingerprint_mismatch`: `extract`, `split`, `blocks`, and `mcp_tools` during output publish or replay validation.

```text
required: [error, dest, hint]
properties:
  error: const destination_exists
  dest: string
  hint: string
additionalProperties: false
```

```text
required: [error, output_path, hint]
properties:
  error: const source_fingerprint_mismatch
  output_path: string
  hint: string
additionalProperties: false
```

Input field and pattern failures:

Applies to:

- `invalid_pattern`, `pattern_too_long`: `split` delimiter validation, `blocks` start/end validation, and `mcp_tools`. Empty regex patterns are `invalid_pattern` for the rejected logical field.
- Field-scoped `control_chars_in_input`: operation input-field validation for `extract`, `split`, `blocks`, `transform`, and `mcp_tools`.

```text
required: [error, field, hint]
properties:
  error: enum [invalid_pattern, pattern_too_long, control_chars_in_input]
  field: string
  hint: string
additionalProperties: false
```

Path-scoped validation failures:

Applies to:

- `src`: source path validation for `extract`, `split`, `blocks`, `transform`, and `mcp_tools`.
- `dest`: extract destination validation and `mcp_tools`.
- `out_dir`: split/blocks output-directory validation and `mcp_tools`.
- `output_path`: generated or planned output-file validation for `split`, `blocks`, and `mcp_tools`.

```text
required: [error, src, hint]
properties:
  error: enum [invalid_input, control_chars_in_input]
  src: string
  hint: string
additionalProperties: false
```

```text
required: [error, dest, hint]
properties:
  error: enum [invalid_input, control_chars_in_input]
  dest: string
  hint: string
additionalProperties: false
```

```text
required: [error, out_dir, hint]
properties:
  error: enum [invalid_input, control_chars_in_input]
  out_dir: string
  hint: string
additionalProperties: false
```

```text
required: [error, output_path, hint]
properties:
  error: enum [invalid_input, control_chars_in_input]
  output_path: string
  hint: string
additionalProperties: false
```

Base validation failures with no useful additional machine context:

Applies to:

- Base `invalid_input`: `extract`, `split`, `blocks`, `transform`, and `mcp_tools`.
- `no_transform_specified`, `invalid_line_endings`: `transform` and `mcp_tools`.

```text
required: [error, hint]
properties:
  error: enum [
    invalid_input,
    no_transform_specified,
    invalid_line_endings
  ]
  hint: string
additionalProperties: false
```

Limit and cardinality failures:

Applies to:

- `max_files_exceeded`, `names_count_mismatch`: `split`, `blocks`, and `mcp_tools`.

```text
required: [error, max_files, would_create_count, hint]
properties:
  error: const max_files_exceeded
  max_files: integer
  would_create_count: integer
  hint: string
additionalProperties: false
```

```text
required: [error, names_count, output_count, hint]
properties:
  error: const names_count_mismatch
  names_count: integer
  output_count: integer
  hint: string
additionalProperties: false
```

CLI dispatch and flag failures:

Applies to:

- `unknown_command`, CLI `unexpected_argument`: `cli_dispatch`.
- `invalid_flag`, `missing_required_flag`: `cli_flags`.

```text
required: [error, command, hint]
properties:
  error: const unknown_command
  command: string
  hint: string
additionalProperties: false
```

```text
required: [error, argument, hint]
properties:
  error: const unexpected_argument
  argument: string
  hint: string
additionalProperties: false
```

```text
required: [error, flag, hint]
properties:
  error: const invalid_flag
  flag: string
  hint: string
additionalProperties: false
```

```text
required: [error, missing_flags, hint]
properties:
  error: const missing_required_flag
  missing_flags: string array
  hint: string
additionalProperties: false
```

MCP argument decoding failures:

Applies to:

- MCP `unexpected_argument` with `field`: `mcp_decode`.

```text
required: [error, field, hint]
properties:
  error: const unexpected_argument
  field: string
  hint: string
additionalProperties: false
```

When JSON tool arguments cannot be decoded to the tool input shape and no specific unknown field name can be recovered (for example malformed JSON or type mismatch without a concrete member path), the server still emits `unexpected_argument` with the MCP-decode variant above and sets `field` to the reserved sentinel string `json` so clients always receive a non-empty logical field for this variant (mutually exclusive with CLI `argument`).

Write and internal failures:

Applies to:

- `write_failed`: `extract`, `split`, `blocks`, and `transform` at the `cli_json` or `mcp_json` serialization surface when using the shared JSON-edge formatter model.
- Base and path-scoped `internal_error`: `extract`, `split`, `blocks`, `transform`, and `mcp_tools`; path fields identify the selected typed context when available.

`internal_error` emits at most one path context field. Prefer typed role-aware path context over fallback context. Use `output_path` when a generated or planned output file is the selected failure context, including when a fallback `src` is also available. Use `src` when a source file is the selected failure context and no `output_path` context is present. Use the base variant when no single typed path context is available.

```text
required: [error, hint]
properties:
  error: enum [write_failed, internal_error]
  hint: string
additionalProperties: false
```

```text
required: [error, src, hint]
properties:
  error: const internal_error
  src: string
  hint: string
additionalProperties: false
```

```text
required: [error, output_path, hint]
properties:
  error: const internal_error
  output_path: string
  hint: string
additionalProperties: false
```

### Error sentinels and exit codes

The `error` field carries the specific sentinel. The exit code carries the broad category. Multiple sentinels map to the same exit code. This table is the target operation contract.

| Sentinel | Exit code | Category | Surfaces / operations |
| --- | ---: | --- | --- |
| `internal_error` | 1 | general error | `extract`, `split`, `blocks`, `transform`, `mcp_tools` |
| `source_fingerprint_mismatch` | 1 | general error | `extract`, `split`, `blocks`, `mcp_tools` |
| `write_failed` | 1 | general error | `extract`, `split`, `blocks`, `transform`, `cli_json`, `mcp_json` |
| `source_not_found` | 2 | source not found | `extract`, `split`, `blocks`, `transform`, `mcp_tools` |
| `binary_source` | 3 | binary source | `extract`, `split`, `blocks`, `transform`, `mcp_tools` |
| `destination_exists` | 4 | destination exists | `extract`, `split`, `blocks`, `mcp_tools` |
| `not_regular_file` | 5 | not regular file | `extract`, `split`, `blocks`, `transform`, `mcp_tools` |
| `directory_not_file` | 5 | not regular file | `extract`, `split`, `blocks`, `transform`, `mcp_tools` |
| `unknown_command` | 6 | validation / input | `cli_dispatch` |
| `invalid_flag` | 6 | validation / input | `cli_flags` |
| `missing_required_flag` | 6 | validation / input | `cli_flags` |
| `unexpected_argument` | 6 | validation / input | `cli_dispatch`, `mcp_decode` |
| `invalid_input` | 6 | validation / input | `extract`, `split`, `blocks`, `transform`, `mcp_tools` |
| `range_exceeds_file` | 6 | validation / input | `extract`, `mcp_tools` |
| `empty_range` | 6 | validation / input | `extract`, `mcp_tools` |
| `invalid_pattern` | 6 | validation / input | `split`, `blocks`, `mcp_tools` |
| `pattern_too_long` | 6 | validation / input | `split`, `blocks`, `mcp_tools` |
| `control_chars_in_input` | 6 | validation / input | `extract`, `split`, `blocks`, `transform`, `mcp_tools` |
| `unclosed_block` | 6 | validation / input | `blocks`, `mcp_tools` |
| `no_blocks_found` | 6 | validation / input | `blocks`, `mcp_tools` |
| `no_delimiter_match` | 6 | validation / input | `split`, `mcp_tools` |
| `names_count_mismatch` | 6 | validation / input | `split`, `blocks`, `mcp_tools` |
| `max_files_exceeded` | 6 | validation / input | `split`, `blocks`, `mcp_tools` |
| `no_transform_specified` | 6 | validation / input | `transform`, `mcp_tools` |
| `invalid_line_endings` | 6 | validation / input | `transform`, `mcp_tools` |

### Stream separation

- CLI **stdout**: success JSON only - empty on failure
- CLI **stderr**: error JSON (on failure) plus slog diagnostics when diagnostics are emitted

An invocation either succeeds or fails. On CLI success the agent reads stdout. On CLI failure the agent reads stderr for the JSON error object and the exit code for the broad category. slog diagnostic lines may also appear on stderr but use a distinct structure (`{"time":..., "level":..., "msg":...}`) and are supplementary - agents branch on exit code and the `error` field, not slog output.

---

## mcp

The `mcp` subcommand cannot print operation JSON to stdout - stdout is the JSON-RPC transport. MCP reuses the same operation payload schemas defined above inside the standard MCP tool result wrapper.

Each MCP tool declares an `outputSchema`. That schema is the JSON Schema form of this document's operation result contract:

- success variants for that tool (`apply` and `preview` shapes where both exist)
- a per-tool subset of the operation error `oneOf` variants defined above (sentinels each tool can actually return); each tool's `$defs.OperationError` lists only branches that tool may return
- CLI-only error shapes are omitted from MCP `outputSchema`
- schemas must use `$defs` / `$ref` (for example `#/$defs/OperationError`) to keep descriptors token-efficient
- schemas must be as token-efficient as possible

Successful tool calls return the operation success payload in `structuredContent`. For compatibility with clients that only read text content, the same object is also serialized as JSON in a `content` text block. MCP examples quote the full JSON-RPC response envelope from `jsonrpc` downward so it is always clear which protocol layer owns each field.

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"files_created\":[\"D:\\\\workspace\\\\repo\\\\out\\\\001.md\",\"D:\\\\workspace\\\\repo\\\\out\\\\002.md\"]}"
      }
    ],
    "structuredContent": {
      "files_created": [
        "D:\\workspace\\repo\\out\\001.md",
        "D:\\workspace\\repo\\out\\002.md"
      ]
    },
    "isError": false
  }
}
```

Operation failures are MCP **tool execution errors**, not protocol errors. They return `isError: true` and place the operation error object in `structuredContent`, with the same object serialized as JSON in a `content` text block.

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"error\":\"no_delimiter_match\",\"src\":\"D:\\\\workspace\\\\repo\\\\doc.md\",\"hint\":\"The delimiter pattern did not match any source lines.\"}"
      }
    ],
    "structuredContent": {
      "error": "no_delimiter_match",
      "src": "D:\\workspace\\repo\\doc.md",
      "hint": "The delimiter pattern did not match any source lines."
    },
    "isError": true
  }
}
```

Protocol-level MCP errors remain JSON-RPC errors and are outside the glyph-shift operation error contract. Use protocol errors for malformed JSON-RPC requests or unknown tools; use tool execution errors for glyph-shift validation and operation failures.

---

## Versioning

These are operation payloads, not a long-lived public API - breaking existing fields per version is allowed, but semver should apply to the overall CLI. We do not attempt to encode or support JSON schema versions, as only one shape can be returned at any one time.
