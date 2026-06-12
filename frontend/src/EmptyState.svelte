<script lang="ts">
  import Icon from './Icon.svelte';

  export let icon = 'info';
  export let title = 'Nothing here yet';
  export let message = '';
  export let actionLabel = '';
  export let onAction: () => void = () => {};
</script>

<div class="empty-state">
  <div class="empty-state-icon">
    <Icon name={icon} size={18} />
  </div>
  <div class="empty-state-copy">
    <div class="empty-state-title">{title}</div>
    {#if message.trim()}
      <div class="empty-state-message">{message}</div>
    {/if}
  </div>
  {#if actionLabel.trim()}
    <button class="empty-state-action" type="button" onclick={onAction}>{actionLabel}</button>
  {/if}
</div>
