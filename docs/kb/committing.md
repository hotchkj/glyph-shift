# Commit Practices

## Signing

- Preserve commit signing; never bypass

## Hooks

- Preserve pre-commit hooks; bypass only with explicit user permission on a temporary working branch

## Workflow

- Use Conventional Commit messages
- Verify changes before commit
- Pass lint and tests before commit
- Run the full quality gate before final completion, not just targeted tests
- Keep secrets and credentials out of commits
- Maintain quality standards; never weaken them to unblock a commit

## Windows

- Powershell does not work with heredoc!
