# Debugging

## Diagnosis Before Action

- State Expected → Observed → Gap before attempting any fix
- Identify root cause before corrective action; symptom-chasing creates compound failures
- Perform root cause analysis even when the fix appears obvious

## Investigation Order

- Start with static analysis and linter output when debugging mysterious failures — code quality warnings often indicate real behavioral issues that manifest as confusing runtime errors
- Investigate newly changed code before stable code — new contracts typically violate existing expectations
- Validate "simple" changes systematically — hidden dependencies cause cascading failures

## Fix Discipline

- Apply one change → test → confirm → next change; avoid batching speculative fixes
- After 3 failed attempts at the same specific error, escalate or decompose
- Fix directly from logs, errors, or failing tests; only ask for clarification when the report lacks information to reproduce
