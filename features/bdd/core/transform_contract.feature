Feature: Transform CLI and MCP JSON contract
  CLI and MCP entrypoints expose exact JSON for successful operations and
  matching machine-readable errors; operation execution is mocked.
  MCP structuredContent is asserted against the MCP tool-declared outputSchema.

  Scenario: Transform apply stdout JSON with line-ending stats matches contract
    Given a source file "code.go" with content "line1\nline2\n"
    And the transform operation reports 2 LF endings converted to CRLF
    When the CLI command is invoked as "glyph-shift transform --source code.go --line-endings crlf"
    Then the operation succeeds
    And stdout JSON is exactly:
      """
      {
        "changed": true,
        "endings_changed": 2,
        "lf_found": 2,
        "lf_converted": 2,
        "cr_found": 0,
        "cr_converted": 0,
        "crlf_found": 0,
        "crlf_converted": 0
      }
      """

  Scenario Outline: Transform MCP contract errors — invalid transform specification
    Given a source file "code.go" with content "line1\nline2\n"
    And the operation reports error "<sentinel>"
    When the CLI command is invoked as "glyph-shift transform --source code.go --line-endings lf"
    Then the operation fails
    And the exit code is <exit_code>
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON field "error" is "<error>"
    And the stderr error JSON fields are exactly "<fields>"
    And the CLI JSON error is saved
    When the MCP tool "transform" is called with JSON:
      """
      {
        "source": "code.go",
        "line_endings": "lf"
      }
      """
    Then the MCP tool result indicates an operation error
    And the CLI error JSON fields are exactly "<fields>"
    And the CLI error JSON field "error" is "<error>"
    And the CLI error JSON field "hint" is "<hint>"
    And the MCP structuredContent error JSON fields are exactly "<fields>"
    And the MCP structuredContent error JSON field "error" is "<error>"
    And the MCP structuredContent error JSON field "hint" is "<hint>"
    And the MCP content error JSON fields are exactly "<fields>"
    And the MCP content error JSON field "error" is "<error>"
    And the MCP content error JSON field "hint" is "<hint>"
    And the MCP structuredContent validates against the MCP outputSchema for tool "transform"

    Examples:
      | sentinel               | error                  | exit_code | fields           | hint                                                                                           |
      | no_transform_specified | no_transform_specified | 6         | error, hint      | specify at least one of --line-endings, --trim-trailing, or --final-newline                   |
      | invalid_line_endings   | invalid_line_endings   | 6         | error, hint      | invalid line-endings value                                                                     |

  Scenario Outline: Transform MCP contract errors — path-scoped failures
    Given a source file "code.go" with content "line1\nline2\n"
    And the operation reports error "<sentinel>"
    When the CLI command is invoked as "glyph-shift transform --source code.go --line-endings lf"
    Then the operation fails
    And the exit code is <exit_code>
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON field "error" is "<error>"
    And the stderr error JSON fields are exactly "<fields>"
    And the CLI JSON error is saved
    When the MCP tool "transform" is called with JSON:
      """
      {
        "source": "code.go",
        "line_endings": "lf"
      }
      """
    Then the MCP tool result indicates an operation error
    And the CLI error JSON fields are exactly "<fields>"
    And the CLI error JSON field "error" is "<error>"
    And the CLI error JSON field "hint" is "<hint>"
    And the CLI error JSON field "src" is workspace path "code.go"
    And the MCP structuredContent error JSON fields are exactly "<fields>"
    And the MCP structuredContent error JSON field "error" is "<error>"
    And the MCP structuredContent error JSON field "hint" is "<hint>"
    And the MCP structuredContent error JSON field "src" is workspace path "code.go"
    And the MCP content error JSON fields are exactly "<fields>"
    And the MCP content error JSON field "error" is "<error>"
    And the MCP content error JSON field "hint" is "<hint>"
    And the MCP content error JSON field "src" is workspace path "code.go"
    And the MCP structuredContent validates against the MCP outputSchema for tool "transform"

    Examples:
      | sentinel           | error              | exit_code | fields                   | hint                                                                                                           |
      | source_not_found   | source_not_found   | 2         | error, hint, src         | Check that the source file exists and is accessible.                                                             |
      | binary_source      | binary_source      | 3         | error, hint, src         | Source file is binary and cannot be processed as text.                                                         |
      | directory_not_file | directory_not_file | 5         | error, hint, src         | Path must point to a regular file; directories are not valid sources.                                         |
      | not_regular_file   | not_regular_file   | 5         | error, hint, src         | Path must point to a regular file, not a directory or device.                                                   |

  Scenario: Transform preview stdout JSON with line-ending stats matches contract
    Given a source file "code.go" with content "line1\nline2\n"
    And the transform operation reports 2 LF endings converted to CRLF
    When the CLI command is invoked as "glyph-shift transform --source code.go --line-endings crlf --preview"
    Then the operation succeeds
    And stdout JSON is exactly:
      """
      {
        "would_change": true,
        "endings_changed": 2,
        "lf_found": 2,
        "lf_converted": 2,
        "cr_found": 0,
        "cr_converted": 0,
        "crlf_found": 0,
        "crlf_converted": 0
      }
      """

  Scenario: Transform apply stdout JSON with trim-trailing stats matches contract
    Given a source file "code.go" with content "line1  \nline2\n"
    And the transform operation reports 1 line trimmed of trailing whitespace
    When the CLI command is invoked as "glyph-shift transform --source code.go --trim-trailing"
    Then the operation succeeds
    And stdout JSON is exactly:
      """
      {
        "changed": true,
        "trailing_trimmed": 1
      }
      """

  Scenario: Transform apply stdout JSON with final-newline stats matches contract
    Given a source file "code.go" with content "line1\nline2"
    And the transform operation reports a final newline was added
    When the CLI command is invoked as "glyph-shift transform --source code.go --final-newline"
    Then the operation succeeds
    And stdout JSON is exactly:
      """
      {
        "changed": true,
        "final_newline_added": true
      }
      """

  Scenario: Transform apply stdout JSON when no change is needed matches contract
    Given a source file "code.go" with content "line1\nline2\n"
    And the transform operation reports no final newline change was needed
    When the CLI command is invoked as "glyph-shift transform --source code.go --final-newline"
    Then the operation succeeds
    And stdout JSON is exactly:
      """
      {
        "changed": false,
        "final_newline_added": false
      }
      """

  Scenario: CLI-only stderr JSON shape for no_transform_specified
    Given a source file "code.go" with content "line1\nline2\n"
    And the operation reports error "no_transform_specified"
    When the CLI command is invoked as "glyph-shift transform --source code.go --line-endings lf"
    Then the operation fails
    And the exit code is 6
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON fields are exactly "error, hint"
    And the stderr error JSON field "error" is "no_transform_specified"

  Scenario: MCP transform apply structuredContent matches contract
    Given a source file "code.go" with content "line1\nline2\n"
    And the transform operation reports 2 LF endings converted to CRLF
    When the MCP tool "transform" is called with JSON:
      """
      {
        "source": "code.go",
        "line_endings": "crlf"
      }
      """
    Then the MCP tool result indicates success
    And the MCP structuredContent is exactly:
      """
      {
        "changed": true,
        "endings_changed": 2,
        "lf_found": 2,
        "lf_converted": 2,
        "cr_found": 0,
        "cr_converted": 0,
        "crlf_found": 0,
        "crlf_converted": 0
      }
      """
    And the MCP structuredContent validates against the MCP outputSchema for tool "transform"

  Scenario: MCP transform trim-trailing apply structuredContent matches contract
    Given a source file "code.go" with content "line1  \nline2\n"
    And the transform operation reports 1 line trimmed of trailing whitespace
    When the MCP tool "transform" is called with JSON:
      """
      {
        "source": "code.go",
        "trim_trailing": true
      }
      """
    Then the MCP tool result indicates success
    And the MCP structuredContent is exactly:
      """
      {
        "changed": true,
        "trailing_trimmed": 1
      }
      """
    And the MCP structuredContent validates against the MCP outputSchema for tool "transform"

  Scenario: MCP transform final-newline apply structuredContent matches contract when changed
    Given a source file "code.go" with content "line1\nline2"
    And the transform operation reports a final newline was added
    When the MCP tool "transform" is called with JSON:
      """
      {
        "source": "code.go",
        "final_newline": true
      }
      """
    Then the MCP tool result indicates success
    And the MCP structuredContent is exactly:
      """
      {
        "changed": true,
        "final_newline_added": true
      }
      """
    And the MCP structuredContent validates against the MCP outputSchema for tool "transform"

  Scenario: MCP transform final-newline apply structuredContent matches contract when no change
    Given a source file "code.go" with content "line1\nline2\n"
    And the transform operation reports no final newline change was needed
    When the MCP tool "transform" is called with JSON:
      """
      {
        "source": "code.go",
        "final_newline": true
      }
      """
    Then the MCP tool result indicates success
    And the MCP structuredContent is exactly:
      """
      {
        "changed": false,
        "final_newline_added": false
      }
      """
    And the MCP structuredContent validates against the MCP outputSchema for tool "transform"

  Scenario: MCP transform preview structuredContent matches contract
    Given a source file "code.go" with content "line1\nline2\n"
    And the transform operation reports 2 LF endings converted to CRLF
    When the MCP tool "transform" is called with JSON:
      """
      {
        "source": "code.go",
        "line_endings": "crlf",
        "preview": true
      }
      """
    Then the MCP tool result indicates success
    And the MCP structuredContent is exactly:
      """
      {
        "would_change": true,
        "endings_changed": 2,
        "lf_found": 2,
        "lf_converted": 2,
        "cr_found": 0,
        "cr_converted": 0,
        "crlf_found": 0,
        "crlf_converted": 0
      }
      """
    And the MCP structuredContent validates against the MCP outputSchema for tool "transform"

  Scenario: Transform preview stdout JSON with trim-trailing stats matches contract
    Given a source file "code.go" with content "line1  \nline2\n"
    And the transform operation reports 1 line trimmed of trailing whitespace
    When the CLI command is invoked as "glyph-shift transform --source code.go --trim-trailing --preview"
    Then the operation succeeds
    And stdout JSON is exactly:
      """
      {
        "would_change": true,
        "trailing_trimmed": 1
      }
      """

  Scenario: Transform preview stdout JSON with final-newline needed matches contract
    Given a source file "code.go" with content "line1\nline2"
    And the transform operation reports a final newline was added
    When the CLI command is invoked as "glyph-shift transform --source code.go --final-newline --preview"
    Then the operation succeeds
    And stdout JSON is exactly:
      """
      {
        "would_change": true,
        "final_newline_needed": true
      }
      """

  Scenario: Transform preview stdout JSON when no final-newline change needed matches contract
    Given a source file "code.go" with content "line1\nline2\n"
    And the transform operation reports no final newline change was needed
    When the CLI command is invoked as "glyph-shift transform --source code.go --final-newline --preview"
    Then the operation succeeds
    And stdout JSON is exactly:
      """
      {
        "would_change": false,
        "final_newline_needed": false
      }
      """

  Scenario: MCP transform trim-trailing preview structuredContent matches contract
    Given a source file "code.go" with content "line1  \nline2\n"
    And the transform operation reports 1 line trimmed of trailing whitespace
    When the MCP tool "transform" is called with JSON:
      """
      {
        "source": "code.go",
        "trim_trailing": true,
        "preview": true
      }
      """
    Then the MCP tool result indicates success
    And the MCP structuredContent is exactly:
      """
      {
        "would_change": true,
        "trailing_trimmed": 1
      }
      """
    And the MCP structuredContent validates against the MCP outputSchema for tool "transform"

  Scenario: MCP transform final-newline preview structuredContent matches contract
    Given a source file "code.go" with content "line1\nline2"
    And the transform operation reports a final newline was added
    When the MCP tool "transform" is called with JSON:
      """
      {
        "source": "code.go",
        "final_newline": true,
        "preview": true
      }
      """
    Then the MCP tool result indicates success
    And the MCP structuredContent is exactly:
      """
      {
        "would_change": true,
        "final_newline_needed": true
      }
      """
    And the MCP structuredContent validates against the MCP outputSchema for tool "transform"

  Scenario: MCP transform final-newline preview not-needed structuredContent matches contract
    Given a source file "code.go" with content "line1\nline2\n"
    And the transform operation reports no final newline change was needed
    When the MCP tool "transform" is called with JSON:
      """
      {
        "source": "code.go",
        "final_newline": true,
        "preview": true
      }
      """
    Then the MCP tool result indicates success
    And the MCP structuredContent is exactly:
      """
      {
        "would_change": false,
        "final_newline_needed": false
      }
      """
    And the MCP structuredContent validates against the MCP outputSchema for tool "transform"
