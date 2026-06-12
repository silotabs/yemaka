<script lang="ts">
  import type { Snippet } from 'svelte';
  import ChatActivityPanel from '../ChatActivityPanel.svelte';
  import ChatSurface from '../ChatSurface.svelte';
  import type { ActivityEntry, ChatMessage, DomainPack, Extension, PermissionItem, SendAskOptions, ToolRun } from '../lib/appTypes';
  import type { ToolRunConversationGroup } from '../lib/toolRunHelpers';

  type PromptChip = {
    label: string;
    icon: string;
    prompt: string;
  };

  type Option = {
    value: string;
    label: string;
    disabled?: boolean;
    meta?: string;
  };

  type RenderSnippet = () => ReturnType<Snippet>;

  export let messages: ChatMessage[] = [];
  export let chatScroll: HTMLDivElement | null = null;
  export let weekdayLabel = '';
  export let prompt = '';
  export let composerFocused = false;
  export let composerTextarea: HTMLTextAreaElement | null = null;
  export let activeConversationId = '';
  export let busy = false;
  export let keepChatPinned = true;
  export let promptChips: PromptChip[] = [];
  export let queuedFollowUpList: RenderSnippet | null = null;
  export let composerCapabilityMenu: RenderSnippet | null = null;
  export let responseMode = 'balanced';
  export let responseModeOptions: Option[] = [];
  export let responseModeDisabled = false;
  export let setResponseMode: (value: string) => Promise<void> | void = () => {};
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
  export let activity: ActivityEntry[] = [];
  export let groupedToolRuns: Array<ToolRunConversationGroup<ToolRun>> = [];
  export let expandedToolRunGroups: Record<string, boolean> = {};
  export let listPages: Record<string, number> = {};
  export let handleChatScroll: () => void = () => {};
  export let scrollChatToBottom: (force?: boolean, behavior?: ScrollBehavior) => Promise<void> | void = () => {};
  export let focusComposer: () => void = () => {};
  export let blurComposer: () => void = () => {};
  export let resizeComposerTextarea: () => Promise<void> | void = () => {};
  export let handleComposerKeydown: (event: KeyboardEvent) => void = () => {};
  export let sendAsk: (contentOverride?: string | Event, options?: SendAskOptions) => Promise<boolean | void> | boolean | void = () => {};
  export let stopGeneration: () => Promise<void> | void = () => {};
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
  export let currentPage: (key: string, total: number) => number = () => 1;
  export let pagedItems: <T>(items: T[] | null | undefined, key: string, pageSize?: number, pages?: Record<string, number>) => T[] = (items) => items ?? [];
  export let setListPage: (key: string, page: number) => void = () => {};
  export let permissionCommandPreview: (item: PermissionItem) => string = () => '';
  export let decidePermission: (item: PermissionItem, decision: 'approved' | 'rejected') => Promise<void> | void = () => {};
  export let activityKey: (item: ActivityEntry, index: number) => string = (_item, index) => String(index);
  export let activityIcon: (kind: string) => string = () => 'chat';
  export let activityKind: (line: string) => string = () => 'system';
  export let toggleToolRunGroup: (key: string) => void = () => {};
  export let toolRunGroupExpanded: (id: string, total: number, expandedState?: Record<string, boolean>) => boolean = () => false;
  export let toolRunTimestamp: (run: ToolRun) => string | undefined = () => '';
  export let formatShortTime: (value: string | undefined) => string = (value) => String(value || '');
  export let formatToolName: (name: string | undefined) => string = (name) => String(name || '');
  export let formatToolStatus: (status: string) => string = (status) => status;
</script>

<div class="grid min-h-0 flex-1 grid-cols-1 xl:grid-cols-[minmax(0,1fr)_336px]">
  <ChatSurface
    {messages}
    bind:chatScroll
    {weekdayLabel}
    bind:prompt
    {composerFocused}
    bind:composerTextarea
    {activeConversationId}
    {busy}
    {keepChatPinned}
    {promptChips}
    {queuedFollowUpList}
    {composerCapabilityMenu}
    {responseMode}
    {responseModeOptions}
    {responseModeDisabled}
    {setResponseMode}
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
    {handleChatScroll}
    {scrollChatToBottom}
    {focusComposer}
    {blurComposer}
    {resizeComposerTextarea}
    {handleComposerKeydown}
    {sendAsk}
    {stopGeneration}
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
  <ChatActivityPanel
    {permissionItems}
    {permissionDecisionBusy}
    {activity}
    {groupedToolRuns}
    {expandedToolRunGroups}
    {listPages}
    {currentPage}
    {pagedItems}
    {setListPage}
    {permissionCommandPreview}
    {decidePermission}
    {activityKey}
    {activityIcon}
    {activityKind}
    {toggleToolRunGroup}
    {toolRunGroupExpanded}
    {toolRunTimestamp}
    {formatShortTime}
    {formatToolName}
    {formatToolStatus}
  />
</div>
