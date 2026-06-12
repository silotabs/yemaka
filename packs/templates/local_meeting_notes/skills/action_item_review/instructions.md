# Action Item Review Workflow

## Steps

- Ask for the notes, transcript, or local document scope if the source is unclear.
- Use rag_search for ingested local meeting notes or project documents.
- Use memory_search for related commitments or standing constraints.
- Extract action items with owner, due date, source, blocker, and confidence.
- Group follow-ups into confirmed, needs clarification, and suggested.
- Present memory-save candidates as a draft only; do not write memory.

## Verification Checks

- Do not schedule reminders, send messages, write files, or save memory.
- Do not assign an owner or due date unless it is present in the notes or local context.
- Keep sensitive details minimal and quote only short necessary snippets.

## Failure Handling

- If no action items are found, say that clearly and list any open questions.
- If local search returns unrelated context, ignore it and explain that it did not match the notes.

## Example Prompts

- Extract action items from my local notes about the release review.
