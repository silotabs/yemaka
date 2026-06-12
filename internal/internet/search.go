package internet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"yemaka/internal/safety"
)

type SearchInput struct {
	Query        string `json:"query"`
	TaskApproved bool   `json:"taskApproved"`
	MaxResults   int    `json:"maxResults"`
	Caller       string `json:"caller"`
}

type SearchResult struct {
	Query     string       `json:"query"`
	Provider  string       `json:"provider"`
	URL       string       `json:"url,omitempty"`
	Items     []SearchItem `json:"items"`
	Message   string       `json:"message,omitempty"`
	FromCache bool         `json:"fromCache"`
	CachedAt  string       `json:"cachedAt,omitempty"`
	FetchedAt string       `json:"fetchedAt"`
}

type SearchItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
	Source  string `json:"source,omitempty"`
}

func (s *Service) Search(ctx context.Context, input SearchInput) (SearchResult, error) {
	if strings.TrimSpace(input.Query) == "" {
		return SearchResult{}, fmt.Errorf("search query is required")
	}
	if input.Caller == "" {
		input.Caller = "internet_search"
	}
	provider := normalizeSearchProvider(s.Config.Search.Provider)
	request := safety.PolicyRequest{
		Domain:             safety.DomainInternet,
		Action:             safety.ActionSearch,
		Level:              safety.LevelConfirm,
		PolicyMode:         s.PolicyMode,
		Actor:              input.Caller,
		Resource:           strings.TrimSpace(input.Query),
		Enabled:            s.Config.Enabled,
		ProfileEnabled:     s.Config.Enabled && s.Config.DefaultMode == "profile_enabled",
		TaskApproved:       input.TaskApproved,
		ProviderConfigured: searchProviderConfiguredForPolicy(s.Config),
		Network:            true,
	}
	decision := safety.EvaluatePolicy(request)
	s.auditPolicy(request, decision)
	if !decision.Allowed {
		return SearchResult{}, fmt.Errorf("%s", decision.Explanation)
	}
	if decision.RequiresConfirmation && !input.TaskApproved {
		return SearchResult{}, fmt.Errorf("%s", decision.Explanation)
	}
	providerCandidates := s.searchProviderCandidates(provider)
	if len(providerCandidates) == 0 {
		return SearchResult{}, fmt.Errorf("internet search provider %q is not supported", s.Config.Search.Provider)
	}
	var lastErr error
	for _, candidate := range providerCandidates {
		fetchInput, err := s.searchFetchInput(input, candidate)
		if err != nil {
			lastErr = err
			if provider == "auto" {
				continue
			}
			return SearchResult{}, err
		}
		fetchInput.Provider = candidate
		fetchResult, err := s.Fetch(ctx, fetchInput)
		if err != nil {
			lastErr = searchProviderRequestError(candidate, fetchInput.URL, err)
			if provider == "auto" {
				continue
			}
			return SearchResult{}, lastErr
		}
		if fetchResult.StatusCode < 200 || fetchResult.StatusCode >= 300 {
			lastErr = searchProviderHTTPError(candidate, fetchResult.StatusCode, fetchResult.Body)
			if provider == "auto" {
				continue
			}
			return SearchResult{}, lastErr
		}
		items, err := parseSearchItems(candidate, fetchResult.Body)
		if err != nil {
			lastErr = err
			if provider == "auto" {
				continue
			}
			return SearchResult{}, err
		}
		items = limitSearchItems(items, maxSearchResults(input.MaxResults, s.Config.Search.MaxResults))
		if provider == "auto" && len(items) == 0 {
			lastErr = fmt.Errorf("%s search provider returned no results", searchProviderLabel(candidate))
			continue
		}
		message := fmt.Sprintf("%d search result(s)", len(items))
		if len(items) == 0 {
			message = "No search results."
		}
		return SearchResult{
			Query:     strings.TrimSpace(input.Query),
			Provider:  candidate,
			URL:       fetchResult.URL,
			Items:     items,
			Message:   message,
			FromCache: fetchResult.FromCache,
			CachedAt:  fetchResult.CachedAt,
			FetchedAt: fetchResult.FetchedAt,
		}, nil
	}
	if lastErr != nil {
		return SearchResult{}, fmt.Errorf("auto search found no usable provider: %w", lastErr)
	}
	return SearchResult{}, fmt.Errorf("auto search found no usable provider")
}

func (s *Service) searchFetchInput(input SearchInput, provider string) (FetchInput, error) {
	searchProvider, ok := searchProviderForName(provider)
	if !ok {
		return FetchInput{}, fmt.Errorf("internet search provider %q is not supported", s.Config.Search.Provider)
	}
	return searchProvider.FetchInput(s.Config, input, maxSearchResults(input.MaxResults, s.Config.Search.MaxResults))
}

func (s *Service) searchProviderCandidates(provider string) []string {
	provider = normalizeSearchProvider(provider)
	if provider == "auto" {
		providers := []string{}
		for _, candidate := range searchProviderPriority(s.Config) {
			if _, ok := searchProviderForName(candidate); ok {
				providers = append(providers, candidate)
			}
		}
		return providers
	}
	if _, ok := searchProviderForName(provider); !ok {
		return nil
	}
	return []string{provider}
}

func normalizeSearchProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "", "off", "disabled", "none":
		return "none"
	case "auto", "fallback", "auto_fallback":
		return "auto"
	case "searx", "searxng":
		return "searxng"
	case "brave", "brave_search":
		return "brave"
	case "wikimedia", "wikipedia", "mediawiki":
		return "wikimedia"
	case "tavily":
		return "tavily"
	case "serper", "serperdev", "serper.dev", "google_serp", "google_serper":
		return "serper"
	case "duckduckgo", "ddg":
		return "duckduckgo"
	case "firecrawl", "firecrawl_search":
		return "firecrawl"
	case "mojeek":
		return "mojeek"
	case "custom", "custom_search", "yemaka_custom", "yemaka_custom_search":
		return "yemaka_custom"
	default:
		return provider
	}
}

func searxngSearchURL(endpoint string, query string, limit int, safeSearch bool) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", fmt.Errorf("searxng search requires internet.search.endpoint")
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		return "", err
	}
	values := parsed.Query()
	values.Set("q", strings.TrimSpace(query))
	values.Set("format", "json")
	values.Set("language", "en")
	values.Set("pageno", "1")
	values.Set("safesearch", "0")
	if safeSearch {
		values.Set("safesearch", "1")
	}
	if limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", limit))
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func braveSearchURL(endpoint string, query string, limit int, safeSearch bool) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = "https://api.search.brave.com/res/v1/web/search"
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		return "", err
	}
	values := parsed.Query()
	values.Set("q", strings.TrimSpace(query))
	values.Set("count", fmt.Sprintf("%d", limit))
	values.Set("safesearch", "off")
	if safeSearch {
		values.Set("safesearch", "strict")
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func wikimediaSearchURL(endpoint string, query string, limit int) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = "https://en.wikipedia.org/w/api.php"
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		return "", err
	}
	values := parsed.Query()
	values.Set("action", "query")
	values.Set("list", "search")
	values.Set("format", "json")
	values.Set("srsearch", strings.TrimSpace(query))
	values.Set("srlimit", fmt.Sprintf("%d", limit))
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func duckDuckGoSearchURL(endpoint string, query string, _ int, safeSearch bool) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = "https://api.duckduckgo.com/"
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		return "", err
	}
	values := parsed.Query()
	values.Set("q", strings.TrimSpace(query))
	values.Set("format", "json")
	values.Set("no_html", "1")
	values.Set("skip_disambig", "1")
	if safeSearch {
		values.Set("kp", "1")
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func mojeekSearchURL(endpoint string, query string, limit int, safeSearch bool) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = "https://api.mojeek.com/search"
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		return "", err
	}
	values := parsed.Query()
	values.Set("q", strings.TrimSpace(query))
	values.Set("fmt", "json")
	if limit > 0 {
		values.Set("num", fmt.Sprintf("%d", limit))
	}
	if safeSearch {
		values.Set("safe", "1")
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func genericSearchURL(endpoint string, query string, limit int, safeSearch bool) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", fmt.Errorf("endpoint is required")
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		return "", err
	}
	values := parsed.Query()
	values.Set("q", strings.TrimSpace(query))
	values.Set("format", "json")
	values.Set("limit", fmt.Sprintf("%d", limit))
	values.Set("safe_search", "false")
	if safeSearch {
		values.Set("safe_search", "true")
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func firecrawlSearchEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = "https://api.firecrawl.dev/v2/search"
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

func tavilySearchEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = "https://api.tavily.com/search"
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

func tavilySearchBody(query string, limit int) (string, error) {
	payload := struct {
		Query             string `json:"query"`
		SearchDepth       string `json:"search_depth"`
		MaxResults        int    `json:"max_results,omitempty"`
		IncludeAnswer     bool   `json:"include_answer"`
		IncludeRawContent bool   `json:"include_raw_content"`
	}{
		Query:             strings.TrimSpace(query),
		SearchDepth:       "basic",
		MaxResults:        limit,
		IncludeAnswer:     false,
		IncludeRawContent: false,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func serperSearchEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = "https://google.serper.dev/search"
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

func serperSearchBody(query string, limit int) (string, error) {
	payload := struct {
		Query string `json:"q"`
		Num   int    `json:"num,omitempty"`
	}{
		Query: strings.TrimSpace(query),
		Num:   limit,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func firecrawlSearchBody(query string, limit int) (string, error) {
	payload := struct {
		Query string `json:"query"`
		Limit int    `json:"limit,omitempty"`
	}{
		Query: strings.TrimSpace(query),
		Limit: limit,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func hostForURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("search provider URL must include host")
	}
	return host, nil
}

func parseSearchItems(provider string, body string) ([]SearchItem, error) {
	searchProvider, ok := searchProviderForName(provider)
	if !ok {
		return nil, fmt.Errorf("internet search provider %q is not supported", provider)
	}
	return searchProvider.Parse(body)
}

func parseSearXNGItems(body string) ([]SearchItem, error) {
	var response struct {
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Engine  string  `json:"engine"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return nil, fmt.Errorf("parse searxng search response: %w", err)
	}
	items := make([]SearchItem, 0, len(response.Results))
	for _, result := range response.Results {
		item := SearchItem{
			Title:   strings.TrimSpace(result.Title),
			URL:     strings.TrimSpace(result.URL),
			Snippet: strings.TrimSpace(result.Content),
			Source:  strings.TrimSpace(result.Engine),
		}
		if item.Title == "" && item.URL == "" {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func parseBraveItems(body string) ([]SearchItem, error) {
	var response struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return nil, fmt.Errorf("parse brave search response: %w", err)
	}
	items := make([]SearchItem, 0, len(response.Web.Results))
	for _, result := range response.Web.Results {
		item := SearchItem{
			Title:   strings.TrimSpace(result.Title),
			URL:     strings.TrimSpace(result.URL),
			Snippet: strings.TrimSpace(result.Description),
			Source:  "brave",
		}
		if item.Title == "" && item.URL == "" {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func parseWikimediaItems(body string) ([]SearchItem, error) {
	var response struct {
		Query struct {
			Search []struct {
				Title   string `json:"title"`
				PageID  int64  `json:"pageid"`
				Snippet string `json:"snippet"`
			} `json:"search"`
		} `json:"query"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return nil, fmt.Errorf("parse wikimedia search response: %w", err)
	}
	items := make([]SearchItem, 0, len(response.Query.Search))
	for _, result := range response.Query.Search {
		title := strings.TrimSpace(result.Title)
		item := SearchItem{
			Title:   title,
			URL:     wikimediaPageURL(title, result.PageID),
			Snippet: stripHTMLTags(result.Snippet),
			Source:  "wikimedia",
		}
		if item.Title == "" && item.URL == "" {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

type duckDuckGoTopic struct {
	Text     string            `json:"Text"`
	FirstURL string            `json:"FirstURL"`
	Topics   []duckDuckGoTopic `json:"Topics"`
}

func parseDuckDuckGoItems(body string) ([]SearchItem, error) {
	var response struct {
		Heading     string `json:"Heading"`
		Abstract    string `json:"Abstract"`
		AbstractURL string `json:"AbstractURL"`
		Results     []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Results"`
		RelatedTopics []duckDuckGoTopic `json:"RelatedTopics"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return nil, fmt.Errorf("parse duckduckgo search response: %w", err)
	}
	items := []SearchItem{}
	if strings.TrimSpace(response.AbstractURL) != "" || strings.TrimSpace(response.Abstract) != "" {
		items = append(items, SearchItem{
			Title:   firstNonEmpty(response.Heading, "DuckDuckGo result"),
			URL:     strings.TrimSpace(response.AbstractURL),
			Snippet: strings.TrimSpace(response.Abstract),
			Source:  "duckduckgo",
		})
	}
	for _, result := range response.Results {
		items = appendSearchItem(items, SearchItem{
			Title:   topicTitle(result.Text),
			URL:     strings.TrimSpace(result.FirstURL),
			Snippet: strings.TrimSpace(result.Text),
			Source:  "duckduckgo",
		})
	}
	for _, topic := range response.RelatedTopics {
		items = appendDuckDuckGoTopic(items, topic)
	}
	if len(items) == 0 {
		return parseGenericSearchItems("duckduckgo", body)
	}
	return items, nil
}

func appendDuckDuckGoTopic(items []SearchItem, topic duckDuckGoTopic) []SearchItem {
	if len(topic.Topics) > 0 {
		for _, child := range topic.Topics {
			items = appendDuckDuckGoTopic(items, child)
		}
		return items
	}
	return appendSearchItem(items, SearchItem{
		Title:   topicTitle(topic.Text),
		URL:     strings.TrimSpace(topic.FirstURL),
		Snippet: strings.TrimSpace(topic.Text),
		Source:  "duckduckgo",
	})
}

func parseFirecrawlItems(body string) ([]SearchItem, error) {
	var response struct {
		Success bool            `json:"success"`
		Error   string          `json:"error"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return nil, fmt.Errorf("parse firecrawl search response: %w", err)
	}
	if !response.Success && strings.TrimSpace(response.Error) != "" {
		return nil, fmt.Errorf("firecrawl search failed: %s", strings.TrimSpace(response.Error))
	}
	items, err := parseFirecrawlData(response.Data)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func parseFirecrawlData(data json.RawMessage) ([]SearchItem, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var list []firecrawlSearchRecord
	if err := json.Unmarshal(data, &list); err == nil {
		return firecrawlRecordsToItems(list), nil
	}
	var grouped struct {
		Web     []firecrawlSearchRecord `json:"web"`
		Results []firecrawlSearchRecord `json:"results"`
		News    []firecrawlSearchRecord `json:"news"`
	}
	if err := json.Unmarshal(data, &grouped); err != nil {
		return nil, fmt.Errorf("parse firecrawl search data: %w", err)
	}
	records := append([]firecrawlSearchRecord{}, grouped.Web...)
	records = append(records, grouped.Results...)
	records = append(records, grouped.News...)
	return firecrawlRecordsToItems(records), nil
}

type firecrawlSearchRecord struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Snippet     string `json:"snippet"`
	Markdown    string `json:"markdown"`
	Metadata    struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		SourceURL   string `json:"sourceURL"`
		URL         string `json:"url"`
	} `json:"metadata"`
}

func firecrawlRecordsToItems(records []firecrawlSearchRecord) []SearchItem {
	items := make([]SearchItem, 0, len(records))
	for _, record := range records {
		urlValue := firstNonEmpty(record.URL, record.Metadata.SourceURL, record.Metadata.URL)
		item := SearchItem{
			Title:   firstNonEmpty(record.Title, record.Metadata.Title, urlValue),
			URL:     strings.TrimSpace(urlValue),
			Snippet: firstNonEmpty(record.Description, record.Snippet, record.Metadata.Description, bodySnippet(record.Markdown, 220)),
			Source:  "firecrawl",
		}
		items = appendSearchItem(items, item)
	}
	return items
}

func parseTavilyItems(body string) ([]SearchItem, error) {
	var response struct {
		Error   string `json:"error"`
		Detail  string `json:"detail"`
		Results []struct {
			Title      string  `json:"title"`
			URL        string  `json:"url"`
			Content    string  `json:"content"`
			RawContent string  `json:"raw_content"`
			Score      float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return nil, fmt.Errorf("parse tavily search response: %w", err)
	}
	if strings.TrimSpace(response.Error) != "" {
		return nil, fmt.Errorf("tavily search failed: %s", strings.TrimSpace(response.Error))
	}
	if strings.TrimSpace(response.Detail) != "" && len(response.Results) == 0 {
		return nil, fmt.Errorf("tavily search failed: %s", strings.TrimSpace(response.Detail))
	}
	items := make([]SearchItem, 0, len(response.Results))
	for _, result := range response.Results {
		item := SearchItem{
			Title:   strings.TrimSpace(result.Title),
			URL:     strings.TrimSpace(result.URL),
			Snippet: firstNonEmpty(result.Content, bodySnippet(result.RawContent, 220)),
			Source:  "tavily",
		}
		items = appendSearchItem(items, item)
	}
	return items, nil
}

func parseSerperItems(body string) ([]SearchItem, error) {
	var response struct {
		Message         string                `json:"message"`
		Error           string                `json:"error"`
		Organic         []genericSearchResult `json:"organic"`
		PeopleAlsoAsk   []genericSearchResult `json:"peopleAlsoAsk"`
		RelatedSearches []genericSearchResult `json:"relatedSearches"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return nil, fmt.Errorf("parse serper search response: %w", err)
	}
	if strings.TrimSpace(response.Error) != "" {
		return nil, fmt.Errorf("serper search failed: %s", strings.TrimSpace(response.Error))
	}
	if strings.TrimSpace(response.Message) != "" && len(response.Organic) == 0 {
		return nil, fmt.Errorf("serper search failed: %s", strings.TrimSpace(response.Message))
	}
	return genericResultsToItems("serper", response.Organic), nil
}

func parseGenericSearchItems(provider string, body string) ([]SearchItem, error) {
	var response struct {
		Results  []genericSearchResult `json:"results"`
		Items    []genericSearchResult `json:"items"`
		Response struct {
			Results []genericSearchResult `json:"results"`
		} `json:"response"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return nil, fmt.Errorf("parse %s search response: %w", provider, err)
	}
	results := response.Results
	if len(results) == 0 {
		results = response.Items
	}
	if len(results) == 0 {
		results = response.Response.Results
	}
	return genericResultsToItems(provider, results), nil
}

func genericResultsToItems(provider string, results []genericSearchResult) []SearchItem {
	items := make([]SearchItem, 0, len(results))
	for _, result := range results {
		urlValue := firstNonEmpty(result.URL, result.Link)
		snippet := firstNonEmpty(result.Snippet, result.Content, result.Description, result.Desc)
		item := SearchItem{
			Title:   strings.TrimSpace(result.Title),
			URL:     strings.TrimSpace(urlValue),
			Snippet: strings.TrimSpace(snippet),
			Source:  firstNonEmpty(strings.TrimSpace(result.Source), provider),
		}
		if item.Title == "" && item.URL == "" {
			continue
		}
		items = append(items, item)
	}
	return items
}

type genericSearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Link        string `json:"link"`
	Snippet     string `json:"snippet"`
	Content     string `json:"content"`
	Description string `json:"description"`
	Desc        string `json:"desc"`
	Source      string `json:"source"`
}

func appendSearchItem(items []SearchItem, item SearchItem) []SearchItem {
	item.Title = strings.TrimSpace(item.Title)
	item.URL = strings.TrimSpace(item.URL)
	item.Snippet = strings.TrimSpace(item.Snippet)
	item.Source = strings.TrimSpace(item.Source)
	if item.Title == "" && item.URL == "" {
		return items
	}
	return append(items, item)
}

func topicTitle(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if separator := strings.Index(text, " - "); separator > 0 {
		return strings.TrimSpace(text[:separator])
	}
	if separator := strings.Index(text, " – "); separator > 0 {
		return strings.TrimSpace(text[:separator])
	}
	return firstWords(text, 8)
}

func firstWords(text string, limit int) string {
	words := strings.Fields(text)
	if limit <= 0 || len(words) <= limit {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:limit], " ")
}

func wikimediaPageURL(title string, pageID int64) string {
	if title != "" {
		return "https://en.wikipedia.org/wiki/" + strings.ReplaceAll(url.PathEscape(title), "+", "%20")
	}
	if pageID > 0 {
		return fmt.Sprintf("https://en.wikipedia.org/?curid=%d", pageID)
	}
	return ""
}

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

func stripHTMLTags(value string) string {
	return strings.Join(strings.Fields(htmlTagPattern.ReplaceAllString(value, " ")), " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func bodySnippet(body string, limit int) string {
	body = strings.Join(strings.Fields(body), " ")
	if body == "" {
		return "empty response body"
	}
	if limit <= 0 || len(body) <= limit {
		return body
	}
	return strings.TrimSpace(body[:limit]) + "..."
}

func searchProviderRequestError(provider string, rawURL string, err error) error {
	label := searchProviderLabel(provider)
	endpoint := providerEndpointForMessage(rawURL)
	detail := sanitizedProviderError(rawURL, err)
	if provider == "searxng" && looksLikeProviderUnavailable(err) {
		return fmt.Errorf("could not reach %s search provider at %s; verify SearXNG is running and the endpoint is reachable: %s", label, endpoint, detail)
	}
	return fmt.Errorf("%s search provider request failed at %s: %s", label, endpoint, detail)
}

func searchProviderHTTPError(provider string, statusCode int, body string) error {
	label := searchProviderLabel(provider)
	detail := bodySnippet(body, 180)
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%s search provider rejected credentials or access with HTTP %d: %s", label, statusCode, detail)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%s search provider quota or rate limit reached with HTTP %d: %s", label, statusCode, detail)
	}
	if statusCode >= 500 {
		return fmt.Errorf("%s search provider endpoint error HTTP %d: %s", label, statusCode, detail)
	}
	return fmt.Errorf("%s search provider returned HTTP %d: %s", label, statusCode, detail)
}

func providerEndpointForMessage(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimSpace(rawURL)
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func searchEndpointForStatus(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimSpace(rawURL)
	}
	if endpointHasSecretQuery(parsed) {
		return providerEndpointForMessage(rawURL)
	}
	return strings.TrimSpace(rawURL)
}

func sanitizedProviderError(rawURL string, err error) string {
	message := strings.TrimSpace(err.Error())
	endpoint := providerEndpointForMessage(rawURL)
	if rawURL != "" && endpoint != "" {
		message = strings.ReplaceAll(message, rawURL, endpoint)
	}
	return message
}

func looksLikeProviderUnavailable(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "connection refused") ||
		strings.Contains(message, "connect: connection refused") ||
		strings.Contains(message, "no such host") ||
		strings.Contains(message, "server misbehaving") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "deadline exceeded") ||
		strings.Contains(message, "timeout")
}

func maxSearchResults(inputLimit int, configLimit int) int {
	limit := inputLimit
	if limit <= 0 {
		limit = configLimit
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	return limit
}

func limitSearchItems(items []SearchItem, limit int) []SearchItem {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}
