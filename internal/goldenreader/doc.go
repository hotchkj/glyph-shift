// Package goldenreader loads committed golden inputs and expected outputs from
// features/testdata on disk. It is the only production package permitted to
// perform OS-backed reads against that tree; BDD steps and CLI integration tests
// delegate here instead of importing os directly.
package goldenreader
