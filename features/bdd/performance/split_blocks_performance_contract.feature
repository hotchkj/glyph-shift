Feature: Split and blocks portable performance contract
  Split and blocks should keep deterministic work small when only unselected tail content grows,
  should not retain huge delimiter-scoped bodies after producing output, and previews should
  never materialize destination bytes.

  Scenario: Split remains bounded when only an irrelevant tail grows after max output files is exceeded (threshold: measured source reads and aggregate allocations each ≤2× the small overrun reference; mirrors extract_performance_contract lines 7-15 "One-line extract remains bounded when only the unselected tail grows")
    Given split overrun sources "small-split-over.md" (small) and "large-split-over.md" (large) for max-files 2 with delimiter line "---"
    And a directory "out-split-over" exists
    When the caller splits "small-split-over.md" by pattern "^---$" into "out-split-over" with a max-files limit of 2
    Then the operation fails
    And the error JSON fields are exactly "error, hint, max_files, would_create_count"
    And the error JSON field "error" is "max_files_exceeded"
    And the error JSON field "hint" is "split: maximum output file count exceeded: would create 3 outputs (limit 2)"
    And the error JSON field "max_files" is integer "2"
    And the error JSON field "would_create_count" is integer "3"
    And directory "out-split-over" contains 0 files
    When the caller splits "large-split-over.md" by pattern "^---$" into "out-split-over" with a max-files limit of 2
    Then the operation fails
    And the error JSON fields are exactly "error, hint, max_files, would_create_count"
    And the error JSON field "error" is "max_files_exceeded"
    And the error JSON field "hint" is "split: maximum output file count exceeded: would create 3 outputs (limit 2)"
    And the error JSON field "max_files" is integer "2"
    And the error JSON field "would_create_count" is integer "3"
    And split measured source reads for "large-split-over.md" are no more than 2 times "small-split-over.md"
    And split measured total allocations for "large-split-over.md" are no more than 2 times "small-split-over.md"

  Scenario: Blocks remains bounded when only an irrelevant tail grows after max output files is exceeded (threshold: measured source reads and aggregate allocations each ≤2× the small overrun reference; mirrors extract_performance_contract lines 7-15 with different delimiter fixture because fenced blocks enumerate outputs differently)
    Given blocks overrun sources "small-blocks-over.md" (small) and "large-blocks-over.md" (large) for max-files 2 with golang fences
    And a directory "out-blocks-over" exists
    When the caller extracts blocks from "small-blocks-over.md" between "^```go$" and "^```$" into "out-blocks-over" with a max-files limit of 2
    Then the operation fails
    And the error JSON fields are exactly "error, hint, max_files, would_create_count"
    And the error JSON field "error" is "max_files_exceeded"
    And the error JSON field "hint" is "blocks: maximum output file count exceeded: would create 3 outputs (limit 2)"
    And the error JSON field "max_files" is integer "2"
    And the error JSON field "would_create_count" is integer "3"
    And directory "out-blocks-over" contains 0 files
    When the caller extracts blocks from "large-blocks-over.md" between "^```go$" and "^```$" into "out-blocks-over" with a max-files limit of 2
    Then the operation fails
    And the error JSON fields are exactly "error, hint, max_files, would_create_count"
    And the error JSON field "error" is "max_files_exceeded"
    And the error JSON field "hint" is "blocks: maximum output file count exceeded: would create 3 outputs (limit 2)"
    And the error JSON field "max_files" is integer "2"
    And the error JSON field "would_create_count" is integer "3"
    And blocks measured source reads for "large-blocks-over.md" are no more than 2 times "small-blocks-over.md"
    And blocks measured total allocations for "large-blocks-over.md" are no more than 2 times "small-blocks-over.md"

  @memstats_residency
  Scenario: Split keeps heavy single-section workloads from retaining disproportionate heap (threshold: measured retained heap ≤2× the small workload; parallels extract_performance_contract line 39 "extract retained memory for large.md is no more than 2 times small.md" but uses delimiter-scoped 1000×48 vs 120000×48 bodies because streaming probes target fenced split sections not extract line numbering)
    Given split single-section heavy-body source file "small-heavy-split.md" with delimiter line "---" and 1000 body lines 48 bytes wide
    And split single-section heavy-body source file "large-heavy-split.md" with delimiter line "---" and 120000 body lines 48 bytes wide
    And a directory "out-heavy-split-small" exists
    And a directory "out-heavy-split-large" exists
    When the caller splits "small-heavy-split.md" by pattern "^---$" into "out-heavy-split-small" with "create directories"
    Then the operation succeeds
    When the caller splits "large-heavy-split.md" by pattern "^---$" into "out-heavy-split-large" with "create directories"
    Then the operation succeeds
    And split measured retained heap for "large-heavy-split.md" is no more than 2 times "small-heavy-split.md"

  @memstats_residency
  Scenario: Blocks keep heavy fenced bodies from retaining disproportionate heap (threshold: measured retained heap ≤2× the small workload; rationale matches the split-heavy scenario above versus extract_performance_contract retained-memory line)
    Given blocks single fenced heavy-body source file "small-heavy-blocks.md" with 1000 inner body lines 48 bytes wide
    And blocks single fenced heavy-body source file "large-heavy-blocks.md" with 120000 inner body lines 48 bytes wide
    And a directory "out-heavy-blocks-small" exists
    And a directory "out-heavy-blocks-large" exists
    When the caller extracts blocks from "small-heavy-blocks.md" between "^```go$" and "^```$" into "out-heavy-blocks-small" with "create directories"
    Then the operation succeeds
    When the caller extracts blocks from "large-heavy-blocks.md" between "^```go$" and "^```$" into "out-heavy-blocks-large" with "create directories"
    Then the operation succeeds
    And blocks measured retained heap for "large-heavy-blocks.md" is no more than 2 times "small-heavy-blocks.md"

  Scenario: Split preview emits plan without writing destinations (threshold: measured destination payload bytes remain 0; mirrors extract_performance_contract preview lines 50-51 destination guarantee — source read ratio capped at 2× is intentionally omitted versus extract line 52 because delimiter discovery previews must inspect the trailing tail whereas extract’s one-line preview does not scan the unchanged tail)
    Given a "---" preamble numbered source file "small-split-prev.md" with 1000 body lines
    And a "---" preamble numbered source file "large-split-prev.md" with 100000 body lines
    When the caller previews splitting "small-split-prev.md" by pattern "^---$" into "out-split-preview"
    Then the operation succeeds
    And 1 files would be created
    When the caller previews splitting "large-split-prev.md" by pattern "^---$" into "out-split-preview"
    Then the operation succeeds
    And 1 files would be created
    And split preview writes no measured destination payload bytes for "small-split-prev.md"
    And split preview writes no measured destination payload bytes for "large-split-prev.md"

  Scenario: Blocks preview emits plan without writing destinations (threshold: measured destination payload bytes remain 0; mirrors extract preview destination guarantee lines 50-51 with the fenced analogue — omitting extract’s 2× source read step at line 52 because block discovery previews scan the remainder of the file for anchors)
    Given a fenced-go numbered source file "small-blocks-prev.md" with 1000 inner body lines
    And a fenced-go numbered source file "large-blocks-prev.md" with 100000 inner body lines
    When the caller previews extracting blocks from "small-blocks-prev.md" between "^```go$" and "^```$" into "out-blocks-preview"
    Then the operation succeeds
    And 1 files would be created
    When the caller previews extracting blocks from "large-blocks-prev.md" between "^```go$" and "^```$" into "out-blocks-preview"
    Then the operation succeeds
    And 1 files would be created
    And blocks preview writes no measured destination payload bytes for "small-blocks-prev.md"
    And blocks preview writes no measured destination payload bytes for "large-blocks-prev.md"

  @timing_strict
  Scenario: Split completes a typical workload within wall-clock bounds
    Given a "---" preamble numbered source file "typical-split.md" with 1000 body lines
    And a directory "out-split-timing" exists
    When the caller splits "typical-split.md" by pattern "^---$" into "out-split-timing" with "create directories"
    Then the operation succeeds
    And the operation completes within 1 second

  @timing_strict
  Scenario: Blocks completes a typical workload within wall-clock bounds
    Given a fenced-go numbered source file "typical-blocks.md" with 1000 inner body lines
    And a directory "out-blocks-timing" exists
    When the caller extracts blocks from "typical-blocks.md" between "^```go$" and "^```$" into "out-blocks-timing" with "create directories"
    Then the operation succeeds
    And the operation completes within 1 second
