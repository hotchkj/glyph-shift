# Incremental Development

## Core Principle

Building on unvalidated work compounds errors. Each validated unit provides a stable foundation; each unvalidated unit is a liability.

## Batching

- Generate small batches (3–5 items) of repetitive code, then validate immediately before continuing
- Validate each batch before starting the next — compound errors from bulk generation cause cognitive overload and lead to requirement abandonment
- Complete one component with full tests before starting the next

## Cadence

- One change → test → fix → next change
- Pass validation before proceeding to the next unit of work
- Run project validation after grouped edits; address failures in the same session

## Incremental Over Revolutionary

- Prefer incremental changes with immediate validation over large refactors
- Confirm each step succeeds before proceeding
- Migrate incrementally during refactoring rather than rewriting in place
