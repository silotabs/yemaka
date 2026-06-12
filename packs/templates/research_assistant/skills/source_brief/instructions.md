# Source Brief Workflow

## Steps

- Ask for the research question, scope, and preferred brief length if they are missing.
- Use rag_search for local document evidence before answering.
- Use memory_search for prior decisions, user-provided constraints, or remembered project context that may affect the answer.
- Produce a short brief with answer, evidence, caveats, and open questions.
- Label every claim as document-backed, memory-backed, user-provided, or inference.
- Cite local document paths or memory labels when available; never imply live web access.

## Verification Checks

- Separate direct evidence from interpretation.
- Say when local sources are thin, stale, conflicting, or missing.
- Do not create tasks, jobs, connectors, web searches, or durable memory.

## Failure Handling

- If rag_search is unavailable or empty, answer only from user-provided context and memory_search results, and call out the missing document evidence.
- If memory_search is unavailable or empty, continue with local document evidence and say that memory was not used.

## Example Prompts

- Build a source-backed brief from my local documents about the extension rollback plan.
