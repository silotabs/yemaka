# Knowledge Base FAQ Workflow

## Steps

- Ask for pasted notes or local document scope if no context is provided.
- Use rag_search when the user points to ingested local docs, manuals, notes, or reference material.
- Use memory_search only for user-stated organization preferences or local context boundaries.
- Create concise question-and-answer entries, source hints, missing details, contradictions, and review questions.
- Label unsupported answers as gaps instead of inventing facts.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not create databases, knowledge graphs, embedding indexes, vector stores, files, PDFs, exports, sync jobs, or scheduled refreshes.
- Do not claim current external coverage or browse for updates.
- Avoid repeating secrets, credentials, private identifiers, personal contact details, account details, or confidential source text unless required for the requested review.

## Failure Handling

- If no local sources are available, ask for notes or local document scope.
- If the user asks for current external facts, require configured/approved search instead of guessing.
- If the user asks to build, export, write, sync, or schedule a knowledge base, route outside this pack.

## Example Prompts

- Create a local knowledge base FAQ from my ingested docs.
