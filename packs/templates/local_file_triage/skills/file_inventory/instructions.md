# File Inventory Workflow

## Steps

- Ask for the workspace folder or scope if it is missing.
- Use file_tree to understand the top-level layout before making claims.
- Use search_files for user-specified patterns, extensions, names, or review topics.
- Use file_stat for specific files when size, type, or modified-time clues matter.
- Group findings by review purpose: active work, drafts, generated artifacts, large files, duplicate-looking names, and unclear items.
- Return a compact inventory with paths, reason for inclusion, and recommended next review step.

## Verification Checks

- Do not move, rename, delete, write, or patch files.
- Do not run shell commands.
- Do not search the internet or use connectors.
- Separate observed file metadata from guesses.

## Failure Handling

- If the workspace is empty or inaccessible, say what was unavailable and ask for a narrower folder or file pattern.
- If a file looks sensitive from its name, mention only the path and the reason it needs careful handling.

## Example Prompts

- Triage my workspace files and group what needs review.
