// Package validate provides input validation and hardening.
//
// # Security: TOCTOU (Time-of-Check-to-Time-of-Use)
//
// ValidatePath(path, root, resolver) checks path safety at call time but does not atomically bind
// the validated path to subsequent file operations. A small TOCTOU window
// remains between validation and the os.OpenFile call.
//
// Mitigation: callers re-validate paths immediately before each OpenFile
// ("late validation"), minimizing the window to microseconds. Destination
// files are opened with O_EXCL (new) or O_TRUNC (force) — no preceding
// stat call is needed, so there is no stat-then-open race.
//
// Accepted residual risk: glyph-shift targets single-user agent workflows where
// the workspace is not concurrently mutated by untrusted actors. For
// shared/multi-tenant environments, consider O_NOFOLLOW (where available)
// or openat-style patterns at the call site.
package validate
