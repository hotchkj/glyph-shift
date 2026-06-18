Feature: Split CLI and MCP JSON contract
  CLI and MCP entrypoints expose exact JSON for successful operations and
  matching machine-readable errors; operation execution is mocked.
  MCP structuredContent is asserted against the MCP tool-declared outputSchema.
  For destination_exists errors, JSON dest is the absolute native path of the rejecting material
  output - the first planned split segment file under out_dir - not the MCP out_dir folder alone nor
  the source path (see docs/glyph-shift-json-contract.md Destination and output path failures).

  Scenario: Split apply stdout JSON matches contract
    Given a source file "doc.md" from testdata "multi-section.md"
    And a directory "out" exists
    And the split operation reports success with files "001.md,002.md,003.md,004.md"
    When the CLI command is invoked as "glyph-shift split --source doc.md --delimiter ^## --output-dir out"
    Then the operation succeeds
    And stdout JSON field "files_created" is a string array of absolute native paths for workspace-relative paths "out/001.md,out/002.md,out/003.md,out/004.md"

  Scenario: Split preview stdout JSON matches contract
    Given a source file "doc.md" from testdata "multi-section.md"
    And a directory "out" exists
    And the split operation reports success with files "001.md,002.md,003.md,004.md"
    When the CLI command is invoked as "glyph-shift split --source doc.md --delimiter ^## --output-dir out --preview"
    Then the operation succeeds
    And stdout JSON field "would_create" is a string array of absolute native paths for workspace-relative paths "out/001.md,out/002.md,out/003.md,out/004.md"

  Scenario Outline: Split MCP contract errors — pattern validation failures
    Given a source file "doc.md" from testdata "multi-section.md"
    And a directory "out" exists
    And the split operation reports pattern validation error "<sentinel>" for field "delimiter"
    When the CLI command is invoked as "glyph-shift split --source doc.md --delimiter ^## --output-dir out"
    Then the operation fails
    And the exit code is <exit_code>
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON field "error" is "<error>"
    And the stderr error JSON fields are exactly "<fields>"
    And the CLI JSON error is saved
    When the MCP tool "split" is called with JSON:
      """
      {
        "source": "doc.md",
        "delimiter": "^##",
        "output_dir": "out"
      }
      """
    Then the MCP tool result indicates an operation error
    And the CLI error JSON fields are exactly "<fields>"
    And the CLI error JSON field "error" is "<error>"
    And the CLI error JSON field "field" is "delimiter"
    And the CLI error JSON field "hint" is "<hint>"
    And the MCP structuredContent error JSON fields are exactly "<fields>"
    And the MCP structuredContent error JSON field "error" is "<error>"
    And the MCP structuredContent error JSON field "field" is "delimiter"
    And the MCP structuredContent error JSON field "hint" is "<hint>"
    And the MCP content error JSON fields are exactly "<fields>"
    And the MCP content error JSON field "error" is "<error>"
    And the MCP content error JSON field "field" is "delimiter"
    And the MCP content error JSON field "hint" is "<hint>"
    And the MCP structuredContent validates against the MCP outputSchema for tool "split"

    Examples:
      | sentinel               | error                  | exit_code | fields              | hint                            |
      | invalid_pattern        | invalid_pattern        | 6         | error, field, hint  | bdd mock: invalid regex pattern |
      | pattern_too_long       | pattern_too_long       | 6         | error, field, hint  | bdd mock: regex pattern too long |
      | control_chars_in_input | control_chars_in_input | 6         | error, field, hint  | bdd mock: control character     |

  Scenario Outline: Split MCP contract errors — max_files_exceeded
    Given a source file "doc.md" from testdata "multi-section.md"
    And a directory "out" exists
    And the operation reports error "<sentinel>"
    When the CLI command is invoked as "glyph-shift split --source doc.md --delimiter ^## --output-dir out"
    Then the operation fails
    And the exit code is <exit_code>
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON field "error" is "<error>"
    And the stderr error JSON fields are exactly "<fields>"
    And the CLI JSON error is saved
    When the MCP tool "split" is called with JSON:
      """
      {
        "source": "doc.md",
        "delimiter": "^##",
        "output_dir": "out"
      }
      """
    Then the MCP tool result indicates an operation error
    And the CLI error JSON fields are exactly "<fields>"
    And the CLI error JSON field "error" is "<error>"
    And the CLI error JSON field "hint" is "<hint>"
    And the CLI error JSON field "max_files" is integer "<max_files>"
    And the CLI error JSON field "would_create_count" is integer "<would_create_count>"
    And the MCP structuredContent error JSON fields are exactly "<fields>"
    And the MCP structuredContent error JSON field "error" is "<error>"
    And the MCP structuredContent error JSON field "hint" is "<hint>"
    And the MCP structuredContent error JSON field "max_files" is integer "<max_files>"
    And the MCP structuredContent error JSON field "would_create_count" is integer "<would_create_count>"
    And the MCP content error JSON fields are exactly "<fields>"
    And the MCP content error JSON field "error" is "<error>"
    And the MCP content error JSON field "hint" is "<hint>"
    And the MCP content error JSON field "max_files" is integer "<max_files>"
    And the MCP content error JSON field "would_create_count" is integer "<would_create_count>"
    And the MCP structuredContent validates against the MCP outputSchema for tool "split"

    Examples:
      | sentinel           | error                | exit_code | fields                                                 | hint                                                                                                                | max_files | would_create_count |
      | max_files_exceeded | max_files_exceeded   | 6         | error, hint, max_files, would_create_count            | maximum output file count exceeded: would create 11 outputs (limit 10)                                              | 10        | 11                   |

  Scenario Outline: Split MCP contract errors — names_count_mismatch
    Given a source file "doc.md" from testdata "multi-section.md"
    And a directory "out" exists
    And the operation reports error "<sentinel>"
    When the CLI command is invoked as "glyph-shift split --source doc.md --delimiter ^## --output-dir out"
    Then the operation fails
    And the exit code is <exit_code>
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON field "error" is "<error>"
    And the stderr error JSON fields are exactly "<fields>"
    And the CLI JSON error is saved
    When the MCP tool "split" is called with JSON:
      """
      {
        "source": "doc.md",
        "delimiter": "^##",
        "output_dir": "out"
      }
      """
    Then the MCP tool result indicates an operation error
    And the CLI error JSON fields are exactly "<fields>"
    And the CLI error JSON field "error" is "<error>"
    And the CLI error JSON field "hint" is "<hint>"
    And the CLI error JSON field "names_count" is integer "<names_count>"
    And the CLI error JSON field "output_count" is integer "<output_count>"
    And the MCP structuredContent error JSON fields are exactly "<fields>"
    And the MCP structuredContent error JSON field "error" is "<error>"
    And the MCP structuredContent error JSON field "hint" is "<hint>"
    And the MCP structuredContent error JSON field "names_count" is integer "<names_count>"
    And the MCP structuredContent error JSON field "output_count" is integer "<output_count>"
    And the MCP content error JSON fields are exactly "<fields>"
    And the MCP content error JSON field "error" is "<error>"
    And the MCP content error JSON field "hint" is "<hint>"
    And the MCP content error JSON field "names_count" is integer "<names_count>"
    And the MCP content error JSON field "output_count" is integer "<output_count>"
    And the MCP structuredContent validates against the MCP outputSchema for tool "split"

    Examples:
      | sentinel             | error                | exit_code | fields                                                 | hint                                                                                   | names_count | output_count |
      | names_count_mismatch | names_count_mismatch | 6         | error, hint, names_count, output_count                  | explicit name count does not match output count: got 2 names for 3 outputs             | 2           | 3            |

  Scenario Outline: Split MCP contract errors — no_delimiter_match, source_not_found, binary_source
    Given a source file "doc.md" from testdata "multi-section.md"
    And a directory "out" exists
    And the operation reports error "<sentinel>"
    When the CLI command is invoked as "glyph-shift split --source doc.md --delimiter ^## --output-dir out"
    Then the operation fails
    And the exit code is <exit_code>
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON field "error" is "<error>"
    And the stderr error JSON fields are exactly "<fields>"
    And the CLI JSON error is saved
    When the MCP tool "split" is called with JSON:
      """
      {
        "source": "doc.md",
        "delimiter": "^##",
        "output_dir": "out"
      }
      """
    Then the MCP tool result indicates an operation error
    And the CLI error JSON fields are exactly "<fields>"
    And the CLI error JSON field "error" is "<error>"
    And the CLI error JSON field "hint" is "<hint>"
    And the CLI error JSON field "src" is workspace path "doc.md"
    And the MCP structuredContent error JSON fields are exactly "<fields>"
    And the MCP structuredContent error JSON field "error" is "<error>"
    And the MCP structuredContent error JSON field "hint" is "<hint>"
    And the MCP structuredContent error JSON field "src" is workspace path "doc.md"
    And the MCP content error JSON fields are exactly "<fields>"
    And the MCP content error JSON field "error" is "<error>"
    And the MCP content error JSON field "hint" is "<hint>"
    And the MCP content error JSON field "src" is workspace path "doc.md"
    And the MCP structuredContent validates against the MCP outputSchema for tool "split"

    Examples:
      | sentinel           | error              | exit_code | fields              | hint                                                                 |
      | no_delimiter_match | no_delimiter_match | 6         | error, hint, src    | The delimiter pattern did not match any source lines.                |
      | source_not_found   | source_not_found   | 2         | error, hint, src    | Check that the source file exists and is accessible.                 |
      | binary_source      | binary_source      | 3         | error, hint, src    | Source file is binary and cannot be processed as text.               |

  Scenario Outline: Split MCP contract errors — destination_exists
    Given a source file "doc.md" from testdata "multi-section.md"
    And a directory "out" exists
    And the operation reports error "<sentinel>"
    When the CLI command is invoked as "glyph-shift split --source doc.md --delimiter ^## --output-dir out"
    Then the operation fails
    And the exit code is <exit_code>
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON field "error" is "<error>"
    And the stderr error JSON fields are exactly "<fields>"
    And the CLI JSON error is saved
    When the MCP tool "split" is called with JSON:
      """
      {
        "source": "doc.md",
        "delimiter": "^##",
        "output_dir": "out"
      }
      """
    Then the MCP tool result indicates an operation error
    And the CLI error JSON fields are exactly "<fields>"
    And the CLI error JSON field "error" is "<error>"
    And the CLI error JSON field "hint" is "Use --force on the CLI or force: true in MCP JSON to overwrite, or append when the operation supports append mode."
    And the CLI error JSON field "dest" is workspace path "out/001.md"
    And the MCP structuredContent error JSON fields are exactly "<fields>"
    And the MCP structuredContent error JSON field "error" is "<error>"
    And the MCP structuredContent error JSON field "hint" is "Use --force on the CLI or force: true in MCP JSON to overwrite, or append when the operation supports append mode."
    And the MCP structuredContent error JSON field "dest" is workspace path "out/001.md"
    And the MCP content error JSON fields are exactly "<fields>"
    And the MCP content error JSON field "error" is "<error>"
    And the MCP content error JSON field "hint" is "Use --force on the CLI or force: true in MCP JSON to overwrite, or append when the operation supports append mode."
    And the MCP content error JSON field "dest" is workspace path "out/001.md"
    And the MCP structuredContent validates against the MCP outputSchema for tool "split"

    Examples:
      | sentinel           | error              | exit_code | fields            |
      | destination_exists | destination_exists | 4         | error, dest, hint |

  Scenario: CLI-only stderr JSON shape for no_delimiter_match
    Given a source file "doc.md" from testdata "multi-section.md"
    And a directory "out" exists
    And the operation reports error "no_delimiter_match"
    When the CLI command is invoked as "glyph-shift split --source doc.md --delimiter ^NOMATCH --output-dir out"
    Then the operation fails
    And the exit code is 6
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON fields are exactly "error, hint, src"
    And the stderr error JSON field "error" is "no_delimiter_match"

  Scenario: MCP split apply structuredContent matches contract
    Given a source file "doc.md" from testdata "multi-section.md"
    And a directory "out" exists
    And the split operation reports success with files "001.md,002.md,003.md,004.md"
    When the MCP tool "split" is called with JSON:
      """
      {
        "source": "doc.md",
        "delimiter": "^##",
        "output_dir": "out"
      }
      """
    Then the MCP tool result indicates success
    And the MCP structuredContent field "files_created" is a string array of absolute native paths for workspace-relative paths "out/001.md,out/002.md,out/003.md,out/004.md"
    And the MCP structuredContent validates against the MCP outputSchema for tool "split"

  Scenario: MCP split preview structuredContent matches contract
    Given a source file "doc.md" from testdata "multi-section.md"
    And a directory "out" exists
    And the split operation reports success with files "001.md,002.md,003.md,004.md"
    When the MCP tool "split" is called with JSON:
      """
      {
        "source": "doc.md",
        "delimiter": "^##",
        "output_dir": "out",
        "preview": true
      }
      """
    Then the MCP tool result indicates success
    And the MCP structuredContent field "would_create" is a string array of absolute native paths for workspace-relative paths "out/001.md,out/002.md,out/003.md,out/004.md"
    And the MCP structuredContent validates against the MCP outputSchema for tool "split"

  # Preview strict validation: failure leaves stdout empty and writes no output files (docs/glyph-shift-intent.md).
  Scenario: Split preview fails with destination_exists and does not write planned output files
    Given a source file "doc.md" from testdata "multi-section.md"
    And a directory "out" exists
    And the operation reports error "destination_exists"
    When the CLI command is invoked as "glyph-shift split --source doc.md --delimiter ^## --output-dir out --preview"
    Then the operation fails
    And the exit code is 4
    And stdout is empty
    And stderr is a JSON error object
    And the stderr error JSON fields are exactly "error, dest, hint"
    And the stderr error JSON field "error" is "destination_exists"
    And "out/001.md" does not exist
    And "out/002.md" does not exist
    And "out/003.md" does not exist
    And "out/004.md" does not exist
