# Follow-up: afero seam cleanup

## Goal

Remove test-only filesystem hooks from production code and collapse remaining parallel filesystem behavior onto one `afero`-backed seam. Keep advisory locking as the narrow OS-specific exception.

## Scope

1. Replace production test hooks.
   - Remove `AtomicPublishStagingDecorator` from `fileops`.
   - Remove `StreamWhitespaceSpillBackingProvider` and production in-memory spill helpers.
   - Keep measurement/fault injection in `internal/testutil` or direct test call sites.

2. Unify scratch/temp behavior.
   - Make memory session temp handles satisfy the same read/write/seek scratch contract as OS temp handles.
   - Route transform pending-whitespace scratch through `FileSession.CreateTemp` for production and tests.

3. Use `afero` uniformly where it already owns the concern.
   - Avoid bespoke memory filesystem behavior when `afero.Fs` can provide it.
   - Retain platform-specific `filelock_*` code for data-file advisory locking.

4. Clean high-level tests.
   - Remove the reserved-name integration skip; reserved-name classification belongs in controlled-seam validation tests.
   - Keep integration tests focused on real OS concerns: symlinks, containment, and native resolver behavior.

## Verification

- `go test -tags=mage ./internal/fileops ./internal/pipeline ./internal/testutil ./internal/validate`
- `go test -tags=integration ./integrations/validate ./integrations/pipeline ./integrations/fileops/...`
- `mage Validate`
- Search production packages (`internal/fileops`, `internal/pipeline`, `internal/validate`) for leftover test hooks:
  - `AtomicPublishStagingDecorator`
  - `StreamWhitespaceSpillBackingProvider`
  - `NewMemWhitespaceSpillBacking`, `memWhitespaceSpillBacking`, `memWhitespaceSpillForTests`
  - Provider type assertions, e.g. `.(StreamWhitespaceSpillBackingProvider)` or `.(AtomicPublishStagingDecorator)`
  - Targeted regex for decorator types: `type\s+\w*Decorator\b`

## Notes

Do this after the Issue 4 filesystem-seam PR merges. The current PR is already large; this cleanup should be a focused follow-up PR with its own review surface.
