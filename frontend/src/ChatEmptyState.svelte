<script lang="ts">
  import type { Snippet } from 'svelte';
  import { MotionDiv } from '@humanspeak/svelte-motion';
  import ChatComposerCard from './ChatComposerCard.svelte';
  import Icon from './Icon.svelte';
  import YemakaIcon from './YemakaIcon.svelte';
  import type { SendAskOptions } from './lib/appTypes';

  type PromptChip = {
    label: string;
    icon: string;
    prompt: string;
  };

  type ResponseModeOption = {
    value: string;
    label: string;
    disabled?: boolean;
    meta?: string;
  };

  type RenderSnippet = () => ReturnType<Snippet>;

  export let weekdayLabel = '';
  export let prompt = '';
  export let composerFocused = false;
  export let composerTextarea: HTMLTextAreaElement | null = null;
  export let busy = false;
  export let promptChips: PromptChip[] = [];
  export let queuedFollowUpList: RenderSnippet | null = null;
  export let composerCapabilityMenu: RenderSnippet | null = null;
  export let responseMode = 'balanced';
  export let responseModeOptions: ResponseModeOption[] = [];
  export let responseModeDisabled = false;
  export let setResponseMode: (value: string) => Promise<void> | void = () => {};
  export let focusComposer: () => void = () => {};
  export let blurComposer: () => void = () => {};
  export let resizeComposerTextarea: () => Promise<void> | void = () => {};
  export let handleComposerKeydown: (event: KeyboardEvent) => void = () => {};
  export let sendAsk: (contentOverride?: string | Event, options?: SendAskOptions) => Promise<boolean | void> | boolean | void = () => {};
  export let stopGeneration: () => Promise<void> | void = () => {};
  export let conversationKey = '';
</script>

<div class="main-empty-state grid min-h-full place-items-center py-8">
  <MotionDiv
    class="main-empty-content w-full max-w-3xl text-center"
    initial={{ opacity: 0, y: 10, scale: 0.995 }}
    animate={{ opacity: 1, y: 0, scale: 1 }}
    transition={{ duration: 0.22, ease: 'easeOut' }}
  >
    <div class="main-empty-mark mx-auto text-pine">
      <YemakaIcon size={42} strokeWidth={1.35} decorative />
    </div>
    <h1 class="premium-title main-empty-title text-3xl font-semibold sm:text-5xl">
      Happy {weekdayLabel || 'day'}.
    </h1>
    <div class="premium-subtitle main-empty-subtitle text-xl sm:text-2xl">How can Yemaka help?</div>

    <ChatComposerCard
      mode="empty"
      bind:prompt
      {composerFocused}
      bind:composerTextarea
      {busy}
      placeholder="Ask about your files, notes, tools, or next task..."
      {queuedFollowUpList}
      {composerCapabilityMenu}
      {responseMode}
      {responseModeOptions}
      {responseModeDisabled}
      {setResponseMode}
      {focusComposer}
      {blurComposer}
      {resizeComposerTextarea}
      {handleComposerKeydown}
      {sendAsk}
      {stopGeneration}
      {conversationKey}
    />

    <div class="main-empty-actions flex flex-wrap justify-center gap-2">
      {#each promptChips as chip}
        <button class="action-chip" type="button" aria-label={`Use prompt: ${chip.label}`} onclick={() => (prompt = chip.prompt)}>
          <Icon name={chip.icon} size={15} />
          <span>{chip.label}</span>
        </button>
      {/each}
    </div>
  </MotionDiv>
</div>
