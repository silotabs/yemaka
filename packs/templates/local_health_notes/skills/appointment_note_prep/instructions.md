# Appointment Note Prep Workflow

## Steps

- Ask for health notes, appointment context, or local document scope if missing.
- Use rag_search when the user references ingested local health notes, symptom notes, medication notes, or appointment notes.
- Use memory_search only for prior user-stated constraints or local context needed for the current note organization.
- Organize notes into symptoms or concerns, timeline, medication notes, questions for a clinician, and missing details to clarify.
- Keep the output as reviewable notes for a qualified clinician.
- If the notes mention urgent, severe, or emergency-sounding symptoms, tell the user to seek urgent medical care or local emergency services.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not diagnose, recommend treatment, change medication, rank medical urgency, or replace clinician advice.
- Do not invent symptoms, dates, medications, dosages, test results, or medical history.

## Failure Handling

- If notes are missing, ask for pasted notes or a local document scope.
- If medication or symptom details are unclear, mark them as `unclear` rather than guessing.
- If the user asks for diagnosis or treatment advice, redirect to clinician review and offer to organize questions.

## Example Prompts

- Organize these appointment notes and list questions for my doctor.
