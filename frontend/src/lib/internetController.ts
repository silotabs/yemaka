import type { InternetCacheSummary, InternetCrawlRunRecord, InternetRequestRecord, InternetStatus } from './appTypes';
import { internetSearchProviderOption } from './appOptions';
import { allowedDomainsFromInput, crawlInternet, fetchInternet, internetActivityLabel, internetCrawlPreview, internetFetchPreview, loadInternetCrawlRun, loadInternetSurface } from './internetActions';
import { previewJSON } from './uiHelpers';

export type InternetControllerContext = {
  getSearchProvider: () => string;
  getUrl: () => string;
  getAllowedDomain: () => string;
  getExtractText: () => boolean;
  getTaskApproved: () => boolean;
  getCrawlMaxPages: () => number;
  getCrawlMaxDepth: () => number;
  getCrawlMaxDurationSeconds: () => number;
  getCrawlMaxLinksPerPage: () => number;
  getCrawlMaxTextChars: () => number;
  setStatus: (status: InternetStatus) => void;
  setRequests: (requests: InternetRequestRecord[]) => void;
  setCrawls: (crawls: InternetCrawlRunRecord[]) => void;
  setCache: (cache: InternetCacheSummary[]) => void;
  setFetchOutput: (output: string) => void;
  setError: (message: string) => void;
  pushActivity: (line: string) => void;
};

export function createInternetController(ctx: InternetControllerContext) {
  function internetSearchProviderLabel(statusValue: InternetStatus | null) {
    const provider = statusValue?.searchProvider || ctx.getSearchProvider() || 'none';
    if (provider === 'none' || !provider) return 'No provider';
    return statusValue?.searchProviderLabel || internetSearchProviderOption(provider).label;
  }

  async function refreshInternet() {
    ctx.setError('');
    try {
      const internetSurface = await loadInternetSurface(20);
      ctx.setStatus(internetSurface.status);
      ctx.setRequests(internetSurface.requests);
      ctx.setCrawls(internetSurface.crawls);
      ctx.setCache(internetSurface.cache);
      ctx.pushActivity(internetActivityLabel(internetSurface.status));
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function internetFetch(method: 'GET' | 'HEAD') {
    ctx.setError('');
    ctx.setFetchOutput('');
    try {
      const result = await fetchInternet(method, {
        url: ctx.getUrl(),
        extractText: ctx.getExtractText(),
        allowedDomains: allowedDomainsFromInput(ctx.getAllowedDomain()),
        taskApproved: ctx.getTaskApproved()
      });
      ctx.setFetchOutput(previewJSON(internetFetchPreview(result)));
      ctx.pushActivity(`internet ${method}: ${result.statusCode}`);
      await refreshInternet();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
      await refreshInternet();
    }
  }

  async function internetCrawl(method: 'GET' | 'HEAD' = 'GET') {
    ctx.setError('');
    ctx.setFetchOutput('');
    try {
      const result = await crawlInternet({
        seedUrl: ctx.getUrl(),
        method,
        extractText: ctx.getExtractText(),
        allowedDomains: allowedDomainsFromInput(ctx.getAllowedDomain()),
        maxPages: ctx.getCrawlMaxPages(),
        maxDepth: ctx.getCrawlMaxDepth(),
        maxDurationSeconds: ctx.getCrawlMaxDurationSeconds(),
        maxLinksPerPage: ctx.getCrawlMaxLinksPerPage(),
        maxTextChars: ctx.getCrawlMaxTextChars(),
        taskApproved: ctx.getTaskApproved()
      });
      ctx.setFetchOutput(previewJSON(internetCrawlPreview(result)));
      ctx.pushActivity(`internet crawl: ${result.status} (${result.fetched} pages)`);
      await refreshInternet();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
      await refreshInternet();
    }
  }

  async function inspectInternetCrawl(runId: string) {
    ctx.setError('');
    try {
      const run = await loadInternetCrawlRun(runId);
      ctx.setFetchOutput(previewJSON(run));
      ctx.pushActivity(`crawl inspect: ${run.status} (${run.fetched} pages)`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  return {
    internetSearchProviderLabel,
    refreshInternet,
    internetFetch,
    internetCrawl,
    inspectInternetCrawl
  };
}
