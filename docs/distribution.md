# Yemaka Distribution Notes

Yemaka public beta is distributed around the local web app, CLI, TUI, localhost
server, and install scripts. Desktop/Wails packaging, Developer ID signing,
DMG distribution, and notarization remain deferred for this beta.

Current release path:

```bash
scripts/install/install.sh
scripts/install/qa.sh
scripts/install/doctor.sh
scripts/install/uninstall.sh
```

Windows:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/install/install.ps1
```

Related docs:

- [Install guide](install.md)
- [Public beta notes](public-beta.md)
- [Release checklist](release.md)

## Public Beta Distribution

The beta installer should be:

```text
local-first
user-local by default
dedicated to a safe Yemaka install directory
guarded against repo/home/common-folder installs
easy to inspect
easy to uninstall
easy to rediscover from an install receipt
free of bundled model artifacts
safe without optional internet, cloud, connectors, or jobs
```

Full data removal is intentionally stricter than shim removal. The uninstall
script refuses `--remove-data` unless the target is a safe Yemaka install home
with a `.yemaka-install-root` marker.

Install scripts write a user-local `install-receipt.env` with the install home,
command shim directory, installed binary, and log path. The uninstaller reads
that receipt to prefill paths, then prompts before removing anything.

The install home must include first-party local assets:

```text
<YEMAKA_HOME>/skills/default
<YEMAKA_HOME>/packs/templates
```

Those assets let installed builds show default skills and built-in pack
templates without depending on the source checkout as the current directory.

The release check should make warnings understandable instead of hiding them.

The public-beta browser QA path is:

```bash
npm --prefix frontend run test:e2e
```

The E2E suite uses mocked local API responses for public-beta UI and routing
readiness. It does not require live internet or a running Ollama model.

## Deferred macOS Desktop Distribution

Yemaka still keeps a scripted path for future macOS desktop packaging:

```bash
scripts/macos/build-app.sh
scripts/macos/package-dmg.sh
scripts/macos/release-check.sh
scripts/macos/manual-qa.sh
```

Artifacts are written to:

```text
dist/macos/
  Yemaka-<version>-macos-<arch>.dmg
  Yemaka-<version>-macos-<arch>.dmg.sha256
  manifest.json
```

## Developer ID Signing

Set the signing identity, then sign the built app:

```bash
export DEVELOPER_ID_APPLICATION="Developer ID Application: Your Name (TEAMID)"
scripts/macos/sign-app.sh
```

The script uses Hardened Runtime with `codesign --options runtime`. The default
entitlements file is `build/darwin/entitlements.plist`, which allows local
network client access for Ollama and local APIs.

## Notarization

After signing and packaging:

```bash
export APPLE_ID="you@example.com"
export APPLE_TEAM_ID="TEAMID"
export APPLE_APP_SPECIFIC_PASSWORD="app-specific-password"
scripts/macos/notarize-dmg.sh
```

Local development does not require Apple credentials. Release check reports
signing and artifact status as pass or warning without blocking local unsigned
builds.

## Hardening Checks

`yemaka release check` validates the macOS hardening contract without requiring
paid Apple credentials:

```text
Hardened Runtime signing script uses --options runtime
signing script uses timestamping and entitlements
entitlements allow local network client access for Ollama
risky debug/JIT/dyld entitlements are not enabled
Yemaka.app metadata targets macOS 13+
no model artifacts are bundled in Yemaka.app
no secret-like files or values are bundled in Yemaka.app
cloud fallback and connectors remain disabled by default
```

The default local development path is not sandboxed. Yemaka relies on explicit
workspace grants, workspace-only file access, write confirmation, snapshots,
rollback, safe shell policy, audit logs, and disabled-by-default connectors.

## macOS Workspace Grants And Bookmarks

The current process workspace remains available. Folders outside that workspace
must be granted explicitly before scan, ingest, or write workflows use them:

```bash
yemaka workspace grant /path/to/project "Project label"
yemaka workspace grants
yemaka workspace revoke <id-or-path>
```

Grants are stored locally in the active profile:

```text
~/Library/Application Support/Yemaka/profiles/default/permissions/workspace_grants.json
```

Desktop folder-picker grants can attach macOS security-scoped bookmark metadata
when built on macOS with cgo support. CLI path grants remain useful for
non-sandbox local development, but they do not create sandbox file access by
themselves.

For public beta, the practical distinction is:

```text
workspace grant = Yemaka may work with the real local path
chat/browser attachment = Yemaka receives a bounded copy for the current session
```

Future signed or sandboxed desktop builds must preserve this distinction and
surface stale bookmark repair clearly instead of silently failing file access.

## RC Smoke Test

Use the smoke script before handing off a release candidate:

```bash
scripts/macos/smoke-rc.sh
```

The default smoke run creates a temporary `YEMAKA_HOME`, runs deterministic
local checks, validates skills, checks frontend route markers, exercises SQLite
memory, ingests docs into SQLite FTS5 RAG, runs eval with `--skip-model`, and
runs release check. It does not pull Ollama models.

To include an already-installed local model:

```bash
YEMAKA_SMOKE_MODEL="your-installed-model" scripts/macos/smoke-rc.sh
```

## Manual Release QA

Generate a manual QA report template:

```bash
scripts/macos/manual-qa.sh
```

The report is written to `dist/macos/manual-qa-<timestamp>.md`. It includes
release-check output and a human checklist for first launch, model selection,
chat, RAG, file-write approval, rollback, risky shell approval, local web, TUI,
and optional sandbox QA.

To append no-download RC smoke output:

```bash
RUN_SMOKE=1 scripts/macos/manual-qa.sh
```

## Future Risky Shell Helper

The future hardened design should keep risky shell execution in a separate
helper process with:

```text
one command per request
workspace-only working directory
bounded stdout/stderr
explicit allowlist/blocklist policy
permission record before execution
tool log after execution
```

The current foundation already routes safe shell/test/git execution through a
hidden `yemaka helper shell` subprocess from app surfaces. It is not privileged
and does not run as a background daemon.
