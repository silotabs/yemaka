# Health Question Review Workflow

## Steps

- Ask for symptom notes, medication notes, question list, or local document scope if missing.
- Use rag_search for ingested local health notes, appointment notes, or medication notes.
- Use memory_search only for prior user-stated local context needed to organize the current questions.
- Produce clear clinician questions, context notes, missing-detail prompts, and items the user may want to verify.
- Separate user-provided notes from assumptions and unknowns.
- If urgent, severe, or emergency-sounding symptoms appear, tell the user to seek urgent medical care or local emergency services.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not diagnose, recommend treatment, change medication, triage emergencies, or interpret test results.
- Do not store health details or repeat sensitive information beyond the requested review.

## Failure Handling

- If context is thin, ask for the relevant notes or appointment scope.
- If a question requires medical judgment, frame it as something to ask a qualified clinician.
- If the user asks for emergency triage or treatment, decline that role and point them to urgent medical care.

## Example Prompts

- Review my local symptom notes and prepare clinician questions.
