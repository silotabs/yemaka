# Follow-up Capture Workflow

## Steps

- Read only the notes or summary the user provides in the current request.
- Use memory_search to check whether similar follow-ups or preferences already exist before proposing new memory.
- Extract a concise list of action items with owner, due hint, and source note when present.
- Ask the user to confirm exactly which items should be remembered.
- Use memory_write only for confirmed items, phrased as brief local follow-up memory.

## Verification Checks

- Do not store raw messages, secrets, credentials, or sensitive personal details.
- Do not create calendar events, reminders, connector messages, or background jobs.
- Report which items were saved and which were left unsaved.

## Failure Handling

- If confirmation is missing, stop after the proposed list and ask what to save.
- If memory_write is unavailable, return the proposed follow-ups without saving them.

## Example Prompts

- Review this meeting summary and capture only the follow-ups I confirm.
