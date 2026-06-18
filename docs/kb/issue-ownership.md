# Issue Ownership

## Encounter → Fix

- Every issue found during work is your problem — fix it regardless of when it was introduced
- If an issue blocks your task, fix the blocker first, then resume
- If a fix is genuinely beyond your capability (credentials, infrastructure, domain knowledge), state the concrete blocker and stop

## Prohibited without user permission

- Investigating whether an issue is "pre-existing" before deciding to fix it
- Running tests on other branches or using git history to classify issue provenance
- Reporting an issue as pre-existing and leaving it unfixed
- Suggesting the user fix an issue separately because it predates the current change

## Rationale

- Provenance investigation is scope-narrowing avoidance, not diligence
- A fixed issue is valuable regardless of origin; a provenance report is worthless
