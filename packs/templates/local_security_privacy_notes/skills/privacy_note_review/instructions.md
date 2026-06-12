# Privacy Note Review Workflow

## Steps

- Ask for pasted privacy notes or local document scope if no context is provided.
- Use rag_search when the user points to ingested local privacy notes, account-setting notes, or policy notes.
- Use memory_search only for user-stated privacy preferences or local review constraints.
- Summarize the notes into a checklist, unresolved questions, and manually reviewable next steps.
- Clearly label assumptions when the local notes do not contain enough evidence.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, memory writes, or live account access.
- Do not request, store, repeat, export, or infer passwords, tokens, recovery codes, API keys, or other secrets.
- Do not change settings, contact services, inspect devices, scan networks, or test websites.

## Failure Handling

- If notes are missing, ask for the notes or local document scope.
- If the user asks for live account changes or external checks, stop and route outside this pack.
- If the user reports active compromise, advise official account-recovery channels and avoiding secret sharing.

## Example Prompts

- Review these privacy notes and make a checklist.
