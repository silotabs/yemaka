# Local File Triage Domain Pack

Read-only local workflows for inventorying a workspace, drafting cleanup plans,
and proposing folder-structure plans. The pack never moves, deletes, renames,
writes, schedules, searches the internet, or opens connectors.

Install:

```bash
yemaka domain-pack install packs/templates/local_file_triage
```

Install leaves the pack disabled:

```bash
yemaka domain-pack list
```

Enable only when wanted:

```bash
yemaka domain-pack enable local_file_triage
```

Safety defaults:

- disabled after install
- local read-only tools only
- no internet, connectors, scheduler, shell, notifications, secrets, memory writes, or filesystem writes
- cleanup and organization output is a proposed plan, not an action
