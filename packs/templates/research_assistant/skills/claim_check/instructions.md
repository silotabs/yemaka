# Claim Check Workflow

## Steps

- Ask the user for the exact claim or draft passage if it is missing.
- Break the claim into checkable statements.
- Use rag_search to find matching or conflicting local document evidence.
- Use memory_search for remembered decisions, corrections, or constraints related to the claim.
- Return a compact table with statement, status, evidence, and notes.
- Use statuses: supported, partly supported, contradicted, or not found.

## Verification Checks

- Do not treat absence of evidence as disproof.
- Keep quoted snippets short and cite the local source path or memory label when available.
- Preserve uncertainty and identify what additional local document would be needed.

## Failure Handling

- If local searches return no evidence, mark statements as not found and ask for the relevant document or memory context.
- If the claim includes sensitive details, minimize repetition and do not save anything.

## Example Prompts

- Check this claim against my docs and memory, and label what is supported or unsupported.
