# Client Call Brief Workflow

## Steps

- Ask for the client, account, document scope, or pasted notes if no context is provided.
- Use rag_search when the user points to ingested local sales notes, client notes, proposals, meeting notes, or account context.
- Use memory_search only for user-stated preferences, recurring client constraints, prior local context, or communication boundaries.
- Produce a compact brief with known context, call goals, evidence-backed talking points, risks, open questions, and assumptions.
- Label each item as user-provided, document-backed, memory-backed, or assumption.
- Keep the brief reviewable; do not send outreach, contact clients, schedule follow-ups, update records, write files, or write memory.

## Verification Checks

- Do not use internet search, internet fetch, connectors, scheduler, shell, file writes, or memory writes.
- Do not invent company facts, market facts, decision makers, budgets, competitors, pricing, commitments, timelines, or relationship history.
- Do not claim current market, company, account, prospect, pricing, or competitor facts unless the local evidence explicitly supports them.

## Failure Handling

- If local evidence is missing, say what context was unavailable and summarize only the user-provided text.
- If the client or call goal is unclear, ask a short follow-up before selecting local sources.
- If evidence conflicts, present the conflict with source labels instead of choosing one silently.

## Example Prompts

- Prepare a client call brief from these local account notes.
