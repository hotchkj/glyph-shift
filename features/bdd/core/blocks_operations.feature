Feature: Extract fenced blocks
  The caller extracts content between matching start/end delimiter
  patterns into separate files. Content is transferred byte-faithfully
  without entering the generative pathway.

  Scenario: Extract delimited blocks from document
    Given a source file "doc.md" from testdata "nested-fences.md"
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```gherkin" and "^```$" into "out"
    Then the operation succeeds
    And the operation reports 2 non-empty blocks and 0 empty blocks
    And 2 files were created
    And the output files match expected "nested-fences-blocks"

  Scenario: Block content excludes delimiters by default
    Given a source file "doc.md" from testdata "single-go-block.md"
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out"
    Then the operation succeeds
    And the output files match expected "go-block-no-delimiters"

  Scenario: Delimiters are included when requested
    Given a source file "doc.md" from testdata "single-go-block.md"
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out" with "include delimiters"
    Then the operation succeeds
    And the output files match expected "go-block-with-delimiters"

  Scenario: Empty block produces no output file
    Given a source file "doc.md" from testdata "empty-go-block.md"
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out"
    Then the operation succeeds
    And the operation reports 0 non-empty blocks and 1 empty block
    And directory "out" contains 0 files

  Scenario: Block content is extracted byte-faithfully
    Given a source file "doc.md" from testdata "two-code-blocks.md"
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```code" and "^```$" into "out"
    Then the operation succeeds
    And the output files match expected "code-blocks-content"

  Scenario: Mixed line endings are preserved across block extraction
    Given a source file "doc.md" from escaped testdata "blocks-mixed-endings.bytes"
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out"
    Then the operation succeeds
    And the operation reports 2 non-empty blocks and 0 empty blocks
    And the output files match escaped expected "blocks-mixed-endings"

  Scenario: UTF-8 BOM is preserved inside block content
    Given a source file "doc.md" from escaped testdata "blocks-bom.bytes"
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out"
    Then the operation succeeds
    And the operation reports 1 non-empty block and 0 empty blocks
    And the output files match escaped expected "blocks-bom"

  Scenario: First end match closes the block regardless of inner delimiter-like lines
    Given a source file "doc.md" from testdata "inner-fence.md"
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```outer" and "^```$" into "out"
    Then the operation succeeds
    And the operation reports 1 non-empty block and 0 empty blocks
    And the output files match expected "inner-fence-blocks"

  Scenario: Output directory path traversal is rejected
    Given a source file "doc.md" with content:
      """
      text
      """
    When the caller extracts blocks from "doc.md" between "^```" and "^```$" into "../../tmp/evil"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, out_dir"
    And the error JSON field "error" is "invalid_input"
    And the error JSON field "out_dir" is workspace path "../../tmp/evil"

  Scenario: Source path outside workspace is rejected
    When the caller extracts blocks from "../../etc/passwd" between "^```" and "^```$" into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "invalid_input"
    And the error JSON field "src" is workspace path "../../etc/passwd"

  Scenario: Source symlink escaping workspace is rejected
    Given the workspace symlink map:
      | link             | target_scope | target    | target_kind |
      | escape-source.md | outside      | secret.md | file        |
    When the caller extracts blocks from "escape-source.md" between "^```" and "^```$" into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "invalid_input"
    And the error JSON field "src" is workspace path "escape-source.md"
    And directory "out" does not exist

  Scenario: Output directory symlink escaping workspace is rejected
    Given a source file "doc.md" from testdata "single-go-block.md"
    And the workspace symlink map:
      | link       | target_scope | target  | target_kind |
      | escape-out | outside      | outside | directory   |
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "escape-out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, out_dir"
    And the error JSON field "error" is "invalid_input"
    And the error JSON field "out_dir" is workspace path "escape-out"

  Scenario: Source symlink staying inside workspace is accepted
    Given a source file "real/doc.md" from testdata "single-go-block.md"
    And a directory "out" exists
    And the workspace symlink map:
      | link             | target_scope | target      | target_kind |
      | inside-source.md | inside       | real/doc.md | file        |
    When the caller extracts blocks from "inside-source.md" between "^```go" and "^```$" into "out"
    Then the operation succeeds
    And the operation reports 1 non-empty block and 0 empty blocks
    And the output files match expected "go-block-no-delimiters"

  Scenario: Invalid start pattern is rejected
    Given a source file "doc.md" from testdata "single-go-block.md"
    When the caller extracts blocks from "doc.md" between "[invalid" and "^```$" into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, field, hint"
    And the error JSON field "error" is "invalid_pattern"
    And the error JSON field "field" is "start_line"

  Scenario: Existing output files are not overwritten without force
    Given a source file "doc.md" from testdata "single-go-block.md"
    And directory "out" exists with file "001.md"
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, dest, hint"
    And the error JSON field "error" is "destination_exists"
    And the error JSON field "dest" is workspace path "out/001.md"

  Scenario: Existing output files are overwritten with force
    Given a source file "doc.md" from testdata "single-go-block.md"
    And directory "out" exists with file "001.md"
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out" with "overwrite"
    Then the operation succeeds
    And the output files match expected "go-block-no-delimiters"

  Scenario: Source file not found is rejected
    When the caller extracts blocks from "nonexistent.md" between "^```" and "^```$" into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "source_not_found"
    And the error JSON field "src" is workspace path "nonexistent.md"

  Scenario: Binary source is rejected
    Given a binary file "data.bin"
    When the caller extracts blocks from "data.bin" between "^```" and "^```$" into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "binary_source"
    And the error JSON field "src" is workspace path "data.bin"

  Scenario: Output directory is created on demand
    Given a source file "doc.md" from testdata "single-go-block.md"
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out" with "create directories"
    Then the operation succeeds
    And directory "out" contains 1 file

  Scenario: Unclosed block is an error
    Given a source file "doc.md" from testdata "unclosed-gherkin-block.md"
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```gherkin" and "^```$" into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src, start_line"
    And the error JSON field "error" is "unclosed_block"
    And directory "out" contains 0 files

  Scenario: No matching blocks is an error
    Given a source file "doc.md" with content:
      """
      plain
      text
      """
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```go$" and "^```$" into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, src"
    And the error JSON field "error" is "no_blocks_found"
    And directory "out" contains 0 files

  Scenario: Multiple empty blocks are counted but produce no files
    Given a source file "doc.md" with 3 empty fenced blocks
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out"
    Then the operation succeeds
    And the operation reports 0 non-empty blocks and 3 empty blocks
    And directory "out" contains 0 files

  Scenario: Block count exceeding limit is rejected
    Given a source file "many-blocks.md" with 51 fenced blocks
    And a directory "out" exists
    When the caller extracts blocks from "many-blocks.md" between "^```" and "^```$" into "out"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, max_files, would_create_count"
    And the error JSON field "error" is "max_files_exceeded"
    And the error JSON field "max_files" is integer "50"
    And the error JSON field "would_create_count" is integer "51"
    And directory "out" contains 0 files

  Scenario: Empty blocks do not consume the max-files limit
    Given a source file "doc.md" with content:
      """
      ```go
      ```
      ```go
      body
      ```
      """
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out" with a max-files limit of 1
    Then the operation succeeds
    And the operation reports 1 non-empty block and 1 empty block
    And directory "out" contains 1 file

  Scenario: Invalid max-files value is rejected consistently
    Given a source file "doc.md" from testdata "two-go-blocks.md"
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out" with a max-files limit of 0
    Then the operation fails
    And the error JSON fields are exactly "error, hint"
    And the error JSON field "error" is "invalid_input"

  Scenario: Output files are named from a provided list
    Given a source file "doc.md" from testdata "two-go-blocks.md"
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out" named "auth,db"
    Then the operation succeeds
    And the output files match expected "go-blocks-named"

  Scenario: Name count mismatch is rejected
    Given a source file "doc.md" from testdata "three-go-blocks.md"
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out" named "first,second"
    Then the operation fails
    And the error JSON fields are exactly "error, hint, names_count, output_count"
    And the error JSON field "error" is "names_count_mismatch"
    And the error JSON field "names_count" is integer "2"
    And the error JSON field "output_count" is integer "3"

  Scenario Outline: Unsafe explicit block output names are rejected
    Given a source file "doc.md" from testdata "two-go-blocks.md"
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out" named "<names>"
    Then the operation fails
    And the error JSON field "error" is "invalid_input"
    And directory "out" contains 0 files

    Examples:
      | names      |
      | dup,dup    |
      | ../evil,ok |
      | CON,ok     |

  Scenario: Output extension defaults to source file extension
    Given a source file "doc.md" from testdata "single-go-block.md"
    And a directory "out" exists
    When the caller extracts blocks from "doc.md" between "^```go" and "^```$" into "out"
    Then the operation succeeds
    And all output files have extension ".md"

  Scenario: Preview reports block metadata without creating files
    Given a source file "doc.md" from testdata "two-go-blocks.md"
    When the caller previews extracting blocks from "doc.md" between "^```go" and "^```$" into "out"
    Then the operation succeeds
    And the operation reports 2 non-empty blocks and 0 empty blocks
    And the preview would create output files "001.md,002.md"
    And directory "out" does not exist
