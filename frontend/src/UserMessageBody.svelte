<script lang="ts">
  import ActionButton from './ActionButton.svelte';
  import Icon from './Icon.svelte';
  import type { ChatAttachment } from './lib/appTypes';

  export let content = '';
  export let attachments: ChatAttachment[] = [];
  export let messageIndex = -1;
  export let isEditing = false;
  export let editingUserPrompt = '';
  export let editingUserBusy = false;
  export let expanded = false;
  export let userMessageVisibleContent: (content: string, expanded: boolean) => string = (value) => value;
  export let userMessageNeedsCollapse: (content: string) => boolean = () => false;
  export let cancelEditUserMessage: () => void = () => {};
  export let submitEditedUserMessage: (index: number) => Promise<void> | void = () => {};
  export let toggleUserMessageExpanded: (index: number) => void = () => {};

  function attachmentSizeLabel(sizeBytes: number) {
    const size = Number(sizeBytes || 0);
    if (!Number.isFinite(size) || size <= 0) return '0 B';
    if (size < 1024) return `${size} B`;
    const units = ['KB', 'MB', 'GB'];
    let current = size / 1024;
    let unitIndex = 0;
    while (current >= 1024 && unitIndex < units.length - 1) {
      current /= 1024;
      unitIndex += 1;
    }
    return `${current.toFixed(current >= 10 ? 0 : 1)} ${units[unitIndex]}`;
  }
</script>

{#if isEditing}
  <div class="message-edit-panel">
    <textarea
      class="message-edit-textarea"
      bind:value={editingUserPrompt}
      rows="4"
      aria-label="Edit sent prompt"
      onkeydown={(event) => {
        if (event.key === 'Escape') cancelEditUserMessage();
        if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') void submitEditedUserMessage(messageIndex);
      }}
    ></textarea>
    <div class="message-edit-actions">
      <ActionButton variant="secondary" size="sm" disabled={editingUserBusy} disabledReason="Wait for the current resend to finish." onclick={cancelEditUserMessage}>
        Cancel
      </ActionButton>
      <ActionButton
        variant="primary"
        size="sm"
        icon="send"
        disabled={editingUserBusy || !editingUserPrompt.trim()}
        disabledReason={editingUserBusy ? 'Yemaka is sending this edited prompt.' : 'Enter a prompt before resending.'}
        busy={editingUserBusy}
        busyLabel="Sending..."
        onclick={() => submitEditedUserMessage(messageIndex)}
      >
        Save and resend
      </ActionButton>
    </div>
  </div>
{:else}
  {#if attachments.length > 0}
    <div class="message-attachment-strip" aria-label="Attached files">
      {#each attachments as attachment (attachment.id)}
        <span class="message-attachment-chip" title={attachment.summary || attachment.fileName}>
          <Icon name="documents" size={14} />
          <span>{attachment.fileName}</span>
          <span class="message-attachment-size">{attachmentSizeLabel(attachment.sizeBytes)}</span>
        </span>
      {/each}
    </div>
  {/if}
  <div class="message-user-content whitespace-pre-wrap leading-6">{userMessageVisibleContent(content, expanded)}</div>
  {#if userMessageNeedsCollapse(content)}
    <button class="message-show-more" type="button" onclick={() => toggleUserMessageExpanded(messageIndex)}>
      {expanded ? 'Show less' : 'Show more'}
    </button>
  {/if}
{/if}
