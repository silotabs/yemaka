# Resume Fit Review Workflow

## Steps

- Ask for resume notes, role description, or local document scope if missing.
- Use rag_search when the user references ingested local resume notes, role descriptions, or career notes.
- Use memory_search only for user-stated career constraints, preferred phrasing, or local context.
- Produce fit signals, gaps, questions to clarify, examples to highlight, and evidence labels.
- Keep the output reviewable; do not update files, store details, contact people, or apply anywhere.
- Label assumptions when the local evidence does not fully support a fit signal or gap.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not invent work history, titles, dates, companies, skills, credentials, or application status.
- Do not claim live role availability or external job-market coverage.

## Failure Handling

- If the role description or resume notes are missing, ask for them or the local document scope.
- If evidence conflicts, show the conflict instead of resolving it silently.
- If personal details are unnecessary, omit or generalize them.

## Example Prompts

- Compare my local resume notes with this role description and list fit gaps.
