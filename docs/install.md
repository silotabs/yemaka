# Yemaka Install Guide

Yemaka public beta ships as a local web app, CLI, TUI, and localhost server.
The installer prepares a local Yemaka home, copies the web bundle when needed,
and creates a command shim. It does not download models, enable cloud, start
background jobs, or turn on internet access.

## Supported Platforms

| Platform | Status | Default Yemaka Home |
|---|---|---|
| macOS | Primary | `~/Library/Application Support/Yemaka` |
| Linux | Supported | `~/.local/share/yemaka` |
| Windows | Supported install script | `%APPDATA%\Yemaka` |

Desktop/Wails packaging is deferred for this public-beta release. The same Go
core is available through local web, CLI, and TUI.

## Quick Install

macOS or Linux:

```bash
scripts/install/install.sh
```

Windows PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/install/install.ps1
```

After installation:

```bash
yemaka --help
yemaka doctor
yemaka serve
yemaka tui
```

The web app runs on loopback by default:

```text
http://127.0.0.1:7727
```

## Installer Choices

The installer may ask for:

- install type: web + CLI/TUI, or CLI/TUI only
- Yemaka home directory
- command shim directory
- optional model role names
- low-memory guidance
- confirmation that optional systems remain disabled

Choosing a model name only records that already-installed model in config.
Yemaka never pulls models during install.

## Install Path Safety

Yemaka must be installed into a dedicated Yemaka directory. The installer
canonicalizes the chosen paths and refuses unsafe targets such as the source
repository, your home directory, common user folders like Downloads/Documents,
root directories, and generic directories whose name does not include
`yemaka`.

Safe examples:

```text
~/Library/Application Support/Yemaka
~/.local/share/yemaka
/private/tmp/yemaka-install-qa
```

Unsafe examples:

```text
.
..
users/name
~/Downloads
~/Documents
~
/
```

The installer writes a `.yemaka-install-root` marker inside the install home.
The uninstall script requires that marker before it will remove the data
directory.

## Platform Notes

macOS and Linux use:

```bash
scripts/install/install.sh
```

Useful QA install:

```bash
scripts/install/install.sh --yes --type web
```

If `~/.local/bin` is not in your shell `PATH`, add:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Windows creates a command shim at:

```text
%LOCALAPPDATA%\Programs\Yemaka\yemaka.cmd
```

Open a new PowerShell after install so the updated user `PATH` is available.

## macOS Workspace Grants

macOS protects folders such as Documents, Downloads, Desktop, iCloud Drive, and
external volumes. Yemaka treats access to those locations as an explicit user
grant, not as a hidden background permission.

Grant a folder before scan, ingest, or file-write workflows use it:

```bash
yemaka workspace grant /Users/you/Documents "Documents"
yemaka workspace grants
```

In the local web app, use the Documents page or chat attachment flow when you
want to provide a browser-selected copy for the current session. Use a workspace
grant when you want Yemaka to work with the real filesystem path.

Future packaged desktop builds can use macOS folder-picker grants with
security-scoped bookmark metadata. CLI grants are path-based and remain useful
for local development and non-sandboxed beta builds.

## Safe Defaults

The installer preserves Yemaka's local-first defaults:

| System | Install Default |
|---|---|
| Cloud fallback | Disabled |
| Internet/search | Disabled |
| Connectors | Disabled |
| Embeddings | Disabled |
| Model downloads | Manual only |
| Background jobs | Not auto-started |
| File writes | Approval, snapshot, diff, rollback |

## Ollama And Models

Install and start Ollama separately before first chat.

Recommended local model sizing:

| Machine | Guidance |
|---|---|
| 4 GB RAM | CLI/TUI low-memory mode with a small 1B-2B quantized model |
| 8 GB RAM | Web app plus a small local model |
| 16 GB+ RAM | Comfortable for stronger 4B local models |

Yemaka can point roles at models that are already installed:

```bash
yemaka model list
yemaka model set low_memory "<installed-model>"
yemaka model set default "<installed-model>"
```

## Verify Installation

Run:

```bash
yemaka doctor
scripts/install/doctor.sh
scripts/install/qa.sh
```

`doctor.sh` reports OS and architecture, RAM, disk, `PATH`, `YEMAKA_HOME`,
config, installed web assets, port availability, Ollama readiness, selected
model readiness, search-provider configuration, workspace permissions, and
local-first defaults.

`qa.sh` is a non-destructive installer contract check. It verifies help output,
unsafe install-home rejection, unsafe command-shim rejection, uninstall marker
safety, Windows installer safety markers, and the no-model-download /
disabled-by-default guarantees.

## Web Asset Location

For web installs, `yemaka serve` uses:

```text
<YEMAKA_HOME>/web/dist
```

You can override the static bundle explicitly:

```bash
yemaka serve --static /path/to/frontend/dist
```

## Uninstall

macOS or Linux:

```bash
scripts/install/uninstall.sh
```

To remove local memory, profiles, generated artifacts, logs, web assets, and
config, pass `--remove-data`. Data removal is refused unless the target is a
safe Yemaka install home with a `.yemaka-install-root` marker:

```bash
scripts/install/uninstall.sh --remove-data
```

Windows currently removes the shim and data directory manually:

```powershell
Remove-Item "$env:LOCALAPPDATA\Programs\Yemaka\yemaka.cmd"
Remove-Item "$env:APPDATA\Yemaka" -Recurse
```

Only remove the data directory if you want to delete local memory, profiles,
documents, generated artifacts, logs, and config.
