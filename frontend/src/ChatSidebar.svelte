<script lang="ts">
  import { DropdownMenu } from 'bits-ui';
  import Icon from './Icon.svelte';
  import EmptyState from './EmptyState.svelte';
  import YemakaIcon from './YemakaIcon.svelte';
  import type { Tab } from './lib/appOptions';
  import type { ConversationSession } from './lib/appTypes';

  type NavTab = {
    id: Tab;
    label: string;
    icon: string;
  };

  type Status = {
    modelReady?: boolean;
    lowMemoryMode?: boolean;
    selectedModel?: string;
    lowMemoryModel?: string;
  };

  export let sidebarVisible = true;
  export let moreOpen = false;
  export let activeTab: Tab = 'chat';
  export let moreTabs: NavTab[] = [];
  export let starredConversations: ConversationSession[] = [];
  export let recentConversations: ConversationSession[] = [];
  export let activeConversationId = '';
  export let status: Status | null = null;
  export let setSidebarVisible: (value: boolean) => void = () => {};
  export let newChat: () => Promise<void> | void = () => {};
  export let openMemorySearch: () => Promise<void> | void = () => {};
  export let selectTab: (tab: Tab) => Promise<void> | void = () => {};
  export let openConversation: (id: string) => Promise<void> | void = () => {};
  export let toggleSidebarConversationStar: (conversation: ConversationSession) => Promise<void> | void = () => {};
  export let renameSidebarConversation: (conversation: ConversationSession) => Promise<void> | void = () => {};
  export let exportSidebarConversationMarkdown: (conversation: ConversationSession) => Promise<void> | void = () => {};
  export let exportSidebarConversationJSON: (conversation: ConversationSession) => Promise<void> | void = () => {};
  export let deleteConversationById: (id: string) => Promise<void> | void = () => {};
  export let promptTitle: (value: string) => string = (value) => value;

  function openSettings() {
    void selectTab('settings');
  }
</script>

{#if sidebarVisible}
  <aside class="sidebar-shell flex min-h-0 min-w-0 flex-col border-b border-line bg-white lg:border-b-0 lg:border-r">
    <div class="px-3 py-2.5 lg:py-3">
      <div class="flex items-center justify-between gap-2">
        <div class="flex min-w-0 items-center gap-1.5">
          <div class="yemaka-mark"><YemakaIcon size={25} strokeWidth={1.35} decorative /></div>
          <div class="truncate text-xl font-semibold tracking-tight">Yemaka</div>
        </div>
        <button class="sidebar-toggle" type="button" title="Hide sidebar" onclick={() => setSidebarVisible(false)}>
          <Icon name="sidebar" size={16} />
        </button>
      </div>
    </div>
    <nav class="relative flex gap-2 overflow-x-auto p-2 lg:block lg:min-h-0 lg:overflow-visible" aria-label="Primary navigation">
      <button
        class={`nav-item mb-1 w-full shrink-0 rounded-xl px-3 py-2 text-left text-sm transition-colors ${activeTab === 'chat' ? 'nav-item-active' : 'text-slate-700'}`}
        aria-current={activeTab === 'chat' ? 'page' : undefined}
        onclick={newChat}
      >
        <span class="flex items-center gap-2">
          <Icon name="write" size={16} />
          <span class="truncate">New chat</span>
        </span>
      </button>
      <button
        class={`nav-item mb-1 w-full shrink-0 rounded-xl px-3 py-2 text-left text-sm transition-colors ${activeTab === 'memory' ? 'nav-item-active' : 'text-slate-700'}`}
        aria-current={activeTab === 'memory' ? 'page' : undefined}
        onclick={openMemorySearch}
      >
        <span class="flex items-center gap-2">
          <Icon name="search" size={16} />
          <span class="truncate">Search memory</span>
        </span>
      </button>
      <button
        class={`nav-item mb-1 w-full shrink-0 rounded-xl px-3 py-2 text-left text-sm transition-colors ${activeTab === 'documents' ? 'nav-item-active' : 'text-slate-700'}`}
        aria-current={activeTab === 'documents' ? 'page' : undefined}
        onclick={() => selectTab('documents')}
      >
        <span class="flex items-center gap-2">
          <Icon name="documents" size={16} />
          <span class="truncate">Documents</span>
        </span>
      </button>
      <DropdownMenu.Root bind:open={moreOpen}>
        <DropdownMenu.Trigger class={`nav-item w-full shrink-0 rounded-xl px-3 py-2 text-left text-sm transition-colors ${moreOpen || moreTabs.some((tab) => tab.id === activeTab) ? 'nav-item-selected text-ink' : 'text-slate-700'}`}>
          <span class="flex items-center justify-between gap-2">
            <span class="flex min-w-0 items-center gap-2">
              <Icon name="more" size={16} />
              <span class="truncate">More</span>
            </span>
            <Icon name="chevron" size={16} />
          </span>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content class="more-popover bits-menu-content" side="right" sideOffset={8} align="start">
            {#each moreTabs as tab}
              {#if tab.id === 'settings'}
                <DropdownMenu.Separator class="chat-menu-divider" />
              {/if}
              <DropdownMenu.Item class={`more-item ${activeTab === tab.id ? 'more-item-active' : ''}`} onSelect={() => selectTab(tab.id)}>
                <Icon name={tab.icon} size={16} />
                <span>{tab.label}</span>
              </DropdownMenu.Item>
            {/each}
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </nav>
    <div class="sidebar-recents hidden min-h-0 flex-1 overflow-y-auto p-2 lg:block">
      {#if starredConversations.length > 0}
        <div class="mb-1.5 mt-2 px-1 text-[10px] font-medium uppercase tracking-wider text-slate-500">Starred</div>
        <div class="mb-3 space-y-0.5">
          {#each starredConversations as item}
            <div class={`conversation-row relative ${activeConversationId === item.id ? 'conversation-row-active' : ''}`}>
              <button
                class={`recent-item flex w-full items-center justify-between gap-2 px-3 py-2 pr-8 text-left hover:text-ink ${activeConversationId === item.id ? 'recent-item-active text-ink' : 'text-slate-700'}`}
                title={item.title}
                aria-current={activeConversationId === item.id ? 'page' : undefined}
                onclick={() => openConversation(item.id)}
              >
                <span class="min-w-0 truncate">{item.title}</span>
              </button>
              <DropdownMenu.Root>
                <DropdownMenu.Trigger class="conversation-ellipsis" aria-label={`Actions for ${item.title}`}>
                  <Icon name="more" size={16} />
                </DropdownMenu.Trigger>
                <DropdownMenu.Portal>
                  <DropdownMenu.Content class="sidebar-chat-menu bits-menu-content" side="right" sideOffset={8} align="start">
                    <DropdownMenu.Item class="chat-menu-item" onSelect={() => toggleSidebarConversationStar(item)}>
                      <Icon name="star" size={16} />
                      <span>Unstar</span>
                    </DropdownMenu.Item>
                    <DropdownMenu.Item class="chat-menu-item" onSelect={() => renameSidebarConversation(item)}>
                      <Icon name="write" size={16} />
                      <span>Rename</span>
                    </DropdownMenu.Item>
                    <DropdownMenu.Item class="chat-menu-item" onSelect={() => exportSidebarConversationMarkdown(item)}>
                      <Icon name="documents" size={16} />
                      <span>Export as Markdown</span>
                    </DropdownMenu.Item>
                    <DropdownMenu.Item class="chat-menu-item" onSelect={() => exportSidebarConversationJSON(item)}>
                      <Icon name="code" size={16} />
                      <span>Export as JSON</span>
                    </DropdownMenu.Item>
                    <DropdownMenu.Separator class="chat-menu-divider" />
                    <DropdownMenu.Item class="chat-menu-item chat-menu-danger" onSelect={() => deleteConversationById(item.id)}>
                      <Icon name="trash" size={16} />
                      <span>Delete</span>
                    </DropdownMenu.Item>
                  </DropdownMenu.Content>
                </DropdownMenu.Portal>
              </DropdownMenu.Root>
            </div>
          {/each}
        </div>
      {/if}
      <div class="mb-1.5 mt-2 px-1 text-[10px] font-medium uppercase tracking-wider text-slate-500">Recents</div>
      {#if recentConversations.length === 0}
        <EmptyState
          icon="chat"
          title="No saved chats yet"
          message="Start a conversation and it will show up here for easy access later."
        />
      {/if}
      <div class="space-y-0.5">
        {#each recentConversations as item}
          <div class={`conversation-row relative ${activeConversationId === item.id ? 'conversation-row-active' : ''}`}>
            <button
              class={`recent-item w-full gap-2 px-3 py-2 pr-8 text-left hover:text-ink ${activeConversationId === item.id ? 'recent-item-active rounded-xl text-ink' : 'text-slate-700'}`}
              title={item.title}
              aria-current={activeConversationId === item.id ? 'page' : undefined}
              onclick={() => openConversation(item.id)}
            >
              <span class="min-w-0 truncate">{promptTitle(item.title)}</span>
            </button>
            <DropdownMenu.Root>
              <DropdownMenu.Trigger class="conversation-ellipsis" aria-label={`Actions for ${item.title}`}>
                <Icon name="more" size={16} />
              </DropdownMenu.Trigger>
              <DropdownMenu.Portal>
                <DropdownMenu.Content class="sidebar-chat-menu bits-menu-content" side="right" sideOffset={8} align="start">
                  <DropdownMenu.Item class="chat-menu-item" onSelect={() => toggleSidebarConversationStar(item)}>
                    <Icon name="star" size={16} />
                    <span>Star</span>
                  </DropdownMenu.Item>
                  <DropdownMenu.Item class="chat-menu-item" onSelect={() => renameSidebarConversation(item)}>
                    <Icon name="write" size={16} />
                    <span>Rename</span>
                  </DropdownMenu.Item>
                  <DropdownMenu.Item class="chat-menu-item" onSelect={() => exportSidebarConversationMarkdown(item)}>
                    <Icon name="documents" size={16} />
                    <span>Export as Markdown</span>
                  </DropdownMenu.Item>
                  <DropdownMenu.Item class="chat-menu-item" onSelect={() => exportSidebarConversationJSON(item)}>
                    <Icon name="code" size={16} />
                    <span>Export as JSON</span>
                  </DropdownMenu.Item>
                  <DropdownMenu.Separator class="chat-menu-divider" />
                  <DropdownMenu.Item class="chat-menu-item chat-menu-danger" onSelect={() => deleteConversationById(item.id)}>
                    <Icon name="trash" size={16} />
                    <span>Delete</span>
                  </DropdownMenu.Item>
                </DropdownMenu.Content>
              </DropdownMenu.Portal>
            </DropdownMenu.Root>
          </div>
        {/each}
      </div>
    </div>
    <div class="mt-auto hidden border-t border-line bg-white px-4 py-3 text-xs text-slate-600 lg:block">
      <div class="mb-3 flex items-center justify-between gap-3">
        <div class="flex min-w-0 items-center gap-2">
          <span class={`h-2.5 w-2.5 rounded-full ${status?.modelReady ? 'bg-pine' : 'bg-ember'}`}></span>
          <span class="truncate">{status?.modelReady ? 'Model ready' : 'Setup needed'}</span>
        </div>
        <span class={`shrink-0 rounded-md px-2.5 py-1 text-[11px] ${status?.lowMemoryMode ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-600'}`}>
          {status?.lowMemoryMode ? 'low memory' : 'standard'}
        </span>
      </div>
      <div class="flex items-center gap-3 border border-pine/20 p-3 rounded-xl bg-pine/5">
        <div class="yemaka-mark"><YemakaIcon size={25} strokeWidth={1.35} decorative /></div>
        <div class="min-w-0">
          <div class="mb-1 flex items-center justify-between gap-2">
            <span class="font-medium text-ink">Local profile</span>
            <button
              class="profile-settings-button"
              type="button"
              title="Open settings"
              aria-label="Open settings"
              aria-current={activeTab === 'settings' ? 'page' : undefined}
              onclick={openSettings}
            >
              <Icon name="settings" size={16} />
            </button>
          </div>
          <div class="truncate text-sm">Model: {status?.selectedModel || status?.lowMemoryModel || 'not loaded'}</div>
        </div>
      </div>
    </div>
  </aside>
{:else}
  <aside class="rail-shell hidden min-h-0 flex-col items-center border-r border-line bg-white py-2.5 lg:flex">
    <button class="rail-button" type="button" title="Show sidebar" onclick={() => setSidebarVisible(true)}>
      <Icon name="sidebar" size={18} />
    </button>
    <div class="mt-4 flex flex-col items-center gap-2">
      <button
        class={`rail-button rounded-xl ${activeTab === 'chat' ? 'rail-button-active' : ''}`}
        type="button"
        title="New chat"
        aria-label="New chat"
        aria-current={activeTab === 'chat' ? 'page' : undefined}
        onclick={newChat}
      >
        <Icon name="write" size={16} />
      </button>
      <button
        class={`rail-button rounded-xl ${activeTab === 'memory' ? 'rail-button-active' : ''}`}
        type="button"
        title="Search memory"
        aria-label="Search memory"
        aria-current={activeTab === 'memory' ? 'page' : undefined}
        onclick={openMemorySearch}
      >
        <Icon name="search" size={16} />
      </button>
      <button
        class={`rail-button rounded-xl ${activeTab === 'documents' ? 'rail-button-active' : ''}`}
        type="button"
        title="Documents"
        aria-label="Documents"
        aria-current={activeTab === 'documents' ? 'page' : undefined}
        onclick={() => selectTab('documents')}
      >
        <Icon name="documents" size={16} />
      </button>
      <DropdownMenu.Root bind:open={moreOpen}>
        <DropdownMenu.Trigger class={`rail-button rounded-xl ${moreOpen || moreTabs.some((tab) => tab.id === activeTab) ? 'rail-button-active' : ''}`} title="More">
          <Icon name="more" size={16} />
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content class="more-popover rail-popover bits-menu-content" side="right" sideOffset={8} align="start">
            {#each moreTabs as tab}
              {#if tab.id === 'settings'}
                <DropdownMenu.Separator class="chat-menu-divider" />
              {/if}
              <DropdownMenu.Item class={`more-item ${activeTab === tab.id ? 'more-item-active' : ''}`} onSelect={() => selectTab(tab.id)}>
                <Icon name={tab.icon} size={16} />
                <span>{tab.label}</span>
              </DropdownMenu.Item>
            {/each}
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </div>
    <div class="mt-auto flex flex-col items-center gap-1.5">
      <span
        class={`rail-status ${status?.modelReady ? 'bg-pine' : 'bg-ember'}`}
        title={`${status?.modelReady ? 'Model ready' : 'Setup needed'} | ${status?.lowMemoryMode ? 'low memory' : 'standard'}`}
      ></span>
      <span class="rail-mode text-sm" title={status?.lowMemoryMode ? 'low memory' : 'standard'}>{status?.lowMemoryMode ? 'L' : 'S'}</span>
      <button
        class={`rail-button rounded-xl ${activeTab === 'settings' ? 'rail-button-active' : ''}`}
        type="button"
        title="Open settings"
        aria-label="Open settings"
        aria-current={activeTab === 'settings' ? 'page' : undefined}
        onclick={openSettings}
      >
        <Icon name="settings" size={16} />
      </button>
      <div class="yemaka-mark">
        <YemakaIcon size={25} strokeWidth={1.35} decorative />
      </div>
    </div>
  </aside>
{/if}
