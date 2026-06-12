# Proposal Follow-Up Review Workflow

## Steps

- Ask for the proposal, client notes, document scope, or pasted context if none is provided.
- Use rag_search when the user references ingested local proposals, client notes, meeting notes, scope notes, or prior draft context.
- Use memory_search only for user-stated client preferences, proposal constraints, prior local context, or communication boundaries.
- Produce a reviewable follow-up analysis with confirmed points, unclear claims, risks, likely questions, next-step options, and evidence labels.
- Mark outreach-ready wording as draft text for review only if the user asks for wording.
- Keep the review local; do not send outreach, contact clients, schedule follow-ups, update records, write files, or write memory.

## Verification Checks

- Do not use internet search, internet fetch, connectors, scheduler, shell, file writes, or memory writes.
- Do not invent client needs, company updates, market conditions, competitor facts, pricing, approvals, deadlines, or commitments.
- Do not claim current market, company, account, prospect, pricing, or competitor facts unless the local evidence explicitly supports them.

## Failure Handling

- If proposal or client context is missing, ask for it or ask for the local document scope.
- If evidence is thin, provide questions and assumptions instead of firm recommendations.
- If local context conflicts, surface the conflict with source labels instead of silently resolving it.

## Example Prompts

- Review these proposal notes and identify follow-up risks and unclear claims.
