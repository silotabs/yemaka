package internet

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yemaka/internal/config"
)

func TestFetchRequiresDisabledApproval(t *testing.T) {
	service := testService(t, config.Default().Internet)
	_, err := service.Fetch(context.Background(), FetchInput{URL: "https://example.com"})
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("Fetch() error = %v, want disabled error", err)
	}
	requests, readErr := service.Requests(10)
	if readErr != nil {
		t.Fatalf("Requests() error = %v", readErr)
	}
	if len(requests) != 1 || requests[0].Allowed {
		t.Fatalf("requests = %+v, want blocked log", requests)
	}
}

func TestFetchGETCachesAndExtractsText(t *testing.T) {
	calls := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return response(200, "text/html", `<html><head><style>.x{}</style></head><body><h1>Hello</h1><script>bad()</script><p>World</p></body></html>`), nil
	})

	cfg := enabledTestConfig()
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: transport}
	result, err := service.Fetch(context.Background(), FetchInput{
		URL:            "https://example.com/page",
		ExtractText:    true,
		AllowedDomains: []string{"example.com"},
		TaskApproved:   true,
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.StatusCode != 200 || result.FromCache || result.BodyBytes == 0 {
		t.Fatalf("result = %+v, want fresh 200 body", result)
	}
	if result.ExtractedText != "Hello World" {
		t.Fatalf("ExtractedText = %q, want Hello World", result.ExtractedText)
	}
	cached, err := service.Fetch(context.Background(), FetchInput{
		URL:            "https://example.com/page",
		ExtractText:    true,
		AllowedDomains: []string{"example.com"},
		TaskApproved:   true,
	})
	if err != nil {
		t.Fatalf("Fetch(cache) error = %v", err)
	}
	if !cached.FromCache || calls != 1 {
		t.Fatalf("cached=%+v calls=%d, want cache hit", cached, calls)
	}
	cacheItems, err := service.CacheList()
	if err != nil {
		t.Fatalf("CacheList() error = %v", err)
	}
	if len(cacheItems) != 1 || cacheItems[0].Expired {
		t.Fatalf("cacheItems = %+v, want one unexpired", cacheItems)
	}
}

func TestHeadDoesNotCacheBody(t *testing.T) {
	service := testService(t, enabledTestConfig())
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(200, "text/plain", "hello"), nil
	})}
	result, err := service.Head(context.Background(), FetchInput{
		URL:            "https://example.com/head",
		AllowedDomains: []string{"example.com"},
		TaskApproved:   true,
	})
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}
	if result.BodyBytes != 0 || result.Body != "" {
		t.Fatalf("HEAD result = %+v, want empty body", result)
	}
}

func TestPolicyBlocksPrivateLocalAndDomainMismatch(t *testing.T) {
	cfg := config.Default().Internet
	cfg.Enabled = true
	service := testService(t, cfg)

	_, err := service.Fetch(context.Background(), FetchInput{URL: "http://127.0.0.1:1234", TaskApproved: true})
	if err == nil || !strings.Contains(err.Error(), "local") {
		t.Fatalf("Fetch(private) error = %v, want private/local block", err)
	}

	cfg.Policy.BlockPrivateIPRanges = false
	cfg.Policy.BlockLocalNetworkByDefault = false
	service = testService(t, cfg)
	_, err = service.Fetch(context.Background(), FetchInput{
		URL:            "https://example.com",
		AllowedDomains: []string{"allowed.example"},
		TaskApproved:   true,
	})
	if err == nil || !strings.Contains(err.Error(), "outside the task allowlist") {
		t.Fatalf("Fetch(domain mismatch) error = %v, want allowlist error", err)
	}
}

func TestResponseSizeLimitAndSearchDisabled(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.MaxResponseBytes = 3
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(200, "text/plain", "too large"), nil
	})}
	_, err := service.Fetch(context.Background(), FetchInput{
		URL:            "https://example.com/large",
		AllowedDomains: []string{"example.com"},
		TaskApproved:   true,
	})
	if err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("Fetch(limit) error = %v, want size limit", err)
	}

	_, err = service.Search(context.Background(), SearchInput{Query: "hello"})
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("Search() error = %v, want disabled search", err)
	}
}

func TestSearchProviderNotConfiguredDoesNotDialNetwork(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "none"
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request to %s", r.URL.String())
		return nil, nil
	})}

	status := service.Status()
	if status.SearchProviderReady || status.SearchProviderConfigured || !status.SearchProviderNeedsConfig {
		t.Fatalf("Status() = %+v, want unconfigured search provider", status)
	}
	if !strings.Contains(status.SearchStatus, "explicitly configured") {
		t.Fatalf("SearchStatus = %q, want provider configuration guidance", status.SearchStatus)
	}

	_, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err == nil || !strings.Contains(err.Error(), "search provider is explicitly configured") {
		t.Fatalf("Search() error = %v, want provider-not-configured error", err)
	}
}

func TestSearchSearXNGProviderUsesFetchPolicyAndCache(t *testing.T) {
	calls := 0
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "searxng"
	cfg.Search.Endpoint = "https://search.example/search"
	cfg.Search.MaxResults = 2
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Hostname() != "search.example" {
			t.Fatalf("host = %s, want search.example", r.URL.Hostname())
		}
		if r.URL.Query().Get("q") != "local agents" || r.URL.Query().Get("format") != "json" {
			t.Fatalf("query = %s, want q and format=json", r.URL.RawQuery)
		}
		body := `{"results":[{"title":"One","url":"https://example.com/one","content":"First","engine":"test"},{"title":"Two","url":"https://example.com/two","content":"Second","engine":"test"},{"title":"Three","url":"https://example.com/three","content":"Third","engine":"test"}]}`
		return response(200, "application/json", body), nil
	})}

	result, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Provider != "searxng" || len(result.Items) != 2 || result.Items[0].Title != "One" {
		t.Fatalf("Search() = %+v, want two searxng items", result)
	}
	cached, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err != nil {
		t.Fatalf("Search(cache) error = %v", err)
	}
	if !cached.FromCache || calls != 1 {
		t.Fatalf("cached=%+v calls=%d, want cache hit", cached, calls)
	}
}

func TestSearchCacheKeepsDifferentQueriesSeparate(t *testing.T) {
	calls := 0
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "searxng"
	cfg.Search.Endpoint = "https://search.example/search"
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		query := r.URL.Query().Get("q")
		title := "Alpha"
		if query == "second query" {
			title = "Beta"
		}
		body := `{"results":[{"title":"` + title + `","url":"https://example.com/` + strings.ToLower(title) + `","content":"` + query + `","engine":"test"}]}`
		return response(200, "application/json", body), nil
	})}

	first, err := service.Search(context.Background(), SearchInput{Query: "first query", TaskApproved: true})
	if err != nil {
		t.Fatalf("Search(first) error = %v", err)
	}
	second, err := service.Search(context.Background(), SearchInput{Query: "second query", TaskApproved: true})
	if err != nil {
		t.Fatalf("Search(second) error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want separate network requests for distinct query URLs", calls)
	}
	if first.FromCache || second.FromCache {
		t.Fatalf("first=%+v second=%+v, did not expect cross-query cache hit", first, second)
	}
	if len(first.Items) == 0 || first.Items[0].Title != "Alpha" || len(second.Items) == 0 || second.Items[0].Title != "Beta" {
		t.Fatalf("first=%+v second=%+v, want query-specific results", first, second)
	}
}

func TestSearchAPIProviderMissingSecretDoesNotDialNetwork(t *testing.T) {
	const apiKeyEnv = "YEMAKA_TEST_BRAVE_SEARCH_API_KEY_MISSING"
	t.Setenv(apiKeyEnv, "")
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "brave"
	cfg.Search.APIKeyEnv = apiKeyEnv
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request to %s", r.URL.String())
		return nil, nil
	})}

	status := service.Status()
	if status.SearchProviderReady || !status.SearchProviderConfigured || !status.SearchProviderNeedsAuth {
		t.Fatalf("Status() = %+v, want configured provider needing auth", status)
	}
	if !strings.Contains(status.SearchStatus, apiKeyEnv) || !strings.Contains(status.SearchStatus, "not set") {
		t.Fatalf("SearchStatus = %q, want missing secret environment guidance", status.SearchStatus)
	}

	_, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err == nil || !strings.Contains(err.Error(), apiKeyEnv) || !strings.Contains(err.Error(), "not set") {
		t.Fatalf("Search() error = %v, want missing API key env error", err)
	}
}

func TestSearchProviderHTTPErrorIsHumanReadable(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "searxng"
	cfg.Search.Endpoint = "https://search.example/search"
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(http.StatusTooManyRequests, "text/plain", "Too Many Requests"), nil
	})}

	_, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err == nil {
		t.Fatal("Search() error = nil, want provider HTTP error")
	}
	if !strings.Contains(err.Error(), "HTTP 429") || !strings.Contains(err.Error(), "Too Many Requests") {
		t.Fatalf("Search() error = %v, want readable HTTP 429 message", err)
	}
	if strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("Search() error = %v, should not expose raw JSON parse error", err)
	}
}

func TestSearchProviderNetworkErrorIsHumanReadable(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "searxng"
	cfg.Search.Endpoint = "https://search.example/search"
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp 127.0.0.1:8888: connect: connection refused")
	})}

	_, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err == nil {
		t.Fatal("Search() error = nil, want provider network error")
	}
	if !strings.Contains(err.Error(), "could not reach SearXNG search provider") || !strings.Contains(err.Error(), "verify SearXNG is running") {
		t.Fatalf("Search() error = %v, want SearXNG reachability guidance", err)
	}
	if strings.Contains(err.Error(), "q=local+agents") {
		t.Fatalf("Search() error = %v, should not echo full search query URL", err)
	}
	requests, readErr := service.Requests(10)
	if readErr != nil {
		t.Fatalf("Requests() error = %v", readErr)
	}
	if len(requests) != 1 || !requests[0].Allowed || requests[0].Error == "" {
		t.Fatalf("requests = %+v, want logged failed provider request", requests)
	}
}

func TestSearchProviderRequiresConfiguredEndpoint(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "searxng"
	service := testService(t, cfg)

	_, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err == nil || !strings.Contains(err.Error(), "endpoint") {
		t.Fatalf("Search() error = %v, want endpoint error", err)
	}
}

func TestStatusReportsSearchProviderReadiness(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.Enabled = false
	cfg.Search.Enabled = true
	cfg.Search.Provider = "searxng"
	cfg.Search.Endpoint = "https://search.example/search"
	status := testService(t, cfg).Status()
	if status.SearchProviderReady || !strings.Contains(status.SearchStatus, "internet access is disabled") {
		t.Fatalf("Status() = %+v, want disabled internet search status", status)
	}

	cfg.Enabled = true
	status = testService(t, cfg).Status()
	if !status.SearchProviderReady || status.SearchStatus != "search provider is ready" {
		t.Fatalf("Status() = %+v, want ready searxng provider", status)
	}

	cfg.Search.Endpoint = ""
	status = testService(t, cfg).Status()
	if status.SearchProviderReady || !strings.Contains(status.SearchStatus, "internet.search.endpoint") {
		t.Fatalf("Status() = %+v, want missing endpoint status", status)
	}
}

func TestSearchProviderReadinessDoesNotProbeNetwork(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "searxng"
	cfg.Search.Endpoint = "https://search.example/search"

	status := testService(t, cfg).Status()
	if !status.SearchProviderReady || !status.SearchProviderConfigured {
		t.Fatalf("Status() = %+v, want configured provider readiness", status)
	}
	if status.SearchNetworkChecked {
		t.Fatalf("Status() = %+v, readiness must not perform live network checks", status)
	}
}

func TestSearchStatusReportsStateHealthFallbackAndRequestEvidence(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "searxng"
	cfg.Search.Endpoint = "https://search.example/search"
	service := testService(t, cfg)

	status := service.Status()
	if status.SearchState != "configured" || status.SearchHealthScore != 70 {
		t.Fatalf("Status() = %+v, want configured search health without runtime evidence", status)
	}
	if len(status.SearchFallbackReadiness) == 0 {
		t.Fatalf("SearchFallbackReadiness empty, want provider readiness list")
	}
	if err := service.logRequest(RequestRecord{
		Timestamp:  service.timestamp(),
		Caller:     "internet_search",
		Method:     http.MethodGet,
		URL:        "https://search.example/search",
		Host:       "search.example",
		Provider:   "searxng",
		Allowed:    true,
		StatusCode: http.StatusOK,
	}); err != nil {
		t.Fatalf("log successful request: %v", err)
	}
	status = service.Status()
	if status.SearchState != "healthy" || status.SearchHealthScore != 100 {
		t.Fatalf("Status() = %+v, want healthy search health from recent success", status)
	}
	if err := service.logRequest(RequestRecord{
		Timestamp:  service.timestamp(),
		Caller:     "internet_search",
		Method:     http.MethodGet,
		URL:        "https://search.example/search",
		Host:       "search.example",
		Provider:   "searxng",
		Allowed:    true,
		StatusCode: http.StatusTooManyRequests,
		Error:      "SearXNG search provider quota or rate limit reached",
	}); err != nil {
		t.Fatalf("log failed request: %v", err)
	}
	status = service.Status()
	if status.SearchState != "degraded" || status.SearchHealthScore != 45 {
		t.Fatalf("Status() = %+v, want degraded search health from latest failure", status)
	}
}

func TestStatusReportsCacheFreshAndStaleCounts(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.Cache.Enabled = true
	cfg.Cache.TTLSeconds = 10
	service := testService(t, cfg)
	start := service.now()
	if err := service.cacheSet(http.MethodGet, "https://example.com/stale", FetchResult{StatusCode: http.StatusOK, Body: "old", BodyBytes: 3}); err != nil {
		t.Fatalf("cacheSet(stale) error = %v", err)
	}
	service.Now = func() time.Time { return start.Add(20 * time.Second) }
	if err := service.cacheSet(http.MethodGet, "https://example.com/fresh", FetchResult{StatusCode: http.StatusOK, Body: "new", BodyBytes: 3}); err != nil {
		t.Fatalf("cacheSet(fresh) error = %v", err)
	}

	items, err := service.CacheList()
	if err != nil {
		t.Fatalf("CacheList() error = %v", err)
	}
	var fresh, stale int
	for _, item := range items {
		if item.State == "fresh" {
			fresh++
		}
		if item.State == "stale" {
			stale++
		}
		if item.AgeSeconds < 0 {
			t.Fatalf("cache item age = %d, want non-negative: %+v", item.AgeSeconds, item)
		}
	}
	if fresh != 1 || stale != 1 {
		t.Fatalf("cache states fresh=%d stale=%d items=%+v", fresh, stale, items)
	}
	status := service.Status()
	if status.CacheFreshEntries != 1 || status.CacheStaleEntries != 1 || status.CacheLatestCachedAt == "" {
		t.Fatalf("Status cache counts = %+v, want one fresh and one stale", status)
	}
}

func TestPostRCSearchProviderNormalization(t *testing.T) {
	tests := map[string]string{
		"wikipedia":            "wikimedia",
		"mediawiki":            "wikimedia",
		"ddg":                  "duckduckgo",
		"firecrawl_search":     "firecrawl",
		"custom":               "yemaka_custom",
		"custom_search":        "yemaka_custom",
		"yemaka_custom_search": "yemaka_custom",
		"brave_search":         "brave",
		"serper.dev":           "serper",
		"google_serper":        "serper",
	}
	for input, want := range tests {
		if got := normalizeSearchProvider(input); got != want {
			t.Fatalf("normalizeSearchProvider(%q) = %q, want %q", input, got, want)
		}
		if _, ok := searchProviderForName(input); !ok {
			t.Fatalf("searchProviderForName(%q) not found", input)
		}
	}
	if got := normalizeSearchProvider("auto_fallback"); got != "auto" {
		t.Fatalf("normalizeSearchProvider(%q) = %q, want auto", "auto_fallback", got)
	}
}

func TestSearchProviderStatusReportsLabelsAndSanitizesSecretEndpoint(t *testing.T) {
	tests := []struct {
		provider string
		label    string
	}{
		{"searxng", "SearXNG"},
		{"brave", "Brave"},
		{"wikimedia", "Wikimedia"},
		{"tavily", "Tavily"},
		{"serper", "Serper.dev"},
		{"duckduckgo", "DuckDuckGo"},
		{"firecrawl", "Firecrawl"},
		{"mojeek", "Mojeek"},
		{"yemaka_custom", "Yemaka Custom Search"},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			cfg := enabledTestConfig()
			cfg.Search.Enabled = true
			cfg.Search.Provider = tt.provider
			cfg.Search.Endpoint = "https://search.example/api?api_key=secret&profile=local"
			status := testService(t, cfg).Status()
			if status.SearchProviderLabel != tt.label {
				t.Fatalf("SearchProviderLabel = %q, want %q", status.SearchProviderLabel, tt.label)
			}
			if strings.Contains(status.SearchEndpoint, "secret") || strings.Contains(status.SearchEndpoint, "api_key") {
				t.Fatalf("SearchEndpoint = %q, want secret query stripped", status.SearchEndpoint)
			}
		})
	}
}

func TestSearchWikimediaProviderUsesSafeGETPlumbing(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "wikimedia"
	cfg.Search.MaxResults = 2
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Hostname() != "en.wikipedia.org" || r.URL.Path != "/w/api.php" {
			t.Fatalf("url = %s, want Wikimedia API endpoint", r.URL.String())
		}
		query := r.URL.Query()
		if query.Get("srsearch") != "local agents" || query.Get("format") != "json" || query.Get("srlimit") != "2" {
			t.Fatalf("query = %s, want Wikimedia search parameters", r.URL.RawQuery)
		}
		body := `{"query":{"search":[{"title":"Local agent","pageid":42,"snippet":"A <span>local</span> result"}]}}`
		return response(200, "application/json", body), nil
	})}

	status := service.Status()
	if !status.SearchProviderReady || status.SearchProviderNeedsAuth || status.SearchProviderNeedsConfig {
		t.Fatalf("Status() = %+v, want ready Wikimedia provider", status)
	}
	result, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Provider != "wikimedia" || len(result.Items) != 1 {
		t.Fatalf("Search() = %+v, want one Wikimedia item", result)
	}
	item := result.Items[0]
	if item.Title != "Local agent" || item.URL != "https://en.wikipedia.org/wiki/Local%20agent" || item.Snippet != "A local result" {
		t.Fatalf("item = %+v, want normalized Wikimedia item", item)
	}
}

func TestSearchDuckDuckGoProviderUsesDefaultEndpointAndParsesInstantAnswer(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "duckduckgo"
	cfg.Search.MaxResults = 3
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Hostname() != "api.duckduckgo.com" {
			t.Fatalf("host = %s, want api.duckduckgo.com", r.URL.Hostname())
		}
		if r.URL.Query().Get("q") != "local agents" || r.URL.Query().Get("format") != "json" {
			t.Fatalf("query = %s, want DuckDuckGo Instant Answer parameters", r.URL.RawQuery)
		}
		body := `{"Heading":"Local agent","Abstract":"A local-first agent.","AbstractURL":"https://example.com/abstract","RelatedTopics":[{"Text":"Yemaka - local agent system","FirstURL":"https://example.com/yemaka"}]}`
		return response(200, "application/json", body), nil
	})}

	result, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Provider != "duckduckgo" || len(result.Items) != 2 {
		t.Fatalf("Search() = %+v, want DuckDuckGo items from default endpoint", result)
	}
	if result.Items[0].Title != "Local agent" || result.Items[1].Title != "Yemaka" {
		t.Fatalf("items = %+v, want normalized DuckDuckGo items", result.Items)
	}
}

func TestSearchFirecrawlProviderUsesVendorPostAndParsesResults(t *testing.T) {
	const apiKeyEnv = "YEMAKA_TEST_FIRECRAWL_KEY"
	t.Setenv(apiKeyEnv, "secret")
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "firecrawl"
	cfg.Search.APIKeyEnv = apiKeyEnv
	cfg.Search.MaxResults = 2
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.String() != "https://api.firecrawl.dev/v2/search" {
			t.Fatalf("url = %s, want Firecrawl search endpoint", r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("headers = %+v, want Firecrawl auth and JSON content type", r.Header)
		}
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Fatalf("read body: %v", readErr)
		}
		if !strings.Contains(string(body), `"query":"local agents"`) || !strings.Contains(string(body), `"limit":2`) {
			t.Fatalf("body = %s, want query and limit", string(body))
		}
		return response(200, "application/json", `{"success":true,"data":{"web":[{"title":"One","url":"https://example.com/one","description":"First"},{"title":"Two","url":"https://example.com/two","description":"Second"}]}}`), nil
	})}

	status := service.Status()
	if !status.SearchProviderReady || status.SearchProviderNeedsAuth || status.SearchProviderNeedsConfig {
		t.Fatalf("Status() = %+v, want ready Firecrawl provider", status)
	}
	result, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Provider != "firecrawl" || len(result.Items) != 2 || result.Items[0].Source != "firecrawl" {
		t.Fatalf("Search() = %+v, want Firecrawl results", result)
	}
}

func TestSearchTavilyProviderUsesVendorPostAndParsesResults(t *testing.T) {
	const apiKeyEnv = "YEMAKA_TEST_TAVILY_KEY"
	t.Setenv(apiKeyEnv, "secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("headers = %+v, want Tavily bearer auth and JSON content type", r.Header)
		}
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Fatalf("read body: %v", readErr)
		}
		if !strings.Contains(string(body), `"query":"local agents"`) || !strings.Contains(string(body), `"max_results":2`) {
			t.Fatalf("body = %s, want Tavily query and max_results", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"One","url":"https://example.com/one","content":"First"},{"title":"Two","url":"https://example.com/two","content":"Second"}]}`))
	}))
	defer server.Close()

	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "tavily"
	cfg.Search.Endpoint = server.URL
	cfg.Search.APIKeyEnv = apiKeyEnv
	cfg.Search.MaxResults = 2
	service := testService(t, cfg)
	status := service.Status()
	if !status.SearchProviderReady || status.SearchProviderNeedsAuth || status.SearchProviderNeedsConfig {
		t.Fatalf("Status() = %+v, want ready Tavily provider", status)
	}
	result, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Provider != "tavily" || len(result.Items) != 2 || result.Items[0].Source != "tavily" {
		t.Fatalf("Search() = %+v, want Tavily results", result)
	}
}

func TestSearchSerperProviderUsesVendorPostAndParsesResults(t *testing.T) {
	const apiKeyEnv = "YEMAKA_TEST_SERPER_KEY"
	t.Setenv(apiKeyEnv, "secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("X-API-KEY") != "secret" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("headers = %+v, want Serper API key and JSON content type", r.Header)
		}
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Fatalf("read body: %v", readErr)
		}
		if !strings.Contains(string(body), `"q":"local agents"`) || !strings.Contains(string(body), `"num":2`) {
			t.Fatalf("body = %s, want Serper query and num", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"organic":[{"title":"One","link":"https://example.com/one","snippet":"First"},{"title":"Two","link":"https://example.com/two","snippet":"Second"}]}`))
	}))
	defer server.Close()

	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "serper"
	cfg.Search.Endpoint = server.URL
	cfg.Search.APIKeyEnv = apiKeyEnv
	cfg.Search.MaxResults = 2
	service := testService(t, cfg)
	status := service.Status()
	if !status.SearchProviderReady || status.SearchProviderNeedsAuth || status.SearchProviderNeedsConfig {
		t.Fatalf("Status() = %+v, want ready Serper provider", status)
	}
	result, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Provider != "serper" || len(result.Items) != 2 || result.Items[0].Source != "serper" {
		t.Fatalf("Search() = %+v, want Serper results", result)
	}
}

func TestAutoSearchProviderFallsBackToFirstReadyProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Query().Get("q") != "local agents" {
			t.Fatalf("query = %s, want local agents", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Results":[{"Text":"Local agent - result","FirstURL":"https://example.com/one"}]}`))
	}))
	defer server.Close()

	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "auto"
	cfg.Search.FallbackProviders = []string{"tavily", "duckduckgo"}
	cfg.Search.Endpoint = server.URL
	cfg.Search.APIKeyEnv = "YEMAKA_TEST_MISSING_SEARCH_KEY"
	cfg.Search.MaxResults = 2
	service := testService(t, cfg)
	status := service.Status()
	if !status.SearchProviderReady || !strings.Contains(status.SearchStatus, "DuckDuckGo") {
		t.Fatalf("Status() = %+v, want auto readiness through DuckDuckGo", status)
	}
	result, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Provider != "duckduckgo" || len(result.Items) != 1 {
		t.Fatalf("Search() = %+v, want DuckDuckGo fallback result", result)
	}
}

func TestSearchProviderHTTPErrorExplainsQuotaAndEndpointFailures(t *testing.T) {
	const apiKeyEnv = "YEMAKA_TEST_SERPER_KEY"
	t.Setenv(apiKeyEnv, "secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"quota exceeded"}`))
	}))
	defer server.Close()

	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "serper"
	cfg.Search.Endpoint = server.URL
	cfg.Search.APIKeyEnv = apiKeyEnv
	service := testService(t, cfg)
	_, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err == nil || !strings.Contains(err.Error(), "quota or rate limit") || !strings.Contains(err.Error(), "HTTP 429") {
		t.Fatalf("Search() error = %v, want quota/rate-limit message", err)
	}
}

func TestConfiguredGETSearchProvidersRequireEndpointAndBuildFetchInput(t *testing.T) {
	providers := []string{"yemaka_custom"}
	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			cfg := enabledTestConfig()
			cfg.Search.Enabled = true
			cfg.Search.Provider = provider
			cfg.Search.MaxResults = 4
			service := testService(t, cfg)

			status := service.Status()
			if status.SearchProviderReady || !status.SearchProviderNeedsConfig || !strings.Contains(status.SearchStatus, "internet.search.endpoint") {
				t.Fatalf("Status() = %+v, want missing endpoint guidance", status)
			}

			cfg.Search.Endpoint = "https://search.example/api"
			service = testService(t, cfg)
			status = service.Status()
			if !status.SearchProviderReady || status.SearchStatus != "search provider is ready" {
				t.Fatalf("Status() = %+v, want ready configured GET provider", status)
			}
			fetchInput, err := service.searchFetchInput(SearchInput{Query: "local agents", TaskApproved: true, Caller: "test"}, normalizeSearchProvider(provider))
			if err != nil {
				t.Fatalf("searchFetchInput() error = %v", err)
			}
			parsed, err := url.Parse(fetchInput.URL)
			if err != nil {
				t.Fatalf("parse fetchInput.URL: %v", err)
			}
			if fetchInput.Method != http.MethodGet || parsed.Hostname() != "search.example" {
				t.Fatalf("fetchInput = %+v, want GET to configured endpoint", fetchInput)
			}
			if parsed.Query().Get("q") != "local agents" || parsed.Query().Get("limit") != "4" || parsed.Query().Get("format") != "json" {
				t.Fatalf("query = %s, want generic search parameters", parsed.RawQuery)
			}
		})
	}

	t.Run("duckduckgo_default_endpoint_and_override", func(t *testing.T) {
		cfg := enabledTestConfig()
		cfg.Search.Enabled = true
		cfg.Search.Provider = "duckduckgo"
		cfg.Search.MaxResults = 4
		service := testService(t, cfg)
		status := service.Status()
		if !status.SearchProviderReady || status.SearchProviderNeedsAuth || status.SearchProviderNeedsConfig {
			t.Fatalf("Status() = %+v, want ready DuckDuckGo default endpoint", status)
		}
		fetchInput, err := service.searchFetchInput(SearchInput{Query: "local agents", TaskApproved: true, Caller: "test"}, "duckduckgo")
		if err != nil {
			t.Fatalf("searchFetchInput(default) error = %v", err)
		}
		parsed, err := url.Parse(fetchInput.URL)
		if err != nil {
			t.Fatalf("parse default URL: %v", err)
		}
		if fetchInput.Method != http.MethodGet || parsed.Hostname() != "api.duckduckgo.com" {
			t.Fatalf("fetchInput = %+v, want GET to DuckDuckGo default endpoint", fetchInput)
		}
		if parsed.Query().Get("q") != "local agents" || parsed.Query().Get("format") != "json" || parsed.Query().Get("no_html") != "1" {
			t.Fatalf("query = %s, want DuckDuckGo Instant Answer parameters", parsed.RawQuery)
		}

		cfg.Search.Endpoint = "https://search.example/duck"
		service = testService(t, cfg)
		fetchInput, err = service.searchFetchInput(SearchInput{Query: "local agents", TaskApproved: true, Caller: "test"}, "duckduckgo")
		if err != nil {
			t.Fatalf("searchFetchInput(override) error = %v", err)
		}
		parsed, err = url.Parse(fetchInput.URL)
		if err != nil {
			t.Fatalf("parse override URL: %v", err)
		}
		if parsed.Hostname() != "search.example" || parsed.Path != "/duck" {
			t.Fatalf("override URL = %s, want configured DuckDuckGo endpoint", fetchInput.URL)
		}
	})

	t.Run("mojeek_configured_endpoint", func(t *testing.T) {
		t.Setenv("YEMAKA_TEST_MOJEEK_KEY", "secret")
		cfg := enabledTestConfig()
		cfg.Search.Enabled = true
		cfg.Search.Provider = "mojeek"
		cfg.Search.APIKeyEnv = "YEMAKA_TEST_MOJEEK_KEY"
		cfg.Search.MaxResults = 4
		service := testService(t, cfg)
		status := service.Status()
		if !status.SearchProviderReady || status.SearchStatus != "search provider is ready" {
			t.Fatalf("Status() = %+v, want ready Mojeek provider", status)
		}
		fetchInput, err := service.searchFetchInput(SearchInput{Query: "local agents", TaskApproved: true, Caller: "test"}, "mojeek")
		if err != nil {
			t.Fatalf("searchFetchInput() error = %v", err)
		}
		parsed, err := url.Parse(fetchInput.URL)
		if err != nil {
			t.Fatalf("parse fetchInput.URL: %v", err)
		}
		if fetchInput.Method != http.MethodGet || parsed.Hostname() != "api.mojeek.com" {
			t.Fatalf("fetchInput = %+v, want GET to Mojeek default endpoint", fetchInput)
		}
		if parsed.Query().Get("q") != "local agents" || parsed.Query().Get("num") != "4" || parsed.Query().Get("fmt") != "json" {
			t.Fatalf("query = %s, want Mojeek search parameters", parsed.RawQuery)
		}
		if parsed.Query().Get("api_key") != "" || fetchInput.SecretQuery["api_key"] != "secret" {
			t.Fatalf("fetchInput URL/secret query = %s / %#v, want api key kept out of logged URL", fetchInput.URL, fetchInput.SecretQuery)
		}
		service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Query().Get("api_key") != "secret" {
				t.Fatalf("request query = %s, want secret API key only on outbound request", r.URL.RawQuery)
			}
			if strings.Contains(fetchInput.URL, "secret") {
				t.Fatalf("fetchInput.URL leaked secret: %s", fetchInput.URL)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"response":{"results":[{"title":"Yemaka","url":"https://example.com/yemaka","desc":"Local agent"}]}}`)),
				Request: &http.Request{
					URL: r.URL,
				},
			}, nil
		})}
		result, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if strings.Contains(result.URL, "api_key") || strings.Contains(result.URL, "secret") {
			t.Fatalf("result.URL leaked secret: %s", result.URL)
		}
		if len(result.Items) != 1 || result.Items[0].Snippet != "Local agent" {
			t.Fatalf("Search() = %+v, want parsed Mojeek response item", result)
		}
	})
}

func TestSearchProvidersRejectSecretQueryEndpoints(t *testing.T) {
	providers := []string{"searxng", "brave", "wikimedia", "tavily", "serper", "duckduckgo", "firecrawl", "mojeek", "yemaka_custom"}
	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			cfg := enabledTestConfig()
			cfg.Search.Enabled = true
			cfg.Search.Provider = provider
			cfg.Search.Endpoint = "https://search.example/api?token=secret"
			if provider == "brave" || provider == "tavily" || provider == "serper" || provider == "firecrawl" || provider == "mojeek" {
				envName := "YEMAKA_TEST_" + strings.ToUpper(provider) + "_KEY"
				t.Setenv(envName, "secret")
				cfg.Search.APIKeyEnv = envName
			}
			service := testService(t, cfg)
			service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected network request to %s", r.URL.String())
				return nil, nil
			})}

			status := service.Status()
			if status.SearchProviderReady || !status.SearchProviderNeedsConfig || !strings.Contains(status.SearchStatus, "secret-like query parameters") {
				t.Fatalf("Status() = %+v, want secret-in-URL readiness guidance", status)
			}
			_, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
			if err == nil || !strings.Contains(err.Error(), "secret-like query parameters") || !strings.Contains(err.Error(), "API keys in the URL") {
				t.Fatalf("Search() error = %v, want secret-in-URL error", err)
			}
		})
	}
}

func TestCustomSearchProviderParsesGenericResults(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "yemaka_custom"
	cfg.Search.Endpoint = "https://search.example/api"
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"results":[{"title":"One","link":"https://example.com/one","description":"First","source":"custom"}]}`
		return response(200, "application/json", body), nil
	})}

	result, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Provider != "yemaka_custom" || len(result.Items) != 1 {
		t.Fatalf("Search() = %+v, want one custom item", result)
	}
	if result.Items[0].URL != "https://example.com/one" || result.Items[0].Snippet != "First" {
		t.Fatalf("item = %+v, want generic result fields", result.Items[0])
	}
}

func TestPostRCProvidersReturnClearUnsupportedOrMissingSecretMessages(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "firecrawl"
	cfg.Search.APIKeyEnv = "fc-API_KEY"
	service := testService(t, cfg)
	status := service.Status()
	if status.SearchProviderReady || !status.SearchProviderNeedsAuth || !strings.Contains(status.SearchStatus, "API key value") || !strings.Contains(status.SearchStatus, "FIRECRAWL_API_KEY") {
		t.Fatalf("Status() = %+v, want raw Firecrawl key guidance", status)
	}
	if strings.Contains(status.SearchStatus, "fc-API_KEY") {
		t.Fatalf("SearchStatus = %q, should not echo raw-looking key material", status.SearchStatus)
	}
	_, err := service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err == nil || !strings.Contains(err.Error(), "API key value") {
		t.Fatalf("Search() error = %v, want raw key guidance", err)
	}

	t.Setenv("YEMAKA_TEST_FIRECRAWL_KEY_MISSING", "")
	cfg.Search.Provider = "firecrawl"
	cfg.Search.APIKeyEnv = "YEMAKA_TEST_FIRECRAWL_KEY_MISSING"
	service = testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request to %s", r.URL.String())
		return nil, nil
	})}
	status = service.Status()
	if status.SearchProviderReady || !status.SearchProviderNeedsAuth || !strings.Contains(status.SearchStatus, "not set") {
		t.Fatalf("Status() = %+v, want missing Firecrawl API key guidance", status)
	}
	_, err = service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err == nil || !strings.Contains(err.Error(), "not set") {
		t.Fatalf("Search() error = %v, want missing Firecrawl API key error", err)
	}

	t.Setenv("YEMAKA_TEST_TAVILY_KEY", "")
	cfg.Search.Provider = "tavily"
	cfg.Search.APIKeyEnv = "YEMAKA_TEST_TAVILY_KEY"
	service = testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request to %s", r.URL.String())
		return nil, nil
	})}

	status = service.Status()
	if status.SearchProviderReady || !status.SearchProviderNeedsAuth || !strings.Contains(status.SearchStatus, "not set") {
		t.Fatalf("Status() = %+v, want missing Tavily API key guidance", status)
	}
	_, err = service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err == nil || !strings.Contains(err.Error(), "not set") {
		t.Fatalf("Search() error = %v, want missing Tavily API key error", err)
	}

	t.Setenv("YEMAKA_TEST_TAVILY_KEY", "secret")
	cfg.Search.Endpoint = "https://api.tavily.test/search"
	service = testService(t, cfg)
	status = service.Status()
	if !status.SearchProviderReady || status.SearchProviderNeedsAuth || status.SearchProviderNeedsConfig {
		t.Fatalf("Status() = %+v, want ready Tavily POST provider", status)
	}

	t.Setenv("YEMAKA_TEST_SERPER_KEY", "")
	cfg.Search.Provider = "serper"
	cfg.Search.APIKeyEnv = "YEMAKA_TEST_SERPER_KEY"
	cfg.Search.Endpoint = ""
	service = testService(t, cfg)
	status = service.Status()
	if status.SearchProviderReady || !status.SearchProviderNeedsAuth || !strings.Contains(status.SearchStatus, "not set") {
		t.Fatalf("Status() = %+v, want missing Serper API key guidance", status)
	}
	_, err = service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err == nil || !strings.Contains(err.Error(), "not set") {
		t.Fatalf("Search() error = %v, want missing Serper API key error", err)
	}

	t.Setenv("YEMAKA_TEST_MOJEEK_KEY", "")
	cfg.Search.Provider = "mojeek"
	cfg.Search.APIKeyEnv = "YEMAKA_TEST_MOJEEK_KEY"
	service = testService(t, cfg)
	status = service.Status()
	if status.SearchProviderReady || !status.SearchProviderNeedsAuth || !strings.Contains(status.SearchStatus, "not set") {
		t.Fatalf("Status() = %+v, want Mojeek missing API key guidance", status)
	}
	_, err = service.Search(context.Background(), SearchInput{Query: "local agents", TaskApproved: true})
	if err == nil || !strings.Contains(err.Error(), "not set") {
		t.Fatalf("Search() error = %v, want Mojeek missing API key error", err)
	}
}

func TestSearchAPIKeyEnvNormalizationCorrectsStaleProviderDefaults(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		input    string
		want     string
	}{
		{name: "firecrawl_from_tavily", provider: "firecrawl", input: "TAVILY_API_KEY", want: "FIRECRAWL_API_KEY"},
		{name: "serper_from_firecrawl", provider: "serper", input: "FIRECRAWL_API_KEY", want: "SERPER_API_KEY"},
		{name: "brave_from_tavily", provider: "brave", input: "TAVILY_API_KEY", want: "BRAVE_SEARCH_API_KEY"},
		{name: "mojeek_from_serper", provider: "mojeek", input: "SERPER_API_KEY", want: "MOJEEK_API_KEY"},
		{name: "custom_env_preserved", provider: "firecrawl", input: "MY_FIRECRAWL_KEY", want: "MY_FIRECRAWL_KEY"},
		{name: "blank_preserved", provider: "firecrawl", input: "", want: ""},
		{name: "no_key_provider_clears_stale", provider: "wikimedia", input: "TAVILY_API_KEY", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeSearchAPIKeyEnvForProvider(tt.provider, tt.input)
			if got != tt.want {
				t.Fatalf("NormalizeSearchAPIKeyEnvForProvider(%q, %q) = %q, want %q", tt.provider, tt.input, got, tt.want)
			}
		})
	}
}

func TestSearchStatusNormalizesStaleDefaultEnvBeforeReadiness(t *testing.T) {
	t.Setenv("FIRECRAWL_API_KEY", "secret")
	t.Setenv("TAVILY_API_KEY", "")
	cfg := enabledTestConfig()
	cfg.Search.Enabled = true
	cfg.Search.Provider = "firecrawl"
	cfg.Search.APIKeyEnv = "TAVILY_API_KEY"
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request to %s", r.URL.String())
		return nil, nil
	})}

	status := service.Status()
	if !status.SearchProviderReady || status.SearchProviderNeedsAuth || status.SearchAPIKeyEnv != "FIRECRAWL_API_KEY" {
		t.Fatalf("Status() = %+v, want ready Firecrawl using normalized FIRECRAWL_API_KEY", status)
	}
	if strings.Contains(status.SearchStatus, "TAVILY_API_KEY") {
		t.Fatalf("SearchStatus = %q, should not mention stale Tavily env", status.SearchStatus)
	}
}

func TestPrimitiveResponsibilitiesKeepSearchFetchAndCrawlerSeparate(t *testing.T) {
	responsibilities := map[PrimitiveName]PrimitiveResponsibility{}
	for _, item := range PrimitiveResponsibilities() {
		responsibilities[item.Name] = item
	}

	search := responsibilities[PrimitiveInternetSearch]
	if search.Responsibility != "find_urls" || search.NetworkScope != "configured search provider endpoint" || !search.Implemented {
		t.Fatalf("search responsibility = %+v, want URL discovery through provider only", search)
	}
	fetch := responsibilities[PrimitiveInternetFetch]
	if fetch.Responsibility != "fetch_one_url" || fetch.NetworkScope != "single approved URL" || !fetch.Implemented {
		t.Fatalf("fetch responsibility = %+v, want one approved URL fetch", fetch)
	}
	crawler := responsibilities[PrimitiveCrawlerTask]
	if crawler.Responsibility != "follow_many_urls" || crawler.NetworkScope != "approved bounded same-domain crawl" || !crawler.Implemented {
		t.Fatalf("crawler responsibility = %+v, want manual bounded crawler primitive", crawler)
	}
}

func TestFullAccessPolicyModeBypassesInternetApprovalGate(t *testing.T) {
	service := testService(t, config.Default().Internet)
	service.PolicyMode = "full_access"
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(200, "text/plain", "hello"), nil
	})}
	result, err := service.Fetch(context.Background(), FetchInput{
		URL:            "https://example.com/full",
		AllowedDomains: []string{"example.com"},
	})
	if err != nil {
		t.Fatalf("Fetch(full_access) error = %v", err)
	}
	if result.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", result.StatusCode)
	}
}

func TestCheckRobotsReportsDisallow(t *testing.T) {
	cfg := enabledTestConfig()
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(200, "text/plain", "User-agent: *\nDisallow: /private\n"), nil
	})}
	result, err := service.CheckRobots(context.Background(), "https://example.com/private/page", "Yemaka", true)
	if err != nil {
		t.Fatalf("CheckRobots() error = %v", err)
	}
	if result.Allowed || !strings.Contains(result.Reason, "disallow") {
		t.Fatalf("robots result = %+v, want disallowed by robots.txt", result)
	}
}

func testService(t *testing.T, cfg config.InternetConfig) *Service {
	t.Helper()
	root := t.TempDir()
	service := New(cfg, root, filepath.Join(root, "logs"))
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	service.Now = func() time.Time { return now }
	return service
}

func enabledTestConfig() config.InternetConfig {
	cfg := config.Default().Internet
	cfg.Enabled = true
	cfg.Policy.BlockPrivateIPRanges = false
	cfg.Policy.BlockLocalNetworkByDefault = false
	cfg.TimeoutSeconds = 2
	return cfg
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func response(status int, contentType string, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header: http.Header{
			"Content-Type": []string{contentType},
		},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       &http.Request{URL: mustURL("https://example.com/final")},
	}
}

func mustURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return parsed
}
