# Account Hygiene Checklist Workflow

## Steps

- Ask for local notes or document scope if the user does not provide account-safety context.
- Use rag_search when the user references ingested account-safety notes or local privacy docs.
- Use memory_search only for user-stated review preferences, not for secrets or raw credentials.
- Produce a manual checklist grouped by account access, recovery, two-factor setup, data sharing, and unresolved questions.
- Keep the checklist high-level and avoid exposing private identifiers unless needed for the user's requested review.

## Verification Checks

- Do not request, reveal, store, export, or validate secrets.
- Do not connect to password managers, email, browsers, websites, apps, or external services.
- Do not run shell, inspect devices, scan repositories, test networks, schedule reminders, or update files.

## Failure Handling

- If the user asks to connect to an account or password manager, require the connector/action path outside this pack.
- If the user asks for reminders, require the scheduler path outside this pack.
- If the user asks for live vulnerability assessment, require the cyber safety gate outside this pack.

## Example Prompts

- Create an account hygiene checklist from my local notes.
