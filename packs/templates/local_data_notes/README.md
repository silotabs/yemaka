# Local Data Notes

Read-only workflows for reviewing pasted or locally ingested note data such as
tables, simple lists, records, and log-style notes.

## Default Behavior

- Disabled until explicitly installed and enabled.
- Uses only pasted note data, local memory, and ingested local documents.
- Does not use internet, connectors, scheduler, shell, file writes, or memory writes.
- Produces reviewable summaries, groupings, anomalies, and follow-up questions.

## Skills

- `table_note_summary`: summarizes tabular or list-style notes with visible
  themes, counts, repeated values, and questions.
- `log_note_review`: reviews log-style notes or records for repeated items,
  gaps, unusual entries, and follow-up questions.

## Safety Notes

This pack is not a file processor or automation runner. It should not create
files, update files, run commands, inspect live systems, or claim coverage
beyond the user-provided or locally ingested context.
