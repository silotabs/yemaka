<script lang="ts">
  import Icon from './Icon.svelte';
  import type { ToastKind } from './lib/toastCenter';
  import { toastIcon, toastTitle } from './lib/toastCenter';

  export let kind: ToastKind = 'info';
  export let title = '';
  export let message = '';
  export let actionLabel = '';
  export let onAction: () => void = () => {};
  export let className = '';

  $: visible = Boolean(title.trim() || message.trim());
  $: heading = title.trim() || toastTitle(kind);
  $: stripClass = `page-status-strip page-status-${kind} ${className}`.trim();
</script>

{#if visible}
  <section class={stripClass} role={kind === 'error' ? 'alert' : 'status'}>
    <div class="page-status-icon">
      <Icon name={toastIcon(kind)} size={18} />
    </div>
    <div class="page-status-copy">
      <div class="page-status-title">{heading}</div>
      {#if message.trim()}
        <div class="page-status-message">{message}</div>
      {/if}
    </div>
    {#if actionLabel.trim()}
      <button class="page-status-action" type="button" onclick={onAction}>{actionLabel}</button>
    {/if}
  </section>
{/if}
