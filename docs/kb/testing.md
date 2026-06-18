# Testing

Apply all requirements strictly to tests.

## General

- Mock external systems rather than calling them: version control, network, filesystem, subprocesses, delays
- Keep test infrastructure out of production code
- Make tests fast and deterministic; design them to fail reliably
- Write clear failure messages indicating expected vs actual behavior
- Mark temporarily disabled tests as skipped with a reason and restore them before committing
- Use deterministic synchronization: explicit callbacks/events, controllable clocks, or bounded polling — never timing-dependent assertions
- Keep tests order-independent and side-effect free: running in any order, repeatedly, or with parallel scheduling produces the same outcome
- Test failure paths and absence of matches as thoroughly as the happy path

## Unit Testing

- Mock objects that would otherwise make real IO calls
- Isolate unit tests from external IO: no network, filesystem, subprocesses, system calls, or sleeps
- Use synchronization primitives appropriate to the language; never use sleep for synchronization
- Build single-concern, composable test doubles — each handles one external dependency; a test composes only the doubles it needs, never a bundle of unrelated concerns
- Centralize test doubles in a shared test-support package; hand-rolling a new fake for a problem solved by an existing double or vetted library is duplication

### Unit Test Isolation

Labeling a test as integration is not a workaround for avoiding unit test isolation. When unit coverage requires IO: refactor to use fakes first; if still blocked, skip the test with a stated reason or document it as an optional test job.

Unit tests exclude:

- Network: live or loopback clients, servers, listeners; no imports of networking packages. Indirect IO via production code from a unit test requires the seam to be faked.
- Subprocesses: no process execution for network or disk access
- Real filesystem: prefer fakes or filesystem abstractions
- System calls: not in unit tests

Real listeners, loopback, or multi-component wiring belong in integration tests when justified — not to bypass unit test rules. Document why real IO is needed.

## Behavior Testing (BDD)

- Cover all production code with BDD features describing user-facing intent
- Step patterns are structurally unique — the framework rejects duplicate registrations; unique wording is a design-time guardrail that forces reuse and prevents duplicate implementations
- Describe user or API outcomes in feature tests, not code behavior
- Keep feature tests loosely coupled to code; code changes only require feature changes when behavior changes
- Make scenarios validatable by multiple methods (manual, API/HTTP, code-level checks); scenarios that only work as implementation tests belong in unit tests
- Use actor-correct language: users/operators/callers/system/external events act; internal components are never actors
- Given steps describe completed preconditions; When steps describe actions/events; Then steps only assert outcomes
- Then steps never perform actions (no setup, mutation, restart, or trigger behavior)
- Share one application initialization step across feature suites; avoid component-specific init phrasing
- Set configuration preconditions before initialization actions in scenarios where init behavior is under test
- Model external conditions as external outcomes/states (provider unavailable, callback arrives), not test-harness setup vocabulary
- Organize feature files by actor-facing behavior/theme rather than internal component ownership
- Keep step phrases free of test harness language and implementation details
- Use Scenario Outlines for input-only variations instead of duplicated scenarios
- Map each behavior/effect to one canonical step phrase; put variations in parameters, not new step wording
- A step with clean single-parameter dispatch (e.g., name-to-function lookup) is structured extension; accreted conditionals — branches bolted on per scenario without examining structure — signal decomposition failure in Given steps

### BDD Suite Path Scoping (Go/godog)

Build tags on step files control compiled step definitions but godog reads all `.feature` files unless `Options.Paths` is scoped. Tagged runs must limit `Paths` to matching feature files.

- Keep one `TestFeatures` in `suite_test.go` consuming a package-level `featurePaths` variable; set `Strict: true`
- Supply `featurePaths` via build-tagged path files — exactly one compiles per build:
  - `suite_paths_default_test.go` (negated constraint excluding all tags) — all features
  - `suite_paths_{name}_test.go` with `//go:build bdd_{name}` — single feature
- The default file's build constraint must negate all feature-specific tags
- Every path file includes a `TestFeaturePathsAreScoped` guard with a hardcoded allowed map matching its `featurePaths` — prevents accidental inclusion
- When adding a new feature file: add to default paths and allowed map, add the new tag to the default negation, create a new tagged path file with its own scope guard

## Integration & End-to-end Testing

- Use real listeners, real clients, or multi-process checks; keep the surface area narrow and intentional
- Verify real wiring, protocols, or full stacks where isolation is insufficient
- Separate integration runs from the fast behavioral suite: their own jobs, optional profiles, or clearly labeled suites
- Integration is not: a catch-all for slow tests, a way to satisfy linters without code changes, or a substitute for fakes when unit tests could use them

### When Integration Tests Are Justified

- Add only after confirming no other pathway (in-process exercise, fakes, direct calls through public seams) provides equivalent confidence
- Use for residual risk that only appears with actual runtime behavior
- Avoid duplicating coverage: if the same outcomes are already proven without real IO, prefer the faster suite

## Test Design

- Make input variation systematic; avoid redundant duplication
- Minimize duplication in test setup; keep concepts consistent
- Keep test seams and scenario representations minimal and consistent
- Focus behavioral scenarios on observable, user-facing outcomes

## Assertions

- String contains tests are always wrong — a sign the test and code have not been properly designed; test sentinels, types or structured output
- Boolean assertions on non-boolean conditions are a code smell — specific assertions (equal, contains, throws) give better failure messages
- "Not empty" on collections is never sufficient — assert specific contents, count, or structure
- Keep defensive coding out of tests — no null coalescing, no optional chaining, no conditional guards; tests fail fast, and defensive patterns mask bugs
- Test absence as well as presence — assert that unexpected items do not appear
- Test collection contents to completion — verify all expected contents and confirm no unexpected extras

## Scaffolding

- Label temporary or partial tests clearly
- Remove scaffolding once a fuller test covers the same behavior
- Never ship scaffolding as the final state of coverage

## Anti-Evasion

- Never hide test gaps by moving tests between suites to evade linters
- Never use integration tests to bypass unit test isolation requirements
- Never rename, retag, or move tests between suites solely to evade linters or quality gates
