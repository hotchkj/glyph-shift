Feature: Split file by delimiter
  The caller partitions a file into multiple output files at each
  occurrence of a regex delimiter. Content is transferred byte-faithfully
  without entering the generative pathway.

  Scenario: Preamble and sections are split into separate files
    Given a source file "doc.md" from testdata "multi-section.md"
    And a directory "out" exists
    When the caller splits "doc.md" by pattern "^## " into "out"
    Then the operation succeeds
    And 4 files were created
    And the output files match expected "multi-section-split"

  Scenario: Empty preamble before first delimiter is skipped
    Given a source file "doc.md" from testdata "heading-start.md"
    And a directory "out" exists
    When the caller splits "doc.md" by pattern "^## " into "out"
    Then the operation succeeds
    And 2 files were created
    And the output files match expected "heading-start-split"

  Scenario: Delimiter lines are excluded when strip is requested
    Given a source file "doc.md" from testdata "heading-start.md"
    And a directory "out" exists
    When the caller splits "doc.md" by pattern "^## " into "out" with "strip delimiters"
    Then the operation succeeds
    And the output files match expected "heading-start-split-stripped"

  Scenario: Existing output files are not overwritten without force
    Given a source file "doc.md" from testdata "heading-start.md"
    And directory "out" exists with file "001.md"
    When the caller splits "doc.md" by pattern "^## " into "out"
    Then the operation fails
    And the error JSON fields are exactly "dest, error, hint"
    And the error JSON field "error" is "destination_exists"
    And the error JSON field "dest" is workspace path "out/001.md"

  Scenario: Existing output files are overwritten with force
    Given a source file "doc.md" from testdata "heading-start.md"
    And directory "out" exists with file "001.md"
    When the caller splits "doc.md" by pattern "^## " into "out" with "overwrite"
    Then the operation succeeds
    And the output files match expected "heading-start-split"

  Scenario: Line endings are preserved across split
    Given a source file "doc.md" from testdata "heading-start-crlf.md"
    And a directory "out" exists
    When the caller splits "doc.md" by pattern "^## " into "out"
    Then the operation succeeds
    And every output file has CRLF terminator

  Scenario: Mixed line endings are preserved across split
    Given a source file "doc.md" from escaped testdata "split-mixed-endings.bytes"
    And a directory "out" exists
    When the caller splits "doc.md" by pattern "^## " into "out"
    Then the operation succeeds
    And 3 files were created
    And the output files match escaped expected "split-mixed-endings-split"

  Scenario: UTF-8 BOM is preserved across split
    Given a source file "doc.md" from escaped testdata "bom-split.bytes"
    And a directory "out" exists
    When the caller splits "doc.md" by pattern "^## " into "out"
    Then the operation succeeds
    And the output files match escaped expected "bom-split"

  Scenario: Invalid regex pattern is rejected
    Given a source file "doc.md" with content:
      """
      text
      """
    When the caller splits "doc.md" by pattern "[invalid" into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, field, hint"
    And the error JSON field "error" is "invalid_pattern"
    And the error JSON field "field" is "delimiter"

  Scenario: Regex pattern length is bounded
    Given a source file "doc.md" with content:
      """
      text
      """
    When the caller splits "doc.md" by a pattern longer than the maximum into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, field, hint"
    And the error JSON field "error" is "pattern_too_long"
    And the error JSON field "field" is "delimiter"

  Scenario: Regex pattern control characters are rejected
    Given a source file "doc.md" with content:
      """
      text
      """
    When the caller splits "doc.md" by a pattern containing a control character into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, field, hint"
    And the error JSON field "error" is "control_chars_in_input"
    And the error JSON field "field" is "delimiter"

  Scenario: Pattern that matches nothing is rejected
    Given a source file "doc.md" from testdata "heading-start.md"
    When the caller splits "doc.md" by pattern "^ZZZNOMATCH" into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "no_delimiter_match"
    And the error JSON field "src" is workspace path "doc.md"
    And directory "out" does not exist

  Scenario: Output directory path traversal is rejected
    Given a source file "doc.md" from testdata "heading-start.md"
    When the caller splits "doc.md" by pattern "^## " into "../../tmp/evil"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, out_dir"
    And the error JSON field "error" is "invalid_input"
    And the error JSON field "out_dir" is workspace path "../../tmp/evil"

  Scenario: Source path outside workspace is rejected
    When the caller splits "../../etc/passwd" by pattern "^## " into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "invalid_input"
    And the error JSON field "src" is workspace path "../../etc/passwd"

  Scenario: Source symlink escaping workspace is rejected
    Given the workspace symlink map:
      | link             | target_scope | target    | target_kind |
      | escape-source.md | outside      | secret.md | file        |
    When the caller splits "escape-source.md" by pattern "^## " into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "invalid_input"
    And the error JSON field "src" is workspace path "escape-source.md"
    And directory "out" does not exist

  Scenario: Output directory symlink escaping workspace is rejected
    Given a source file "doc.md" from testdata "heading-start.md"
    And the workspace symlink map:
      | link       | target_scope | target  | target_kind |
      | escape-out | outside      | outside | directory   |
    When the caller splits "doc.md" by pattern "^## " into "escape-out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, out_dir"
    And the error JSON field "error" is "invalid_input"
    And the error JSON field "out_dir" is workspace path "escape-out"

  Scenario: Source symlink staying inside workspace is accepted
    Given a source file "real/doc.md" from testdata "heading-start.md"
    And a directory "out" exists
    And the workspace symlink map:
      | link             | target_scope | target      | target_kind |
      | inside-source.md | inside       | real/doc.md | file        |
    When the caller splits "inside-source.md" by pattern "^## " into "out"
    Then the operation succeeds
    And 2 files were created
    And the output files match expected "heading-start-split"

  Scenario: Source file not found is rejected
    When the caller splits "nonexistent.md" by pattern "^## " into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "source_not_found"
    And the error JSON field "src" is workspace path "nonexistent.md"

  Scenario: Binary source is rejected
    Given a binary file "data.bin"
    When the caller splits "data.bin" by pattern "^## " into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "binary_source"
    And the error JSON field "src" is workspace path "data.bin"

  Scenario: Output directory is created on demand
    Given a source file "doc.md" from testdata "heading-start.md"
    When the caller splits "doc.md" by pattern "^## " into "out" with "create directories"
    Then the operation succeeds
    And directory "out" contains 2 files

  Scenario: Output file count exceeding default limit is rejected
    Given a source file "many-sections.md" with 51 delimited sections
    And a directory "out" exists
    When the caller splits "many-sections.md" by pattern "^---$" into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, max_files, would_create_count"
    And the error JSON field "error" is "max_files_exceeded"
    And the error JSON field "max_files" is integer "50"
    And the error JSON field "would_create_count" is integer "51"
    And directory "out" contains 0 files

  Scenario: Explicit max-files limit is enforced
    Given a source file "doc.md" from testdata "heading-start.md"
    And a directory "out" exists
    When the caller splits "doc.md" by pattern "^## " into "out" with a max-files limit of 1
    Then the operation fails
    And the error JSON fields are exactly "error, hint, max_files, would_create_count"
    And the error JSON field "error" is "max_files_exceeded"
    And the error JSON field "max_files" is integer "1"
    And the error JSON field "would_create_count" is integer "2"
    And directory "out" contains 0 files

  Scenario: Invalid max-files value is rejected consistently
    Given a source file "doc.md" from testdata "heading-start.md"
    When the caller splits "doc.md" by pattern "^## " into "out" with a max-files limit of 0
    Then the operation fails
    And the error JSON fields are exactly "error, hint"
    And the error JSON field "error" is "invalid_input"

  Scenario: Output files are named from a provided list
    Given a source file "doc.md" from testdata "heading-start.md"
    And a directory "out" exists
    When the caller splits "doc.md" by pattern "^## " into "out" named "first,second"
    Then the operation succeeds
    And "out/first.md" exists
    And "out/second.md" exists

  Scenario: Name count mismatch is rejected
    Given a source file "doc.md" from testdata "multi-section.md"
    And a directory "out" exists
    When the caller splits "doc.md" by pattern "^## " into "out" named "only-one"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, names_count, output_count"
    And the error JSON field "error" is "names_count_mismatch"
    And the error JSON field "names_count" is integer "1"
    And the error JSON field "output_count" is integer "4"

  Scenario Outline: Unsafe explicit output names are rejected
    Given a source file "doc.md" from testdata "heading-start.md"
    And a directory "out" exists
    When the caller splits "doc.md" by pattern "^## " into "out" named "<names>"
    Then the operation fails
    And the error JSON field "error" is "invalid_input"
    And directory "out" contains 0 files

    Examples:
      | names      |
      | dup,dup    |
      | ../evil,ok |
      | CON,ok     |

  Scenario: Output extension defaults to source file extension
    Given a source file "doc.md" from testdata "heading-start.md"
    And a directory "out" exists
    When the caller splits "doc.md" by pattern "^## " into "out"
    Then the operation succeeds
    And all output files have extension ".md"

  Scenario: Preview reports split metadata without creating files
    Given a source file "doc.md" from testdata "heading-start.md"
    When the caller previews splitting "doc.md" by pattern "^## " into "out"
    Then the operation succeeds
    And the preview would create output files "001.md,002.md"
    And directory "out" does not exist
