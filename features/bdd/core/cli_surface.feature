Feature: CLI operational surfaces
  The glyph-shift binary exposes non-operation entrypoints that are outside the
  operation JSON contract but still need smoke coverage.

  Scenario: MCP server help is available as a CLI surface
    When the caller requests MCP help
    Then the operation succeeds
