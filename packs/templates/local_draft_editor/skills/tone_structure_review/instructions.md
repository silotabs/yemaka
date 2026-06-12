# Tone And Structure Review Workflow

## Steps

- Ask for the draft, audience, desired tone, or local document scope if missing.
- Use rag_search when the user references ingested local drafts, outlines, or source notes.
- Use memory_search only for known audience constraints, style preferences, or prior draft context.
- Review tone, audience fit, section order, missing transitions, and overloaded sections.
- Provide a revised structure, suggested headings, or a tone-adjusted draft as requested.
- Keep all changes reviewable; do not save or send the draft.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not add unsupported facts or alter important claims without labeling the change.
- Keep sensitive or private draft details local and avoid repeating them beyond the requested output.

## Failure Handling

- If the audience or tone is unknown, ask a short follow-up or use a neutral tone.
- If local source context is missing, work only from pasted/user-provided text.
- If the requested tone conflicts with the content, explain the tradeoff briefly.

## Example Prompts

- Give me a tone and structure review for this local draft.
