<script lang="ts">
  import Icon from './Icon.svelte';
  import type { ChatMessage } from './lib/appTypes';

  export let message: ChatMessage;
  export let messageIndex = -1;
  export let busy = false;
  export let editingUserBusy = false;
  export let copiedMessageIndex: number | null = -1;
  export let userVariantCount: (message: ChatMessage) => number = () => 1;
  export let userVariantPosition: (message: ChatMessage) => number = () => 1;
  export let assistantVariantCount: (message: ChatMessage) => number = () => 1;
  export let assistantVariantPosition: (message: ChatMessage) => number = () => 1;
  export let switchUserVariant: (index: number, direction: number) => void = () => {};
  export let switchAssistantVariant: (index: number, direction: number) => void = () => {};
  export let copyMessage: (index: number) => Promise<void> | void = () => {};
  export let startEditUserMessage: (index: number) => void = () => {};
  export let retryPromptForMessage: (index: number) => string = () => '';
  export let retryMessage: (index: number) => Promise<void> | void = () => {};
</script>

<div class="message-actions" aria-label="Message actions">
  {#if message.role === 'user' && userVariantCount(message) > 1}
    <div class="response-variant-switch" aria-label="Prompt variants">
      <button
        class="message-action icon-only rotate-180"
        type="button"
        disabled={(message.activeVariantIndex ?? 0) <= 0}
        aria-label="Previous prompt variant"
        onclick={() => switchUserVariant(messageIndex, -1)}
        title="Previous prompt"
      >
        <Icon name="chevron" size={16} />
      </button>
      <span>{userVariantPosition(message)}/{userVariantCount(message)}</span>
      <button
        class="message-action icon-only"
        type="button"
        disabled={userVariantPosition(message) >= userVariantCount(message)}
        aria-label="Next prompt variant"
        onclick={() => switchUserVariant(messageIndex, 1)}
        title="Next prompt"
      >
        <Icon name="chevron" size={16} />
      </button>
    </div>
  {/if}
  {#if message.role === 'assistant' && assistantVariantCount(message) > 1}
    <div class="response-variant-switch" aria-label="Response variants">
      <button
        class="message-action icon-only rotate-180"
        type="button"
        disabled={(message.activeVariantIndex ?? 0) <= 0}
        aria-label="Previous response variant"
        onclick={() => switchAssistantVariant(messageIndex, -1)}
        title="Previous response"
      >
        <Icon name="chevron" size={16} />
      </button>
      <span>{assistantVariantPosition(message)}/{assistantVariantCount(message)}</span>
      <button
        class="message-action icon-only"
        type="button"
        disabled={assistantVariantPosition(message) >= assistantVariantCount(message)}
        aria-label="Next response variant"
        onclick={() => switchAssistantVariant(messageIndex, 1)}
        title="Next response"
      >
        <Icon name="chevron" size={16} />
      </button>
    </div>
  {/if}
  <button class="message-action" type="button" aria-label="Copy message" onclick={() => copyMessage(messageIndex)} title="Copy message">
    <Icon name="copy" size={16} />
    <span>{copiedMessageIndex === messageIndex ? 'Copied' : 'Copy'}</span>
  </button>
  {#if message.role === 'user'}
    <button
      class="message-action"
      type="button"
      disabled={busy || editingUserBusy || !message.id}
      aria-label="Edit and resend this prompt"
      onclick={() => startEditUserMessage(messageIndex)}
      title="Edit and resend this prompt"
    >
      <Icon name="write" size={16} />
      <span>Edit</span>
    </button>
  {/if}
  <button
    class="message-action"
    type="button"
    disabled={busy || !retryPromptForMessage(messageIndex)}
    aria-label="Try this prompt again"
    onclick={() => retryMessage(messageIndex)}
    title="Try this prompt again"
  >
    <Icon name="retry" size={16} />
    <span>Try again</span>
  </button>
</div>
