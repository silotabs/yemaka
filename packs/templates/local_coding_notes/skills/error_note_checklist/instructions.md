# Error Note Checklist Workflow

## Steps

- Ask for pasted error notes or local document scope if no context is provided.
- Use rag_search when the user points to ingested local logs, error notes, debugging notes, or implementation notes.
- Use memory_search only for user-stated review preferences or local context boundaries.
- Organize symptoms, likely context, missing details, safe manual checks, assumptions, and questions.
- Keep recommendations as a manual checklist; do not execute anything.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, direct file reads, or memory writes.
- Do not patch code, run tests, run commands, install dependencies, use git, build, deploy, fetch remote docs, or create generated extensions.
- Do not claim the issue is fixed unless the user provides evidence.
- Avoid repeating secrets, tokens, private repository URLs, credentials, customer data, or proprietary source text unless required for the requested review.

## Failure Handling

- If no local error context is available, ask for notes or local document scope.
- If the user asks to execute checks or apply a fix, route outside this pack.
- If local notes conflict, surface the conflict instead of choosing silently.

## Example Prompts

- Turn my ingested developer notes into a troubleshooting checklist.
