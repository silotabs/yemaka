<script lang="ts">
  import Icon from './Icon.svelte';
  import YemakaIcon from './YemakaIcon.svelte';
  import type { ChatMessage } from './lib/appTypes';

  type MessageMetaItem = {
    icon: string;
    label: string;
  };

  export let message: ChatMessage;
  export let isAssistantActiveMeta: (meta: string | undefined) => boolean = () => false;
  export let assistantMetaItems: (message: ChatMessage) => MessageMetaItem[] = () => [];
</script>

{#if message.role === 'assistant'}
  <div class="message-meta-row mb-2 flex flex-wrap items-center gap-1.5">
    {#if isAssistantActiveMeta(message.meta)}
      <span class="message-state-pill working-shimmer" aria-live="polite">
        <YemakaIcon size={15} strokeWidth={1.35} decorative />
        <span class="working-shimmer-label">{message.meta === 'streaming' ? 'Writing' : 'Working'}</span>
      </span>
    {/if}
    {#each assistantMetaItems(message) as item}
      <span class="message-meta-pill">
        <Icon name={item.icon} size={16} />
        {item.label}
      </span>
    {/each}
  </div>
{/if}

{#if message.meta && message.role === 'system'}
  <div class="mb-2 flex items-center gap-1 text-xs opacity-70"><Icon name="settings" size={15} /> {message.meta}</div>
{/if}

{#if message.role === 'user'}
  <div class="message-user-label"><Icon name="user" size={15} /> {message.role}</div>
{/if}
