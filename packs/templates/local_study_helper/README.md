# Local Study Helper Domain Pack

Read-only local workflows for turning notes and ingested documents into explanations, study guides, and practice questions.

Install:

```bash
yemaka domain-pack install-template local_study_helper
```

Install leaves the pack disabled:

```bash
yemaka domain-pack list
```

Enable only when wanted:

```bash
yemaka domain-pack enable local_study_helper
```

Safety defaults:

- disabled after install
- uses local documents, memory, and user-provided notes only
- no internet, connectors, scheduler, shell, filesystem writes, notifications, secrets, or memory writes
