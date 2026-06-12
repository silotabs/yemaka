<script lang="ts">
  import { DropdownMenu } from 'bits-ui';
  import Icon from './Icon.svelte';

  type ConversationSession = {
    id: string;
    starred?: boolean;
  };

  export let chatTitle = 'New chat';
  export let renamingChatTitle = false;
  export let chatMenuOpen = false;
  export let conversations: ConversationSession[] = [];
  export let activeConversationId = '';
  export let renameChatTitle: (nextTitle: string) => Promise<void> | void = () => {};
  export let startRenameChatTitle: () => Promise<void> | void = () => {};
  export let toggleStarChat: () => Promise<void> | void = () => {};
  export let exportConversationMarkdown: () => Promise<void> | void = () => {};
  export let exportConversationJSON: () => Promise<void> | void = () => {};
  export let deleteChat: () => Promise<void> | void = () => {};

  $: activeConversation = conversations.find((conversation) => conversation.id === activeConversationId);

  function finishRename() {
    void renameChatTitle(chatTitle);
    renamingChatTitle = false;
  }
</script>

<div class="chat-title-wrap">
  {#if renamingChatTitle}
    <input
      class="chat-title-input"
      bind:value={chatTitle}
      onblur={finishRename}
      onkeydown={(event) => {
        if (event.key === 'Enter') finishRename();
        if (event.key === 'Escape') {
          renamingChatTitle = false;
        }
      }}
    />
  {:else}
    <button class="chat-title-button" type="button" onclick={startRenameChatTitle}>{chatTitle}</button>
  {/if}
  <DropdownMenu.Root bind:open={chatMenuOpen}>
    <DropdownMenu.Trigger class="chat-title-menu-button" aria-label="Chat actions">
      <Icon name="chevron" size={16} />
    </DropdownMenu.Trigger>
    <DropdownMenu.Portal>
      <DropdownMenu.Content class="chat-menu bits-menu-content" side="right" sideOffset={8} align="start">
        <DropdownMenu.Item class="chat-menu-item" onSelect={toggleStarChat}>
          <Icon name="star" size={16} />
          <span>{activeConversation?.starred ? 'Unstar' : 'Star'}</span>
        </DropdownMenu.Item>
        <DropdownMenu.Item class="chat-menu-item" onSelect={startRenameChatTitle}>
          <Icon name="write" size={16} />
          <span>Rename</span>
        </DropdownMenu.Item>
        <DropdownMenu.Item class="chat-menu-item" onSelect={exportConversationMarkdown}>
          <Icon name="documents" size={16} />
          <span>Export as Markdown</span>
        </DropdownMenu.Item>
        <DropdownMenu.Item class="chat-menu-item" onSelect={exportConversationJSON}>
          <Icon name="code" size={16} />
          <span>Export as JSON</span>
        </DropdownMenu.Item>
        <DropdownMenu.Separator class="chat-menu-divider" />
        <DropdownMenu.Item class="chat-menu-item chat-menu-danger" onSelect={deleteChat}>
          <Icon name="trash" size={16} />
          <span>Delete</span>
        </DropdownMenu.Item>
      </DropdownMenu.Content>
    </DropdownMenu.Portal>
  </DropdownMenu.Root>
</div>
