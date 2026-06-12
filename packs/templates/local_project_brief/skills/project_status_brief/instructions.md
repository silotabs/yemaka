# Project Status Brief Workflow

## Steps

- Ask for the project, document scope, or pasted context if none is provided.
- Use rag_search when the user points to ingested local project notes, release notes, plans, or status documents.
- Use memory_search for prior project constraints, decisions, known blockers, or recurring next steps.
- Produce a compact brief with current state, recent evidence, risks, blockers, next steps, and open questions.
- Label each item as user-provided, document-backed, memory-backed, or assumption.
- Keep the brief reviewable; do not save tasks, update files, schedule reminders, or notify anyone.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not invent owners, dates, commitments, metrics, or decisions.
- Separate evidence from assumptions and unresolved questions.

## Failure Handling

- If local evidence is missing, say what context was unavailable and summarize only the user-provided text.
- If scope is unclear, ask a short follow-up before selecting local sources.
- If evidence conflicts, present the conflict with source labels instead of choosing one silently.

## Example Prompts

- Give me a local project status brief from the ingested project notes.
