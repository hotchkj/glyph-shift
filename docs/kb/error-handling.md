# Error Handling

## Failure Visibility

- Fail explicitly — a function that detects invalid input and silently returns gives the caller no signal that anything went wrong
- Catch specific error types and propagate; catch-all handlers mask bugs
- Reject access to closed or released resources immediately (fail-fast lifecycle)

## Null and Missing Data

- Return result types, optionals, or error values — never null
- Surface missing data as explicit errors; defaulting to empty values (`""`, `[]`, `{}`) hides the absence downstream
- Throw or return an error when contracts are violated; silent defaults are a design failure

## Error Classification

- Classify errors by contract type using sentinel errors, typed exceptions, or error codes for consistent caller handling
- Standardize and reuse error definitions and helpers; avoid redefining them per module

## Tests

- Prefer `errors.Is` / `errors.As` against exported sentinels in tests. Avoid assertions on full error strings or `fmt.Errorf` message equality; those couple tests to copy and break when diagnostics evolve.

## Diagnostic Structure (Agent Consumers)

Format failures for agent consumers using the ERROR/Fix/Hint pattern defined in docs/kb/agent-output.md.
