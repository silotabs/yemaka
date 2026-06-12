# Invoice Note Checklist Workflow

## Steps

- Ask for invoice notes or local document scope if no invoice context is provided.
- Use rag_search when the user references ingested local invoice, payment, or project billing notes.
- Use memory_search only for user-stated review preferences, not for account or payment details.
- Organize invoice status, dates, amounts if provided, missing fields, unclear entries, and follow-up questions.
- Present a checklist for manual review without sending, filing, updating, or paying anything.

## Verification Checks

- Do not send invoices, pay invoices, connect to payment or accounting services, update files, write memory, or schedule follow-ups.
- Do not provide tax, accounting, legal, investment, trading, lending, or collections advice.
- Do not invent amounts, due dates, customer details, payment status, or external records.

## Failure Handling

- If the user asks to pay, send, file, or update an invoice, require the appropriate connector/action path outside this pack.
- If the request needs current tax law or external financial data, require configured/approved search or qualified professional review rather than guessing.
- If invoice context is missing, ask for the notes or local document scope.

## Example Prompts

- Make an invoice checklist from my local notes.
