import { httpCall, jsonPost } from './api';
import type { ChatAttachment } from './appTypes';
import { visibleTextLength } from './chatResultHelpers';

export type ChatStreamResult = {
  text: string;
  model: string;
  conversationId: string;
  skill: string;
  sourceKind: string;
  sources: string[];
  toolCalls?: string[];
  streamedVisibleChars?: number;
  userMessageId?: string;
  assistantMessageId?: string;
  parentMessageId?: string;
  variantIndex?: number;
  attachments?: ChatAttachment[];
};

export type ChatStreamEvent = {
  type?: string;
  Type?: string;
  token?: string;
  Token?: string;
  message?: string;
  Message?: string;
  data?: Record<string, string>;
  Data?: Record<string, string>;
  result?: ChatStreamResult;
  Result?: ChatStreamResult;
  error?: string;
  Error?: string;
};

type StreamAskHTTPOptions = {
  content: string;
  skill: string;
  conversationId: string;
  parentMessageId?: string;
  attachmentIds?: string[];
  signal?: AbortSignal | null;
  onEvent?: (event: ChatStreamEvent) => void;
  yieldPaint?: (force?: boolean) => Promise<void>;
};

export async function streamAskHTTP(options: StreamAskHTTPOptions): Promise<ChatStreamResult> {
  const { content, skill, conversationId, parentMessageId = '', attachmentIds = [], signal, onEvent, yieldPaint } = options;
  const init = jsonPost({ content, skill, conversationId, parentMessageId, attachmentIds });
  const headers = new Headers(init.headers ?? {});
  headers.set('Accept', 'text/event-stream');
  init.headers = headers;
  if (signal) init.signal = signal;

  const response = await fetch('/api/ask/stream', init);
  if (!response.ok) {
    const payload = await response.json().catch(() => ({}));
    throw new Error(payload.error || 'Ask stream failed');
  }
  if (!response.body) {
    return await httpCall<ChatStreamResult>('Ask', content, skill, conversationId, parentMessageId, attachmentIds);
  }

  const result: ChatStreamResult = {
    text: '',
    model: '',
    conversationId: '',
    skill: '',
    sourceKind: '',
    sources: [],
    toolCalls: []
  };
  let finalResult: ChatStreamResult | null = null;
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let streamedVisibleChars = 0;
  let lastStreamPaint = 0;

  const maybeYieldPaint = async (force = false) => {
    if (!yieldPaint) return;
    const now = performance.now();
    if (!force && now - lastStreamPaint < 32) return;
    lastStreamPaint = now;
    await yieldPaint(force);
  };

  const consumeEvent = (raw: unknown) => {
    const event = normalizeStreamEvent(raw);
    if (event.error) {
      throw new Error(event.error);
    }
    if (event.type === 'result' && event.result) {
      finalResult = event.result;
      return;
    }
    if (event.type === 'model.token' && event.token) {
      result.text += event.token;
      streamedVisibleChars += visibleTextLength(event.token);
    }
    if (event.type === 'model.selected' && event.data?.model) {
      result.model = event.data.model;
    }
    if (event.type === 'workspace.used') {
      result.sourceKind = event.data?.source_kind || result.sourceKind;
      result.sources = String(event.data?.sources || '')
        .split(',')
        .map((source) => source.trim())
        .filter(Boolean);
    }
    if (event.type === 'tool.completed') {
      const toolSources = String(event.data?.sources || '')
        .split(',')
        .map((source) => source.trim())
        .filter(Boolean);
      if (toolSources.length > 0) {
        result.sourceKind = event.data?.source_kind || result.sourceKind;
        result.sources = toolSources;
      }
    }
    if (event.type === 'skill.selected' && event.data?.name) {
      result.skill = event.data.name;
    }
    if (event.type === 'message.saved') {
      const data = event.data ?? {};
      if (data.conversation_id) {
        result.conversationId = data.conversation_id;
      }
      if (data.role === 'user') {
        result.userMessageId = data.message_id || result.userMessageId;
      }
      if (data.role === 'assistant') {
        result.assistantMessageId = data.message_id || result.assistantMessageId;
        result.parentMessageId = data.parent_message_id || result.parentMessageId;
        result.variantIndex = Number(data.variant_index || result.variantIndex || 0);
        if (data.content) {
          result.text = data.content;
        }
      }
    }
    if (event.type === 'agent.completed' && event.data?.conversation_id) {
      result.conversationId = event.data.conversation_id;
    }
    if (event.type === 'model.tool_call') {
      result.toolCalls = [
        ...(result.toolCalls ?? []),
        ...String(event.data?.names || '')
          .split(',')
          .map((name) => name.trim())
          .filter(Boolean)
      ];
    }
    onEvent?.(event);
    return event.type === 'model.token' && Boolean(event.token);
  };

  const consumeJSONPayload = (payload: string) => {
    const trimmed = payload.trim();
    if (!trimmed) return false;
    return consumeEvent(JSON.parse(trimmed));
  };

  const consumeSSEFrame = (frame: string) => {
    const data: string[] = [];
    for (const rawLine of frame.split(/\r?\n/)) {
      const line = rawLine.trimEnd();
      if (!line || line.startsWith(':')) continue;
      if (line.startsWith('data:')) {
        data.push(line.slice(5).trimStart());
      }
    }
    if (data.length > 0) {
      return consumeJSONPayload(data.join('\n'));
    }
    if (frame.trim().startsWith('{')) {
      return consumeJSONPayload(frame);
    }
    return false;
  };

  const streamType = response.headers.get('Content-Type') ?? '';
  const isSSE = streamType.includes('text/event-stream');

  while (true) {
    const { value, done } = await reader.read();
    buffer += decoder.decode(value ?? new Uint8Array(), { stream: !done });
    if (isSSE) {
      let boundary = nextSSEBoundary(buffer);
      while (boundary) {
        const frame = buffer.slice(0, boundary.index);
        buffer = buffer.slice(boundary.index + boundary.length);
        if (consumeSSEFrame(frame)) {
          await maybeYieldPaint();
        }
        boundary = nextSSEBoundary(buffer);
      }
    } else {
      const lines = buffer.split('\n');
      buffer = lines.pop() ?? '';
      for (const line of lines) {
        if (consumeJSONPayload(line)) {
          await maybeYieldPaint();
        }
      }
    }
    if (done) break;
  }
  if (buffer.trim()) {
    if (isSSE) {
      if (consumeSSEFrame(buffer)) {
        await maybeYieldPaint(true);
      }
    } else {
      if (consumeJSONPayload(buffer)) {
        await maybeYieldPaint(true);
      }
    }
  }
  const completed = finalResult ?? result;
  completed.streamedVisibleChars = streamedVisibleChars;
  return completed;
}

function normalizeStreamEvent(event: unknown): ChatStreamEvent {
  const raw = (event || {}) as ChatStreamEvent;
  return {
    type: raw.type ?? raw.Type ?? '',
    token: raw.token ?? raw.Token ?? '',
    message: raw.message ?? raw.Message ?? '',
    data: raw.data ?? raw.Data ?? {},
    result: raw.result ?? raw.Result,
    error: raw.error ?? raw.Error ?? ''
  };
}

function nextSSEBoundary(input: string) {
  const lf = input.indexOf('\n\n');
  const crlf = input.indexOf('\r\n\r\n');
  if (lf === -1 && crlf === -1) return null;
  if (lf === -1) return { index: crlf, length: 4 };
  if (crlf === -1) return { index: lf, length: 2 };
  return crlf < lf ? { index: crlf, length: 4 } : { index: lf, length: 2 };
}
