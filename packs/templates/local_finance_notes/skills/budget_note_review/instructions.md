# Budget Note Review Workflow

## Steps

- Ask for pasted budget notes or local document scope if no context is provided.
- Use rag_search when the user points to ingested local budget, expense, or payment notes.
- Use memory_search only for user-stated review preferences or local context boundaries.
- Summarize categories, repeated items, unclear entries, missing details, and questions to review.
- Label assumptions and avoid making financial decisions for the user.

## Verification Checks

- Do not use internet search, connectors, scheduler, shell, file writes, or memory writes.
- Do not provide tax, investment, accounting, lending, insurance, legal, or trading advice.
- Do not connect to banks, payment processors, accounting tools, wallets, brokers, or tax services.
- Do not move money, pay invoices, place trades, retrieve live prices, create files, update ledgers, or store financial details.

## Failure Handling

- If no local context is available, ask for the notes or local document scope.
- If the user asks for financial advice, say this pack can organize notes and questions but cannot decide what action to take.
- If local entries conflict, surface the conflict instead of choosing silently.

## Example Prompts

- Review these budget notes and summarize expense categories.
