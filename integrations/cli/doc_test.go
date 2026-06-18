//go:build integration

// Package cli_test is Layer 3 CLI integration coverage: real glyph-shift subprocess,
// real argv parsing, and real OS workspace writes. Cases mirror BDD scenarios under
// features/bdd/core; see integrations/cli/README.md for why Layer 3 exists. Inputs and
// expected file bytes reuse features/testdata via internal/goldenreader; stdout/stderr
// bytes live under integrations/cli/testdata/golden. BDD symlink-map scenarios are not
// duplicated here (in-memory in BDD only; real OS symlinks are not CI-portable).
//
// Run mage integration or mage validate — Integration depends on CrossCompile (GoReleaser
// snapshot) and stageHostReleaseCLI to copy the host release binary into bin/ before tests.
// Do not run bare go test -tags integration without that staged binary.
package cli_test
