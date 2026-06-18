// Package testutil provides shared in-memory test doubles for the glyph-shift project.
//
// Files are grouped by concern: [MemFileSession] and session handles in mem_session.go,
// pipeline source/output/resolver fakes in fs.go, output write intents in output_intents.go,
// and performance measurement helpers in perf_*.go. None of these packages perform host
// filesystem I/O; production OS behavior is limited to internal/fileops production adapters.
package testutil
