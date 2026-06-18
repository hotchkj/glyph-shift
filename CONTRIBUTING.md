# Contributing to glyph-shift

Thank you for your interest in contributing to glyph-shift! This guide will help you get started with development.

## Table of Contents

- [Development Environment](#development-environment)
- [Project Structure](#project-structure)
- [Development Workflow](#development-workflow)
- [Testing](#testing)
- [Code Quality](#code-quality)
- [Commit Messages](#commit-messages)
- [Pull Request Process](#pull-request-process)
- [Reporting Issues](#reporting-issues)

## Development Environment

### Prerequisites

- **Go 1.26.1** or later (see `go.mod`)
- **Mage** build tool (Go-based)

### Initial Setup

1. **Install dependencies**:

   ```bash
   go mod download
   ```

2. **Verify your setup** by running the quality gate:

   ```bash
   go run github.com/magefile/mage@v1.17.0 validate
   ```

## Project Structure

```text
glyph-shift/
├── cmd/                    # CLI entry points and command implementations
├── internal/
│   ├── fileops/           # Core file operations (byte-faithful)
│   ├── fsnorm/            # Filesystem path normalization
│   ├── goldenreader/      # Test fixture reader (exception for os package)
│   ├── linparse/          # Line range parsing
│   ├── mcpserver/         # MCP server implementation
│   ├── pipeline/          # Core pipeline library
│   ├── testutil/          # Test utilities and fakes
│   └── validate/          # Input validation
├── features/
│   ├── bdd/              # BDD feature files (executable specifications)
│   ├── steps/            # Step definitions for BDD
│   └── testdata/         # Test fixtures
├── integrations/         # Integration tests
├── magefiles/            # Mage build system configuration
└── docs/
    ├── kb/              # Knowledge base (coding standards, testing, etc.)
    ├── glyph-shift-intent.md    # Product intent document
    └── glyph-shift-json-contract.md  # JSON/MCP contract
```

### Key Concepts

- **Byte fidelity**: All operations preserve exact bytes - no encoding conversion or line ending mutation unless explicitly requested
- **BDD as specification**: Feature files define the behavioral contract; code satisfies them
- **Layer boundaries**: Tests are organized at layer boundaries (unit, BDD, integration)
- **MCP and CLI parity**: Both expose the same operations with the same parameter semantics

## Development Workflow

### 1. Plan Your Change

- Read the relevant documentation in `docs/kb/`
- For new features, start with BDD feature files
- For bug fixes, write a failing test first

### 2. Implement Incrementally

Follow the [Incremental Development](docs/kb/incremental-development.md) approach:

- Make small, focused changes
- Run tests after each change
- Validate before proceeding to the next unit of work

### 3. Development Commands

**Run the full quality gate** (recommended before committing):

```bash
go run github.com/magefile/mage@v1.17.0 validate
```

**Run specific targets**:

```bash
# Lint only
go run github.com/magefile/mage@v1.17.0 lint

# Unit tests only
go run github.com/magefile/mage@v1.17.0 test

# Integration tests
go run github.com/magefile/mage@v1.17.0 integration

# Performance tests
go run github.com/magefile/mage@v1.17.0 performance
```

## Testing

glyph-shift has a comprehensive test suite with three layers:

### 1. Unit Tests

- Intended for AI to test logic, not considered authoritative
- Located alongside source files (`*_test.go`)
- Use in-memory fakes and mocks
- Must not import: `os`, `net`, `net/http`, `os/exec`, `syscall`
- Must not use: `t.TempDir()`, `time.Sleep`, `os.Getwd()`

```bash
go test ./internal/...
```

### 2. BDD Feature Tests

- Intended for AI to validate logic, considered authoritative
- Located in `features/bdd/`
- Define behavioral contracts
- Step definitions in `features/steps/`

```bash
go test ./features/ -tags=bdd
```

### 3. Integration Tests

- Intended for AI to validate logic, considered authoritative
- Located in `integrations/`
- Test real MCP stdio wiring and subprocess behavior
- Require a compiled binary

```bash
go run github.com/magefile/mage@v1.17.0 integration
```

### Writing Tests

Follow these principles from `docs/kb/testing.md`:

- Mock external systems rather than calling them
- Use deterministic synchronization (no timing-dependent assertions)
- Test failure paths as thoroughly as happy paths
- Assert specific outcomes, not string contains

## Code Quality

### Linting

The project uses golangci-lint with extensive configuration. Run:

```bash
go run github.com/magefile/mage@v1.17.0 lint
```

Reducing these will result in rejection of contributions.
By contrast, improvements to these are extremely welcome!

### Coverage

Coverage, CRAP and mutations are all tested and have required levels for contributions.
Reducing these will result in rejection of contributions.

### Code Standards

- Follow existing patterns - read surrounding code first
- Inject all dependencies for testability
- Return errors, never print them
- Delete commented-out code
- Write comments for non-obvious intent only

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/). Your PR title will be validated against this format.

### Signing

Prefer commit signing to enable reduction of supply chain attacks.

## Pull Request Process

### PR Requirements

- **One concern per PR**: Keep changes focused on a single feature or fix
- **Tests included**: Add tests for new functionality
- **Documentation updated**: Update relevant docs if behavior changes
- **No breaking changes**: Unless discussed in an issue first
- **Clean history**: Squash fixup commits before merging

## Reporting Issues

- **Issues**: GitHub Issues for bug reports and feature requests
- **Discussions**: GitHub Discussions for questions and ideas
- **Pull Requests**: Welcome for all contributions

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.

---

Thank you for contributing to glyph-shift!
