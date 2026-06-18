# CLI Design

## Help

- `--help` (optionally `-h`) prints usage, flags, purpose, and exits 0

## Input/Output

- Use structured JSON for input (options structs, typed parameters) and output (typed results, exit codes, machine-readable)
- Avoid prose output for structured data
- Use typed parameters rather than proliferating flag combinations

## Safety

- Default mutating tools to preview mode
- Require `--yes` / `--force` for mutations; never allow unconfirmed mutating operations

## Self-Description

- Make input and output schemas queryable at runtime for agents

## Input Validation

- Treat every CLI input as hostile; validate thoroughly
- Reject invalid input with a clear error message; never silently default to a fallback value

## Agent-First Output

- CLIs consumed by agents follow agent-output.md — every output is a prompt, success is silence, failure is structured
- Machine-readable output (JSON, structured errors) is the default for agent callers
- The CLI is the portable agent interface; protocol shims (MCP, etc.) are features, not architecture

## Scripts

- Keep scripts human-readable, short, and decomposed (do one thing well)
- Log every code and script path for CI diagnosis
