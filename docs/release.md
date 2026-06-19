# Public Beta Release Checklist

Use this checklist before cutting a Yemaka public-beta build. The goal is to
prove the local-first defaults, install path, web build, diagnostics, and core
runtime are safe and understandable on a clean profile.

## Release Scope

Included:

```text
yemaka --help
yemaka doctor
yemaka release check
yemaka serve
yemaka tui
scripts/install/install.sh
scripts/install/install.ps1
scripts/install/uninstall.sh
scripts/install/doctor.sh
```

Deferred:

```text
Desktop app packaging
Required code signing and notarization
Hosted cloud routing
Browser/computer-use automation
Autopilot/mission mode
Connector marketplace
```

Remove stale `.DS_Store` files from the worktree before release. Do not remove
or rewrite unrelated user changes.

## Build And Test

```bash
go test ./internal/agent -run TestDeterministicAgentRouterFoundationContract -v
go test ./...
npm --prefix frontend run build
npm --prefix frontend run test:smoke
npm --prefix frontend run test:e2e
scripts/install/qa.sh
scripts/macos/smoke-rc.sh
go run ./cmd/yemaka release check
YEMAKA_HOME=/private/tmp/yemaka-release-clean-check go run ./cmd/yemaka release check
```

If browser E2E cannot run because Playwright browser binaries are missing,
record that as a QA environment gap and install the browser dependency on the
QA machine before packaging sign-off.

## Foundation Checkpoint

Before expansion or release sign-off, confirm the deterministic routing kernel
still holds:

```text
message frame -> session state -> route candidates -> arbiter -> execution -> truth gate
```

The guard test is:

```bash
go test ./internal/agent -run TestDeterministicAgentRouterFoundationContract -v
```

This test protects the public-beta bugs that are easiest to accidentally
reintroduce: rewrite prompts stolen by route correction, completed tasks
continuing old web-search routes, `apply it` without pending operation evidence,
last-response export using operational messages instead of the final answer,
and unsupported action-success claims.

Maintainers keep this test as a release invariant. New behavior is introduced
with a focused failing case first, preserving the thin-core routing contract.
See [Routing kernel](routing-kernel.md).

## Install QA

macOS or Linux:

```bash
scripts/install/install.sh --yes --type web --home /private/tmp/yemaka-install-qa --bin-dir /private/tmp/yemaka-install-qa/bin
/private/tmp/yemaka-install-qa/bin/yemaka --help
/private/tmp/yemaka-install-qa/bin/yemaka doctor
```

Windows:

```powershell
.\scripts\install\install.ps1 -Type web
```

Confirm:

```text
Yemaka home is created
install home is a dedicated safe Yemaka path
.yemaka-install-root marker is created
config.yaml is created safely
web/dist is copied for web installs
command shim works
Ollama/model warnings are readable
no optional systems are enabled by default
scripts/install/qa.sh passes
```

## Release Warnings

Active-profile warnings can reflect local developer settings. The clean-profile
release check is the authority for packaged defaults. Do not hide warnings;
make them specific enough for a user or operator to act on them.

Expected clean defaults:

```text
internet/search disabled
cloud disabled
connectors disabled
embeddings disabled
jobs require approval
file writes require approval, snapshot, diff, rollback
telemetry disabled
```

## Installer Contract

The public-beta installer should:

```text
detect OS and architecture
prefer user-local paths
reject repo/home/root/common folders as install homes
mark the install home before any future data removal is allowed
check prerequisites
build or copy the local binary
copy web assets for web installs
create a command shim
run doctor
avoid Docker
avoid forced model downloads
avoid enabling optional systems
```

This is intentionally lighter than a desktop installer. The beta release should
be easy to verify, easy to uninstall, and safe by default.
