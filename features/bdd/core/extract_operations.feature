Feature: Extract line range
  The caller copies an exact range of lines from source to destination.
  Content is transferred byte-faithfully without entering the generative pathway.

  Scenario Outline: Extract a range of lines
    Given a source file "plan.md" with 100 numbered lines
    When the caller extracts lines <range> from "plan.md" to "output.txt"
    Then the operation succeeds
    And <count> lines were extracted
    And "output.txt" contains exactly lines <start> through <end> from "plan.md"

    Examples:
      | range | count | start | end |
      | 45-55 | 11    | 45    | 55  |
      | 95-   | 6     | 95    | 100 |
      | -10   | 10    | 1     | 10  |

  Scenario: Line numbers are 1-based
    Given a source file "three.txt" from testdata "three-lines.txt"
    When the caller extracts lines 2-2 from "three.txt" to "output.txt"
    Then the operation succeeds
    And 1 line was extracted
    And "output.txt" contains exactly lines 2 through 2 from "three.txt"

  Scenario: Empty range is rejected
    Given a source file "plan.md" with 100 numbered lines
    When the caller extracts lines 50-49 from "plan.md" to "output.txt"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, range_end, range_start, src"
    And the error JSON field "error" is "empty_range"
    And the error JSON field "range_start" is integer "50"
    And the error JSON field "range_end" is integer "49"
    And the error JSON field "src" is workspace path "plan.md"
    And "output.txt" does not exist

  Scenario: Invalid extract line syntax is rejected as invalid_input
    Given a source file "plan.md" with 100 numbered lines
    When the caller extracts lines "not-a-range" from "plan.md" to "output.txt"
    Then the operation fails
    And the error JSON fields are exactly "error, hint"
    And the error JSON field "error" is "invalid_input"
    And "output.txt" does not exist

  Scenario: Existing destination is not overwritten without force
    Given a source file "plan.md" with 100 numbered lines
    And a file "output.txt" already exists
    When the caller extracts lines 1-10 from "plan.md" to "output.txt"
    Then the operation fails
    And the error JSON fields are exactly "error, dest, hint"
    And the error JSON field "error" is "destination_exists"
    And the error JSON field "dest" is workspace path "output.txt"
    And "output.txt" is unchanged

  Scenario: Existing destination is overwritten with force
    Given a source file "plan.md" with 100 numbered lines
    And a file "output.txt" already exists with content "old"
    When the caller extracts lines 1-10 from "plan.md" to "output.txt" with "overwrite"
    Then the operation succeeds
    And "output.txt" contains exactly lines 1 through 10 from "plan.md"

  Scenario: Content is appended to existing destination
    Given a source file "plan.md" with 10 numbered lines
    And the file "output.txt" pre-populated from testdata "existing-content.txt"
    When the caller extracts lines 1-3 from "plan.md" to "output.txt" with "append"
    Then the operation succeeds
    And "output.txt" begins with the content of testdata "existing-content.txt"
    And "output.txt" ends with lines 1 through 3 from "plan.md"

  Scenario: Destination directories are created on demand
    Given a source file "plan.md" with 10 numbered lines
    When the caller extracts lines 1-5 from "plan.md" to "deep/nested/output.txt" with "create directories"
    Then the operation succeeds
    And "deep/nested/output.txt" contains exactly lines 1 through 5 from "plan.md"

  Scenario: Mixed line endings are preserved across extraction
    Given a source file "mixed.txt" from testdata "mixed-endings.txt"
    When the caller extracts lines 2-5 from "mixed.txt" to "output.txt"
    Then the operation succeeds
    And "output.txt" matches expected "mixed-endings-extract/output.txt"

  Scenario: UTF-8 BOM is preserved across extraction
    Given a source file "bom.txt" from escaped testdata "bom-lines.bytes"
    When the caller extracts lines 1-2 from "bom.txt" to "output.txt"
    Then the operation succeeds
    And "output.txt" matches escaped expected "bom-extract/output.txt.bytes"

  Scenario: Source path outside workspace is rejected
    Given a source file "plan.md" with 10 numbered lines
    When the caller extracts lines 1-10 from "../../etc/passwd" to "output.txt"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "invalid_input"
    And the error JSON field "src" is workspace path "../../etc/passwd"
    And "output.txt" does not exist

  Scenario: Source symlink escaping workspace is rejected
    Given the workspace symlink map:
      | link             | target_scope | target    | target_kind |
      | escape-source.md | outside      | secret.md | file        |
    When the caller extracts lines 1-10 from "escape-source.md" to "output.txt"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "invalid_input"
    And the error JSON field "src" is workspace path "escape-source.md"
    And "output.txt" does not exist

  Scenario: Destination parent symlink escaping workspace is rejected
    Given a source file "plan.md" with 10 numbered lines
    And the workspace symlink map:
      | link       | target_scope | target  | target_kind |
      | escape-out | outside      | outside | directory   |
    When the caller extracts lines 1-10 from "plan.md" to "escape-out/output.txt"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, dest"
    And the error JSON field "error" is "invalid_input"
    And the error JSON field "dest" is workspace path "escape-out/output.txt"
    And "escape-out/output.txt" does not exist

  Scenario: Source symlink staying inside workspace is accepted
    Given a source file "real/plan.md" with 10 numbered lines
    And the workspace symlink map:
      | link             | target_scope | target       | target_kind |
      | inside-source.md | inside       | real/plan.md | file        |
    When the caller extracts lines 1-10 from "inside-source.md" to "output.txt"
    Then the operation succeeds
    And "output.txt" contains exactly lines 1 through 10 from "real/plan.md"

  Scenario: Missing source is rejected
    When the caller extracts lines 1-10 from "nonexistent.md" to "output.txt"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "source_not_found"
    And the error JSON field "src" is workspace path "nonexistent.md"
    And "output.txt" does not exist

  Scenario: Destination path outside workspace is rejected
    Given a source file "plan.md" with 10 numbered lines
    When the caller extracts lines 1-10 from "plan.md" to "../../tmp/evil.txt"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, dest"
    And the error JSON field "error" is "invalid_input"
    And the error JSON field "dest" is workspace path "../../tmp/evil.txt"
    And "../../tmp/evil.txt" does not exist

  Scenario: Destination that is a directory is rejected
    Given a source file "plan.md" with 10 numbered lines
    And a directory "outdir" exists
    When the caller extracts lines 1-10 from "plan.md" to "outdir"
    Then the operation fails
    And the error JSON fields are exactly "error, dest, hint"
    And the error JSON field "error" is "destination_exists"
    And the error JSON field "dest" is workspace path "outdir"

  Scenario: Range beyond file length is an error
    Given a source file "short.txt" with 5 numbered lines
    When the caller extracts lines 3-999 from "short.txt" to "output.txt"
    Then the operation fails
    And the error JSON fields are exactly "error, file_lines, hint, range_end, range_start, src"
    And the error JSON field "error" is "range_exceeds_file"
    And the error JSON field "file_lines" is integer "5"
    And the error JSON field "range_start" is integer "3"
    And the error JSON field "range_end" is integer "999"
    And the error JSON field "src" is workspace path "short.txt"
    And "output.txt" does not exist

  Scenario: Binary source is rejected
    Given a binary file "data.bin"
    When the caller extracts lines 1-5 from "data.bin" to "output.txt"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "binary_source"
    And the error JSON field "src" is workspace path "data.bin"
    And "output.txt" does not exist

  Scenario: Preview reports what would be extracted without writing
    Given a source file "plan.md" with 100 numbered lines
    When the caller previews extracting lines 45-55 from "plan.md" to "output.txt"
    Then the operation succeeds
    And 11 lines would be extracted
    And preview would create "output.txt"
    And "output.txt" does not exist
