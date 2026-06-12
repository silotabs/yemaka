<script lang="ts">
  import { AnimatePresence, MotionDiv } from '@humanspeak/svelte-motion';
  import Icon from './Icon.svelte';

  export let trace: string[] = [];
  export let active = false;
  let expanded = false;
  let wasActive = active;

  type TimelineItem = {
    id: string;
    label: string;
    detail: string;
    icon: string;
    status: 'active' | 'done' | 'needs' | 'quiet';
  };

  function titleCase(value: string) {
    return value
      .replace(/[_-]/g, ' ')
      .replace(/\s+/g, ' ')
      .trim()
      .replace(/\b\w/g, (match) => match.toUpperCase());
  }

  function afterColon(value: string) {
    const index = value.indexOf(':');
    return index >= 0 ? value.slice(index + 1).trim() : value;
  }

  function cleanToolName(value: string) {
    return titleCase(value.replace(/\(.+\)$/, '').trim());
  }

  function normalizeTrace(line: string, index: number, total: number): TimelineItem {
    const clean = String(line || '').replace(/\s+/g, ' ').trim();
    const lower = clean.toLowerCase();
    const isLatest = index === total - 1;
    let item: TimelineItem = {
      id: `${index}-${clean}`,
      label: clean,
      detail: '',
      icon: 'automation',
      status: active && isLatest ? 'active' : 'done'
    };

    if (lower.startsWith('model:')) {
      item = { ...item, label: 'Model selected', detail: afterColon(clean), icon: 'models', status: 'done' };
    } else if (lower.startsWith('model tool call:')) {
      item = { ...item, label: 'Model requested tool', detail: afterColon(clean), icon: 'tools', status: active && isLatest ? 'active' : 'done' };
    } else if (lower.startsWith('task:')) {
      item = { ...item, label: 'Understanding task', detail: afterColon(clean), icon: 'chat', status: active && isLatest ? 'active' : 'done' };
    } else if (lower.startsWith('plan:')) {
      item = { ...item, label: 'Planning route', detail: afterColon(clean), icon: 'automation', status: active && isLatest ? 'active' : 'done' };
    } else if (lower.startsWith('route advisory:')) {
      item = { ...item, label: 'Routing advisor', detail: afterColon(clean), icon: 'automation', status: 'done' };
    } else if (lower.startsWith('executor:')) {
      const detail = afterColon(clean);
      const parts = detail.split(' ').filter(Boolean);
      const status = parts.shift() || 'ready';
      const tool = parts.join(' ');
      item = {
        ...item,
        label: status === 'ready' ? `Preparing ${cleanToolName(tool || 'tool')}` : titleCase(`Executor ${status}`),
        detail: tool ? cleanToolName(tool) : detail,
        icon: 'tools',
        status: active && isLatest && status !== 'skipped' ? 'active' : 'done'
      };
    } else if (lower.startsWith('capability:')) {
      item = { ...item, label: 'Capability proposed', detail: afterColon(clean), icon: 'extensions', status: 'needs' };
    } else if (lower.startsWith('permission needed:')) {
      item = { ...item, label: 'Permission needed', detail: afterColon(clean), icon: 'health', status: 'needs' };
    } else if (lower.startsWith('permission:')) {
      item = { ...item, label: 'Permission check', detail: afterColon(clean), icon: 'health', status: 'needs' };
    } else if (lower.startsWith('edit proposed:')) {
      item = { ...item, label: 'Edit proposed', detail: afterColon(clean), icon: 'write', status: 'needs' };
    } else if (lower.startsWith('tool:') || lower.startsWith('tool completed:')) {
      item = { ...item, label: 'Tool completed', detail: cleanToolName(afterColon(clean)), icon: 'tools', status: 'done' };
    } else if (lower.startsWith('workspace:')) {
      item = { ...item, label: 'Workspace context', detail: afterColon(clean), icon: 'documents', status: 'done' };
    } else if (lower.startsWith('rag:')) {
      item = { ...item, label: 'Retrieved documents', detail: afterColon(clean), icon: 'documents', status: 'done' };
    } else if (lower.startsWith('memory:')) {
      item = { ...item, label: 'Memory used', detail: afterColon(clean), icon: 'memory', status: 'done' };
    } else if (lower.startsWith('skill:')) {
      item = { ...item, label: 'Skill loaded', detail: afterColon(clean), icon: 'skills', status: 'done' };
    } else if (lower.startsWith('cloud fallback:')) {
      item = { ...item, label: 'Fallback model used', detail: afterColon(clean), icon: 'models', status: 'done' };
    } else if (lower.startsWith('verification:')) {
      const detail = afterColon(clean);
      item = { ...item, label: 'Verification', detail, icon: 'health', status: detail.toLowerCase().includes('pass') ? 'done' : 'needs' };
    }

    if (!active && item.status === 'active') {
      item.status = 'done';
    }
    return item;
  }

  $: items = trace
    .map((line) => String(line || '').trim())
    .filter(Boolean)
    .slice(-7)
    .map((line, index, lines) => normalizeTrace(line, index, lines.length));
  $: summary = items.length === 1 ? '1 step' : `${items.length} steps`;
  $: title = active ? 'Yemaka activity' : 'Completed steps';
  $: if (active) {
    expanded = true;
    wasActive = true;
  }
  $: if (!active && wasActive) {
    expanded = false;
    wasActive = false;
  }
</script>

<AnimatePresence>
  {#if items.length}
    <MotionDiv
      key="agent-timeline"
      class={`agent-timeline ${active ? 'agent-timeline-active' : 'agent-timeline-complete'}`}
      aria-label="Agent activity"
      initial={{ opacity: 0, y: 8, scale: 0.995 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, y: -4, scale: 0.995 }}
      transition={{ duration: 0.18, ease: 'easeOut' }}
    >
      <button class="agent-timeline-header" type="button" aria-expanded={active || expanded} onclick={() => !active && (expanded = !expanded)}>
        <span class="agent-timeline-spark" class:is-active={active}></span>
        <span>{title}</span>
        <span class="agent-timeline-summary">{active ? 'live' : summary}</span>
        {#if !active}
          <span class="agent-timeline-toggle"><Icon name={expanded ? 'eye-off' : 'eye'} size={13} /> {expanded ? 'Hide' : 'Inspect'}</span>
        {/if}
      </button>
      {#if active || expanded}
        <div class="agent-timeline-list" role="list" aria-label={title}>
          {#each items as item, index (`${item.id}-${index}`)}
            <MotionDiv
              key={`${item.id}-${index}`}
              layout="position"
              class={`agent-step agent-step-${item.status}${index === items.length - 1 ? ' agent-step-last' : ''}`}
              role="listitem"
              aria-label={`${item.label}${item.detail ? `: ${item.detail}` : ''}`}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -4 }}
              transition={{ duration: 0.16, ease: 'easeOut' }}
            >
              <div class="agent-step-rail">
                <div class="agent-step-icon">
                  {#if !active && item.status === 'done'}
                    <Icon name="check" size={13} />
                  {:else}
                    <Icon name={item.icon} size={13} />
                  {/if}
                </div>
              </div>
              <div class="agent-step-copy">
                <div class="agent-step-label">{item.label}</div>
                {#if item.detail}
                  <div class="agent-step-detail">{item.detail}</div>
                {/if}
              </div>
            </MotionDiv>
          {/each}
        </div>
      {/if}
    </MotionDiv>
  {/if}
</AnimatePresence>
