Feature: Transform line endings and whitespace
  The caller applies mechanical byte-level transforms to a single file.
  The target is specified explicitly — no auto-detection.
  File bytes are not sent through an LLM or generative pathway.
  Transforms execute by default; preview requires an explicit flag.

  Scenario: CRLF endings are converted to LF
    Given a file "test.txt" with 5 lines using CRLF endings
    When the caller transforms "test.txt" to LF line endings
    Then the operation succeeds
    And the file was changed
    And 5 endings were changed
    And every line in "test.txt" has LF terminator

  Scenario: LF endings are converted to CRLF
    Given a file "test.txt" with 5 lines using LF endings
    When the caller transforms "test.txt" to CRLF line endings
    Then the operation succeeds
    And the file was changed
    And 5 endings were changed
    And every line in "test.txt" has CRLF terminator

  Scenario: Mixed endings are normalized
    Given a file "mixed.txt" with lines using mixed CRLF, LF, and CR endings
    When the caller transforms "mixed.txt" to LF line endings
    Then the operation succeeds
    And the file was changed
    And 3 endings were changed
    And every line in "mixed.txt" has LF terminator

  Scenario: Transform on already-normalized file makes no changes
    Given a file "test.txt" with 5 lines using LF endings
    When the caller transforms "test.txt" to LF line endings
    Then the operation succeeds
    And no changes were made

  Scenario: Trailing whitespace is trimmed
    Given a file "test.txt" with trailing whitespace on lines
    When the caller transforms "test.txt" trimming trailing whitespace
    Then the operation succeeds
    And the file was changed
    And no line in "test.txt" has trailing whitespace

  Scenario: Final newline is ensured
    Given a file "test.txt" that ends without a newline
    When the caller transforms "test.txt" ensuring a final newline
    Then the operation succeeds
    And the file was changed
    And "test.txt" ends with a newline

  Scenario: Final newline is not duplicated
    Given a file "test.txt" from testdata "already-has-final-newline.txt"
    When the caller transforms "test.txt" ensuring a final newline
    Then the operation succeeds
    And the file was not changed
    And "test.txt" ends with exactly one newline

  Scenario: Multiple transforms are applied in one invocation
    Given a file "test.txt" with CRLF endings, trailing whitespace, and no final newline
    When the caller transforms "test.txt" to LF line endings, trimming trailing whitespace, and ensuring a final newline
    Then the operation succeeds
    And the file was changed
    And every line in "test.txt" has LF terminator
    And no line in "test.txt" has trailing whitespace
    And "test.txt" ends with a newline

  Scenario: No transform specified is rejected
    Given a file "test.txt" from testdata "no-trailing-newline.txt"
    When the caller transforms "test.txt" with no operations
    Then the operation fails
    And the error JSON fields are exactly "error, hint"
    And the error JSON field "error" is "no_transform_specified"

  Scenario: Invalid line ending target is rejected consistently
    Given a file "test.txt" with 5 lines using LF endings
    When the caller transforms "test.txt" to invalid line endings
    Then the operation fails
    And the error JSON fields are exactly "error, hint"
    And the error JSON field "error" is "invalid_line_endings"

  Scenario: Directory path is rejected as source
    Given a directory "src" exists
    When the caller transforms "src" to LF line endings
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "directory_not_file"
    And the error JSON field "src" is workspace path "src"

  Scenario: Source path is treated literally without glob expansion
    Given files "a.txt" and "b.txt" exist in directory "src"
    When the caller transforms "src/*.txt" to LF line endings
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "source_not_found"
    And the error JSON field "src" is workspace path "src/*.txt"

  Scenario: Missing source is rejected
    When the caller transforms "nonexistent.txt" to LF line endings
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "source_not_found"
    And the error JSON field "src" is workspace path "nonexistent.txt"

  Scenario: Source symlink escaping workspace is rejected
    Given the workspace symlink map:
      | link              | target_scope | target     | target_kind |
      | escape-source.txt | outside      | secret.txt | file        |
    When the caller transforms "escape-source.txt" to LF line endings
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "invalid_input"
    And the error JSON field "src" is workspace path "escape-source.txt"

  Scenario: Source symlink staying inside workspace is accepted
    Given a file "real/test.txt" with 5 lines using CRLF endings
    And the workspace symlink map:
      | link              | target_scope | target        | target_kind |
      | inside-source.txt | inside       | real/test.txt | file        |
    When the caller transforms "inside-source.txt" to LF line endings
    Then the operation succeeds
    And the file was changed
    And every line in "real/test.txt" has LF terminator

  Scenario: Binary source is rejected
    Given a binary file "image.png"
    When the caller transforms "image.png" to LF line endings
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "binary_source"
    And the error JSON field "src" is workspace path "image.png"
    And "image.png" is unchanged

  Scenario: Preview reports what would change without modifying
    Given a file "test.txt" with 5 lines using CRLF endings
    When the caller previews transforming "test.txt" to LF line endings
    Then the operation succeeds
    And the result reports changes would occur
    And every line in "test.txt" has CRLF terminator

  Scenario: Preview on already-normalized file reports no changes
    Given a file "test.txt" with 5 lines using LF endings
    When the caller previews transforming "test.txt" to LF line endings
    Then the operation succeeds
    And the result reports no changes would occur
    And every line in "test.txt" has LF terminator

  Scenario: Preview reports trailing whitespace without modifying
    Given a file "test.txt" with trailing whitespace on lines
    When the caller previews transforming "test.txt" trimming trailing whitespace
    Then the operation succeeds
    And the result reports changes would occur
    And "test.txt" still has trailing whitespace

  Scenario: Preview reports missing final newline without modifying
    Given a file "test.txt" that ends without a newline
    When the caller previews transforming "test.txt" ensuring a final newline
    Then the operation succeeds
    And the result reports changes would occur
    And "test.txt" still ends without a newline

  Scenario: CR line ending conversion
    Given a file "test.txt" with 5 lines using LF endings
    When the caller transforms "test.txt" to CR line endings
    Then the operation succeeds
    And the file was changed
    And 5 endings were changed
    And every line in "test.txt" has CR terminator

  Scenario: Result distinguishes line ending types
    Given a file "mixed.txt" with 3 LF, 2 CR, and 1 CRLF line endings
    When the caller transforms "mixed.txt" to LF line endings
    Then the operation succeeds
    And the result reports 3 LF endings found and 0 converted
    And the result reports 2 CR endings found and 2 converted
    And the result reports 1 CRLF ending found and 1 converted

