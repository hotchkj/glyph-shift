# Agent-First Output

## Every output is a prompt

Tool output is the next instruction the agent follows. ERROR/Fix/Hint programs the agent's next action.

## Success is silence

Suppress pass counts, timing summaries, and celebration. A single confirmation line at most. Success noise wastes tokens and creates false confidence.

The `[Standards:Checked]` compliance tag required by coding rules is a machine token, not output noise — it is exempt from this rule.

## Failure is structured (ERROR/Fix/Hint)

This is the canonical format for agent-consumable failure messages:

- ERROR: what broke (fact, not interpretation)
- Fix: what to do next (imperative, specific)
- Hint: where to look (file path, tool command, MCP query, artifact)

Every failure message must be self-contained enough for an agent to act on without additional context. A well-formed error message is a prompt — it programs the agent's next step.

## Infrastructure is reusable

Command execution, error conventions, output mode handling, and artifact tracking serve any tooling that invokes external commands. Lock these in a domain package.

## Dependencies are visible

Every function call shows what it consumes. No hidden containers. A reader should infer behavior from the call site alone.

## Build provenance is agent-critical

Agents cannot carry implicit context about what produced an artifact, with what flags, for what scope. Structured metadata (stage, tool, scope) replaces that context.
