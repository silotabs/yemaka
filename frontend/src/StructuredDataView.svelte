<script lang="ts">
  import ActionButton from './ActionButton.svelte';
  import Badge from './Badge.svelte';
  import {
    formatStructuredData,
    type StructuredDataLanguage,
    type StructuredLine
  } from './lib/structuredDisplay';

  export let value: unknown = '';
  export let title = '';
  export let language: StructuredDataLanguage = 'auto';
  export let filename = '';
  export let maxPreviewLines = 18;
  export let defaultExpanded = false;
  export let className = '';

  let expanded = defaultExpanded;
  let rawMode = false;
  let copied = false;
  let copyTimer: ReturnType<typeof setTimeout> | null = null;
  let displayLines: StructuredLine[] = [];

  $: data = formatStructuredData(value, language, filename);
  $: displayLines = rawMode
    ? data.rawText.split(/\r?\n/).map((line) => ({ parts: [{ text: line, kind: 'plain' as const }] }))
    : data.lines;
  $: clipped = !expanded && displayLines.length > maxPreviewLines;
  $: visibleLines = clipped ? displayLines.slice(0, maxPreviewLines) : displayLines;
  $: hiddenLineCount = Math.max(0, displayLines.length - visibleLines.length);
  $: copyText = rawMode ? data.rawText : data.formattedText;
  $: viewClass = `structured-data-view ${className}`.trim();

  async function copyStructuredData() {
    if (!copyText.trim() || !navigator?.clipboard) return;
    await navigator.clipboard.writeText(copyText);
    copied = true;
    if (copyTimer) clearTimeout(copyTimer);
    copyTimer = setTimeout(() => (copied = false), 1400);
  }

  function toggleRawMode() {
    rawMode = !rawMode;
  }

  function toggleExpanded() {
    expanded = !expanded;
  }
</script>

<div class={viewClass}>
  <div class="structured-data-header">
    <div class="structured-data-title">
      {#if title}
        <span>{title}</span>
      {:else}
        <span>Structured Data</span>
      {/if}
      <Badge size="xs" variant={rawMode ? 'neutral' : 'info'}>{rawMode ? 'raw' : data.language}</Badge>
    </div>
    <div class="structured-data-actions">
      <ActionButton size="xs" variant="ghost" icon={rawMode ? 'code' : 'eye'} onclick={toggleRawMode}>
        {rawMode ? 'Formatted' : 'Raw'}
      </ActionButton>
      <ActionButton
        size="xs"
        variant="ghost"
        icon="copy"
        disabled={!copyText.trim()}
        disabledReason="There is no structured data to copy."
        onclick={copyStructuredData}
      >
        {copied ? 'Copied' : 'Copy'}
      </ActionButton>
      {#if displayLines.length > maxPreviewLines}
        <ActionButton size="xs" variant="ghost" icon={expanded ? 'eye-off' : 'eye'} onclick={toggleExpanded}>
          {expanded ? 'Less' : 'More'}
        </ActionButton>
      {/if}
    </div>
  </div>
  {#if data.empty}
    <div class="structured-data-empty">No data returned.</div>
  {:else}
    <pre class="structured-data-body" data-language={data.language}>{#each visibleLines as line, index}<span class="structured-data-line">{#each line.parts as part}<span class={`structured-token structured-token-${part.kind}`} title={part.title || undefined}>{part.text}</span>{/each}</span>{index < visibleLines.length - 1 ? '\n' : ''}{/each}</pre>
    {#if clipped}
      <div class="structured-data-footer">{hiddenLineCount} more line{hiddenLineCount === 1 ? '' : 's'} hidden for readability.</div>
    {/if}
  {/if}
</div>
