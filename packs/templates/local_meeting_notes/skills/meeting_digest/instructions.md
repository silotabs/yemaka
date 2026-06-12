# Meeting Digest Workflow

## Steps

- Ask for the notes, meeting topic, or local document scope if none is provided.
- Use rag_search when the user points to ingested local notes, transcripts, or documents.
- Use memory_search for prior project decisions, recurring commitments, or known constraints.
- Produce a compact digest with context, decisions, action items, risks, open questions, and suggested next review.
- Mark each item as user-provided, document-backed, memory-backed, or inference.
- Keep action items as a draft; do not save them to memory or schedule anything.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, or file writes.
- Do not invent attendees, owners, dates, or decisions.
- Separate decisions from suggestions and unresolved questions.

## Failure Handling

- If local evidence is missing, summarize only the pasted/user-provided notes and say what context was unavailable.
- If owners or dates are ambiguous, leave them as `unassigned` or `date unclear` instead of guessing.

## Example Prompts

- Summarize these meeting notes and list decisions, risks, and open questions.
