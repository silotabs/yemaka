# Milestone Plan Review Workflow

## Steps

- Ask for the project, planning horizon, or local document scope if none is provided.
- Use rag_search when the user points to ingested local project plans, release notes, roadmaps, or decision notes.
- Use memory_search for prior project constraints, known decisions, recurring blockers, or user-stated planning preferences.
- Draft reviewable milestones, dependencies, assumptions, risks, unclear owners, unclear dates, and next-step questions.
- Label each item as user-provided, document-backed, memory-backed, or assumption.
- Keep the output as a planning draft for the user to review manually.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not assign work, create tickets, update boards, schedule deadlines, send status updates, write project files, or store commitments.
- Do not invent owners, dates, priorities, commitments, metrics, or decisions.
- Separate evidence from assumptions and unresolved questions.

## Failure Handling

- If local evidence is missing, ask for local notes, pasted context, or a narrower planning horizon.
- If owners or dates are unclear, mark them as `unclear` rather than guessing.
- If the user asks to perform project-management actions, route outside this pack.

## Example Prompts

- Draft a milestone review plan from my local project notes without assigning work.
