<script lang="ts">
  import Icon from './Icon.svelte';
  import ActionButton from './ActionButton.svelte';

  type QueuedFollowUpPrompt = {
    id: string;
    content: string;
    skill: string;
    editing?: boolean;
    draft: string;
  };

  export let queuedFollowUps: QueuedFollowUpPrompt[] = [];
  export let promptTitle: (value: string) => string = (value) => value;
  export let skillDisplayLabel: (skill: string) => string = (skill) => skill || 'Auto skill';
  export let updateQueuedFollowUpDraft: (id: string, draft: string) => void = () => {};
  export let cancelEditQueuedFollowUp: (id: string) => void = () => {};
  export let saveQueuedFollowUp: (id: string) => void = () => {};
  export let startEditQueuedFollowUp: (id: string) => void = () => {};
  export let deleteQueuedFollowUp: (id: string) => void = () => {};
</script>

{#if queuedFollowUps.length > 0}
  <div class="queued-followups" aria-label="Queued follow-up prompts">
    <div class="queued-followups-head">
      <div class="queued-followups-title">
        <span class="queued-followups-dot"></span>
        <span>Follow-up queue</span>
      </div>
      <span class="queued-followups-count">{queuedFollowUps.length} waiting</span>
    </div>
    <div class="queued-followups-list">
      {#each queuedFollowUps as item, index (item.id)}
        <div class="queued-followup-item" class:queued-followup-next={index === 0}>
          <span class="queued-followup-index">{index === 0 ? 'Next' : `#${index + 1}`}</span>
          {#if item.editing}
            <div class="queued-followup-edit-panel">
              <textarea
                class="queued-followup-edit"
                rows="2"
                aria-label="Edit queued follow-up prompt"
                value={item.draft}
                oninput={(event) => updateQueuedFollowUpDraft(item.id, (event.currentTarget as HTMLTextAreaElement).value)}
                onkeydown={(event) => {
                  if (event.key === 'Escape') cancelEditQueuedFollowUp(item.id);
                  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') saveQueuedFollowUp(item.id);
                }}
              ></textarea>
              <div class="queued-followup-actions">
                <ActionButton variant="ghost" size="xs" onclick={() => cancelEditQueuedFollowUp(item.id)}>
                  Cancel
                </ActionButton>
                <ActionButton
                  variant="primary"
                  size="xs"
                  icon="check"
                  disabled={!item.draft.trim()}
                  disabledReason="Enter follow-up text before saving."
                  onclick={() => saveQueuedFollowUp(item.id)}
                >
                  Save
                </ActionButton>
              </div>
            </div>
          {:else}
            <div class="queued-followup-copy">
              <div class="queued-followup-text">{promptTitle(item.content)}</div>
              <div class="queued-followup-meta">
                <span>{skillDisplayLabel(item.skill)}</span>
                <span>{index === 0 ? 'runs after current response' : 'waiting'}</span>
              </div>
            </div>
            <div class="queued-followup-actions">
              <button
                class="message-action icon-only queued-followup-action-button"
                type="button"
                title="Edit queued prompt"
                onclick={() => startEditQueuedFollowUp(item.id)}
              >
                <Icon name="write" size={15} />
              </button>
              <button
                class="message-action icon-only queued-followup-action-button queued-followup-delete-button"
                type="button"
                title="Delete queued prompt"
                onclick={() => deleteQueuedFollowUp(item.id)}
              >
                <Icon name="trash" size={15} />
              </button>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </div>
{/if}
