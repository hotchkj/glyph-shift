# Planning

A plan is a structural blueprint: building blocks, interfaces, execution order. Nothing else.

- Resolve questions with the user before writing plan items — a plan with open questions is a draft masquerading as a decision
- Present options and get a decision first; plans record the chosen path, not the decision tree
- Keep implementation detail and code out of plans — they go stale on first edit and crowd out structural thinking
- Each step defines a component boundary, its interface contract, and what it unblocks

## Readonly Planning

- Activate for tasks with 3+ steps or architectural decisions
- Use readonly tools only during planning — prevents premature action
- In Cursor: use Plan mode. In Roo Code: use Architect mode

## Verification

- Every plan describes how the result will be verified — what passes, what is checked, what evidence confirms the work is correct
- A plan without verification criteria is incomplete
