---
name: filesystem-copy
description: Use filesystem operations for copying, moving, duplicating, or backing up existing file content. Apply when the task is to copy content between paths, mirror an existing file, preserve bytes, or move generated artifacts without changing their contents.
---

# Filesystem Copy

## Instructions

When the goal is to copy, move, duplicate, or back up existing content, use filesystem operations instead of reading the content into context and regenerating it.

- Prefer direct copy/move tools such as `Copy-Item`, `Move-Item`, or equivalent file APIs for whole-file or directory copies.
- Use specialized repository tools such as `glyph-shift` for byte-faithful partial file operations when copying ranges, blocks, or split outputs.
- Do not reconstruct copied content from memory, summaries, or prior tool output.
- Verify the destination exists after the operation when the copy is important.
- Only read file contents when inspection or semantic editing is required, not merely to transfer bytes.

## Examples

- Copying a plan from Cursor's plan directory into the workspace `.cursor/plans` directory.
- Duplicating fixture files, expected outputs, generated reports, or unchanged artifacts.
- Moving files between directories without transforming their content.
