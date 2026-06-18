# Why CLI integration tests exist

glyph-shift is built for agents: a real binary, real flags, real files on disk. Most of the test suite deliberately avoids that shape so contracts stay fast and deterministic.

**Layer 1 (BDD operations)** proves byte fidelity through in-memory filesystem seams. It does not run the compiled binary or exercise how the OS actually opens, locks, and writes paths.

**Layer 2 (BDD contract)** proves the CLI and MCP JSON success and error shapes through mocked dispatch. It does not prove that argument parsing, path sandboxing, and publication behave correctly when a subprocess runs for real.

**This package (Layer 3)** closes that gap: invoke the production `glyph-shift` binary as a subprocess with the same flag style agents use, in a real temp workspace, and assert exit codes, stdout/stderr JSON, and on-disk results. It reuses the same committed fixtures as BDD (`features/testdata`) so file expectations stay aligned without duplicating bytes.

Intent doc note: subprocess tests are not the default *specification* style for every scenario ([glyph-shift-intent.md](../../docs/glyph-shift-intent.md)); they are an additional proof layer on top of BDD, not a replacement.

## What is in scope here

Scenarios from `features/bdd/core` that exercise the **CLI** (operations plus contract features where the assertion is CLI stdout or stderr). That is the agent path most like local and CI usage of the binary.

## What stays elsewhere

- MCP-only steps (tool JSON, `structuredContent`) — still covered in BDD with mocks.
- Performance and strict-timing scenarios.
- Workspace symlink-map scenarios — modeled in memory in BDD; not reproduced with real OS symlinks across CI machines.

## Golden files

CLI stdout/stderr expectations live under `testdata/golden/` as byte-faithful snapshots. Git treats that tree as binary (`-text` in `.gitattributes`) so line endings are not rewritten on checkout.

Case tables reference a logical name such as `extract/stdout/preview-45-55.golden`. At read time, `resolveCLIGoldenRel` selects an OS family variant when present:

- `*.windows.golden` on Windows
- `*.unix.golden` on Linux and macOS

Goldens that do not embed native path strings (for example transform count JSON) stay as a single unsuffixed `*.golden`.

Path-bearing JSON uses OS-family pairs above. Traversal rejection errors are different: stderr echoes the caller-supplied path slash style from the argv actually sent (for example `../../etc/passwd`), not a re-rendered native absolute. Those cases use one portable unsuffixed golden shared across OS families.

Committed golden bytes are compared verbatim. Only live subprocess stdout/stderr is passed through `normalizeCLIJSON`, which substitutes ephemeral temp workspace and traversal probe paths with committed `WORKSPACE/...` placeholders before comparison.

### Portable traversal goldens

Some stderr goldens are shared across OS families because traversal rejection echoes the caller-supplied path slash style from argv (not a re-rendered native absolute). Those files use unsuffixed `*.golden` names and committed `WORKSPACE/...` placeholders instead of `.windows`/`.unix` pairs.

When you add or change such a golden:

1. Keep path placeholders as `WORKSPACE/...` with forward slashes only.
2. Register the golden's repo-relative path and exact placeholder list in `portableCLIGoldenPaths` in `cli_golden_test.go`. The registry keys are full paths such as `extract/stderr/invalid-input-src-outside.golden`; values are the placeholder strings committed in that file.
3. `TestCLIGoldenPortableRegistryFilesExist` verifies registry files exist; `assertUnsuffixedCLIGoldenPathPolicy` (via the OS-family walk) verifies each portable golden's placeholders match the registry exactly.

Unsuffixed goldens that are not in the portable registry must not contain `WORKSPACE/` placeholders unless they also have `.windows`/`.unix` variants.

## How to run

`mage integration` or `mage validate`. The `Integration` mage target runs `CrossCompile` (GoReleaser snapshot), then copies the host release CLI from `dist/` into `bin/glyph-shift[.exe]` before all `-tags=integration` packages (including this one). Tests never invoke `go build` or GoReleaser themselves — do not run bare `go test -tags integration` without that staged binary.
