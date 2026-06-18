# Yemaka Troubleshooting

Start with diagnostics:

```bash
yemaka doctor
yemaka release check
scripts/install/doctor.sh
scripts/install/qa.sh
```

These commands report local environment, web asset, Ollama, model, permission,
and default-safety status.

`scripts/install/qa.sh` is non-destructive. Use it when install or uninstall
behavior looks suspicious; it verifies unsafe path rejection, uninstall marker
safety, disabled defaults, and the no-model-download installer contract.

## `yemaka` Command Not Found

The installer creates a command shim. If your shell cannot find it, add the
shim directory to `PATH`. On macOS and Linux, the default shim location is
`~/.local/bin/yemaka`.

macOS or Linux:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Windows:

Open a new PowerShell after install, or run:

```powershell
$env:PATH = "$env:LOCALAPPDATA\Programs\Yemaka;$env:PATH"
```

## Web App Does Not Load

Run:

```bash
yemaka doctor
yemaka serve
```

Check the doctor output for:

```text
serve_port_available
serve_static_dir
<YEMAKA_HOME>/web/dist/index.html
```

If port `7727` is busy:

```bash
yemaka serve --addr 127.0.0.1:7788
```

If the installed web bundle is missing, rebuild and reinstall:

```bash
npm --prefix frontend run build
scripts/install/install.sh --type web
```

## Ollama Or Models Are Not Ready

Yemaka does not install Ollama or pull models automatically.

Check:

```bash
ollama list
yemaka model list
yemaka model set low_memory "<installed-model>"
yemaka model set default "<installed-model>"
```

Use a small local model on low-memory machines.

## Search Or Internet Fails

This is expected until internet/search is explicitly configured. Yemaka keeps
network access disabled by default.

Inspect status:

```bash
yemaka internet status
```

Configure a provider only when you want controlled search access. API-key
providers should use environment variable names, not raw key values in config.

## File Access Or Document Ingest Fails

Grant external folders explicitly:

```bash
yemaka workspace grant /path/to/folder "Label"
yemaka workspace grants
```

On macOS, protected locations such as Documents, Downloads, Desktop, iCloud
Drive, and external volumes may require an explicit grant before Yemaka can
scan, ingest, or write through a real filesystem path.

Use the right path depending on the workflow:

| Workflow | Use |
|---|---|
| Current chat only | Chat attachment or browser-selected upload copy |
| Reusable document/RAG source | Workspace grant |
| File create/edit at a real path | Workspace grant plus approval |

If a file was attached through the browser, Yemaka receives a bounded session
copy. That is different from granting the original folder path.

File writes require approval, snapshot, diff, and rollback metadata before
Yemaka applies the change.

## Active-Profile Warnings

The active developer profile may point to custom models or optional systems.
Check packaged defaults with a clean profile:

```bash
YEMAKA_HOME=/private/tmp/yemaka-release-clean-check go run ./cmd/yemaka release check
```

The clean-profile result is the authority for public-beta defaults.

## Uninstall

macOS or Linux:

```bash
scripts/install/uninstall.sh
```

If an install receipt exists, the uninstaller prefills the recorded Yemaka home
and command shim directory. You can override the receipt location with
`YEMAKA_INSTALL_RECEIPT=/path/to/install-receipt.env`.

Keep the data directory unless you want to delete local memory, documents,
profiles, logs, generated artifacts, and config.

If `--remove-data` refuses to run, treat that as a safety stop. Yemaka only
removes a data directory when the path is a dedicated Yemaka install home and
contains `.yemaka-install-root`. Older or accidental installs inside the source
repository, Downloads, Documents, or another broad folder should be inspected
and cleaned up manually instead of removed with a recursive script.
