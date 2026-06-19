# Yemaka Public Beta

## Release Summary

Yemaka is a local-first AI agent for low-resource computers and small Ollama
models. This beta includes the local web app, CLI, TUI, localhost server,
install scripts, diagnostics, SQLite memory, SQLite FTS5 RAG, approval-gated
tools, skills, domain packs, generated extensions, scheduler jobs, heartbeat
checks, replay/QA review, and safe local-first defaults.

Protected defaults:

- Internet/search disabled
- Cloud fallback disabled
- Connectors disabled
- Embeddings/vector DB disabled
- Telemetry disabled
- Model downloads manual only
- File writes require snapshot, diff, confirmation, and rollback
- Scheduler jobs require explicit approval

Current limits:

- Ollama and models are installed separately
- Desktop packaging/signing/notarization are deferred
- Autopilot, browser automation, voice, and capability marketplace are future
  work

## Details

Yemaka public beta is a local-first release for users who want a practical
assistant built around small local models, SQLite memory, document retrieval,
typed tools, and explicit approval gates.

This beta is focused on reliability over breadth. It ships the local web app,
CLI, TUI, localhost server, install scripts, diagnostics, and release checks.
Desktop/Wails packaging is deferred; the core runtime remains shared by every
surface.

## Included Surfaces

| Surface | Status |
|---|---|
| Local web app | Included |
| CLI | Included |
| TUI | Included |
| Local server | Included |
| Install scripts | Included |
| Environment checks | Included |
| Desktop app packaging | Deferred |

## Included Capabilities

- Local chat with installed Ollama models.
- Response Mode settings for Auto, Fast, Balanced, and Deep local model
  behavior; raw model thinking remains hidden by default.
- Settings retain an unsaved draft during background health refreshes, with
  explicit restart and shutdown controls for local runtime changes.
- SQLite memory and SQLite FTS5 RAG.
- Local document ingestion with explicit workspace grants.
- Approval-gated file writes with snapshot, diff, and rollback.
- Skills, domain packs, workflows, and generated extensions.
- Scheduler jobs that require approval before creation and enablement.
- Route recovery, context compaction, replay traces, QAReview, and
  product-grade failure messages.
- Deterministic routing-kernel regression coverage for rewrite/editorial
  prompts, route-correction teaching, continuation, pending approval/apply,
  last-response artifact actions, local-action evidence, and truth-gated action
  claims.
- Disabled-by-default internet/search, connectors, embeddings, cloud fallback,
  crawler influence, and generated connector runtime.

## Protected Defaults

Clean-profile defaults must remain:

```text
internet/search: disabled
cloud fallback: disabled
connectors: disabled
embeddings: disabled
knowledge graph influence: disabled unless explicitly enabled
generated connector runtime: gated
raw model thinking trace: hidden by default
model downloads: manual only
background loops: not auto-started
```

Scheduler support may exist in config, but jobs do not run until a user creates,
approves, and enables them.

Route Recovery v1 and Context Compaction v1 are supporting systems. They help
Yemaka explain failures, preserve task state, and keep small local models on
track, but they are not new routing authorities and cannot bypass policy,
approvals, tool lanes, or the selected final route.

Knowledge graph records and domain packs are advisory only. They may contribute
reviewed context or capability hints, but they cannot silently override routing
or enable optional systems.

## Routing Foundation

Yemaka's routing foundation is deterministic and state-first:

```text
message frame -> session state -> route candidates -> arbiter -> execution -> truth gate
```

Route correction, continuation, domain packs, skills, source selection, file
actions, and tool lanes contribute typed signals or candidates to the same
arbitration path. The main UI labels executor activity as Local action, Approval
required, File change prepared, or Result check, while backend tool-run and
replay evidence remains available through Inspect and diagnostics.

The engineering details and regression coverage are documented in
[Routing kernel](routing-kernel.md).

## Release Gate

Before packaging a public beta build:

```bash
go test ./...
npm --prefix frontend run build
npm --prefix frontend run test:smoke
npm --prefix frontend run test:e2e
scripts/install/qa.sh
scripts/macos/smoke-rc.sh
go run ./cmd/yemaka release check
YEMAKA_HOME=/private/tmp/yemaka-release-clean-check go run ./cmd/yemaka release check
```

If browser E2E cannot run because Playwright browsers are missing on the test
machine, record it as a QA environment gap. Do not hide it, and do not let it
obscure clean-profile release readiness.

## Current Limits

- Ollama and models are installed separately.
- Internet/search requires explicit provider configuration.
- Connectors remain disabled until configured with token references.
- Desktop packaging, signing, and notarization are not part of this beta.
- Autopilot/mission mode is future work.
- Browser/computer-use automation is future work.
- Scheduled crawler jobs are future work.
- Knowledge graph records are advisory and cannot override routing or policy.

## First-Line Support

Use:

```bash
yemaka doctor
yemaka release check
scripts/install/doctor.sh
```

Related docs:

- [Install guide](install.md)
- [Routing kernel](routing-kernel.md)
- [Release checklist](release.md)
- [Troubleshooting](troubleshooting.md)
