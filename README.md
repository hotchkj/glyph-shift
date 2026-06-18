<!-- gomarklint-disable heading-level -->
<!-- markdownlint-disable MD033 MD041 -->
<p align="center">
  <img src="assets/glyph-shift.png" alt="glyph-shift logo" width="220">
</p>

<h1 align="center">glyph-shift</h1>

<p align="center">
  <a href="https://github.com/hotchkj/glyph-shift/actions/workflows/ci.yml"><img src="https://github.com/hotchkj/glyph-shift/actions/workflows/ci.yml/badge.svg" alt="CI status"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT License"></a>
</p>
<!-- markdownlint-enable MD033 MD041 -->

---

## Why glyph-shift

Agents are bad at byte-faithful file surgery.

The usual agent file path is read into context, generate a replacement, write it back. That is powerful when the work is creative. It is the wrong path for mechanical work: extracting exact ranges, splitting a large file, pulling fenced blocks out, or normalising line endings. The bytes should not have to pass through the model at all.

`glyph-shift` is a single Go binary for byte-faithful file operations. The agent identifies the operation and parameters; the binary moves the bytes. It exposes the same four operations through CLI subcommands and MCP tools: `extract`, `split`, `blocks`, and `transform`.

It is not a content editor, a regex replacement tool, an encoding converter, or a batch processor. One invocation works on one source file. Agents compose multiple calls when they need a larger workflow.

## Install

Download the latest release from [GitHub Releases](https://github.com/hotchkj/glyph-shift/releases) and extract the binary for your platform.

Alternatively, build from source:

```bash
go install github.com/hotchkj/glyph-shift@latest
```

## Usage

Operations execute by default. Use `--preview` when the agent needs to inspect the plan without writing. Preview performs the same validation as apply.

Extract an exact editor-line range:

```bash
glyph-shift extract --source notes.md --lines 45-120 --destination out/section.md --mkdir
```

Split a file at delimiter lines:

```bash
glyph-shift split --source plan.md --delimiter "^## " --output-dir out --mkdir --max-files 20
```

Extract marker-delimited blocks:

```bash
glyph-shift blocks --source plan.md --start-line "^BEGIN_SECTION" --end-line "^END_SECTION" --output-dir sections --mkdir
```

Apply deterministic in-place transforms:

```bash
glyph-shift transform --source generated.txt --line-endings lf --trim-trailing --final-newline
```

Run as an MCP server over stdio:

```bash
glyph-shift mcp --workspace-root .
```

The MCP tools use the same logical arguments as the CLI. CLI flags use kebab-case (`--output-dir`); MCP tool inputs use snake_case (`output_dir`).

## Output

Operation subcommands emit success JSON on stdout and failure JSON on stderr. The streams do not mix. Help, `--help`, and `version` are plain-text operational metadata.

Agents branch on the exit code and `error` sentinel, not prose. Exact payload shapes live in [`docs/glyph-shift-json-contract.md`](docs/glyph-shift-json-contract.md).

## Safety Model

All agent-supplied input is treated as hostile. Paths are canonicalised at the workspace boundary and must remain inside it. Regexes are validated under Go RE2 with length and control-character checks. Binary sources are rejected using Git's binary-detection rule.

Write operations validate the source and planned outputs before publication where practical. `extract`, `split`, and `blocks` replay planned byte spans into temporary destinations and verify fingerprints before publishing. `transform` stages changed content and atomically replaces the source on publish. Multi-file operations do not promise cross-file atomicity: if one output fails, earlier completed outputs can remain, but the process still fails loudly and never reports partial work as success.

## Release Assets

| Asset | Purpose |
| ----- | ------- |
| `glyph-shift_{version}_{os}_{arch}.tar.gz` / `.zip` | CLI binary |
| `checksums.txt` | SHA256 checksums |

## Further Reading

- Product intent and design: [`docs/glyph-shift-intent.md`](docs/glyph-shift-intent.md)
- JSON and MCP payload contract: [`docs/glyph-shift-json-contract.md`](docs/glyph-shift-json-contract.md)
- Dogfood gate configuration: [`magefiles/mage_gate_config.go`](magefiles/mage_gate_config.go)
