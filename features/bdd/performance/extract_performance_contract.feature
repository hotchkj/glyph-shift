Feature: Extract performance contract
  The caller can extract small closed ranges from large sources without work scaling with unselected content.
  The portable contract uses deterministic work metrics; strict wall-clock bounds are tagged separately.

  Scenario: One-line extract remains bounded when only the unselected tail grows
    Given a source file "small.md" with 1000 numbered lines
    And a source file "large.md" with 100000 numbered lines
    When the caller extracts lines 1-1 from "small.md" to "small-output.txt"
    Then the operation succeeds
    And 1 line was extracted
    When the caller extracts lines 1-1 from "large.md" to "large-output.txt"
    Then the operation succeeds
    And 1 line was extracted
    And extract source work for "large.md" is no more than 2 times "small.md"
    And extract allocated memory for "large.md" is no more than 2 times "small.md"

  Scenario: Mid-file closed range remains bounded when only trailing content grows
    Given a source file "small.md" with 1000 numbered lines
    And a source file "large.md" with 100000 numbered lines
    When the caller extracts lines 500-510 from "small.md" to "small-output.txt"
    Then the operation succeeds
    And 11 lines were extracted
    When the caller extracts lines 500-510 from "large.md" to "large-output.txt"
    Then the operation succeeds
    And 11 lines were extracted
    And extract source work for "large.md" is no more than 2 times "small.md"
    And extract allocated memory for "large.md" is no more than 2 times "small.md"

  @memstats_residency
  Scenario: Open-ended extract may read to EOF without retaining the full source
    Given a source file "small.md" with 1000 numbered lines
    And a source file "large.md" with 100000 numbered lines
    When the caller extracts lines 2- from "small.md" to "small-output.txt"
    Then the operation succeeds
    And 999 lines were extracted
    When the caller extracts lines 2- from "large.md" to "large-output.txt"
    Then the operation succeeds
    And 99999 lines were extracted
    And extract source work for "large.md" grows with selected output
    And extract retained memory for "large.md" is no more than 2 times "small.md"

  Scenario: Previewing a closed range remains bounded when only the unselected tail grows
    Given a source file "small.md" with 1000 numbered lines
    And a source file "large.md" with 100000 numbered lines
    When the caller previews extracting lines 1-1 from "small.md" to "small-output.txt"
    Then the operation succeeds
    And 1 line would be extracted
    When the caller previews extracting lines 1-1 from "large.md" to "large-output.txt"
    Then the operation succeeds
    And 1 line would be extracted
    And extract preview writes no destination bytes for "small.md"
    And extract preview writes no destination bytes for "large.md"
    And extract preview source work for "large.md" is no more than 2 times "small.md"

  @timing_strict
  Scenario: One-line extract is sub-second for a typical source
    Given a source file "typical.md" with 1000 numbered lines
    When the caller extracts lines 1-1 from "typical.md" to "typical-output.txt"
    Then the operation succeeds
    And 1 line was extracted
    And the operation completes within 1 second

  @timing_strict
  Scenario: One-line extract from a large source remains fast
    Given a source file "large.md" with 100000 numbered lines
    When the caller extracts lines 1-1 from "large.md" to "large-output.txt"
    Then the operation succeeds
    And 1 line was extracted
    And the operation completes within 2 seconds
