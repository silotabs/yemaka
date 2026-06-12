# Content Calendar Outline Workflow

## Steps

- Ask for pasted campaign notes or local document scope if no context is provided.
- Use rag_search when the user points to ingested local campaign notes, marketing notes, or draft content.
- Use memory_search only for user-stated audience, tone, or review preferences.
- Group themes, draft content slots, candidate timing, assumptions, missing context, and questions to review.
- Keep the calendar as a reviewable outline, not a scheduled or published plan.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not post, publish, send newsletters, schedule posts, generate media assets, contact audiences, or update files.
- Do not invent current trends, platform rules, ad-policy compliance, legal clearance, brand approval, dates, or metrics.

## Failure Handling

- If campaign context is missing, ask for notes or local document scope.
- If current trends or platform rules are needed, require configured/approved search instead of guessing.
- If the user asks to post, publish, send, or schedule, route outside this pack.

## Example Prompts

- Create a content calendar outline from my local campaign notes.
