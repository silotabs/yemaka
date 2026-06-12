# Document Prep Review Workflow

## Steps

- Ask for pasted document-prep notes or local document scope if no context is provided.
- Use rag_search when the user points to ingested local application, form, checklist, or supporting-document notes.
- Use memory_search only for user-stated review preferences or local context boundaries.
- Organize required documents, missing items, unclear entries, assumptions, and review questions.
- Keep the output as preparation notes for the user to verify manually.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not submit, upload, email, mail, file, sign, generate PDFs, update files, contact services, or store document details.
- Do not decide eligibility, compliance, legal, tax, medical, immigration, financial, benefits, or identity-verification outcomes.
- Avoid repeating sensitive identifiers unless necessary for the requested review.

## Failure Handling

- If no local document-prep context is available, ask for notes or local document scope.
- If the user asks for current external requirements, require configured/approved search instead of using stale local context.
- If the user asks for external action, route outside this pack.

## Example Prompts

- Review these document-prep notes and list missing fields.
