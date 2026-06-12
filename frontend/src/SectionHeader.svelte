<script lang="ts">
  import Icon from './Icon.svelte';

  export let icon = 'info';
  export let title = '';
  export let description = '';
  export let eyebrow = '';
  export let meta = '';
  export let compact = false;
  export let className = '';

  $: headerClass = `section-header ${compact ? 'section-header-compact' : ''} ${className}`.trim();
</script>

<div class={headerClass}>
  <div class="section-header-main">
    <div class="section-header-icon">
      <Icon name={icon} size={16} />
    </div>
    <div class="section-header-copy">
      {#if eyebrow.trim()}
        <div class="section-header-eyebrow">{eyebrow}</div>
      {/if}
      <div class="section-header-title">{title}</div>
      {#if description.trim()}
        <div class="section-header-description">{description}</div>
      {/if}
    </div>
  </div>
  <div class="section-header-meta">
    {#if meta.trim()}
      <span>{meta}</span>
    {/if}
    <slot name="actions" />
  </div>
</div>
