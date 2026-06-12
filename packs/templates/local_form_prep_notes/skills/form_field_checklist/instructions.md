# Form Field Checklist Workflow

## Steps

- Ask for pasted form notes or local document scope if no context is provided.
- Use rag_search when the user points to ingested local form, application, or document-prep notes.
- Use memory_search only for user-stated review preferences or local context boundaries, not for sensitive identifiers.
- List visible fields, likely missing details, assumptions, conflicts, and questions to review.
- Keep the output as a manual checklist for the user to complete themselves.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not fill, sign, submit, upload, email, mail, file, generate PDFs, update files, contact services, or store form details.
- Do not provide legal, tax, medical, immigration, financial, benefits, eligibility, or compliance decisions.
- Avoid repeating identity numbers, passport numbers, account numbers, payment details, addresses, dates of birth, or contact details unless required for the requested review.

## Failure Handling

- If no local form context is available, ask for notes or local document scope.
- If the user asks for external requirements or current rules, require configured/approved search instead of guessing.
- If the user asks to fill, sign, submit, upload, email, file, or schedule, route outside this pack.

## Example Prompts

- Make a form field checklist from these local notes.
