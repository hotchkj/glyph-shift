Feature: Extract CLI and MCP JSON contract
  CLI and MCP entrypoints expose exact JSON for successful operations and
  matching machine-readable errors; operation execution is mocked.
  MCP structuredContent is asserted against the MCP tool-declared outputSchema.
  For destination_exists errors, JSON dest is the refusing extract destination path (the MCP destination
  or CLI --destination value), emitted absolute-native per docs/glyph-shift-json-contract.md - not the source path.

  Scenario: Extract apply stdout JSON matches contract
    Given a source file "plan.md" with 10 numbered lines
    And the extract operation reports success extracting 42 lines
    When the CLI command is invoked as "glyph-shift extract --source plan.md --lines 1-42 --destination output.txt"
    Then the operation succeeds
    And stdout JSON is exactly:
      """
      {
        "lines_extracted": 42
      }
      """

  Scenario: Invalid extract line range reports invalid_input on CLI stderr and MCP structured errors
    Given a source file "plan.md" with 10 numbered lines
    When the CLI command is invoked as "glyph-shift extract --source plan.md --lines not-a-range --destination output.txt"
    Then the operation fails
    And the exit code is 6
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON fields are exactly "error, hint"
    And the stderr error JSON field "error" is "invalid_input"
    And the CLI JSON error is saved
    When the MCP tool "extract" is called with JSON:
      """
      {
        "source": "plan.md",
        "lines": "not-a-range",
        "destination": "output.txt"
      }
      """
    Then the MCP tool result indicates an operation error
    And the CLI error JSON fields are exactly "error, hint"
    And the CLI error JSON field "error" is "invalid_input"
    And the CLI error JSON field "hint" is exactly:
      """
      parse lines: parse line range: invalid integer "not"
      """
    And the MCP structuredContent error JSON fields are exactly "error, hint"
    And the MCP structuredContent error JSON field "error" is "invalid_input"
    And the MCP structuredContent error JSON field "hint" is exactly:
      """
      parse lines: parse line range: invalid integer "not"
      """
    And the MCP content error JSON fields are exactly "error, hint"
    And the MCP content error JSON field "error" is "invalid_input"
    And the MCP content error JSON field "hint" is exactly:
      """
      parse lines: parse line range: invalid integer "not"
      """
    And the MCP structuredContent validates against the MCP outputSchema for tool "extract"

  Scenario Outline: Extract MCP contract errors — empty_range
    Given a source file "plan.md" with 10 numbered lines
    And the operation reports error "<sentinel>"
    When the CLI command is invoked as "glyph-shift extract --source plan.md --lines 1-2 --destination output.txt"
    Then the operation fails
    And the exit code is <exit_code>
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON field "error" is "<error>"
    And the stderr error JSON fields are exactly "<fields>"
    And the CLI JSON error is saved
    When the MCP tool "extract" is called with JSON:
      """
      {
        "source": "plan.md",
        "lines": "1-2",
        "destination": "output.txt"
      }
      """
    Then the MCP tool result indicates an operation error
    And the CLI error JSON fields are exactly "<fields>"
    And the CLI error JSON field "error" is "<error>"
    And the CLI error JSON field "hint" is "The requested line range is empty (start after end). Adjust --lines (CLI) or the lines input (MCP)."
    And the CLI error JSON field "src" is workspace path "plan.md"
    And the CLI error JSON field "range_start" is integer "2"
    And the CLI error JSON field "range_end" is integer "1"
    And the MCP structuredContent error JSON fields are exactly "<fields>"
    And the MCP structuredContent error JSON field "error" is "<error>"
    And the MCP structuredContent error JSON field "hint" is "The requested line range is empty (start after end). Adjust --lines (CLI) or the lines input (MCP)."
    And the MCP structuredContent error JSON field "src" is workspace path "plan.md"
    And the MCP structuredContent error JSON field "range_start" is integer "2"
    And the MCP structuredContent error JSON field "range_end" is integer "1"
    And the MCP content error JSON fields are exactly "<fields>"
    And the MCP content error JSON field "error" is "<error>"
    And the MCP content error JSON field "hint" is "The requested line range is empty (start after end). Adjust --lines (CLI) or the lines input (MCP)."
    And the MCP content error JSON field "src" is workspace path "plan.md"
    And the MCP content error JSON field "range_start" is integer "2"
    And the MCP content error JSON field "range_end" is integer "1"
    And the MCP structuredContent validates against the MCP outputSchema for tool "extract"

    Examples:
      | sentinel    | error       | exit_code | fields                                               |
      | empty_range | empty_range | 6         | error, hint, range_end, range_start, src             |

  Scenario Outline: Extract MCP contract errors — range_exceeds_file
    Given a source file "plan.md" with 10 numbered lines
    And the operation reports error "<sentinel>"
    When the CLI command is invoked as "glyph-shift extract --source plan.md --lines 1-2 --destination output.txt"
    Then the operation fails
    And the exit code is <exit_code>
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON field "error" is "<error>"
    And the stderr error JSON fields are exactly "<fields>"
    And the CLI JSON error is saved
    When the MCP tool "extract" is called with JSON:
      """
      {
        "source": "plan.md",
        "lines": "1-2",
        "destination": "output.txt"
      }
      """
    Then the MCP tool result indicates an operation error
    And the CLI error JSON fields are exactly "<fields>"
    And the CLI error JSON field "error" is "<error>"
    And the CLI error JSON field "hint" is "range exceeds file: file has 50 lines but requested range start 45 end 120 cannot be satisfied"
    And the CLI error JSON field "src" is workspace path "plan.md"
    And the CLI error JSON field "file_lines" is integer "50"
    And the CLI error JSON field "range_start" is integer "45"
    And the CLI error JSON field "range_end" is integer "120"
    And the MCP structuredContent error JSON fields are exactly "<fields>"
    And the MCP structuredContent error JSON field "error" is "<error>"
    And the MCP structuredContent error JSON field "hint" is "range exceeds file: file has 50 lines but requested range start 45 end 120 cannot be satisfied"
    And the MCP structuredContent error JSON field "src" is workspace path "plan.md"
    And the MCP structuredContent error JSON field "file_lines" is integer "50"
    And the MCP structuredContent error JSON field "range_start" is integer "45"
    And the MCP structuredContent error JSON field "range_end" is integer "120"
    And the MCP content error JSON fields are exactly "<fields>"
    And the MCP content error JSON field "error" is "<error>"
    And the MCP content error JSON field "hint" is "range exceeds file: file has 50 lines but requested range start 45 end 120 cannot be satisfied"
    And the MCP content error JSON field "src" is workspace path "plan.md"
    And the MCP content error JSON field "file_lines" is integer "50"
    And the MCP content error JSON field "range_start" is integer "45"
    And the MCP content error JSON field "range_end" is integer "120"
    And the MCP structuredContent validates against the MCP outputSchema for tool "extract"

    Examples:
      | sentinel           | error              | exit_code | fields                                               |
      | range_exceeds_file | range_exceeds_file | 6         | error, file_lines, hint, range_end, range_start, src |

  Scenario Outline: Extract MCP contract errors — source_not_found and binary_source
    Given a source file "plan.md" with 10 numbered lines
    And the operation reports error "<sentinel>"
    When the CLI command is invoked as "glyph-shift extract --source plan.md --lines 1-2 --destination output.txt"
    Then the operation fails
    And the exit code is <exit_code>
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON field "error" is "<error>"
    And the stderr error JSON fields are exactly "<fields>"
    And the CLI JSON error is saved
    When the MCP tool "extract" is called with JSON:
      """
      {
        "source": "plan.md",
        "lines": "1-2",
        "destination": "output.txt"
      }
      """
    Then the MCP tool result indicates an operation error
    And the CLI error JSON fields are exactly "<fields>"
    And the CLI error JSON field "error" is "<error>"
    And the CLI error JSON field "hint" is "<hint>"
    And the CLI error JSON field "src" is workspace path "plan.md"
    And the MCP structuredContent error JSON fields are exactly "<fields>"
    And the MCP structuredContent error JSON field "error" is "<error>"
    And the MCP structuredContent error JSON field "hint" is "<hint>"
    And the MCP structuredContent error JSON field "src" is workspace path "plan.md"
    And the MCP content error JSON fields are exactly "<fields>"
    And the MCP content error JSON field "error" is "<error>"
    And the MCP content error JSON field "hint" is "<hint>"
    And the MCP content error JSON field "src" is workspace path "plan.md"
    And the MCP structuredContent validates against the MCP outputSchema for tool "extract"

    Examples:
      | sentinel         | error            | exit_code | fields                           | hint                                                                 |
      | source_not_found | source_not_found | 2         | error, hint, src                 | Check that the source file exists and is accessible.                 |
      | binary_source    | binary_source    | 3         | error, hint, src                 | Source file is binary and cannot be processed as text.               |

  Scenario Outline: Extract MCP contract errors — destination_exists
    Given a source file "plan.md" with 10 numbered lines
    And the operation reports error "<sentinel>"
    When the CLI command is invoked as "glyph-shift extract --source plan.md --lines 1-2 --destination output.txt"
    Then the operation fails
    And the exit code is <exit_code>
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON field "error" is "<error>"
    And the stderr error JSON fields are exactly "<fields>"
    And the CLI JSON error is saved
    When the MCP tool "extract" is called with JSON:
      """
      {
        "source": "plan.md",
        "lines": "1-2",
        "destination": "output.txt"
      }
      """
    Then the MCP tool result indicates an operation error
    And the CLI error JSON fields are exactly "<fields>"
    And the CLI error JSON field "error" is "<error>"
    And the CLI error JSON field "hint" is "Use --force on the CLI or force: true in MCP JSON to overwrite, or append when the operation supports append mode."
    And the CLI error JSON field "dest" is workspace path "output.txt"
    And the MCP structuredContent error JSON fields are exactly "<fields>"
    And the MCP structuredContent error JSON field "error" is "<error>"
    And the MCP structuredContent error JSON field "hint" is "Use --force on the CLI or force: true in MCP JSON to overwrite, or append when the operation supports append mode."
    And the MCP structuredContent error JSON field "dest" is workspace path "output.txt"
    And the MCP content error JSON fields are exactly "<fields>"
    And the MCP content error JSON field "error" is "<error>"
    And the MCP content error JSON field "hint" is "Use --force on the CLI or force: true in MCP JSON to overwrite, or append when the operation supports append mode."
    And the MCP content error JSON field "dest" is workspace path "output.txt"
    And the MCP structuredContent validates against the MCP outputSchema for tool "extract"

    Examples:
      | sentinel           | error              | exit_code | fields                   |
      | destination_exists | destination_exists | 4         | error, dest, hint        |

  Scenario: Extract preview stdout JSON matches contract
    Given a source file "plan.md" with 10 numbered lines
    And the extract operation reports success extracting 11 lines
    When the CLI command is invoked as "glyph-shift extract --source plan.md --lines 45-55 --destination output.txt --preview"
    Then the operation succeeds
    And stdout JSON field "would_extract_lines" is 11
    And stdout JSON field "would_create" is the absolute native path for workspace-relative path "output.txt"

  Scenario: MCP extract preview structuredContent matches contract
    Given a source file "plan.md" with 10 numbered lines
    And the extract operation reports success extracting 11 lines
    When the MCP tool "extract" is called with JSON:
      """
      {
        "source": "plan.md",
        "lines": "45-55",
        "destination": "output.txt",
        "preview": true
      }
      """
    Then the MCP tool result indicates success
    And the MCP structuredContent field "would_extract_lines" is 11
    And the MCP structuredContent field "would_create" is the absolute native path for workspace-relative path "output.txt"
    And the MCP structuredContent validates against the MCP outputSchema for tool "extract"

  Scenario: CLI-only stderr JSON shape for range_exceeds_file
    Given a source file "plan.md" with 10 numbered lines
    And the extract operation reports error "range_exceeds_file"
    When the CLI command is invoked as "glyph-shift extract --source plan.md --lines 1-999 --destination output.txt"
    Then the operation fails
    And the exit code is 6
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON fields are exactly "error, file_lines, hint, range_end, range_start, src"
    And the stderr error JSON field "error" is "range_exceeds_file"

  Scenario: MCP extract apply structuredContent matches contract
    Given a source file "plan.md" with 10 numbered lines
    And the extract operation reports success extracting 42 lines
    When the MCP tool "extract" is called with JSON:
      """
      {
        "source": "plan.md",
        "lines": "1-42",
        "destination": "output.txt"
      }
      """
    Then the MCP tool result indicates success
    And the MCP structuredContent is exactly:
      """
      {
        "lines_extracted": 42
      }
      """
    And the MCP structuredContent validates against the MCP outputSchema for tool "extract"

  # unexpected_argument: CLI trailing positional + MCP surplus property (docs/glyph-shift-json-contract.md).
  # MCP outputSchema intentionally omits host/decode argument-shape failures, so this direct-dispatch
  # seam asserts structuredContent/content parity without outputSchema validation.
  Scenario: CLI extract rejects a trailing positional token with unexpected_argument
    Given a source file "plan.md" with 10 numbered lines
    When the CLI command is invoked as "glyph-shift extract --source plan.md --lines 1-3 --destination output.txt extra-positional-arg"
    Then the operation fails
    And the exit code is 6
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON fields are exactly "argument, error, hint"
    And the stderr error JSON field "error" is "unexpected_argument"
    And the stderr error JSON field "hint" is "extract accepts --source, --lines, --destination, plus optional --force, --append, --mkdir, and --preview; no trailing positional arguments. Other CLI commands include split, blocks, transform, version, mcp, and help."

  Scenario: MCP extract rejects an unexpected tool input field with unexpected_argument
    Given a source file "plan.md" with 10 numbered lines
    When the MCP tool "extract" is called with JSON:
      """
      {
        "source": "plan.md",
        "lines": "1-3",
        "destination": "output.txt",
        "not_a_declared_extract_field": true
      }
      """
    Then the MCP tool result indicates an operation error
    And the MCP structuredContent error JSON fields are exactly "error, field, hint"
    And the MCP structuredContent error JSON field "error" is "unexpected_argument"
    And the MCP structuredContent error JSON field "field" is "not_a_declared_extract_field"
    And the MCP structuredContent error JSON field "hint" is "accepted fields: source, lines, destination, preview, force, append, mkdir (see tool input schema)"
    And the MCP content error JSON fields are exactly "error, field, hint"
    And the MCP content error JSON field "error" is "unexpected_argument"
    And the MCP content error JSON field "field" is "not_a_declared_extract_field"
    And the MCP content error JSON field "hint" is "accepted fields: source, lines, destination, preview, force, append, mkdir (see tool input schema)"
