# Email Draft Review Workflow

## Steps

- Ask for the message goal, recipient context, or local document scope if no draft context is provided.
- Use rag_search when the user points to ingested local notes, project context, or prior drafts.
- Use memory_search only for user-stated tone preferences, audience notes, or communication constraints.
- Produce a reviewable draft, optional subject line, and short notes about assumptions or missing details.
- Keep the user's intent, facts, names, commitments, dates, and boundaries unchanged unless the user asks for a change.

## Verification Checks

- Do not send messages, open email clients, use connectors, use internet search, schedule follow-ups, run shell, write files, or write memory.
- Do not invent facts, attachments, commitments, quotes, or external context.
- Do not repeat private contact details unless they are needed for the requested draft.

## Failure Handling

- If no goal or message context is available, ask for the goal and recipient context.
- If the tone is unclear, use a neutral professional tone and mention that assumption.
- If local context conflicts, surface the conflict instead of silently choosing one version.

## Example Prompts

- Draft a polite email from these notes.
