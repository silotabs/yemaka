<script lang="ts">
  import { MotionDiv } from '@humanspeak/svelte-motion';
  import AgentTimeline from './AgentTimeline.svelte';
  import AssistantSourceStrip from './AssistantSourceStrip.svelte';
  import CapabilityHandoffCard from './CapabilityHandoffCard.svelte';
  import ChatMessageActions from './ChatMessageActions.svelte';
  import ChatMessageHeader from './ChatMessageHeader.svelte';
  import Markdown from './Markdown.svelte';
  import PermissionInlineCard from './PermissionInlineCard.svelte';
  import SchedulerHandoffCard from './SchedulerHandoffCard.svelte';
  import UserMessageBody from './UserMessageBody.svelte';
  import type { ChatMessage, DomainPack, Extension, PermissionItem } from './lib/appTypes';

  type Option = {
    value: string;
    label: string;
  };

  export let message: ChatMessage;
  export let messageIndex = -1;
  export let busy = false;
  export let editingUserBusy = false;
  export let editingUserMessageIndex = -1;
  export let editingUserPrompt = '';
  export let expandedMessageIndexes: Record<number, boolean> = {};
  export let copiedMessageIndex: number | null = -1;
  export let jobScheduleTypeOptions: Option[] = [];
  export let extensions: Extension[] = [];
  export let domainPacks: DomainPack[] = [];
  export let capabilityHandoffBusy = false;
  export let schedulerHandoffBusy = false;
  export let permissionItems: PermissionItem[] = [];
  export let permissionDecisionBusy: Record<string, boolean> = {};
  export let isAssistantActiveMeta: (meta: string | undefined) => boolean = () => false;
  export let assistantMetaItems: (message: ChatMessage) => Array<{ icon: string; label: string }> = () => [];
  export let capabilityKindLabel: (kind: string | undefined) => string = (kind) => String(kind || '');
  export let setSchedulerHandoffInput: (index: number, value: string) => void = () => {};
  export let setCapabilityRunAfterGenerate: (index: number, enabled: boolean) => void = () => {};
  export let setCapabilityRunInput: (index: number, value: string) => void = () => {};
  export let setCapabilityScheduleAfterGenerate: (index: number, enabled: boolean) => void = () => {};
  export let setCapabilityScheduleType: (index: number, value: string) => void = () => {};
  export let setCapabilityScheduleExpr: (index: number, value: string) => void = () => {};
  export let setCapabilityScheduleEnabled: (index: number, enabled: boolean) => void = () => {};
  export let setCapabilityScheduleInput: (index: number, value: string) => void = () => {};
  export let openMessageDomainPacks: (index: number) => Promise<void> | void = () => {};
  export let openMessageCapabilityInExtensions: (index: number) => void = () => {};
  export let approveChatCapability: (index: number) => Promise<void> | void = () => {};
  export let reuseExtension: (name: string) => Promise<void> | void = () => {};
  export let createSchedulerJobFromChat: (index: number) => Promise<void> | void = () => {};
  export let openAutomation: () => void = () => {};
  export let userMessageVisibleContent: (content: string, expanded: boolean) => string = (value) => value;
  export let userMessageNeedsCollapse: (content: string) => boolean = () => false;
  export let cancelEditUserMessage: () => void = () => {};
  export let submitEditedUserMessage: (index: number) => Promise<void> | void = () => {};
  export let toggleUserMessageExpanded: (index: number) => void = () => {};
  export let compactSource: (source: string) => string = (source) => source;
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

  export let permissionCommandPreview: (item: PermissionItem) => string = () => '';
  export let decidePermission: (item: PermissionItem, decision: 'approved' | 'rejected') => Promise<void> | void = () => {};
  export let formatToolName: (name: string | undefined) => string = (name) => String(name || '');
  export let formatToolStatus: (status: string) => string = (status) => status;

  function normalizeText(value: string | undefined) {
    return String(value || '').toLowerCase();
  }

  function permissionMatchesMessage(item: PermissionItem, candidate: ChatMessage) {
    if (candidate.role !== 'assistant') return false;
    if (item.assistantMessageId) return item.assistantMessageId === candidate.id;
    if (item.userMessageId && candidate.parentId && item.userMessageId !== candidate.parentId) return false;

    const tool = normalizeText(item.toolName);
    if (!tool) return false;
    const trace = normalizeText((candidate.trace ?? []).join(' '));
    if (trace.includes('permission') && trace.includes(tool)) return true;

    const content = normalizeText(candidate.content);
    const approvalText =
      content.includes('needs your approval') ||
      content.includes('approval first') ||
      content.includes('requires confirmation') ||
      content.includes('review the diff');
    return approvalText && content.includes(tool);
  }

  $: inlinePermissionItems = permissionItems.filter((item) => permissionMatchesMessage(item, message));
</script>

<MotionDiv
  layout="position"
  class={`message-shell mb-4 ${message.role === 'user' ? 'message-shell-user' : ''}`}
  initial={{ opacity: 0, y: 8 }}
  animate={{ opacity: 1, y: 0 }}
  transition={{ duration: 0.16, ease: 'easeOut' }}
>
  <div class={`message-bubble max-w-full break-words ${message.role === 'user' ? 'message-user' : ''}`}>
    <ChatMessageHeader {message} {isAssistantActiveMeta} {assistantMetaItems} />
    {#if message.role === 'assistant'}
      {#if message.trace?.length && isAssistantActiveMeta(message.meta)}
        <AgentTimeline trace={message.trace} active={isAssistantActiveMeta(message.meta)} />
      {/if}
      <Markdown content={message.content} streaming={isAssistantActiveMeta(message.meta)} />
      {#each inlinePermissionItems as item (item.requestId)}
        <PermissionInlineCard
          {item}
          {permissionDecisionBusy}
          {permissionCommandPreview}
          {decidePermission}
          {formatToolName}
          {formatToolStatus}
          persistenceKey={`message:${message.id || message.parentId || messageIndex}:permission:${item.requestId}`}
        />
      {/each}
      {#if message.capabilityHandoff}
        <CapabilityHandoffCard
          handoff={message.capabilityHandoff}
          {messageIndex}
          {jobScheduleTypeOptions}
          {extensions}
          {domainPacks}
          {capabilityHandoffBusy}
          {capabilityKindLabel}
          {setCapabilityRunAfterGenerate}
          {setCapabilityRunInput}
          {setCapabilityScheduleAfterGenerate}
          {setCapabilityScheduleType}
          {setCapabilityScheduleExpr}
          {setCapabilityScheduleEnabled}
          {setCapabilityScheduleInput}
          {openMessageDomainPacks}
          {openMessageCapabilityInExtensions}
          {approveChatCapability}
          {reuseExtension}
          persistenceKey={`message:${message.id || message.parentId || messageIndex}:capability:${message.capabilityHandoff.name || message.capabilityHandoff.title}`}
        />
      {/if}
      {#if message.schedulerHandoff}
        <SchedulerHandoffCard
          handoff={message.schedulerHandoff}
          {messageIndex}
          {capabilityKindLabel}
          {schedulerHandoffBusy}
          {setSchedulerHandoffInput}
          {createSchedulerJobFromChat}
          {openAutomation}
          persistenceKey={`message:${message.id || message.parentId || messageIndex}:scheduler:${message.schedulerHandoff.targetType || ''}:${message.schedulerHandoff.targetName || ''}`}
        />
      {/if}
    {:else}
      <UserMessageBody
        content={message.content}
        attachments={message.attachments ?? []}
        {messageIndex}
        isEditing={editingUserMessageIndex === messageIndex}
        bind:editingUserPrompt
        {editingUserBusy}
        expanded={Boolean(expandedMessageIndexes[messageIndex])}
        {userMessageVisibleContent}
        {userMessageNeedsCollapse}
        {cancelEditUserMessage}
        {submitEditedUserMessage}
        {toggleUserMessageExpanded}
      />
    {/if}
    {#if message.role === 'assistant' && message.trace?.length && !isAssistantActiveMeta(message.meta)}
      <AgentTimeline trace={message.trace} active={false} />
    {/if}
    {#if message.role === 'assistant' && message.sources?.length && !isAssistantActiveMeta(message.meta)}
      <AssistantSourceStrip sources={message.sources} {compactSource} />
    {/if}
  </div>
  <ChatMessageActions
    {message}
    {messageIndex}
    {busy}
    {editingUserBusy}
    {copiedMessageIndex}
    {userVariantCount}
    {userVariantPosition}
    {assistantVariantCount}
    {assistantVariantPosition}
    {switchUserVariant}
    {switchAssistantVariant}
    {copyMessage}
    {startEditUserMessage}
    {retryPromptForMessage}
    {retryMessage}
  />
</MotionDiv>
