import type { AskResult, ChatAttachmentUploadResult } from './appTypes';
import { call, desktopAPI, isLocalWebPage } from './api';

export async function askOnce(content: string, skill: string, conversationId: string, parentMessageId = '', attachmentIds: string[] = []) {
  return await call<AskResult>('Ask', content, skill, conversationId, parentMessageId, attachmentIds);
}

export async function uploadChatAttachment(file: File) {
  if (desktopAPI()?.UploadChatAttachment && !isLocalWebPage()) {
    return await call<ChatAttachmentUploadResult>('UploadChatAttachment', {
      fileName: file.name,
      contentType: file.type || 'application/octet-stream',
      dataBase64: await fileToBase64(file),
      retention: 'conversation'
    });
  }
  return await call<ChatAttachmentUploadResult>('UploadChatAttachment', file);
}

async function fileToBase64(file: File) {
  const buffer = await file.arrayBuffer();
  let binary = '';
  const bytes = new Uint8Array(buffer);
  const chunkSize = 0x8000;
  for (let cursor = 0; cursor < bytes.length; cursor += chunkSize) {
    const chunk = bytes.subarray(cursor, cursor + chunkSize);
    binary += String.fromCharCode(...chunk);
  }
  return btoa(binary);
}
