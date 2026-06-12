import type { InternetCacheSummary, InternetCrawlResult, InternetCrawlRunRecord, InternetFetchResult, InternetRequestRecord, InternetStatus } from './appTypes';
import { call } from './api';
import { asArray } from './uiHelpers';

export type InternetSurface = {
  status: InternetStatus;
  requests: InternetRequestRecord[];
  crawls: InternetCrawlRunRecord[];
  cache: InternetCacheSummary[];
};

export type InternetFetchInput = {
  url: string;
  extractText: boolean;
  allowedDomains: string[];
  taskApproved: boolean;
};

export type InternetCrawlInput = {
  seedUrl: string;
  method: 'GET' | 'HEAD';
  extractText: boolean;
  allowedDomains: string[];
  maxPages: number;
  maxDepth: number;
  maxDurationSeconds: number;
  maxLinksPerPage: number;
  maxTextChars: number;
  taskApproved: boolean;
};

export async function loadInternetSurface(requestLimit = 20): Promise<InternetSurface> {
  return {
    status: await call<InternetStatus>('InternetStatus'),
    requests: asArray(await call<InternetRequestRecord[]>('InternetRequests', requestLimit)),
    crawls: asArray(await call<InternetCrawlRunRecord[]>('InternetCrawls', requestLimit)),
    cache: asArray(await call<InternetCacheSummary[]>('InternetCache'))
  };
}

export function allowedDomainsFromInput(value: string) {
  return String(value || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
}

export async function fetchInternet(method: 'GET' | 'HEAD', input: InternetFetchInput) {
  return method === 'HEAD' ? await call<InternetFetchResult>('InternetHead', input) : await call<InternetFetchResult>('InternetFetch', input);
}

export async function crawlInternet(input: InternetCrawlInput) {
  return await call<InternetCrawlResult>('InternetCrawl', input);
}

export async function loadInternetCrawlRun(id: string) {
  return await call<InternetCrawlRunRecord>('InternetCrawlRun', id);
}

export function internetFetchPreview(result: InternetFetchResult) {
  return {
    url: result.url,
    statusCode: result.statusCode,
    contentType: result.contentType,
    bodyBytes: result.bodyBytes,
    fromCache: result.fromCache,
    text: result.extractedText || result.body
  };
}

export function internetCrawlPreview(result: InternetCrawlResult) {
  return {
    seedUrl: result.seedUrl,
    runId: result.runId,
    status: result.status,
    failureReason: result.failureReason,
    limits: {
      maxPages: result.maxPages,
      maxDepth: result.maxDepth,
      maxDurationSeconds: result.maxDurationSeconds,
      maxLinksPerPage: result.maxLinksPerPage,
      maxTextChars: result.maxTextChars
    },
    allowedDomains: result.allowedDomains,
    counts: {
      visited: result.visited,
      fetched: result.fetched,
      skipped: result.skipped,
      skipOverflow: result.skipOverflow ?? 0
    },
    pages: (result.pages || []).map((page) => ({
      url: page.url,
      finalUrl: page.finalUrl,
      depth: page.depth,
      statusCode: page.statusCode,
      contentType: page.contentType,
      bodyBytes: page.bodyBytes,
      fromCache: page.fromCache,
      links: page.links,
      text: page.extractedText
    })),
    skips: result.skips || []
  };
}

export function internetActivityLabel(status: InternetStatus) {
  return `internet: ${status.enabled ? status.defaultMode : 'disabled'}`;
}
