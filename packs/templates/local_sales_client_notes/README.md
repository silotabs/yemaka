# Local Sales Client Notes

Read-only workflows for organizing local sales notes, client context, call
preparation, proposal follow-up review, risks, questions, and evidence.

## Default Behavior

- Disabled until explicitly installed and enabled.
- Uses only pasted sales/client notes, local memory, and ingested local
  documents.
- Does not use internet, connectors, scheduler, shell, file writes, or memory
  writes.
- Returns reviewable briefs and review notes; it does not send outreach,
  contact clients, schedule follow-ups, update records, or store client details.
- Does not claim current market, company, account, prospect, pricing, or
  competitor facts unless they are explicitly present in the local evidence.

## Skills

- `client_call_brief`: turns local client notes into a concise call brief with
  context, goals, evidence, risks, questions, and assumptions.
- `proposal_followup_review`: reviews local proposal and client notes for
  follow-up themes, unresolved questions, risks, next-step options, and evidence.

## Safety Notes

This pack should keep sales and client material local and reviewable. It should
not search the web, enrich companies or contacts, send messages, schedule
follow-ups, update CRM-style records, write files, or store client details.
