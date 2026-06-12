# Research Assistant Domain Pack

Small local workflows for source-backed research using memory and ingested local documents. The pack does not require internet, connectors, filesystem writes, embeddings, scheduler jobs, or cloud services.

Install:

```bash
yemaka domain-pack install packs/templates/research_assistant
```

Install leaves the pack disabled:

```bash
yemaka domain-pack list
```

Enable only when wanted:

```bash
yemaka domain-pack enable research_assistant
```

Safety defaults:

- disabled after install
- no internet, connectors, scheduler, shell, filesystem, notifications, or secrets permissions
- uses only `rag_search` and `memory_search`
- marks unsupported or conflicting claims instead of inventing missing evidence
