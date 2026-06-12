<script lang="ts">
  import Icon from './Icon.svelte';

  export let variant: 'neutral' | 'success' | 'warning' | 'danger' | 'info' = 'neutral';
  export let size: 'xs' | 'sm' = 'xs';
  export let icon = '';
  export let title = '';
  export let className = '';

  $: badgeClass = `ui-badge ui-badge-${variant} ui-badge-${size} ${className}`.trim();
</script>

<span class={badgeClass} {title}>
  {#if icon}
    <Icon name={icon} size={size === 'xs' ? 12 : 13} />
  {/if}
  <span class="ui-badge-label"><slot /></span>
</span>
