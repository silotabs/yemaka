# Cleanup Plan Workflow

## Steps

- Confirm the cleanup goal and protected paths if they are missing.
- Use file_tree to inspect the relevant workspace area.
- Use search_files for repeated names, extensions, generated-output folders, old drafts, or user-specified patterns.
- Use file_stat for specific cleanup candidates when size or modified-time clues affect prioritization.
- Produce a plan with proposed keep, archive, review, and possible-delete groups.
- Include a confirmation checklist the user can review before any separate write-capable tool is used.

## Verification Checks

- Never delete, move, rename, edit, or create files.
- Never imply cleanup has been performed.
- Do not use shell, internet, connectors, scheduler, memory_write, or edit_file.
- Label risky suggestions as review-only.

## Failure Handling

- If the scope is too broad, ask for a folder, extension, date range, or file pattern.
- If evidence is thin, provide only a review checklist instead of a confident cleanup plan.

## Example Prompts

- Make a cleanup plan for duplicate-looking notes and old drafts, but do not change anything.
