# Reply Quality Review Workflow

## Steps

- Ask for the reply draft, ticket context, and intended outcome if they are not provided.
- Use rag_search when the user references ingested local support policies, product notes, troubleshooting notes, or ticket context.
- Use memory_search only for user-stated support style preferences, escalation rules, or local process constraints.
- Review the reply for clarity, empathy, factual support, unsupported promises, policy fit, missing context, and escalation risks.
- Provide concise review notes and optional revised draft wording for the user to verify and send manually.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not send, publish, schedule, store, update tickets, connect to support systems, or contact customers.
- Do not add unsupported facts, account actions, refunds, credits, timelines, warranties, compliance claims, or policy commitments.
- Keep customer identifiers and private support history minimal unless needed for the requested review.

## Failure Handling

- If ticket context or desired tone is unclear, use neutral, empathetic wording and mention the assumption.
- If local policy or product context conflicts, surface the conflict instead of choosing one silently.
- If the user asks to send, update, escalate, schedule, or contact someone, route outside this pack.

## Example Prompts

- Review this customer support reply before I send it myself.
