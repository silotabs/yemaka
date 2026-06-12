<script lang="ts">
  import Icon from './Icon.svelte';

  export let variant: 'primary' | 'secondary' | 'danger' | 'ghost' = 'secondary';
  export let size: 'xs' | 'sm' | 'md' = 'md';
  export let icon = '';
  export let type: 'button' | 'submit' | 'reset' = 'button';
  export let disabled = false;
  export let disabledReason = '';
  export let busy = false;
  export let busyLabel = '';
  export let title = '';
  export let className = '';
  export let ariaLabel = '';
  export let onclick: (event: MouseEvent) => void | Promise<void> = () => {};

  let buttonEl: HTMLButtonElement | null = null;

  $: effectiveDisabled = disabled || busy;
  $: effectiveTitle = effectiveDisabled && disabledReason.trim() ? disabledReason : title;
  $: label = busy && busyLabel.trim() ? busyLabel : '';
  $: buttonClass = `action-button action-button-${variant} action-button-${size} ${className}`.trim();

  export function focus() {
    buttonEl?.focus();
  }

  function handleClick(event: MouseEvent) {
    if (effectiveDisabled) return;
    void onclick(event);
  }
</script>

<button
  bind:this={buttonEl}
  class={buttonClass}
  {type}
  disabled={effectiveDisabled}
  title={effectiveTitle}
  aria-label={ariaLabel || undefined}
  aria-busy={busy || undefined}
  onclick={handleClick}
>
  {#if icon}
    <Icon name={icon} size={size === 'xs' ? 13 : size === 'sm' ? 14 : 16} />
  {/if}
  <span class="action-button-label">
    {#if label}
      {label}
    {:else}
      <slot />
    {/if}
  </span>
</button>
