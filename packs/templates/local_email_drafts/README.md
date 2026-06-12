# Local Email Drafts

Read-only communication drafting workflows for users who want local help with
email replies, subject lines, tone, and structure without connecting to any
mail service.

## Default Behavior

- Disabled until explicitly installed and enabled.
- Uses only pasted message text, local memory, and ingested local documents.
- Does not use internet, connectors, scheduler, shell, file writes, or memory writes.
- Returns reviewable draft text only; it does not send, schedule, or save messages.

## Skills

- `email_draft_review`: prepares or improves an email draft from provided
  context.
- `reply_tone_review`: reviews a reply for tone, clarity, boundaries, and
  audience fit.

## Safety Notes

This pack should keep communication local, avoid exposing private contact
details unnecessarily, and clearly separate drafting from sending. Requests to
send, publish, schedule, connect to an inbox, or contact someone must remain
connector- or scheduler-gated outside this pack.
