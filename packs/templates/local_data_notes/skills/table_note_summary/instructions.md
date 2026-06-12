# Table Note Summary Workflow

## Steps

- Ask for the note data or local document scope if none is provided.
- Use rag_search when the user references ingested local tables, lists, records, or notes.
- Use memory_search only for prior user-stated categories, naming preferences, or known local context.
- Summarize visible fields, themes, counts, repeated values, gaps, and follow-up questions.
- Keep calculations simple and evidence-bound; label inferred groupings as assumptions.
- Present a reviewable summary only; do not create files, save data, or run commands.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not invent missing columns, rows, values, dates, totals, or sources.
- Do not claim statistical certainty from incomplete notes.

## Failure Handling

- If the note data is missing, ask for pasted notes or a local document scope.
- If fields are ambiguous, describe the ambiguity and use neutral labels.
- If values conflict, show the conflict instead of silently resolving it.

## Example Prompts

- Summarize these table notes and identify repeated items.
