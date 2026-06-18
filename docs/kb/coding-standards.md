# Coding Standards

Standards are not negotiable — stop and request assistance rather than weaken them.
Check every action against standards: will this reduce code or testing quality?

## Development Order

1. Discovery (what exists)
2. BDD (user-facing behavior)
3. TDD (unit tests)
4. Implement
5. Lint + test before proceeding

## Dependencies and Design

- Inject all dependencies; keep the dependency tree mockable and separate from implementation
- Eliminate circular dependencies — they indicate a design failure
- Pass dependencies explicitly; never assume or hardcode them
- Use vetted third-party libraries over hand-rolled code when available
- Search existing codebase and dependencies before writing new code — test doubles are the highest-risk duplication target; search for existing fakes and test-support packages first
- Dedicate each function, file, and package to a single theme
- Avoid wrappers or compatibility shims for internal packages

## Naming

Read docs/kb/code-smells.md before naming components — vague or multi-variant names are rejected.

## Input Validation

- Treat all external input as hostile; validate thoroughly on every input surface
- Define minimum validation rules for each input type
- Treat any input that crosses a process boundary as untrusted
- Canonicalize filesystem paths at the boundary: validate, then normalize OS-native separators and redundant elements.
- Once a path is canonicalized, all subsequent manipulation (relative paths, parent directories, joins, basenames) must preserve that canonical form. Do not feed canonical strings back into OS-native path functions except at explicit seams: security containment (absolute-path resolution for escape checks) or OS-level IO (open, exec)
- Types that carry file paths should canonicalize during construction; enforce construction via constructors or factory functions so raw uncanonicalized values cannot enter the system

Read docs/kb/error-handling.md for error classification and sentinel error patterns.

## Task Execution Order

- Introduce a failing test or lint that demonstrates the problem before fixing bugs or tightening restrictions

## Method Design

- Every method: unique purpose, single responsibility, preferably reusable
- Unique: no other method in the codebase duplicates this functionality
- Purposeful: clear reason for existence beyond arbitrary code organization
- Reusable: designed for potential reuse, not over-specialized to one call site
- When adding a parameter, migrate all call sites rather than creating a new function name

## Code Hygiene

- Write small composable unit-tested code
- Keep code reusable and refactorable; split by theme
- Centralize common mocks (e.g., filesystem mocking done once) to avoid duplication
- Eliminate duplication; build only what is needed now
- Keep scripts human-readable, appropriately short, and decomposed
- Delete commented-out code; source control is history
- Write comments for non-obvious intent only, not narration
- Follow existing patterns; read surrounding code first
- Minimize indirection so readers trace fewer layers to understand behavior
- Minimize implicit state and scattered context so readers hold fewer facts in mind

## Bug Fixing

Read docs/kb/debugging.md before investigating failures — symptom-chasing creates compound failures.

## Change Scope

- Touch minimal code necessary per change
- Prefer simple solutions over clever ones when both work

## Workflow

Read docs/kb/incremental-development.md for build-validate cadence and batching rules.
