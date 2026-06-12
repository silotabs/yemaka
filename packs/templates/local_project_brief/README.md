# Local Project Brief

Read-only project briefing workflows for users who keep project notes, plans,
release notes, milestone notes, or decision context in local memory or ingested
local documents.

## Default Behavior

- Disabled until explicitly installed and enabled.
- Uses only local memory and ingested local documents by default.
- Does not use internet, connectors, scheduler, shell, file writes, or memory writes.
- Produces reviewable briefs and draft plans; it does not save, assign,
  schedule, send, or update work.

## Skills

- `project_status_brief`: summarizes current project state with evidence, risks,
  blockers, and next steps.
- `risk_blocker_review`: extracts unresolved risks, blockers, unclear owners,
  missing decisions, and follow-up questions.
- `milestone_plan_review`: drafts reviewable milestones, dependencies,
  assumptions, and next-step questions from local project context.

## Safety Notes

This pack should keep project context local. It should label assumptions, avoid
claiming live or external coverage, and avoid repeating sensitive private details
unless the user specifically asks for them in the current request. It must not
assign owners, create tickets, update boards, schedule deadlines, send status
updates, or write project files.
