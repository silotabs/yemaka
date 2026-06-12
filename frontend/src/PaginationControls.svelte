<script lang="ts">
  import Icon from './Icon.svelte';
  import ActionButton from './ActionButton.svelte';
  export let page = 1;
  export let total = 0;
  export let pageSize = 12;
  export let label = 'items';
  export let onChange: (page: number) => void = () => {};

  $: pages = Math.max(1, Math.ceil(Math.max(0, total) / Math.max(1, pageSize)));
  $: current = Math.min(Math.max(1, page), pages);
  $: start = total === 0 ? 0 : (current - 1) * pageSize + 1;
  $: end = Math.min(total, current * pageSize);

  function setPage(next: number) {
    const clamped = Math.min(Math.max(1, next), pages);
    if (clamped !== page) onChange(clamped);
  }
</script>

{#if total > pageSize}
  <div class="pagination-controls" aria-label={`${label} pagination`}>
    <span class="pagination-summary">{start}-{end} of {total} {label}</span>
    <div class="pagination-buttons">
      <!-- <button
        type="button"
        disabled={current <= 1}
        aria-label={`Previous ${label} page`}
        onclick={() => setPage(current - 1)}
      >
        <Icon name="previous" size={16} />
      </button> -->
      <ActionButton
        variant="secondary"
        size="xs"
        disabled={current <= 1}
        disabledReason="You are on the first page."
        onclick={() => setPage(current - 1)}
      >
        <Icon name="previous" size={16} />
      </ActionButton>
      <span>{current} / {pages}</span>
      <!-- <button
        type="button"
        disabled={current >= pages}
        aria-label={`Next ${label} page`}
        onclick={() => setPage(current + 1)}
      >
        <Icon name="next" size={16} />
      </button> -->
      <ActionButton
        variant="secondary"
        size="xs"
        disabled={current >= pages}
        disabledReason="You are on the last page."
        onclick={() => setPage(current + 1)}
      >
        <Icon name="next" size={16} />
      </ActionButton>
    </div>
  </div>
{/if}
