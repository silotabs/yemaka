# Support Ticket Summary Workflow

## Steps

- Ask for pasted ticket text, customer context, or local document scope if no ticket context is provided.
- Use rag_search when the user points to ingested local support notes, product docs, troubleshooting notes, or policy context.
- Use memory_search only for user-stated support style preferences, local process notes, or account-context boundaries.
- Summarize the issue, customer goal, relevant facts, attempted fixes, likely missing details, risks, and draft next-step notes.
- Keep the output as reviewable support notes or draft wording for the user to verify manually.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not send messages, update tickets, contact customers, connect to helpdesk systems, schedule follow-ups, or store customer details.
- Do not invent product behavior, policy commitments, refunds, deadlines, account status, order status, or external facts.
- Avoid repeating customer names, contact details, account identifiers, payment details, addresses, order numbers, or private support history unless required for the requested summary.

## Failure Handling

- If no local ticket context is available, ask for the ticket text or local document scope.
- If policy, product, or account context is missing, mark it as an assumption or missing detail instead of guessing.
- If the user asks to send, update, escalate, schedule, or contact someone, route outside this pack.

## Example Prompts

- Summarize this support ticket into issue, facts, missing details, and next steps.
