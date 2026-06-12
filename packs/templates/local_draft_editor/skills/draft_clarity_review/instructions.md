# Draft Clarity Review Workflow

## Steps

- Ask for the draft text or local document scope if no draft is provided.
- Use rag_search when the user points to ingested local drafts, notes, or source context.
- Use memory_search only for user-stated style constraints, audience notes, or prior draft preferences.
- Identify unclear wording, repetition, weak flow, and places where meaning could be misunderstood.
- Provide a concise revision or a short set of review notes, depending on the user's request.
- Mark any substantial addition as an assumption and keep unsupported facts out of the draft.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not change the user's meaning, claims, names, numbers, dates, or commitments without saying so.
- Do not invent citations, facts, quotes, or external context.

## Failure Handling

- If no draft is available, ask for the draft or local document scope.
- If the audience or tone is unclear, make a neutral edit and mention the assumption.
- If source context conflicts with the draft, surface the conflict instead of silently rewriting it.

## Example Prompts

- Review this draft for clarity and make it more concise.
