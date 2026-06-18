# CLI-First Architecture for Human and Agent Consumers

## Intent

The CLI binary is the universal agent interface. For tools that wrap an API, the primary investment is a CLI—not a second API, not MCP as the core boundary, not framework-specific adapters.

## Why

**System of record unchanged.** The authoritative surface remains a containerized REST API. Agent integration is about access, not replacing that layer.

**Tool ecosystems fragment.** MCP, OpenAI function calling, OpenClaw, Gemini extensions, CrewAI, LangChain tools, and others each define their own tool abstraction; not all expose MCP. Any framework can shell out to a static CLI. The CLI is the portable interface across current and future stacks.

**MCP addressed agent HTTP pain; CLI generalizes it.** Raw HTTP from agents (curl, headers, auth, encoding) is slow and error-prone. A CLI encapsulates auth, URLs, headers, and response parsing: intent in, protocol mechanics handled. That pattern works regardless of whether the caller uses MCP or not.

**Execution primitive is durable.** “Agents can run commands” holds broadly. “Agents speak MCP” is a bet on one protocol. CLI-first can still ship MCP; it does not depend on MCP’s longevity.

## How

### Architecture

```text
REST API (containerized)     ← capability; unchanged
        │
CLI binary (e.g. Go)         ← universal adapter
  • Auth
  • Input hardening (agent failure modes)
  • Introspection (--help, schema subcommand)
  • --output json (machine-readable default)
  • --dry-run (validate before mutate)
        │
   ┌────┼────────────┐
   │    │            │
 MCP  OpenClaw   future frameworks
 shim skills    (thin shims)
```

### Responsibilities of the CLI

1. **Intent → API** — Typed commands map to HTTP against the same API humans would use.
2. **Input hardening** — Validate against agent-specific risks: path traversal, query fragments embedded in IDs, double-encoded strings, control characters. Treat agent-supplied input as untrusted.
3. **Output control** — JSON, field masks, context-aware trimming. Unbounded payloads waste tokens and degrade reasoning.

### MCP as a feature

`myapi mcp` (or equivalent) runs an MCP server on stdio that forwards tool calls into the same core library the CLI uses. MCP adoption does not define the architecture; a new protocol gets another thin shim over the same binary.

### Skills vs CLI mechanics

Skill files describe **when** and **why** to use commands. **How** to invoke them comes from tool schemas (MCP-compatible paths) or from `--help` / schema introspection (CLI-direct paths).

### Implementation notes (Go)

- Single static binary: low distribution friction.
- Fast cold start: per-invocation overhead stays small.
- Straightforward cross-compilation for heterogeneous agent hosts.
- Narrower surface area than dynamic stacks: fewer places for generated calls to go wrong.

### Input hardening (checklist)

| Concern | Mitigation |
| ------- | ---------- |
| File paths | Canonicalize; sandbox to allowed roots |
| Control characters | Reject below ASCII `0x20` |
| Resource IDs | Reject embedded `?` and `#` |
| Encoding | Reject stray `%` patterns that imply pre-encoding / double-encoding |
| All inputs | Validate before any outbound API call |

### Durability comparison

| Risk | CLI-first | MCP-first |
| ---- | --------- | --------- |
| MCP protocol drift | Shim absorbs; core stable | Core contract may break |
| New protocol | New shim; CLI unchanged | Often full rework |
| Framework without MCP | Still usable | Blocked or bridged ad hoc |
| Agent calls REST directly | CLI still adds hardening and ergonomics | MCP layer optional |

### Dogfooding loop

1. Ship typed CLI commands against the API.
2. Add optional MCP mode as a subcommand.
3. Run agents against it (IDEs, desktop agents, CI).
4. Log failures: bad inputs, context blowups, ambiguous flags.
5. Harden CLI and schemas from observed failure modes.
6. Repeat.
