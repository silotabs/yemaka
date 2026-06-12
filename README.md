# Yemaka

<p align="center">
  <img src="docs/Yemaka-Logo.png" alt="Yemaka logo" width="112" />
</p>

<p align="center">
  <strong>Local-first AI agent for low-resource computers and small Ollama models.</strong>
</p>

<p align="center">
  Web app | CLI | TUI | SQLite memory/RAG | deterministic routing | approval-gated tools
</p>

Yemaka turns small local models into practical assistants by pairing Ollama
with deterministic routing, local memory, typed tools, document retrieval,
reusable skills, approval-gated automation, and replayable QA. The trusted core
stays small; capabilities grow through inspected local packs, workflows, and
extensions.

Design principle:

```text
Small local model + strong local agent system = useful low-cost AI agent
```

## Public Beta

The current public beta focuses on the surfaces that are ready for reliable
local use:

- local web app
- CLI
- TUI
- local server
- install scripts and environment checks
- macOS, Linux, and Windows install docs

Desktop/Wails app packaging is deferred for this public-beta release. The same
Go core remains shared by every surface.

## Quick Install

macOS / Linux:

```bash
scripts/install/install.sh
```

Windows:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/install/install.ps1
```

After install:

```bash
yemaka --help
yemaka doctor
yemaka serve
yemaka tui
```

The local web app binds to loopback by default:

```text
http://127.0.0.1:7727
```

## Capabilities

- Chat with local Ollama models.
- Search local memory and SQLite FTS5 RAG.
- Ingest local documents with explicit workspace grants.
- Read and search workspace files.
- Use macOS-aware workspace grants for protected local folders.
- Draft file edits, then apply only after snapshot, diff, and approval.
- Run typed local tools through policy gates and audit logs.
- Use local skills, domain packs, and workflows.
- Generate/test/register local extensions without trusting them automatically.
- Create scheduler jobs only after explicit approval.
- Monitor health through doctor, heartbeat, tool logs, replay traces, and QAReview.

## Safety Defaults

| System | Default |
|---|---|
| Cloud fallback | Disabled |
| Internet/search | Disabled |
| Connectors | Disabled |
| Embeddings/vector DB | Disabled |
| Telemetry | Disabled |
| Model downloads | Manual only |
| File writes | Snapshot + diff + approval + rollback |
| Scheduler jobs | Approval required |
| Generated extensions | Manifest + tests + policy + audit |
| Secrets | Environment/reference based, never raw config values |

## Architecture At A Glance

```text
Ollama runtime -> deterministic router -> route-level tool lane
SQLite memory -> FTS5 RAG -> bounded context compiler
Typed tools -> policy gate -> permission request -> audit log
File write -> diff preview -> snapshot -> approved apply -> rollback
Skills/domain packs/extensions -> advisory capability layer -> core policy
Replay/eval/QAReview -> generic regression suggestions -> safer routing
Scheduler/heartbeat -> local status -> notifications -> operating timeline
```

## Common Commands

```bash
yemaka doctor
yemaka model list
yemaka model set low_memory "<installed-model>"
yemaka chat "Say hello in one short sentence"
yemaka serve
yemaka tui
yemaka ingest ./docs
yemaka rag search "model routing"
yemaka workspace grant /path/to/project "Project label"
yemaka file read README.md
yemaka extension list
yemaka job list
yemaka heartbeat status
yemaka release check
```

From source:

```bash
go run ./cmd/yemaka doctor
npm --prefix frontend run build
go run ./cmd/yemaka serve
```

## Documentation

- [Install guide](docs/install.md)
- [Public beta notes](docs/public-beta.md)
- [Release checklist](docs/release.md)
- [Troubleshooting](docs/troubleshooting.md)
<!-- - [Product blueprint](docs/blueprint.md)
- [Roadmap](docs/roadmap.md)
- [Implementation status](docs/implementation-status.md) -->
- [Distribution notes](docs/distribution.md)

## License

Yemaka is released under the [MIT License](LICENSE).

## Internet search API keys

Internet search is disabled by default. When you enable a provider that needs
an API key, Yemaka stores only the environment variable name in settings, never
the raw key value.

In web settings:

- Choose the provider.
- Enter an API key environment variable name, for example
  `TAVILY_API_KEY`, `SERPER_API_KEY`, `FIRECRAWL_API_KEY`, or
  `BRAVE_SEARCH_API_KEY`.
- Do not paste the raw API key into Yemaka settings.

Set the real secret in the environment before starting Yemaka:

```bash
export TAVILY_API_KEY="your-tavily-key"
export SERPER_API_KEY="your-serper-key"
export FIRECRAWL_API_KEY="fc-your-real-key"
export BRAVE_SEARCH_API_KEY="your-brave-key"
yemaka serve
```

For a future packaged macOS desktop app launched from Finder, set the variable
through `launchctl`, then fully restart Yemaka:

```bash
launchctl setenv TAVILY_API_KEY "your-tavily-key"
launchctl setenv SERPER_API_KEY "your-serper-key"
launchctl setenv FIRECRAWL_API_KEY "fc-your-real-key"
launchctl setenv BRAVE_SEARCH_API_KEY "your-brave-key"
```

Do not create or commit a repository `.env` file for Yemaka secrets. Yemaka
currently reads process environment variables with `os.Getenv`; it does not
auto-load `.env` files from the workspace.

Provider notes:

- Auto fallback tries ready providers in this order: Tavily, Serper.dev,
  Brave, Firecrawl, Wikimedia, DuckDuckGo, then SearXNG.
- Tavily requires `TAVILY_API_KEY` by default and uses
  `https://api.tavily.com/search` unless overridden. It is a POST vendor API
  provider suited to agent/RAG search.
- Serper.dev requires `SERPER_API_KEY` by default and uses
  `https://google.serper.dev/search` unless overridden. Basic web search is
  supported first; news-specific search can be added later.
- Brave requires `BRAVE_SEARCH_API_KEY` by default. It is a freemium/free-credit
  API provider, not a pure no-key provider.
- Firecrawl requires `FIRECRAWL_API_KEY` by default and uses
  `https://api.firecrawl.dev/v2/search` unless overridden. Prefer it for
  extraction/crawl-oriented workflows rather than every routine search.
- Wikimedia works without an API key and is useful as an encyclopedia fallback,
  not a complete general web search provider.
- DuckDuckGo works without an API key and defaults to
  `https://api.duckduckgo.com/`; the endpoint can be overridden. This is the
  Instant Answer API, not a complete web SERP API.
- SearXNG remains optional for local/self-hosted search and requires a
  user-provided endpoint. Yemaka does not bundle SearXNG or require Docker.
- Mojeek requires `MOJEEK_API_KEY` by default and uses
  `https://api.mojeek.com/search` unless overridden. The key is kept out of
  logged URLs.

Ordinary `internet_fetch` and crawler-style targets remain GET/HEAD by default.
Yemaka only uses POST for named vendor API endpoints that are implemented as
search providers and still pass the internet policy gate.

Yemaka does not require pulling the blueprint default models if suitable low-end
Ollama models are already installed. Use `model list`, then point a role at an
installed model:

```bash
go run ./cmd/yemaka model providers
go run ./cmd/yemaka model show "<your-installed-1b-to-3b-model>"
go run ./cmd/yemaka model set low_memory "<your-installed-1b-to-3b-model>"
go run ./cmd/yemaka model set default "<your-installed-1b-to-3b-model>"
```

Ollama remains the default runtime. Local llama.cpp or OpenAI-compatible
servers can be selected explicitly per model role with a loopback URL:

```bash
go run ./cmd/yemaka model set coding "<local-server-model>" --provider llamacpp --base-url http://127.0.0.1:8080/v1
```

Remote/cloud providers are still off by default and are not used for model
roles.

For a no-download release-candidate smoke pass, use:

```bash
scripts/macos/smoke-rc.sh
```

By default, the smoke script creates a fresh temporary `YEMAKA_HOME` for each
run so active-profile customizations do not pollute release-default checks.

To include a real local model smoke, pass an already-installed model name:

```bash
YEMAKA_SMOKE_MODEL="<your-installed-1b-to-3b-model>" scripts/macos/smoke-rc.sh
```

## Low-resource defaults

- Ollama-first
- SQLite + FTS5 memory
- Compact local conversation summaries after the configured threshold
- Bounded prompt memory injection with `max_relevant_memories`
- Low-memory mode enabled
- SQLite FTS5 RAG
- Embeddings disabled
- Embeddings can be enabled explicitly with an already-installed Ollama embedding model
- External vector DBs are research-gated, disabled by default, and never required for SQLite RAG
- PDF/docx ingestion is available through `ingest`; extraction is local and size-limited
- Safe shell limited to an allowlist
- Safe shell/test/git execution uses a bounded helper subprocess from app surfaces
- Model-requested tools are limited to bounded validated low-risk local observations
- Skills are local, declarative, compact, and network-disabled by default
- Desktop settings cannot enable embeddings, cloud fallback, or connectors without required local configuration
- External workspace folders require explicit local grants before scan or ingest
- Local unsigned/ad-hoc signed builds remain possible; Developer ID signing is optional and documented
- Release checks fail if model artifacts or secret-like material are bundled in the app
- Cloud fallback disabled unless explicitly configured; API keys are referenced by env var name only
- Connectors disabled unless explicitly configured; `local_api` requires a token env var when enabled
- Explicit adapters stay disabled by default; `mcp_server` is local stdio only, webhook adapters are token-gated, and nothing polls in the background
- Telemetry disabled
