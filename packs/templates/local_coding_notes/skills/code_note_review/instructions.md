# Code Note Review Workflow

## Steps

- Ask for pasted developer notes or local document scope if no context is provided.
- Use rag_search when the user points to ingested local developer notes, API notes, architecture notes, or implementation notes.
- Use memory_search only for user-stated review preferences or local context boundaries.
- Summarize APIs, assumptions, gaps, risks, unclear terms, and review questions.
- Label anything inferred from incomplete notes as an assumption.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, direct file reads, or memory writes.
- Do not patch code, write files, run tests, run commands, install dependencies, use git, build, deploy, fetch remote docs, or create generated extensions.
- Do not repeat secrets, tokens, private repository URLs, credentials, customer data, or proprietary source text unless required for the requested review.

## Failure Handling

- If no local developer context is available, ask for notes or local document scope.
- If the user asks for live docs or package information, require configured/approved search instead of guessing.
- If the user asks to edit, run, build, deploy, or generate a tool, route outside this pack.

## Example Prompts

- Review these local coding notes and list API questions.
