<script lang="ts">
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';

  export let content = '';
  export let streaming = false;

  type RenderedMarkdown = {
    html: string;
    codeBlocks: string[];
  };

  type CalloutMatch = {
    label: string;
    body: string;
  };

  let copiedCodeIndex: number | null = null;
  let copyResetTimer: ReturnType<typeof setTimeout> | undefined;
  let markdownRoot: HTMLDivElement | null = null;
  let markdownMounted = false;
  const calloutLabels = new Set([
    'note',
    'source',
    'warning',
    'important',
    'tip',
    'result',
    'verdict',
    'why',
    'next step',
    'action needed',
    'missing details',
    'setup path',
    'fallback',
    'status note'
  ]);

  function clearCopyResetTimer() {
    if (!copyResetTimer) return;
    clearTimeout(copyResetTimer);
    copyResetTimer = undefined;
  }

  function escapeHTML(value: string) {
    return String(value || '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function escapeAttr(value: string) {
    return escapeHTML(value).replace(/`/g, '&#96;');
  }

  function slugClass(value: string) {
    return String(value || 'note')
      .toLowerCase()
      .replace(/[^a-z0-9_-]+/g, '-')
      .replace(/^-+|-+$/g, '')
      .slice(0, 24);
  }

  function cleanLabel(value: string) {
    return String(value || '')
      .trim()
      .replace(/^[#>*\s_-]+/, '')
      .replace(/[*_`]+/g, '')
      .replace(/\s+/g, ' ')
      .trim();
  }

  function trailingURLPunctuation(value: string) {
    let url = String(value || '');
    let suffix = '';

    while (/[.,!?;:]$/.test(url)) {
      suffix = `${url.slice(-1)}${suffix}`;
      url = url.slice(0, -1);
    }

    for (const [open, close] of [['(', ')'], ['[', ']']] as const) {
      while (url.endsWith(close)) {
        const opens = (url.match(new RegExp(`\\${open}`, 'g')) || []).length;
        const closes = (url.match(new RegExp(`\\${close}`, 'g')) || []).length;
        if (closes <= opens) break;
        suffix = `${close}${suffix}`;
        url = url.slice(0, -1);
      }
    }

    return { url, suffix };
  }

  function anchorHTML(href: string, label = href) {
    return `<a href="${escapeAttr(href)}" target="_blank" rel="noopener noreferrer">${escapeHTML(label)}</a>`;
  }

  function renderInline(value: string) {
    const placeholders: string[] = [];
    let raw = String(value || '');

    raw = raw.replace(/`([^`]+)`/g, (_match, code) => {
      const index = placeholders.push(`<code>${escapeHTML(code)}</code>`) - 1;
      return `\u0000HTML${index}\u0000`;
    });

    raw = raw.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+|mailto:[^\s)]+)\)/g, (_match, label, href) => {
      const index = placeholders.push(anchorHTML(href, label)) - 1;
      return `\u0000HTML${index}\u0000`;
    });

    raw = raw.replace(/(^|[\s([{])((?:https?:\/\/|mailto:)[^\s<>"']+)/g, (_match, prefix, candidate) => {
      const { url, suffix } = trailingURLPunctuation(candidate);
      if (!url) return `${prefix}${candidate}`;
      const index = placeholders.push(anchorHTML(url)) - 1;
      return `${prefix}\u0000HTML${index}\u0000${suffix}`;
    });

    let escaped = escapeHTML(raw);
    escaped = escaped.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    escaped = escaped.replace(/__([^_]+)__/g, '<strong>$1</strong>');
    escaped = escaped.replace(/(^|[\s(])\*([^*\n]+)\*/g, '$1<em>$2</em>');
    escaped = escaped.replace(/(^|[\s(])_([^_\n]+)_/g, '$1<em>$2</em>');
    return escaped.replace(/\u0000HTML(\d+)\u0000/g, (_match, index) => placeholders[Number(index)] ?? '');
  }

  function isFence(line: string) {
    return line.trim().startsWith('```');
  }

  function codeLanguage(line: string) {
    return line
      .trim()
      .replace(/^```/, '')
      .trim()
      .replace(/[^\w.+#-]/g, '')
      .slice(0, 32);
  }

  function splitTableRow(line: string) {
    return line
      .trim()
      .replace(/^\|/, '')
      .replace(/\|$/, '')
      .split('|')
      .map((cell) => cell.trim());
  }

  function isTableSeparator(line: string) {
    const cells = splitTableRow(line);
    return cells.length > 1 && cells.every((cell) => /^:?-{3,}:?$/.test(cell.trim()));
  }

  function isListLine(line: string) {
    return /^(\s*)([-*+]|\d+[.)])\s+/.test(line);
  }

  function listLineParts(line: string) {
    const match = /^(\s*)([-*+]|\d+[.)])\s+(.+)$/.exec(line);
    if (!match) return null;
    const raw = match[3].trim();
    const task = /^\[( |x|X|-)\]\s+(.+)$/.exec(raw);
    return {
      marker: match[2],
      ordered: /^\d+[.)]$/.test(match[2]),
      taskState: task ? task[1].toLowerCase() : '',
      text: task ? task[2] : raw
    };
  }

  function stepMatch(line: string) {
    return /^\s*(step|phase|milestone)\s+(\d+|[ivxlcdm]+)\s*[:.)-]\s+(.+)$/i.exec(line);
  }

  function isStepLine(line: string) {
    return Boolean(stepMatch(line));
  }

  function calloutMatch(line: string) {
    const match = /^(?:[-*]\s+)?(?:📌\s*)?(?:\*\*)?([a-z][a-z ]+)(?:\*\*)?\s*:\s+(.+)$/i.exec(String(line || '').trim());
    if (!match) return null;
    const label = cleanLabel(match[1]).toLowerCase();
    if (!calloutLabels.has(label)) return null;
    return { label, body: match[2] } satisfies CalloutMatch;
  }

  function isSetupBlockStart(line: string) {
    return /^\s*scheduler_setup\s*:\s*$/i.test(line) || /^\s*scheduler_setup\s*:\s*\S+/i.test(line);
  }

  function isSetupBlockLine(line: string) {
    return (
      isSetupBlockStart(line) ||
      /^\s*(target_type|target_name|schedule_type|schedule_expr|task|url|input|enabled|approved)\s*[:=]\s*.+/i.test(line) ||
      /^\s*["']?(target_type|target_name|schedule_type|schedule_expr|task|url|input|enabled|approved)["']?\s*[:=]\s*.+/i.test(line)
    );
  }

  function isBlockStart(lines: string[], index: number) {
    const line = lines[index] || '';
    return (
      line.trim() === '' ||
      isFence(line) ||
      isStepLine(line) ||
      isListLine(line) ||
      /^#{1,6}\s+/.test(line) ||
      /^>\s?/.test(line) ||
      /^---+$/.test(line.trim()) ||
      Boolean(calloutMatch(line)) ||
      isSetupBlockStart(line) ||
      (index + 1 < lines.length && line.includes('|') && isTableSeparator(lines[index + 1]))
    );
  }

  function codeBlockKind(code: string, language = '') {
    const trimmed = String(code || '').trim();
    const lower = trimmed.toLowerCase();
    const lang = String(language || '').toLowerCase();

    if (lower.startsWith('scheduler_setup:') || (lower.includes('target_type:') && lower.includes('schedule_'))) {
      return { label: 'Scheduler Setup', className: 'scheduler' };
    }
    if (trimmed.startsWith('{') || trimmed.startsWith('[') || lang === 'json') {
      return { label: 'JSON', className: 'json' };
    }
    if (/^(?:\$?\s*)?yemaka\s+\S+/m.test(trimmed)) {
      return { label: 'Command', className: 'command' };
    }
    if (lang === 'yaml' || lang === 'yml') {
      return { label: 'YAML', className: 'yaml' };
    }
    if (language) {
      return { label: language, className: slugClass(language) };
    }
    return { label: 'Text', className: 'text' };
  }

  function renderCodeBlock(code: string, language: string, codeIndex: number, copied: boolean) {
    const kind = codeBlockKind(code, language);
    return `<div class="md-code-block md-code-block-${slugClass(kind.className)}"><div class="md-code-toolbar"><span>${escapeHTML(kind.label)}</span><button type="button" class="md-code-copy ${copied ? 'is-copied' : ''}" data-copy-code="${codeIndex}">${copied ? 'Copied' : 'Copy'}</button></div><pre><code>${escapeHTML(code)}</code></pre></div>`;
  }

  function renderTable(lines: string[], start: number) {
    const headers = splitTableRow(lines[start]);
    const separator = splitTableRow(lines[start + 1]);
    let index = start + 2;
    const rows: string[][] = [];

    while (index < lines.length && lines[index].includes('|') && lines[index].trim() !== '') {
      rows.push(splitTableRow(lines[index]));
      index++;
    }

    const alignments = separator.map((cell) => {
      const value = cell.trim();
      if (value.startsWith(':') && value.endsWith(':')) return 'center';
      if (value.endsWith(':')) return 'right';
      return 'left';
    });

    const head = headers
      .map((cell, column) => `<th class="md-align-${alignments[column] || 'left'}">${renderInline(cell)}</th>`)
      .join('');
    const body = rows
      .map((row) => {
        const cells = headers.map((_header, column) => `<td class="md-align-${alignments[column] || 'left'}">${renderInline(row[column] || '')}</td>`);
        return `<tr>${cells.join('')}</tr>`;
      })
      .join('');

    return {
      html: `<div class="md-table-wrap"><table><thead><tr>${head}</tr></thead><tbody>${body}</tbody></table></div>`,
      next: index
    };
  }

  function renderList(lines: string[], start: number) {
    const first = listLineParts(lines[start]);
    const ordered = Boolean(first?.ordered);
    const tag = ordered ? 'ol' : 'ul';
    let index = start;
    const items: string[] = [];
    let taskList = false;

    while (index < lines.length && isListLine(lines[index])) {
      const parts = listLineParts(lines[index]);
      if (!parts || parts.ordered !== ordered) break;

      let text = parts.text;
      let next = index + 1;
      while (next < lines.length && lines[next].trim() !== '' && !isBlockStart(lines, next) && /^\s{2,}\S/.test(lines[next])) {
        text += ` ${lines[next].trim()}`;
        next++;
      }

      if (parts.taskState) {
        taskList = true;
        const checked = parts.taskState === 'x';
        const mixed = parts.taskState === '-';
        items.push(
          `<li class="${checked ? 'md-task-checked' : mixed ? 'md-task-mixed' : ''}"><span class="md-task-box" aria-hidden="true"></span><span>${renderInline(text)}</span></li>`
        );
      } else {
        items.push(`<li>${renderInline(text)}</li>`);
      }
      index = next;
    }

    return {
      html: `<${tag} class="${taskList ? 'md-task-list' : ''}">${items.join('')}</${tag}>`,
      next: index
    };
  }

  function renderStepList(lines: string[], start: number) {
    let index = start;
    const items: string[] = [];

    while (index < lines.length && isStepLine(lines[index])) {
      const match = stepMatch(lines[index]);
      if (!match) break;
      const label = `${match[1][0].toUpperCase()}${match[1].slice(1).toLowerCase()} ${match[2].toUpperCase()}`;
      items.push(`<li><span class="md-step-marker">${escapeHTML(label)}</span><div>${renderInline(match[3].trim())}</div></li>`);
      index++;
    }

    return {
      html: `<ol class="md-step-list">${items.join('')}</ol>`,
      next: index
    };
  }

  function renderCallout(lines: string[], start: number) {
    const first = calloutMatch(lines[start]);
    if (!first) return { html: '', next: start + 1 };

    const label = first.label;
    const parts = [first.body.trim()];
    let index = start + 1;

    while (index < lines.length && lines[index].trim() !== '' && !isBlockStart(lines, index)) {
      parts.push(lines[index].trim());
      index++;
    }

    return {
      html: `<aside class="md-callout md-callout-${slugClass(label)}"><span>${escapeHTML(label)}</span><div>${renderInline(parts.join(' '))}</div></aside>`,
      next: index
    };
  }

  function renderParagraph(lines: string[], start: number) {
    const parts: string[] = [];
    let index = start;

    while (index < lines.length && !isBlockStart(lines, index)) {
      parts.push(lines[index].trim());
      index++;
    }

    const body = renderInline(parts.filter(Boolean).join(' '));

    return {
      html: `<p>${body}</p>`,
      next: index
    };
  }

  function renderMarkdown(value: string, copiedIndex: number | null): RenderedMarkdown {
    const lines = String(value || '').replace(/\r\n/g, '\n').split('\n');
    const html: string[] = [];
    const codeBlocks: string[] = [];
    let index = 0;

    while (index < lines.length) {
      const line = lines[index];
      const trimmed = line.trim();

      if (trimmed === '') {
        index++;
        continue;
      }

      if (isFence(line)) {
        const language = codeLanguage(line);
        index++;
        const codeLines: string[] = [];
        while (index < lines.length && !isFence(lines[index])) {
          codeLines.push(lines[index]);
          index++;
        }
        if (index < lines.length) index++;

        const code = codeLines.join('\n');
        const codeIndex = codeBlocks.push(code) - 1;
        html.push(renderCodeBlock(code, language, codeIndex, copiedIndex === codeIndex));
        continue;
      }

      if (isSetupBlockStart(line)) {
        const codeLines: string[] = [];
        while (index < lines.length && lines[index].trim() !== '' && isSetupBlockLine(lines[index])) {
          codeLines.push(lines[index].trim());
          index++;
        }
        const code = codeLines.join('\n');
        const codeIndex = codeBlocks.push(code) - 1;
        html.push(renderCodeBlock(code, 'scheduler', codeIndex, copiedIndex === codeIndex));
        continue;
      }

      if (index + 1 < lines.length && line.includes('|') && isTableSeparator(lines[index + 1])) {
        const table = renderTable(lines, index);
        html.push(table.html);
        index = table.next;
        continue;
      }

      const heading = /^(#{1,6})\s+(.+)$/.exec(line);
      if (heading) {
        const level = Math.min(heading[1].length + 1, 4);
        html.push(`<h${level}>${renderInline(heading[2].trim())}</h${level}>`);
        index++;
        continue;
      }

      if (/^>\s?/.test(line)) {
        const quoteLines: string[] = [];
        while (index < lines.length && /^>\s?/.test(lines[index])) {
          quoteLines.push(lines[index].replace(/^>\s?/, ''));
          index++;
        }
        html.push(`<blockquote>${renderInline(quoteLines.join(' '))}</blockquote>`);
        continue;
      }

      if (/^---+$/.test(trimmed)) {
        html.push('<hr />');
        index++;
        continue;
      }

      if (calloutMatch(line)) {
        const callout = renderCallout(lines, index);
        html.push(callout.html);
        index = callout.next;
        continue;
      }

      if (isStepLine(line)) {
        const steps = renderStepList(lines, index);
        html.push(steps.html);
        index = steps.next;
        continue;
      }

      if (isListLine(line)) {
        const list = renderList(lines, index);
        html.push(list.html);
        index = list.next;
        continue;
      }

      const paragraph = renderParagraph(lines, index);
      html.push(paragraph.html);
      index = paragraph.next;
    }

    return { html: html.join('\n'), codeBlocks };
  }

  async function handleMarkdownClick(event: MouseEvent) {
    if (!markdownMounted) return;
    const target = event.target as HTMLElement | null;
    const button = target?.closest<HTMLButtonElement>('[data-copy-code]');
    if (!button) return;

    const index = Number(button.dataset.copyCode || '-1');
    const code = renderedResult.codeBlocks[index];
    if (!code) return;

    try {
      await navigator.clipboard.writeText(code);
      if (!markdownMounted) return;
      copiedCodeIndex = index;
      clearCopyResetTimer();
      copyResetTimer = setTimeout(() => {
        if (!markdownMounted) return;
        copiedCodeIndex = null;
        copyResetTimer = undefined;
      }, 1400);
    } catch {
      if (!markdownMounted) return;
      copiedCodeIndex = null;
    }
  }

  onMount(() => {
    markdownMounted = true;
    const root = markdownRoot;
    if (!root) return;

    const handleClick = (event: MouseEvent) => {
      void handleMarkdownClick(event);
    };
    root.addEventListener('click', handleClick);
    return () => {
      markdownMounted = false;
      clearCopyResetTimer();
      root.removeEventListener('click', handleClick);
    };
  });

  $: renderedResult = renderMarkdown(content, copiedCodeIndex);
</script>

<div bind:this={markdownRoot} class="markdown-content" class:markdown-streaming={streaming}>
  {#if !content.trim() && streaming}
    <span class="md-empty-stream" aria-hidden="true"></span>
  {:else}
    {@html renderedResult.html}
    {#if streaming}
       <div class="md-stream-status" aria-hidden="true" aria-label="Streaming">
        <Icon name="more" size={20} />
       </div>
    {/if}
  {/if}
</div>
