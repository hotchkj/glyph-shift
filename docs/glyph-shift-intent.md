# glyph-shift - Intent Document

## What glyph-shift is

A single Go binary for byte-faithful file operations. Four subcommands - extract, split, blocks, transform - that move, partition, and mechanically transform file content without reading it into an LLM's context. Content never enters the generative pathway; the agent identifies parameters, invokes the tool, and the binary handles faithful byte transfer.

## Why glyph-shift exists

Agents handle file content through the generative pathway: read into context, regenerate via write. For tasks that are fundamentally mechanical (extract a section, split a file, fix line endings), this wastes tokens and introduces hallucination risk. The content should never enter the LLM context at all.

The agent's generative ability is used only for what it does well: understanding structure, identifying boundaries, choosing parameters. The binary handles faithful byte transfer.

## Core principles

### Byte fidelity

What goes in comes out. No encoding conversion, no line ending mutation, no BOM alteration unless the caller explicitly requests a specific transform (e.g. `--line-endings lf`). Encoding is outside the purview of these tools. If a file has a UTF-8 BOM, split/extract/blocks preserve it in the output. The tools do not interpret or re-encode text - they operate on raw bytes with line-boundary awareness.

### CLI and MCP for agents

CLI and MCP are the same agent-facing operation surface. They expose the same logical arguments for `extract`, `split`, `blocks`, and `transform`: CLI spells them as kebab-case flags and MCP spells them as snake_case `inputSchema` fields. There are no human users. Operation subcommands are the agent-facing contract; `help`, `--help`, and `version` are operational metadata and may print plain text.

All design decisions follow from this:

- **JSON for operation output.** For `extract`, `split`, `blocks`, and `transform`, a successful run emits JSON on stdout; failures emit JSON on stderr. Output structures are flat (no nested objects) to ensure efficient encoding via [TOON](https://github.com/toon-format/toon) for token-conscious agents. Help and version output are outside this JSON contract.
- **Execute by default.** Operations execute when called - matching every other tool in an agent's vocabulary. Safety comes from input hardening and output limits, not from requiring a second confirmation step. `--preview` is available as an opt-in flag for agents that want to inspect before committing, but it is not the default path. Preview always wins i.e. force flags do not override preview behavior.
- **Preview without writes.** `--preview` only inspects and is entirely read-only; it never publishes output files or mutates source files. Validations for preview are absolutely identical to normal runs otherwise.
- **Output limits.** Multi-file operations (`split`, `blocks`) accept `--max-files N` (default: 50). If the operation would produce more files than this limit, it errors - the pattern is likely wrong. An agent that genuinely expects more files sets the limit explicitly.
- **Stream separation (operation subcommands).** On success, stdout carries only the success JSON object. On failure, stdout is empty and stderr carries the operation error JSON object plus optional slog diagnostics. Success and failure shapes do not mix on stdout. Help and version requests send plain text to stdout (and may use stderr for errors); that path is not part of the operation JSON contract.
- **Distinct exit categories.** Each broad error category has a stable exit code so agents can branch on the integer without parsing text. The JSON `error` field carries the specific sentinel.
- **No environment variables.** Configuration through flags only.
- **No silent fixups.** Agents supply arguments that mean exactly what they say. If the arguments are wrong (range exceeds file length, path does not exist, delimiter pattern matches no lines where the contract requires a split), the tool fails clearly with a stable error category and specific JSON sentinel. No partial results unless unavoidable, no silent truncation, no false success, no "close enough" guesswork. The agent must correct its understanding, not receive an ambiguous result.
- **Writes, validation, and partial failure.** Validate inputs and planned outputs where practical before writing (paths, ranges, section counts, patterns). Multi-file operations do not promise cross-file atomicity: a failure while writing one section can leave earlier fully written outputs on disk. The process must still fail loudly (stderr JSON, non-zero exit) and must not report success or omit errors when the operation did not complete as defined. Each individual destination file must be published fully or not published at all; failed writes must not expose partial destination contents. All source reads must operate on a locked source file so the source cannot change during the operation. Source locks must be held until output writes have completed, except for the narrow rename-swap boundary required when atomically replacing the same source file.

### Input hardening

All agent-supplied input is treated as untrusted.

Paths are canonicalized at input, kept canonical, and sandboxed to the effective workspace root; path control characters are rejected.

The effective workspace root is the real, cleaned filesystem location selected for the operation boundary. CLI operations use the current directory as the default root. MCP uses `--workspace-root` when supplied and the current directory otherwise. Relative operation paths resolve under that boundary. Absolute operation paths are accepted only when their resolved target remains inside that boundary.

Symlinks, junctions, and other filesystem indirections are security-relevant only when they can change whether an operation path stays inside the effective workspace root. Existing symlinks at or below the boundary are allowed when they resolve inside the boundary and rejected when they resolve outside it. Missing output paths are allowed only when their existing parent chain remains inside the boundary. Filesystem ancestors above the effective boundary are outside the tool's authority and are not treated as escape evidence.

Validation protects against hostile path input, not hostile same-user filesystem mutation after validation. Race-hardening against symlink swaps between validation and open would require a separate dirfd/openat-style publication design and is out of scope for this contract.

Regex patterns must be non-empty and compile under Go RE2 with length bounds and control-character rejection; dedicated validation sentinels cover pattern, path, range, and extension failures (see [glyph-shift-json-contract.md](glyph-shift-json-contract.md)). Output file counts are capped. Avoid converting relative to absolute or vice-versa except for final native-filesystem usage (inputs to commands) or final display to consumer. The JSON contract owns operation preview field names and path-display semantics.

- **Paths**: canonicalized, sandboxed to the effective workspace root, symlink targets at or below that root validated, reserved OS device names rejected, control characters rejected
- **Regex patterns**: non-empty, compiled via Go RE2 (no catastrophic backtracking) with length bounds, control-character rejection, and dedicated validation sentinels
- **Extensions**: alphanumeric only after leading dot
- **Output file count**: `--max-files N` (default: 50) on `split` and `blocks` - error if the operation would exceed the limit, preventing pathological splitting from a wrong pattern
- **Binary detection**: Git's algorithm (null byte in first 8KB, matching `FIRST_FEW_BYTES` in `xdiff-interface.c`); binary sources rejected with distinct exit code

### Performance and memory

Operations must be sub-second for typical files. Performance is a quality attribute, but **correctness tests and performance tests are separate concepts**: the default correctness path proves business and output correctness; performance BDD may assert measurement preconditions and thresholds and must not be the primary place where correctness is established.

Performance testing is physically separated from core correctness features and is exercised through **non-coverage** test targets, not through the same pass as normal correctness coverage. Portable scalar performance contracts (for example, relative boundedness such as "no more than 2 times") are part of the default quality gate; strict wall-clock timing remains a separate target.

Memory usage must assume multi-GB input files. The implementation must not require buffering an entire file in memory to produce results. Streaming or bounded-memory strategies are the target - read, process, and write in passes without holding the full content. The contract (params in, result out) is designed to allow this without external changes.

### Stable contracts, free internals

How the implementation processes the source is an internal detail that must be changeable without external API changes. The contract prescribes the result shape and the write/no-write guarantee, not the processing strategy.

This means the CLI layer, MCP layer, and BDD features are insulated from internal refactoring.

### Single file per invocation

Each invocation operates on one source file. No glob expansion, no recursive directory walking, no multi-file batching. The agent composes multiple invocations when needed. This keeps the tool simple, predictable, and easy to reason about.

### Composable

Each subcommand does one thing. Combine via sequential shell invocations, not internal pipelines.

### BDD as specification

BDD features are the leading artifact that defines the behavioral contract. Features are written first; code satisfies them. The implementation strategy is free to vary as long as features stay green. Assertions use structured outcomes (JSON fields, exit codes, file existence), not substring matches on prose output. Help and version text are out of scope for JSON assertions.

There is one narrow OS-operation exemption for committed golden fixtures: only [internal/goldenreader](../internal/goldenreader) may read golden inputs and expected outputs under `features/testdata/` so fixtures stay byte-faithful and reviewable. BDD steps delegate to goldenreader; they do not import `os` for fixture reads. That exemption does not extend to the operations under test. Operation behavior specs must not rely on the real filesystem for the source, destination, directory, or write behavior being tested; those boundaries are supplied by controlled in-memory or mocked seams. All normal linting applies otherwise.

Tests are organized at **layer boundaries** (three complementary styles):

1. **Operation behavior against controlled seams.** Load golden fixture bytes when needed, then exercise file operations through the operation layer's public surface with in-memory or mocked filesystem seams. These tests prove byte fidelity, write/no-write behavior, output names, and counts without using the real filesystem as the operation's source of truth.
2. **CLI or MCP JSON surface.** Construct JSON tool inputs and assert on JSON success and machine-readable error payloads for the CLI or MCP entrypoints, with lower layers mocked so these tests do not depend on internals covered by (1).
3. **Subprocess integration.** Invokes the production `glyph-shift` binary as a subprocess with real argv parsing and a real temp workspace; assert exit codes, stdout/stderr JSON bytes, and on-disk outputs.

Subprocess tests against a compiled `glyph-shift` binary are not the default style for the core contract; prefer in-process boundaries above.

## Operations

### `extract`

Copy an exact range of lines from source to destination. Content never enters LLM context.

**Parameters:**

- `--source` (required): source file path
- `--lines` (required): range - `45-120`, `45-` (to end), `-10` (from start). 1-based, matching editor line numbers.
- `--destination` (required): destination file path
- `--preview`: inspect and report what would happen without writing
- `--force`: overwrite existing destination
- `--append`: append to existing destination
- `--mkdir`: create destination directories if needed

**Behavior:** Validate paths, open source (binary guard, line count), validate range against actual content. Write the exact byte slice to destination. Refuse to overwrite unless `--force`. Line endings, encoding, BOM are preserved exactly.

**Safety lifecycle (contract):**

- While the source is locked, a planning pass derives the expected fingerprint of the extracted bytes.
- A replay pass writes those bytes to a temporary destination and derives the actual fingerprint of what was written.
- Publication replaces the final destination only when expected and actual fingerprints match; on mismatch or other failure, no partial output is exposed at the destination path.

**Preview mode (`--preview`):** Same validation and source inspection. Report only. No destination opened or written.

**Edge cases:**

- Range beyond file length: error, distinct exit code. The agent specified a range that does not exist - it must correct its understanding of the file, not receive a silently truncated result.
- Empty range (start > end): error, distinct exit code
- Destination exists without `--force` or `--append`: error, distinct exit code
- Binary source: error, distinct exit code

### `split`

Split a file into multiple files at each occurrence of a regex delimiter pattern.

**Parameters:**

- `--source` (required): source file path
- `--delimiter` (required): regex pattern to split on
- `--output-dir` (required): output directory
- `--preview`: inspect and report what would happen without writing
- `--extension`: file extension for output files (default: source file's extension)
- `--names`: optional comma-delimited list of output filenames. Must match section count exactly or error. When omitted, files are named sequentially (`001`, `002`, etc.) with the source file's extension.
- `--max-files`: maximum output file count (default: 50). Error if the operation would produce more sections than this limit.
- `--strip-delimiter`: exclude delimiter line from output
- `--force`: overwrite existing output files
- `--mkdir`: create output directory if needed

**Behavior:** Validate paths and pattern, open source, split by delimiter. Write each section to a separate file. Content before the first delimiter becomes the first section (skipped if empty). If `--names` provided and count does not match, error. The agent can rename files afterward if needed - the tool does not guess names from content. If no delimiters are found, error.

**Safety lifecycle (contract):**

- Under a held source lock, the scan records planned output sections, their byte spans, and a fingerprint per section.
- Preview returns planned metadata only and creates no output files.
- On apply, each span is replayed into a temporary destination and verified against the planned per-section fingerprint before publish.
- Each destination file publishes independently; there is no cross-file atomicity across sections.

**Preview mode (`--preview`):** Same validation and source inspection. Compute section metadata (count, line counts per section, sequential filenames). No output files created.

### `blocks`

Extract all content between matching start/end delimiter patterns into separate files.

**Parameters:**

- `--source` (required): source file path
- `--start-line` (required): regex for block start delimiter
- `--end-line` (required): regex for block end delimiter
- `--output-dir` (required): output directory
- `--preview`: inspect and report what would happen without writing
- `--extension`: file extension (default: source file's extension)
- `--names`: optional comma-delimited list of output filenames. Must match `content_blocks_found` exactly or error. When omitted, files are named sequentially (`001`, `002`, etc.) with the source file's extension.
- `--max-files`: maximum output file count (default: 50). Error if the operation would produce more files than this limit.
- `--include-delimiters`: include start/end delimiter lines in output
- `--force`: overwrite existing output files
- `--mkdir`: create output directory if needed

**Behavior:** Validate paths and patterns, open source, find and extract blocks. Write each block to a separate file. Block content excludes delimiter lines by default.

**Safety lifecycle (contract):**

- Same lifecycle as split for matched block output spans: under a held source lock, the scan records planned output sections, their byte spans, and a fingerprint per section.
- Preview returns planned metadata only and creates no output files.
- Empty blocks contribute metadata only and produce no output file.
- On apply, each non-empty output span is replayed and verified against its planned fingerprint; each destination file is published atomically.

**Preview mode (`--preview`):** Same validation and source inspection. Compute block metadata (`content_blocks_found`, `empty_blocks_found` when non-zero, line counts per content block). No output files created.

**Edge cases:**

- Failing to find any valid blocks, error, distinct exit code.
- Unclosed block (start without matching end): error, distinct exit code. The agent's regex was wrong - it misunderstood the file structure or hallucinated the pattern. The error reports the unclosed block location so the agent can correct its patterns. No output files are created.
- Empty block (start immediately followed by end): valid. The agent asked for blocks matching those delimiters and got an empty one. Empty blocks are counted in `empty_blocks_found` when non-zero, are not counted in `content_blocks_found`, and produce no output file. `--include-delimiters` does not change the definition of empty for metadata.
- Nested delimiters: not supported - first end match closes the block. This is deterministic and documented; the agent can reason about it.

### `transform`

Apply mechanical byte-level transforms to a single file in-place.

**Parameters:**

- `--source` (required): source file path (single file, no globs)
- `--preview`: inspect and report what would change without modifying
- `--line-endings`: target line ending - `lf`, `crlf`, or `cr` (classic Mac). The caller specifies explicitly; no auto-detection.
- `--trim-trailing`: remove trailing whitespace from each line
- `--final-newline`: ensure file ends with exactly one newline

**Line ending semantics:** When `--line-endings` is specified, every line ending that does not match the target is replaced. The tool does not interpret intent - if the agent asks for `crlf`, then both `\n` and `\r` become `\r\n`. The result reports flat per-type counts (`lf_found`, `lf_converted`, `cr_found`, `cr_converted`, `crlf_found`, `crlf_converted`) so the agent sees exactly what the file's line ending state was.

**Behavior:** Read file, apply transforms, write back. Multiple transforms combine in one invocation. Idempotent - running the same transform twice produces identical output.

**Safety lifecycle (contract):**

- Inspect and apply both read through a locked source so the source cannot change during read and transform work.
- Transformed content is staged to a temporary file; publication replaces the original path by atomic rename from that staged file.
- Publication by atomic rename onto the original path is the narrow rename-swap boundary required when atomically replacing the same source file (the writes-rule exception); the usual requirement to hold the source lock until output writes have completed cannot apply across that boundary.
- Preview inspects only and never modifies the source.

**Preview mode (`--preview`):** Inspect file, determine what would change. Report per-type line ending counts, trailing whitespace count, final newline status. No file modified.

**Edge cases:**

- Binary source: error, distinct exit code. Binary detection uses Git's own algorithm (null byte in the first 8KB, matching `FIRST_FEW_BYTES` in `xdiff-interface.c`). The agent asked to transform a binary file - that is a mistake, not something to silently skip.
- No transform specified: error, distinct exit code
- Directory or non-regular file: error, distinct exit code

### `mcp`

Launches the binary as an MCP server over stdio. The MCP server exposes the same CLI operations as JSON-RPC tool calls, forwarding into the same pipeline library. CLI flags and MCP tool `inputSchema` fields expose the same logical operation arguments: CLI spells multi-word arguments as kebab-case flags, and MCP spells the same arguments as snake_case JSON fields. Each tool's **`inputSchema`** is the strict JSON Schema form of that aligned argument contract; each tool's **`outputSchema`** is the strict JSON Schema form of the mirrored operation output contract.

```text
glyph-shift mcp [--workspace-root <path>]
```

- `--workspace-root`: workspace root for path validation (default: cwd)
- Runs until stdin closes. All tool calls are serialized (global mutex) to prevent path-conflict races between concurrent callers.
- **MCP outputSchema variance:** Per-tool **`outputSchema`** describes operation success and operation-level failures returned in **`structuredContent`**. It does not describe argument-shape failures rejected by the MCP host as JSON-RPC **`invalid params` (-32602)** before glyph-shift produces a tool result. See the error model for the transport-specific **`unexpected_argument`** behavior.

## Architecture

```text
Filesystem (workspace root)      <- system of record
        |
Core library (pipeline/fileops)  <- byte-faithful operations
        |
   +----+----------+
   |               |
  CLI            MCP server
  (cmd/)         (internal/mcpserver/)
                 JSON-RPC tool server
```

Both CLI and MCP call the same pipeline functions with the same parameter structs. MCP is a thin shim that translates JSON-RPC tool calls into pipeline invocations. MCP-specific concerns (path sanitization in errors, global mutex for concurrency) live in the MCP layer, not the core.

## Error model

- Each broad error category maps to a stable exit code; the JSON `error` field carries the specific sentinel
- Errors go to stderr as JSON: required keys include `error` (sentinel name), a non-empty `hint`, and variant-specific path or token fields per the operation error `oneOf` in [glyph-shift-json-contract.md](glyph-shift-json-contract.md) (for example `src`, `dest`, `out_dir`, `output_path`, `field`, `argument`, `flag`). There is no single-column `resource` field. Stdout is empty on failure.
- CLI unexpected trailing positional tokens after recognized flags and operands emit stderr JSON with `error` set to `unexpected_argument`, the offending `argument`, and exit category validation / input `6`. Live MCP argument-shape failures are protocol errors: missing, surplus, or type-invalid tool arguments are rejected as JSON-RPC `invalid params` (-32602) before glyph-shift produces `structuredContent`. Lower-level decode contract tests may still assert an operation-error payload with `unexpected_argument` diagnostics for malformed tool argument objects; that remains test coverage for decode classification, not a live MCP tool result schema branch.
- Sentinel errors are defined per-package, no duplicates across packages for the same concept
- Constructors return errors, not panics, for nil dependencies
- No silent fallbacks - every failure path produces a diagnostic

## Exit codes

- `0` = success
- `1` = general / unknown error
- `2` = source not found
- `3` = binary source
- `4` = destination exists
- `5` = not regular file
- `6` = validation / input error

## Quality gates

- The quality gate protects the product promises in this document: byte-faithful output, explicit failures, bounded memory, portable performance, and platform-safe filesystem behavior.
- The gate should exercise the whole product, not a convenient subset. Build and test inputs include production code and the harness code needed to prove behavior; scoring and risk metrics should focus on production behavior rather than test scaffolding.
- Correctness, portable performance, integration behavior, and strict wall-clock timing are separate signals. Correctness is always part of the default gate. Portable scalar performance contracts are also part of the default gate, but they do not stand in for correctness or code coverage. Strict timing remains separate because wall-clock checks are environment-sensitive.
- BDD scenarios are executable product contracts via testable seams. New feature files must be intentionally categorized so core correctness cannot silently omit product behavior and performance contracts cannot drift into the covered correctness suite by accident.
- Static analysis, coverage, complexity/risk scoring, mutation signals, and dead-code checks are quality signals, not goals in isolation.
- Suppressions are exceptional. `nolint` and security-analysis suppressions must be documented with the boundary or invariant that makes them safe.

## What glyph-shift is not

- Not an encoding converter - encoding is preserved, never altered
- Not a content editor - no find/replace, no regex substitution on content
- Not a multi-file batch tool - one source file per invocation
- Not an API wrapper - the filesystem is the system of record
- Not locked to one agent transport - CLI and MCP expose the same filesystem-backed operations
