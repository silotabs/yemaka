# Topic Index Review Workflow

## Steps

- Ask for pasted notes or local document scope if no context is provided.
- Use rag_search when the user points to ingested local docs, manuals, notes, or reference material.
- Use memory_search only for user-stated organization preferences or local context boundaries.
- Group visible topics, glossary items, source hints, repeated themes, gaps, contradictions, and review questions.
- Keep the output as a reviewable outline, not a persisted index.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not create or update a database, graph, vector store, embedding index, file, PDF, export, sync target, or scheduled refresh.
- Do not claim coverage beyond local sources.
- Avoid repeating sensitive source text or private identifiers unless required for the requested review.

## Failure Handling

- If no local sources are available, ask for notes or local document scope.
- If the user asks for a persisted index, export, sync, or automation, route outside this pack.
- If local sources conflict, surface the conflict instead of choosing silently.

## Example Prompts

- Turn these local notes into a topic index with source hints.
