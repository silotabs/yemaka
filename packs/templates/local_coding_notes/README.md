# Local Coding Notes

Read-only coding-note workflows for users who want local help organizing pasted
or ingested developer notes, API notes, error notes, assumptions, gaps, and
review questions.

## Default Behavior

- Disabled until explicitly installed and enabled.
- Uses only pasted developer notes, local memory, and ingested local documents.
- Does not use internet, connectors, scheduler, shell, file writes, or memory writes.
- Produces reviewable notes only; it does not edit files, run tests, install
  dependencies, use git, build, deploy, or generate extensions.

## Skills

- `code_note_review`: organizes local developer notes into API notes,
  assumptions, gaps, and questions.
- `error_note_checklist`: turns pasted or locally ingested error notes into a
  manual troubleshooting checklist.

## Safety Notes

This pack is not the workspace coding agent. It must not inspect files directly,
patch code, run commands, run tests, install dependencies, use git, build,
deploy, fetch remote docs, or create generated extensions. Those actions belong
to the normal tool, extension, or approval-gated coding paths.
