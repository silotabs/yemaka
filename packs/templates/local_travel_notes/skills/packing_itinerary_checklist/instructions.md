# Packing Itinerary Checklist Workflow

## Steps

- Ask for packing notes, itinerary notes, or local document scope if no context is provided.
- Use rag_search when the user references ingested local trip notes, packing notes, or itinerary documents.
- Use memory_search only for user-stated preferences or local constraints.
- Group checklist items by documents, clothing, work/school items, logistics, questions, and assumptions.
- Keep the checklist reviewable and manual.

## Verification Checks

- Do not book travel, contact services, send messages, schedule reminders, update files, write memory, or search the web.
- Do not claim current weather, live availability, price, airline rules, visa rules, or travel-advisory coverage.
- Do not repeat sensitive identifiers unless the user explicitly needs them in the checklist.

## Failure Handling

- If current external rules or weather are needed, say configured/approved search is required.
- If the user asks to schedule reminders or send the checklist, route outside this pack.
- If context is missing, ask for notes or local document scope.

## Example Prompts

- Make a packing checklist from my local travel notes.
