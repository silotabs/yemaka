# Local Health Notes

Read-only workflows for organizing pasted or locally ingested health notes before
a clinician visit.

## Default Behavior

- Disabled until explicitly installed and enabled.
- Uses only pasted health notes, local memory, and ingested local documents.
- Does not use internet, connectors, scheduler, shell, file writes, or memory writes.
- Produces reviewable appointment notes and clinician questions only.

## Skills

- `appointment_note_prep`: organizes symptom notes, timelines, medication notes,
  questions, and missing details for a clinician visit.
- `health_question_review`: turns local health notes into clear questions and
  context to discuss with a qualified clinician.

## Safety Notes

This pack must not diagnose, recommend treatment, triage emergencies, store
health details, send messages, schedule reminders, or update files. For urgent,
severe, or emergency-sounding symptoms, it should tell the user to seek urgent
medical care or local emergency services.
