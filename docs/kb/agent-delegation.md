# Agent Delegation

Orchestrators (Cursor, Roo, etc.) must follow these rules. Subagents inherit only what the Task prompt carries.

## Vague requests still require full prompts

A user request like "audit the codebase" or "run reviewers" does not exempt you from building an execution-context packet before delegating to a review-oriented Task. User omission does not waive this obligation.

Prompt MUST define: scope, goal, constraints, patterns/examples, critical-reviewer requirement before completion.

Prescriptive prompts (exact code, paths, signatures) enable fast models. Vague prompts must be tightened or use stronger model.

## Hard gate: execution-context packet before review Tasks

Build execution-context packet from repo sources listed there - embed this in every review prompt. User omission does not waive this. See sub-agent-prompting skill for prompt structure.

Review prompts fail without packet. If showing findings to user, include/restate packet in reply so verification rules (`docs/kb/verification.md`) apply to review findings, not just code edits.

## Subagent prompts

Every prompt includes:

1. Task + specific files/functions
2. Execution context from repo
3. Rules and skills to apply
4. User constraints (subagents cannot see chat history)
5. Pattern examples (if specific style required)
6. What not to do
7. Completion: "Fully implement. Do not simplify."
8. Testing: "Run targeted tests if identifiable."

Subagents find and apply workspace rules independently; prompts reinforce session-specific constraints.

## Subagent output review

Treat as draft. After completion:

1. Read full output, not summary
2. Check vs prompt requirements
3. Verify rules followed
4. Reconcile review claims vs execution-context packet and repo evidence
5. Check for scope reduction or incomplete work
6. Confirm targeted tests ran

If wrong: re-delegate with corrective prompt. Max 2 re-delegations per task; then escalate or split. Do not fix in orchestrator.

## Parallel subagents

When launching parallel agents:

1. Each owns distinct files (no overlap)
2. Define shared interfaces before launch
3. Run a reviewer after all are complete
4. Do not run full codebase tests that would fail on parallel incomplete changes

## Completion checklist

Before declaring done:

- Full test suite passes (not just targeted tests)
- Lint and static analysis pass per project standards (`docs/kb/linting.md`, quality gate)
- reviewer has run on all substantive changes
- Evidence reconciled per `docs/kb/verification.md`
