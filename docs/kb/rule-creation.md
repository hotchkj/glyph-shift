# Rule Creation

Rules are short pointers with consequence language; full specifications live under `docs/kb/` (repo-root knowledge base).

- Keep rules terse — one or two inline constraints maximum
- State consequences: "will be rejected", "will fail the quality gate"
- Lead with the most violation-prone constraint per domain
- Write descriptions as warning labels, not summaries
- Vary sentence structure across rules — identical patterns normalize into noise

## Mirroring

Every `.cursor/rules/*.mdc` has a `.roo/rules/*.md` mirror with identical body but no frontmatter.
Every `.cursor/skills/<name>/SKILL.md` has a `.roo/skills/<name>/SKILL.md` mirror with identical content.

Roo mirrors have no YAML; Cursor-only fields (for example `globs` on `00-meta.mdc`) do not replicate to `.roo/rules`.

## Cursor `globs` on rules

Use `globs` so editing files under `.cursor/rules/` or `.roo/rules/` auto-attaches the meta rule without relying on the agent to remember to open `00-meta`. Keeps rule-creation KB obligations mechanically in-context for that edit surface.

## `description` field (frontmatter)

Treat as a second body for requestable rules and for tooling that shows only frontmatter: put the highest-risk MUST and triggers there; keep the pointer line in the rule body for alwaysApply and for mirrors.
