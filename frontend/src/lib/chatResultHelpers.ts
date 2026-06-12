import type { AskResult, ChatMessage } from './appTypes';
import { asArray } from './uiHelpers';

export function visibleTextLength(value: string) {
  return String(value || '').replace(/\s+/g, '').length;
}

export function normalizedAssistantTraceLine(line: string) {
  return String(line || '').replace(/\s+/g, ' ').trim();
}

export function appendAssistantTraceLine(message: ChatMessage, line: string, limit = 10) {
  const clean = normalizedAssistantTraceLine(line);
  if (!clean) return message;
  const current = message.trace ?? [];
  if (current[current.length - 1] === clean) return message;
  return { ...message, trace: [...current, clean].slice(-limit) };
}

export function assistantResultMessage(result: AskResult, previous: ChatMessage | undefined): ChatMessage {
  return {
    id: result.assistantMessageId || previous?.id,
    role: 'assistant',
    content: result.text,
    model: result.model,
    skill: result.skill,
    sourceKind: result.sourceKind,
    sources: asArray(result.sources),
    parentId: result.parentMessageId || previous?.parentId,
    variantIndex: result.variantIndex || previous?.variantIndex,
    activeVariant: true,
    trace: [...(previous?.trace ?? []), ...asArray(result.toolCalls).map((name) => `model tool call: ${name}`)],
    capabilityHandoff: previous?.capabilityHandoff,
    schedulerHandoff: previous?.schedulerHandoff
  };
}
