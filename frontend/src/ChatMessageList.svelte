<script lang="ts">
  import ChatMessageItem from './ChatMessageItem.svelte';
  import type { ChatMessage, DomainPack, Extension, PermissionItem } from './lib/appTypes';

  type Option = {
    value: string;
    label: string;
  };

  export let messages: ChatMessage[] = [];
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
</script>

<div class="mx-auto max-w-3xl">
  {#each messages as message, index (`${message.role}-${index}`)}
    <ChatMessageItem
      {message}
      messageIndex={index}
      {busy}
      {editingUserBusy}
      {editingUserMessageIndex}
      bind:editingUserPrompt
      {expandedMessageIndexes}
      {copiedMessageIndex}
      {jobScheduleTypeOptions}
      {extensions}
      {domainPacks}
      {capabilityHandoffBusy}
      {schedulerHandoffBusy}
      {permissionItems}
      {permissionDecisionBusy}
      {isAssistantActiveMeta}
      {assistantMetaItems}
      {capabilityKindLabel}
      {setSchedulerHandoffInput}
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
      {createSchedulerJobFromChat}
      {openAutomation}
      {userMessageVisibleContent}
      {userMessageNeedsCollapse}
      {cancelEditUserMessage}
      {submitEditedUserMessage}
      {toggleUserMessageExpanded}
      {compactSource}
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
      {permissionCommandPreview}
      {decidePermission}
      {formatToolName}
      {formatToolStatus}
    />
  {/each}
</div>
