<script lang="ts">
  import Icon from './Icon.svelte';

  export let sources: string[] = [];
  export let compactSource: (source: string) => string = (source) => source;
</script>

{#if sources.length}
  <div class="source-strip mt-4">
    <div class="source-title">
      <Icon name="documents" size={13} />
      sources used
    </div>
    <div class="source-list">
      {#each sources.slice(0, 6) as source}
        {#if source.startsWith('http')}
           <a href={source} target="_blank" rel="noopener noreferrer" title={source}>
            <span title={source}>{compactSource(source)}</span>
          </a>
        {:else}
           <span title={source}>{compactSource(source)}</span>
        {/if}
      {/each}
    </div>
  </div>
{/if}
