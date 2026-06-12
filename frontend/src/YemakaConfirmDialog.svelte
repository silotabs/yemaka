<script lang="ts">
  import { tick } from 'svelte';
  import ActionButton from './ActionButton.svelte';
  import Icon from './Icon.svelte';

  export let open = false;
  export let title = 'Confirm action';
  export let message = '';
  export let confirmLabel = 'Continue';
  export let cancelLabel = 'Cancel';
  export let destructive = false;
  export let onConfirm: () => void = () => {};
  export let onCancel: () => void = () => {};

  let confirmButton: { focus: () => void } | null = null;

  $: if (open) {
    void focusConfirmButton();
  }

  async function focusConfirmButton() {
    await tick();
    confirmButton?.focus();
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!open) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      onCancel();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
  <div class="yemaka-confirm-overlay" role="presentation">
    <button class="yemaka-confirm-backdrop" type="button" aria-label="Cancel confirmation" onclick={onCancel}></button>
    <div class="yemaka-confirm-card" role="dialog" aria-modal="true" aria-labelledby="yemaka-confirm-title">
      <div class={`yemaka-confirm-icon ${destructive ? 'is-destructive' : ''}`}>
        <Icon name={destructive ? 'trash' : 'health'} size={18} />
      </div>
      <div class="min-w-0 flex-1">
        <h2 id="yemaka-confirm-title" class="yemaka-confirm-title">{title}</h2>
        <p class="yemaka-confirm-message">{message}</p>
        <div class="yemaka-confirm-actions">
          <ActionButton variant="secondary" size="sm" onclick={onCancel}>
            {cancelLabel}
          </ActionButton>
          <ActionButton bind:this={confirmButton} variant={destructive ? 'danger' : 'primary'} size="sm" icon={destructive ? 'trash' : 'check'} onclick={onConfirm}>
            {confirmLabel}
          </ActionButton>
        </div>
      </div>
    </div>
  </div>
{/if}
