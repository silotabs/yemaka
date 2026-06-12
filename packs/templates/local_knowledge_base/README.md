# Local Knowledge Base

Read-only knowledge-base workflows for users who want local help turning
ingested documents and notes into FAQs, topic indexes, glossary items, source
summaries, gaps, and review questions.

## Default Behavior

- Disabled until explicitly installed and enabled.
- Uses only pasted notes, local memory, and ingested local documents.
- Does not use internet, connectors, scheduler, shell, file writes, or memory writes.
- Produces reviewable knowledge notes only; it does not create databases,
  knowledge graphs, vector stores, exports, or files.

## Skills

- `knowledge_base_faq`: creates source-backed FAQ entries from local context.
- `topic_index_review`: organizes local documents into topics, glossary items,
  source hints, gaps, and questions.

## Safety Notes

This pack must not create files, PDFs, exports, databases, knowledge graphs,
embedding indexes, vector stores, sync jobs, scheduled refreshes, connectors,
or live web research. Current external facts require the normal approved
internet/search path, and generated infrastructure belongs outside this pack.
