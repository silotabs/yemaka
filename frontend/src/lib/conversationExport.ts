import type { ConversationDetail } from './appTypes';
import { promptTitle } from './uiHelpers';

export type ConversationExportFormat = 'markdown' | 'json';

export function exportConversationDetail(detail: ConversationDetail, format: ConversationExportFormat) {
  const title = promptTitle(detail.conversation.title || 'conversation');
  const fileBase = safeExportFileName(title || detail.conversation.id || 'conversation');
  const exportedAt = new Date().toISOString();
  if (format === 'json') {
    downloadTextFile(`${fileBase}.json`, JSON.stringify(conversationExportJSON(detail, exportedAt), null, 2), 'application/json;charset=utf-8');
    return;
  }
  downloadTextFile(`${fileBase}.md`, conversationExportMarkdown(detail, exportedAt), 'text/markdown;charset=utf-8');
}

export function conversationExportJSON(detail: ConversationDetail, exportedAt: string) {
  return {
    schema: 'yemaka.conversation.export.v1',
    exportedAt,
    conversation: detail.conversation,
    messages: detail.messages
  };
}

export function conversationExportMarkdown(detail: ConversationDetail, exportedAt: string) {
  const conversation = detail.conversation;
  const lines = [
    `# ${escapeMarkdownHeading(promptTitle(conversation.title || 'Conversation'))}`,
    '',
    `- Conversation ID: ${conversation.id}`,
    `- Created: ${conversation.createdAt || 'unknown'}`,
    `- Updated: ${conversation.updatedAt || 'unknown'}`,
    `- Exported: ${exportedAt}`,
    ''
  ];
  for (const message of detail.messages) {
    lines.push(`## ${roleLabel(message.role)}`);
    if (message.createdAt) lines.push(`_Time: ${message.createdAt}_`);
    if (message.model) lines.push(`_Model: ${message.model}_`);
    if (message.sourceKind) lines.push(`_Source: ${message.sourceKind}_`);
    if (message.sources?.length) lines.push(`_Sources: ${message.sources.join(', ')}_`);
    if (message.attachments?.length) {
      lines.push(`_Attachments: ${message.attachments.map((item) => `${item.fileName} (${item.status})`).join(', ')}_`);
    }
    lines.push('', message.content || '_Empty message_', '');
  }
  return lines.join('\n').trimEnd() + '\n';
}

function roleLabel(role: string) {
  if (role === 'assistant') return 'Assistant';
  if (role === 'user') return 'User';
  if (role === 'system') return 'System';
  return promptTitle(role || 'Message');
}

function safeExportFileName(value: string) {
  const clean = value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 80);
  return clean || 'conversation';
}

function escapeMarkdownHeading(value: string) {
  return value.replace(/[#\n\r]/g, ' ').replace(/\s+/g, ' ').trim();
}

function downloadTextFile(fileName: string, content: string, type: string) {
  const blob = new Blob([content], { type });
  const href = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = href;
  link.download = fileName;
  link.rel = 'noopener';
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.setTimeout(() => URL.revokeObjectURL(href), 0);
}
