# Maintenance Log Review Workflow

## Steps

- Ask for pasted logs or local document scope if no context is provided.
- Use rag_search for ingested local maintenance logs, warranty notes, appliance notes, service records, or property notes.
- Use memory_search only for review preferences or local context boundaries, not sensitive identifiers.
- Organize issues, dates or sequence if provided, prior actions, patterns, unresolved items, questions, and professional cautions.
- Label anything inferred from incomplete local notes as an assumption.
- Keep the output as reviewable notes for the user or a qualified professional.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not provide hazardous repair instructions, bypass safety devices, or guide electrical, gas, plumbing, structural, roofing, or other dangerous work.
- Do not diagnose hidden failures, guarantee safety, decide code compliance, or replace licensed professional advice.
- Avoid repeating addresses, access codes, account, warranty, tenant, contact, payment, or serial details unless required.

## Failure Handling

- If context is unavailable, ask for notes or local document scope.
- If notes imply urgent danger, active leak, fire risk, gas smell, electrical hazard, structural movement, mold, asbestos, carbon monoxide, or injury risk, tell the user to stop and contact emergency services, utility providers, or a qualified professional as appropriate.
- If asked for repair steps or hazardous troubleshooting, redirect to safe note organization and professional questions.

## Example Prompts

- Review these maintenance log notes and list unresolved issues.
