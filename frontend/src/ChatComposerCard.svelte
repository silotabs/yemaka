<script lang="ts">
  import type { Snippet } from 'svelte';
  import { MotionDiv } from '@humanspeak/svelte-motion';
  import BitSelect from './BitSelect.svelte';
  import Icon from './Icon.svelte';
  import type { ChatAttachment, SendAskOptions } from './lib/appTypes';
  import { uploadChatAttachment } from './lib/chatActions';

  type ComposerMode = 'empty' | 'dock';
  type RenderSnippet = () => ReturnType<Snippet>;
  type ResponseModeOption = {
    value: string;
    label: string;
    disabled?: boolean;
    meta?: string;
  };
  type ComposerAttachmentStatus = 'uploading' | 'ready' | 'error';
  type ComposerAttachment = {
    clientId: string;
    id?: string;
    fileName: string;
    contentType: string;
    sizeBytes: number;
    status: ComposerAttachmentStatus;
    message?: string;
    preview?: string;
    summary?: string;
    attachment?: ChatAttachment;
  };

  export let mode: ComposerMode = 'dock';
  export let prompt = '';
  export let composerFocused = false;
  export let composerTextarea: HTMLTextAreaElement | null = null;
  export let busy = false;
  export let placeholder = 'Ask Yemaka...';
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

  let attachmentInput: HTMLInputElement | null = null;
  let attachments: ComposerAttachment[] = [];
  let attachmentSequence = 0;
  let lastConversationKey = conversationKey;

  $: isEmpty = mode === 'empty';
  $: promptHasText = Boolean(prompt.trim());
  $: composerClass = `composer-card ${isEmpty ? 'main-empty-composer mx-auto text-left' : 'composer-after-chat mx-auto'} ${composerFocused ? 'composer-expanded' : ''} ${promptHasText ? 'composer-has-content' : ''}`;
  $: iconSize = isEmpty ? 16 : 20;
  $: readyAttachmentIds = attachments
    .filter((attachment) => attachment.status === 'ready' && Boolean(attachment.id))
    .map((attachment) => attachment.id as string);
  $: hasUploadingAttachments = attachments.some((attachment) => attachment.status === 'uploading');
  $: hasReadyAttachments = readyAttachmentIds.length > 0;
  $: sendDisabled = busy || !promptHasText || hasUploadingAttachments;
  $: queueDisabled = !promptHasText || hasUploadingAttachments || hasReadyAttachments;
  $: composerAction = busy ? (promptHasText ? 'queue' : 'stop') : 'send';
  $: composerActionDisabled =
    composerAction === 'send' ? sendDisabled : composerAction === 'queue' ? queueDisabled : !busy;
  $: composerActionLabel = composerAction === 'queue' ? 'Queue' : composerAction === 'stop' ? 'Stop' : 'Send';
  $: composerActionIcon = composerAction === 'stop' ? 'stop' : 'send';
  $: if (conversationKey !== lastConversationKey) {
    lastConversationKey = conversationKey;
    attachments = [];
  }

  function chooseAttachments() {
    attachmentInput?.click();
  }

  function attachmentStatusFromUpload(attachment: ChatAttachment): ComposerAttachmentStatus {
    const status = String(attachment.status || '').toLowerCase();
    if (status === 'error' || status === 'failed' || status === 'broken') return 'error';
    return attachment.id ? 'ready' : 'uploading';
  }

  function attachmentStatusLabel(attachment: ComposerAttachment) {
    if (attachment.status === 'uploading') return 'Uploading';
    if (attachment.status === 'error') return attachment.message || 'Upload failed';
    return attachment.message || attachment.summary || 'Ready';
  }

  function attachmentStatusIcon(attachment: ComposerAttachment) {
    if (attachment.status === 'ready') return 'check';
    if (attachment.status === 'error') return 'warning';
    return 'more';
  }

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
    const decimals = current >= 10 ? 0 : 1;
    return `${current.toFixed(decimals)} ${units[unitIndex]}`;
  }

  function updateAttachment(clientId: string, updater: (attachment: ComposerAttachment) => ComposerAttachment) {
    attachments = attachments.map((attachment) => (attachment.clientId === clientId ? updater(attachment) : attachment));
  }

  async function uploadComposerAttachment(file: File) {
    const clientId = `attachment_${Date.now()}_${++attachmentSequence}`;
    attachments = [
      ...attachments,
      {
        clientId,
        fileName: file.name,
        contentType: file.type || 'application/octet-stream',
        sizeBytes: file.size,
        status: 'uploading'
      }
    ];
    try {
      const result = await uploadChatAttachment(file);
      const uploaded = result.attachment;
      updateAttachment(clientId, (attachment) => ({
        ...attachment,
        attachment: uploaded,
        id: uploaded.id || attachment.id,
        fileName: uploaded.fileName || attachment.fileName,
        contentType: uploaded.contentType || attachment.contentType,
        sizeBytes: uploaded.sizeBytes || attachment.sizeBytes,
        status: attachmentStatusFromUpload(uploaded),
        message: result.message || uploaded.summary || '',
        preview: uploaded.preview,
        summary: uploaded.summary
      }));
    } catch (err) {
      updateAttachment(clientId, (attachment) => ({
        ...attachment,
        status: 'error',
        message: err instanceof Error ? err.message : String(err)
      }));
    }
  }

  async function handleAttachmentInput(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const files = Array.from(input.files ?? []);
    input.value = '';
    if (files.length === 0) return;
    await Promise.all(files.map((file) => uploadComposerAttachment(file)));
  }

  function removeAttachment(clientId: string) {
    attachments = attachments.filter((attachment) => attachment.clientId !== clientId);
  }

  function clearSentAttachments(ids: string[]) {
    const sent = new Set(ids);
    attachments = attachments.filter((attachment) => !attachment.id || !sent.has(attachment.id));
  }

  async function submitComposerAsk() {
    if (sendDisabled) return;
    const attachmentIds = [...readyAttachmentIds];
    const attachmentItems = attachments
      .filter((attachment) => attachment.status === 'ready' && attachment.attachment && attachment.id)
      .map((attachment) => attachment.attachment as ChatAttachment);
    const sent = await sendAsk(
      undefined,
      attachmentIds.length > 0 ? { attachmentIds, attachments: attachmentItems } : undefined
    );
    if (sent !== false && attachmentIds.length > 0) {
      clearSentAttachments(attachmentIds);
    }
  }

  async function handleComposerAction() {
    if (composerActionDisabled) return;
    if (composerAction === 'stop') {
      await stopGeneration();
      return;
    }
    if (composerAction === 'queue') {
      await sendAsk();
      return;
    }
    await submitComposerAsk();
  }

  function handleLocalComposerKeydown(event: KeyboardEvent) {
    const hasSubmitModifier = event.key === 'Enter' && (event.metaKey || event.ctrlKey);
    if (hasSubmitModifier && !event.shiftKey && hasUploadingAttachments) {
      event.preventDefault();
      event.stopPropagation();
      return;
    }
    if (!hasSubmitModifier || event.shiftKey || !hasReadyAttachments) {
      handleComposerKeydown(event);
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    void submitComposerAsk();
  }
</script>

<MotionDiv
  layout={isEmpty ? undefined : 'position'}
  class={composerClass}
  initial={{ opacity: 0, y: 8 }}
  animate={{ opacity: 1, y: 0 }}
  transition={isEmpty ? { duration: 0.2, delay: 0.04, ease: 'easeOut' } : { duration: 0.18, ease: 'easeOut' }}
>
  <textarea
    bind:this={composerTextarea}
    class="composer-textarea"
    bind:value={prompt}
    rows="1"
    {placeholder}
    onfocus={focusComposer}
    onblur={blurComposer}
    oninput={() => void resizeComposerTextarea()}
    onkeydown={handleLocalComposerKeydown}
  ></textarea>
  <input bind:this={attachmentInput} class="sr-only" type="file" multiple onchange={handleAttachmentInput} aria-label="Choose chat attachments" />
  {#if attachments.length > 0}
    <div class="composer-attachments" aria-live="polite">
      {#each attachments as attachment (attachment.clientId)}
        <div class={`composer-attachment composer-attachment-${attachment.status}`} title={attachmentStatusLabel(attachment)}>
          <Icon name={attachmentStatusIcon(attachment)} size={14} />
          <div class="composer-attachment-main">
            <span class="composer-attachment-name" title={attachment.fileName}>{attachment.fileName}</span>
            <span class="composer-attachment-meta">{attachmentSizeLabel(attachment.sizeBytes)} | {attachmentStatusLabel(attachment)}</span>
          </div>
          <button
            class="composer-attachment-remove"
            type="button"
            aria-label={`Remove ${attachment.fileName}`}
            title={`Remove ${attachment.fileName}`}
            onclick={() => removeAttachment(attachment.clientId)}
          >
            <Icon name="close" size={13} />
          </button>
        </div>
      {/each}
    </div>
  {/if}
  {#if queuedFollowUpList}
    {@render queuedFollowUpList()}
  {/if}
  <div class="composer-controls">
    <button class="icon-button composer-attach-button" type="button" title="Attach files" aria-label="Attach files" disabled={busy} onclick={chooseAttachments}>
      <Icon name="documents" size={iconSize} />
    </button>
    {#if composerCapabilityMenu}
      {@render composerCapabilityMenu()}
    {/if}
    {#if responseModeOptions.length > 0}
      <BitSelect
        value={responseMode}
        options={responseModeOptions}
        placeholder="Mode"
        disabled={responseModeDisabled}
        className="ml-auto composer-response-mode"
        contentClass="composer-response-mode-menu"
        onChange={setResponseMode}
      />
    {/if}
    <div class={`${responseModeOptions.length > 0 ? '' : 'ml-auto'} flex items-center gap-2`}>
      <button
        class={`send-button ${composerAction === 'queue' ? 'send-button-queue' : ''} ${composerAction === 'stop' ? 'send-button-stop' : ''}`}
        disabled={composerActionDisabled}
        aria-label={composerAction === 'queue' ? 'Queue follow-up prompt' : composerAction === 'stop' ? 'Stop generation' : 'Send prompt'}
        title={composerAction === 'queue' ? 'Queue follow-up' : composerAction === 'stop' ? 'Stop generation' : 'Send prompt'}
        onclick={handleComposerAction}
      >
        <Icon name={composerActionIcon} size={iconSize} />
        {#if composerAction !== 'send'}
          <span>{composerActionLabel}</span>
        {/if}
      </button>
    </div>
  </div>
</MotionDiv>
