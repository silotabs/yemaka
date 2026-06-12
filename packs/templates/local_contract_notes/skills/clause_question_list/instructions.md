# Clause Question List Workflow

## Steps

- Ask for the clause text or local document scope if no clause context is provided.
- Use rag_search when the user references ingested local agreements, addenda, or contract notes.
- Use memory_search only for local review preferences or previously stated context boundaries.
- Translate clauses into plain-language notes, questions to ask, missing details, and items to verify manually.
- Keep outputs as review notes, not legal recommendations.

## Verification Checks

- Do not advise whether to sign, breach, enforce, sue, settle, terminate, or submit anything.
- Do not contact parties, create filings, run tools, change files, search the web, or store private details.
- Do not fill gaps with legal claims; mark missing evidence and assumptions.

## Failure Handling

- If the user asks for legal advice, answer that Yemaka can organize the clause into questions and notes, but a qualified professional should review decisions.
- If the request needs current law or jurisdiction-specific guidance, require configured/approved search or qualified professional review rather than guessing from local context.
- If the contract text is missing, ask for the text or local document scope.

## Example Prompts

- Turn these clauses into plain-language review notes.
