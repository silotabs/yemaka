# Trip Note Brief Workflow

## Steps

- Ask for pasted trip notes or local document scope if no context is provided.
- Use rag_search when the user points to ingested local itinerary notes, travel docs, or trip plans.
- Use memory_search only for user-stated travel preferences or local review constraints, not for sensitive identifiers.
- Summarize itinerary items, constraints, dates if provided, missing details, assumptions, and questions to review.
- Label anything inferred from incomplete local notes as an assumption.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not book, cancel, pay, contact providers, send itineraries, schedule reminders, update files, or store travel details.
- Do not provide current weather, prices, availability, travel advisories, visa/immigration advice, health advice, or legal advice from stale local context.

## Failure Handling

- If no travel context is available, ask for notes or local document scope.
- If the user asks for live travel facts, require configured/approved search instead of guessing.
- If the user asks to book, send, pay, or schedule, route outside this pack.

## Example Prompts

- Summarize these trip notes into an itinerary checklist.
