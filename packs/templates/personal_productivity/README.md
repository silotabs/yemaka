# Personal Productivity Domain Pack

Small local workflows for planning a day and capturing follow-ups. The pack uses the existing domain pack manifest format and packaged skill format.

Install:

```bash
yemaka domain-pack install packs/templates/personal_productivity
```

Install leaves the pack disabled:

```bash
yemaka domain-pack list
```

Enable only when wanted:

```bash
yemaka domain-pack enable personal_productivity
```

Safety defaults:

- disabled after install
- no internet, connectors, scheduler, shell, filesystem, notifications, or secrets permissions
- memory writes are only for confirmed user-approved follow-up notes
