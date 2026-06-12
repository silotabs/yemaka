<script lang="ts">
  import ChatTitleMenu from './ChatTitleMenu.svelte';
  import ActionButton from './ActionButton.svelte';
  import Badge from './Badge.svelte';
  import Icon from './Icon.svelte';

  type NavTab = {
    id: string;
    label: string;
    icon: string;
  };

  type ConversationSession = {
    id: string;
    title: string;
    starred?: boolean;
  };

  type Status = {
    ollamaOk?: boolean;
    sqliteOk?: boolean;
  };

  export let activeTab = 'chat';
  export let tabs: NavTab[] = [];
  export let tabSubtitles: Record<string, string> = {};
  export let status: Status | null = null;
  export let chatTitle = '';
  export let renamingChatTitle = false;
  export let chatMenuOpen = false;
  export let conversations: ConversationSession[] = [];
  export let activeConversationId = '';
  export let automationBusy = false;
  export let learningBusy = false;
  export let extensionBusy = false;
  export let domainPackBusy = false;
  export let renameChatTitle: (nextTitle: string) => Promise<void> | void = () => {};
  export let startRenameChatTitle: () => void = () => {};
  export let toggleStarChat: () => Promise<void> | void = () => {};
  export let exportActiveConversationMarkdown: () => Promise<void> | void = () => {};
  export let exportActiveConversationJSON: () => Promise<void> | void = () => {};
  export let deleteChat: () => Promise<void> | void = () => {};
  export let refreshActiveTab: () => Promise<void> | void = () => {};

  $: activeTabMeta = tabs.find((tab) => tab.id === activeTab);
  $: refreshLabelText = automationBusy
    ? 'Checking'
    : learningBusy || extensionBusy || domainPackBusy
      ? 'Refreshing'
      : activeTab === 'automation'
        ? 'Check'
        : 'Refresh';
</script>

<header class="topbar flex flex-wrap items-center justify-between gap-3 border-b border-line bg-white px-4 py-2.5 sm:px-5">
  {#if activeTab === 'chat'}
    <ChatTitleMenu
      bind:chatTitle
      bind:renamingChatTitle
      bind:chatMenuOpen
      {conversations}
      {activeConversationId}
      {renameChatTitle}
      {startRenameChatTitle}
      {toggleStarChat}
      exportConversationMarkdown={exportActiveConversationMarkdown}
      exportConversationJSON={exportActiveConversationJSON}
      {deleteChat}
    />
  {:else}
    <div class="min-w-0">
      <div class="flex min-w-0 items-center gap-2">
        <span class="text-pine"><Icon name={activeTabMeta?.icon || 'chat'} size={17} /></span>
        <div class="truncate text-sm font-semibold">{activeTabMeta?.label}</div>
        <Badge variant={status?.ollamaOk ? 'success' : 'warning'}>
          {status?.ollamaOk ? 'ollama' : 'ollama off'}
        </Badge>
        <span class="hidden sm:inline-flex">
          <Badge variant={status?.sqliteOk ? 'info' : 'danger'}>
            {status?.sqliteOk ? 'sqlite' : 'sqlite off'}
          </Badge>
        </span>
      </div>
      <div class="mt-0.5 truncate text-xs text-slate-600">{tabSubtitles[activeTab]}</div>
    </div>
    <ActionButton
      variant="secondary"
      size="sm"
      icon={activeTab === 'automation' ? 'health' : 'retry'}
      disabled={automationBusy || learningBusy || extensionBusy || domainPackBusy}
      disabledReason="This page is already refreshing."
      onclick={refreshActiveTab}
    >
      {refreshLabelText}
    </ActionButton>
  {/if}
</header>
