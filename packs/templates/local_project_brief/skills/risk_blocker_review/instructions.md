# Risk And Blocker Review Workflow

## Steps

- Ask for the project, timeframe, or local document scope if none is provided.
- Use rag_search for ingested local project notes, issue summaries, release notes, or planning documents.
- Use memory_search for prior constraints, known unresolved decisions, and recurring blockers.
- Extract risks, blockers, unresolved decisions, unclear owners, unclear dates, and follow-up questions.
- Group findings by urgency when evidence supports it.
- Include a short evidence note for each finding.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not assign owners, deadlines, severity, or status unless the local evidence says so.
- Do not turn the review into an action plan that performs changes; keep it as a draft for the user.

## Failure Handling

- If evidence is thin, say so and ask for the relevant local notes or pasted context.
- If owners or dates are unclear, mark them as `unclear` rather than guessing.
- If a risk is inferred, label it as an assumption.

## Example Prompts

- Review the local release notes and list blockers, risks, and next steps.
