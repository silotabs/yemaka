# Log Note Review Workflow

## Steps

- Ask for the log-style notes, records, timeframe, or local document scope if missing.
- Use rag_search for ingested local note logs, record lists, or status notes.
- Use memory_search only for prior user-stated labels, recurring categories, or local context.
- Review repeated items, gaps, unusual entries, possible clusters, and follow-up questions.
- Separate evidence from assumptions and avoid claiming live-system coverage.
- Present a reviewable note review only; do not run commands, inspect systems, create files, or save findings.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not diagnose system state, security issues, health issues, or live operational status from notes alone.
- Do not invent timestamps, sources, severity, owners, or missing records.

## Failure Handling

- If the local notes are too sparse, say what is missing and ask for the relevant note scope.
- If unusual entries are inferred, label them as assumptions.
- If the user asks for live inspection or command execution, explain that this pack only reviews provided/local notes.

## Example Prompts

- Review these log-style notes for patterns, gaps, and follow-up questions.
