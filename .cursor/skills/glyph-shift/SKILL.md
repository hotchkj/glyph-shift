---
name: glyph-shift
description: Use glyph-shift for byte-faithful mechanical file operations such as extracting exact ranges, splitting files, and moving blocks without rewriting bytes through the model.
---

# glyph-shift (CLI Mode)

Use `glyph-shift` for byte-faithful mechanical operations (extracting ranges, splitting files, extracting blocks, or transforming line endings/whitespace) to avoid passing large file contents through LLM context.

## CLI Invocation Syntax

Commands execute by default; pass `--preview` to inspect without modifying. `--preview` is completely read-only and always takes precedence over `--force` or other write operations.

### extract

Extract a 1-based inclusive line range to a destination.

```bash
glyph-shift extract --source <src> --lines <range> --destination <dest> [--force] [--append] [--mkdir] [--preview]
```

- `--lines`: `45-55` (inclusive), `95-` (to end), or `-10` (start to 10).

### split

Split a file into multiple files at each matching delimiter line.

```bash
glyph-shift split --source <src> --delimiter <regex> --output-dir <dir> [--extension <ext>] [--names <csv>] [--max-files <n>] [--strip-delimiter] [--force] [--mkdir] [--preview]
```

### blocks

Extract lines between start and end delimiter patterns into separate files.

```bash
glyph-shift blocks --source <src> --start-line <regex> --end-line <regex> --output-dir <dir> [--extension <ext>] [--names <csv>] [--max-files <n>] [--include-delimiters] [--force] [--mkdir] [--preview]
```

### transform

Apply line-ending and whitespace normalizations in-place.

```bash
glyph-shift transform --source <src> [--line-endings <lf|crlf|cr>] [--trim-trailing] [--final-newline] [--preview]
```

- Must specify at least one transform option.

## Behavior & Error Recovery

For normal operations (excluding plain-text `--help` or `version`), success writes JSON to stdout (exit `0`) and stderr is empty. On failure, stdout is empty and stderr writes structured JSON.

If a command fails, inspect the error message and retry with corrected arguments:

- Line ranges in `--lines` must be 1-indexed.
- Destination directories must exist, or pass `--mkdir` to create them automatically.
- For `split` and `blocks`, use `--mkdir` instead of pre-creating empty/placeholder files.
- Source file must exist and not be binary (contains no null bytes in the first 8000 bytes).
- Relative paths resolve from the workspace root; absolute paths are accepted only if they target a path within the sandboxed root.
- Subcommand names (`extract`, `split`, `blocks`, `transform`) are case-sensitive.
- Blocks patterns are regexes; anchor at line start with `^` when matching marker lines (e.g., `^# Heading` or `^```).
