# Yemaka Routing Kernel

Yemaka routing is deterministic, thin-core, and state-first. This page is the
engineering reference for the routing foundation used by chat, tools, local
actions, replay, and expansion work.

## Turn Contract

Every chat turn uses the existing kernel shape:

```text
raw user input
message frame
session state
route candidates
single arbiter
execution
verification/truth gate
final response
```

Route-like layers produce typed signals or candidates instead of final
responses. Route correction, continuation, domain packs, skills, source
selection, file actions, and tool lanes feed the same arbiter. The arbiter
selects one route, then execution follows policy.

Allowed pre-routing exceptions are limited to validated pending
approval/rejection/resume flows that already have stored session and permission
evidence.

## State Beats Text

Short prompts such as `yes`, `apply it`, `continue`, `save that`, and
`did you create it?` resolve from stored state and trusted evidence before they
are interpreted as new tasks.

Important state sources:

- `SessionContract`
- pending operation fields
- stored permission requests and permission results
- `tool_runs`
- replay traces
- `LastFinalMessageID`

When state is missing, stale, or ambiguous, Yemaka asks one clarification or
reports that no pending operation exists. The router does not invent a new file
write, approval, continuation, or success claim.

## Final Answers And Local Actions

Final assistant answers are conversation artifacts. They may be reused for
requests such as `save your last response as test.md`.

Local action messages are operational evidence. They are stored as structured
tool runs, replay traces, permission records, and, when user-visible,
operational assistant messages. They are separate from the last final answer
artifact unless they are genuinely the final answer.

For `last response` exports, use `LastFinalMessageID` first. Operational
messages such as approvals, status updates, failed tool notes, and executor
activity are not replacement final-answer artifacts.

## User-Facing Labels

The backend may preserve raw actor/tool names for Inspect, replay, and audits.
The main UI uses product language:

```text
yemaka-executor -> Local action
executor/tool activity -> Activity
permission required -> Approval required
edit proposal -> File change prepared
verification -> Result check
```

Raw evidence remains available in Inspect/debug surfaces. The result is a
friendlier main UI with the same trusted backend evidence.

## Guard Test

The routing foundation is guarded by:

```bash
go test ./internal/agent -run TestDeterministicAgentRouterFoundationContract -v
```

The test covers the bug-prone product scenarios from public-beta hardening:

- rewrite/editorial prompts do not trigger route correction
- explicit reusable route teaching still routes to learning
- incomplete `continue` resumes the old route
- completed web-task `continue` clarifies instead of reopening old web search
- local/RAG study sessions with selected sources may continue safely
- social and standalone prompts after completed work do not get hijacked
- `save last response` uses the final-answer artifact path
- `apply it` requires a valid pending operation and stored permission evidence
- `yes` without pending approval stays ordinary chat
- `LastFinalMessageID` beats operational messages
- `tool_runs` rebuild local action activity
- unsupported action success claims are downgraded unless trusted evidence exists

Maintainers run this guard before expansion work that touches routing, session
state, tool evidence, replay, file-write approval, local-action UI, or context
compilation.

## Expansion Rule

Expansion keeps smartness in the thin-core shape: deterministic routing,
bounded context compilation, typed tools, evidence, replay, evals, and generated
capabilities outside protected policy. Heavy LLM routers, vector routers, global
dedupe, and broad prompt-handler rewrites are outside the default foundation.
Generic primitives belong in core only when they serve many capabilities.
Domain-specific behavior belongs in skills, generated extensions, workflows,
connectors, or domain packs outside the trusted core.
