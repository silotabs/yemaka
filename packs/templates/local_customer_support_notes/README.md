# Local Customer Support Notes

Read-only customer-support workflows for users who want local help summarizing
tickets, reviewing response quality, and preparing support notes without
connecting to any helpdesk, CRM, inbox, or messaging service.

## Default Behavior

- Disabled until explicitly installed and enabled.
- Uses only pasted ticket text, local memory, and ingested local support documents.
- Does not use internet, connectors, scheduler, shell, file writes, or memory writes.
- Returns draft and review material only; it does not send messages, update tickets, or contact customers.

## Skills

- `support_ticket_summary`: summarizes a support ticket into issue, customer
  context, known facts, missing details, and draft next-step notes.
- `reply_quality_review`: reviews a proposed support reply for clarity,
  empathy, policy fit, factual support, and escalation risks.

## Safety Notes

This pack keeps support work local and manual. It must not send, publish,
schedule, store, update tickets, change customer records, connect to support
systems, or contact customers. Outputs are draft or review notes for the user
to verify before taking any action outside this pack.
