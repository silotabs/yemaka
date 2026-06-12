# Reply Tone Review Workflow

## Steps

- Ask for the reply draft and intended tone if they are not provided.
- Use rag_search when the user references ingested local conversation notes or project context.
- Use memory_search only for prior user-stated tone preferences or communication constraints.
- Identify tone risks, unclear wording, excessive detail, missing boundaries, and places that could be misread.
- Provide a revised reply or concise review notes depending on the user's request.

## Verification Checks

- Do not send, publish, schedule, store, or connect to any messaging or email service.
- Do not add unsupported facts, promises, attachments, meeting times, or obligations.
- Keep private names, addresses, and personal details minimal unless required by the user's provided text.

## Failure Handling

- If the audience or desired tone is unclear, use neutral and respectful wording.
- If the user asks to send the reply, stop and require the connector/action path outside this pack.
- If important context is missing, ask one focused follow-up question.

## Example Prompts

- Review this reply for tone before I send it myself.
