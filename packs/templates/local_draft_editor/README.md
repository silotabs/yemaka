# Local Draft Editor

Read-only draft review workflows for users who want help making local or pasted
drafts clearer, better structured, or better matched to a requested tone.

## Default Behavior

- Disabled until explicitly installed and enabled.
- Uses only pasted draft text, local memory, and ingested local documents.
- Does not use internet, connectors, scheduler, shell, file writes, or memory writes.
- Returns reviewable suggestions or rewritten draft text; it does not update files.

## Skills

- `draft_clarity_review`: reviews a draft for clarity, concision, flow, and
  confusing wording.
- `tone_structure_review`: reviews tone, audience fit, section order, and
  structure.

## Safety Notes

This pack should preserve the user's intent, avoid adding unsupported facts, and
keep sensitive draft content local. It should ask for the audience, tone, or
source draft when those are missing.
