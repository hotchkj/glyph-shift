# Verification

## During Development

- Run targeted tests for changed code after each edit cycle
- Check static analysis and linter output
- Treat review findings as hypotheses until they are reconciled with repo-local version, lint, and platform facts

## Before Completion

- Run the full quality gate, not just targeted tests
- Diff against the main branch for behavioral changes
- Demonstrate correctness with concrete evidence, not assertions
- If a review claim conflicts with repo-local lint, static analysis, or the project gate, stop and reconcile before editing further
