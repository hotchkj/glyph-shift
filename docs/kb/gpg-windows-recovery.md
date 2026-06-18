# GPG Windows Signing Recovery

## Policy

- Preserve commit signing; never bypass
- `commit.gpgsign` must remain on; `--no-gpg-sign` is forbidden
- Recovery means fixing the agent, not disabling signing

## Symptoms

- `git commit` hangs at signing prompt
- `gpg: cannot connect to the agent`
- Stale or unresponsive pinentry dialog
- `gpg failed to sign the data`

## Hard Limits

- Never disable signing to unblock a commit
- Do not kill processes outside the GPG agent and its pinentry
- Escalate to user if two full reset cycles fail without a good signature
