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

## Choosing An Install Home

The default macOS install home is `~/Library/Application Support/Yemaka`
because it is the standard user-local place for app support files: config,
SQLite memory, logs, profiles, bundled local assets, generated artifacts, and
the install marker. It does not require administrator privileges, keeps Yemaka
data out of visible folders such as Downloads/Documents/Desktop, and keeps
uninstall safer because everything lives under one dedicated app directory.

`~/.yemaka` is acceptable for CLI-only or custom Unix-style installs, but it is
more developer-oriented and still hidden. `~/yemaka` is easier to see, but it
clutters the home folder and is easier to move or delete by accident. For most
macOS users, keep the default. For managed or advanced installs, pass an
explicit home:

```bash
scripts/install/install.sh --home "$HOME/.yemaka"
scripts/install/install.sh --home "$HOME/yemaka"
```

Whichever path you choose, use a dedicated directory whose name includes
`yemaka`; the installer rejects broad folders for safety.

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

The installer shows protected defaults as a summary, not as a choice to
override. Internet/search, cloud fallback, connectors, embeddings/vector DB,
background jobs, and model downloads remain off during install. Enable optional
systems later from Settings or the `yemaka` CLI.

Choosing a model name only records that already-installed model in config.
Yemaka never pulls models during install.

When installing from source, Yemaka builds the local app binary on your machine.
The first build can take a minute because Go may compile the SQLite driver and
other local runtime packages. The installer shows progress and writes details
to the install log.

The installer also writes a user-local install receipt. The receipt records the
Yemaka home, command shim directory, installed binary, install type, and latest
install log so later uninstall runs can prefill the correct paths.

Built-in default skills are copied to `<YEMAKA_HOME>/skills/default`, so
`yemaka skill list` works after install even when you run Yemaka outside the
source checkout. Domain-pack templates are copied separately and remain inactive
until you explicitly install and enable a pack.

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

It also writes an install receipt outside the source checkout:

| Platform | Receipt |
|---|---|
| macOS | `~/Library/Application Support/Yemaka/Installer/install-receipt.env` |
| Linux | `${XDG_STATE_HOME:-~/.local/state}/yemaka/install-receipt.env` |
| Windows | `%LOCALAPPDATA%\Yemaka\install-receipt.env` |

Set `YEMAKA_INSTALL_RECEIPT=/path/to/install-receipt.env` when testing or when
you need the receipt in a managed location.

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

The macOS/Linux installer defaults the `yemaka` command shim to
`~/.local/bin/yemaka`. You can choose another command directory with
`--bin-dir`, but the user-local default avoids Homebrew or system-wide
locations.

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
safety, install receipt discovery, installed default-skill and template
discovery outside the source checkout, Windows installer safety markers, and
the no-model-download / disabled-by-default guarantees.

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

If an install receipt is present, the uninstaller uses it to prefill the Yemaka
home and command shim directory. It still asks for confirmation before removing
the command shim, binary, or local data.

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
