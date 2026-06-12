#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT_DIR"

: "${YEMAKA_HOME:=/private/tmp/yemaka-manual-qa}"
: "${GOCACHE:=/private/tmp/yemaka-go-build-cache}"
: "${DIST_DIR:=dist/macos}"
: "${RUN_SMOKE:=0}"

export YEMAKA_HOME
export GOCACHE

mkdir -p "$DIST_DIR"
STAMP=$(date -u +"%Y%m%dT%H%M%SZ")
REPORT="$DIST_DIR/manual-qa-$STAMP.md"

{
  printf '%s\n\n' '# Yemaka Manual Release QA'
  printf '%s\n' "- Date UTC: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  printf '%s%s%s\n' '- YEMAKA_HOME: `' "$YEMAKA_HOME" '`'
  printf '%s%s%s\n' '- GOCACHE: `' "$GOCACHE" '`'
  printf '%s\n\n' '- Commit: manual workspace'

  printf '## Automated Preflight\n\n'
  printf '```text\n'
  go run ./cmd/yemaka release check || true
  printf '```\n\n'

  printf '## Manual Checklist\n\n'
  printf '%s\n' '- [ ] Launch `build/bin/Yemaka.app` on macOS 13+.'
  printf '%s\n' '- [ ] Confirm first-run state does not pull or bundle models.'
  printf '%s\n' '- [ ] Confirm cloud fallback is disabled by default.'
  printf '%s\n' '- [ ] Confirm connectors are disabled by default.'
  printf '%s\n' '- [ ] Confirm model selector lists only installed Ollama models.'
  printf '%s\n' '- [ ] Confirm external workspace scan/ingest requires an explicit workspace grant.'
  printf '%s\n' '- [ ] Configure an already-installed low-end model if needed.'
  printf '%s\n' '- [ ] Send one short chat request and confirm streaming works.'
  printf '%s\n' '- [ ] Ingest `docs/` and confirm RAG search shows local sources.'
  printf '%s\n' '- [ ] Create a file-write preview and confirm no write happens before approval.'
  printf '%s\n' '- [ ] Approve one safe write in a temporary workspace and confirm a snapshot is created.'
  printf '%s\n' '- [ ] Roll back the write and confirm original content is restored.'
  printf '%s\n' '- [ ] Try a risky shell command and confirm approval is required or command is blocked.'
  printf '%s\n' '- [ ] Confirm Memory, Documents, Skills, Tool Logs, and Settings screens render.'
  printf '%s\n' '- [ ] Confirm local web binds to loopback only if explicitly started.'
  printf '%s\n' '- [ ] Confirm TUI starts and exits cleanly.'
  printf '%s\n' '- [ ] Confirm packaged DMG, checksum, and manifest are present when packaging is run.'
  printf '%s\n\n' '- [ ] Record any warnings from `release check` and decide whether they block this RC.'

  printf '## Optional Sandboxed Build QA\n\n'
  printf '%s\n' '- [ ] Build normally first and confirm the non-sandbox local development path still works.'
  printf '%s\n' '- [ ] Sign a sandbox test build with `SANDBOX=1 scripts/macos/sign-app.sh` or `ENTITLEMENTS=build/darwin/entitlements.sandbox.plist scripts/macos/sign-app.sh`.'
  printf '%s\n' '- [ ] Launch the sandboxed app and grant an external folder through the desktop folder picker.'
  printf '%s\n' '- [ ] Confirm the grant shows bookmark-backed access in the Documents screen.'
  printf '%s\n' '- [ ] Ingest/search the granted external folder and confirm access succeeds without broad filesystem permissions.'
  printf '%s\n' '- [ ] Revoke the grant and confirm external access is blocked again.'
  printf '%s\n\n' '- [ ] If a bookmark is reported stale, re-grant the folder and confirm the prompt is clear.'

  printf '## Notes\n\n'
  printf '%s\n' '- No model pull is part of this QA script.'
  printf '%s\n' '- Real model smoke is optional and must use `YEMAKA_SMOKE_MODEL=<already-installed-model>`.'
  printf '%s\n' '- Apple notarization remains optional for local unsigned development builds.'
} > "$REPORT"

if [ "$RUN_SMOKE" = "1" ]; then
  {
    printf '\n## No-Download Smoke Output\n\n'
    printf '```text\n'
  } >> "$REPORT"
  smoke_status=0
  scripts/macos/smoke-rc.sh >> "$REPORT" 2>&1 || smoke_status=$?
  printf '```\n' >> "$REPORT"
  if [ "$smoke_status" -ne 0 ]; then
    printf 'Manual QA report: %s\n' "$REPORT"
    exit "$smoke_status"
  fi
fi

printf 'Manual QA report: %s\n' "$REPORT"
