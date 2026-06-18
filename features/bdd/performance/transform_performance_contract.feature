Feature: Transform portable performance contract
  Transform preview and apply should stream through the same engine without retaining full-file buffers
  proportional to large sources. Previews must avoid publish temp files.

  @memstats_residency
  Scenario: Transform preview retained memory stays bounded for large CRLF sources
    Given transform performance CRLF sources "small-tr-perf.txt" (1000 lines) and "large-tr-perf.txt" (120000 lines)
    When the caller previews transform with line-endings lf for performance source "small-tr-perf.txt"
    When the caller previews transform with line-endings lf for performance source "large-tr-perf.txt"
    Then the operation succeeds
    And transform retained heap for "large-tr-perf.txt" is no more than 2 times "small-tr-perf.txt"

  Scenario: Transform apply retained memory stays bounded when normalizing CRLF sources
    Given transform performance CRLF sources "small-ta-perf.txt" (1000 lines) and "large-ta-perf.txt" (120000 lines)
    When the caller applies transform with line-endings lf for performance source "small-ta-perf.txt"
    When the caller applies transform with line-endings lf for performance source "large-ta-perf.txt"
    Then the operation succeeds
    And transform retained heap for "large-ta-perf.txt" is no more than 2 times "small-ta-perf.txt"

  Scenario: Transform apply retained memory stays bounded when trimming trailing whitespace on large CRLF sources
    Given transform performance CRLF sources with trailing spaces "small-trim-perf.txt" (1000 lines) and "large-trim-perf.txt" (120000 lines)
    When the caller applies transform with line-endings lf and trim-trailing for performance source "small-trim-perf.txt"
    When the caller applies transform with line-endings lf and trim-trailing for performance source "large-trim-perf.txt"
    Then the operation succeeds
    And transform retained heap for "large-trim-perf.txt" is no more than 2 times "small-trim-perf.txt"

  @memstats_residency
  Scenario: Transform apply retained memory stays bounded when enforcing a final newline on large CRLF sources
    Given transform performance CRLF sources without final newline "small-fn-perf.txt" (1000 lines) and "large-fn-perf.txt" (120000 lines)
    When the caller applies transform with line-endings lf and final-newline for performance source "small-fn-perf.txt"
    When the caller applies transform with line-endings lf and final-newline for performance source "large-fn-perf.txt"
    Then the operation succeeds
    And transform retained heap for "large-fn-perf.txt" is no more than 2 times "small-fn-perf.txt"

  Scenario: Transform preview does not create publish temp files
    Given transform performance CRLF sources "small-prev-temp.txt" (1000 lines) and "large-prev-temp.txt" (120000 lines)
    When the caller previews transform with line-endings lf measuring publish temps for performance source "small-prev-temp.txt"
    When the caller previews transform with line-endings lf measuring publish temps for performance source "large-prev-temp.txt"
    Then the operation succeeds
    And transform preview records zero temp creates for "small-prev-temp.txt"
    And transform preview records zero temp creates for "large-prev-temp.txt"

  @timing_strict
  Scenario: Transform preview completes a typical workload within wall-clock bounds
    Given transform performance CRLF sources "typical-tr-timing.txt" (1000 lines) and "unused.txt" (10 lines)
    When the caller previews transform with line-endings lf for performance source "typical-tr-timing.txt"
    Then the operation succeeds
    And the operation completes within 1 second

  @timing_strict
  Scenario: Transform apply completes a typical workload within wall-clock bounds
    Given transform performance CRLF sources "typical-ta-timing.txt" (1000 lines) and "unused2.txt" (10 lines)
    When the caller applies transform with line-endings lf for performance source "typical-ta-timing.txt"
    Then the operation succeeds
    And the operation completes within 1 second
