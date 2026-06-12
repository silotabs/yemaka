# Contract Note Review Workflow

## Steps

- Ask for pasted contract text, notes, or local document scope if no context is provided.
- Use rag_search when the user points to ingested local contract notes, agreements, or related documents.
- Use memory_search only for user-stated review preferences or local context boundaries.
- Summarize parties, dates, obligations, payment terms, renewal or termination notes, unclear clauses, and questions to review.
- Label assumptions and keep legal decisions out of the answer.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not provide legal advice, decide enforceability, interpret jurisdiction-specific law, sign documents, submit forms, contact anyone, or update files.
- Do not invent clauses, facts, dates, obligations, citations, or legal requirements.

## Failure Handling

- If no contract context is available, ask for the text or local document scope.
- If the user asks what legal action to take, say this pack can organize notes and questions, but legal decisions require qualified professional review.
- If local documents conflict, surface the conflict instead of choosing silently.

## Example Prompts

- Review these contract notes and list questions.
