# Local Meeting Notes Domain Pack

Read-only local workflows for turning pasted notes, memory, or ingested local documents into meeting summaries and action-item drafts.

Install:

```bash
yemaka domain-pack install-template local_meeting_notes
```

Install leaves the pack disabled:

```bash
yemaka domain-pack list
```

Enable only when wanted:

```bash
yemaka domain-pack enable local_meeting_notes
```

Safety defaults:

- disabled after install
- no internet, connectors, scheduler, shell, filesystem writes, notifications, or secrets permissions
- no memory writes; suggested follow-ups are presented as drafts for the user to approve elsewhere
