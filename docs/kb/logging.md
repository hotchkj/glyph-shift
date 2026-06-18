# Logging

## Principles

- Log every error path; silent failures are unacceptable
- Log operational decisions and state transitions, not just errors
- Assume no debugger is available; make every failure diagnosable from CI logs alone
- Suppress success messages when outputting to LLM consumers — success noise triggers satisficing (see docs/kb/agent-output.md)

## Implementation

- Use structured logging with appropriate log levels
- Include context in every log entry
- Produce machine-parseable output; avoid prose logs when structured output is expected
