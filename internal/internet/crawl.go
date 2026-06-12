package internet

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	defaultCrawlMaxPages        = 8
	absoluteCrawlMaxPages       = 25
	defaultCrawlMaxDepth        = 1
	absoluteCrawlMaxDepth       = 3
	defaultCrawlDurationSecs    = 30
	absoluteCrawlDurationSecs   = 120
	defaultCrawlLinksPerPage    = 50
	absoluteCrawlLinksPerPage   = 200
	defaultCrawlMaxTextChars    = 4000
	absoluteCrawlMaxTextChars   = 12000
	maxRecordedCrawlSkips       = 500
	defaultCrawlCaller          = "internet_crawl"
	defaultCrawlRobotsUserAgent = "Yemaka"
)

type CrawlInput struct {
	SeedURL            string   `json:"seedUrl"`
	Method             string   `json:"method,omitempty"`
	ExtractText        bool     `json:"extractText"`
	AllowedDomains     []string `json:"allowedDomains"`
	MaxPages           int      `json:"maxPages"`
	MaxDepth           int      `json:"maxDepth"`
	MaxDurationSeconds int      `json:"maxDurationSeconds"`
	MaxLinksPerPage    int      `json:"maxLinksPerPage"`
	MaxTextChars       int      `json:"maxTextChars"`
	TaskApproved       bool     `json:"taskApproved"`
	Caller             string   `json:"caller"`
	UserAgent          string   `json:"userAgent,omitempty"`
}

type CrawlResult struct {
	RunID              string      `json:"runId"`
	SeedURL            string      `json:"seedUrl"`
	Method             string      `json:"method"`
	Status             string      `json:"status"`
	FailureReason      string      `json:"failureReason,omitempty"`
	StartedAt          string      `json:"startedAt"`
	FinishedAt         string      `json:"finishedAt"`
	AllowedDomains     []string    `json:"allowedDomains"`
	MaxPages           int         `json:"maxPages"`
	MaxDepth           int         `json:"maxDepth"`
	MaxDurationSeconds int         `json:"maxDurationSeconds"`
	MaxLinksPerPage    int         `json:"maxLinksPerPage"`
	MaxTextChars       int         `json:"maxTextChars"`
	Visited            int         `json:"visited"`
	Fetched            int         `json:"fetched"`
	Skipped            int         `json:"skipped"`
	SkipOverflow       int         `json:"skipOverflow,omitempty"`
	Pages              []CrawlPage `json:"pages"`
	Skips              []CrawlSkip `json:"skips"`
}

type CrawlPage struct {
	URL           string        `json:"url"`
	FinalURL      string        `json:"finalUrl"`
	Depth         int           `json:"depth"`
	StatusCode    int           `json:"statusCode"`
	ContentType   string        `json:"contentType,omitempty"`
	BodyBytes     int           `json:"bodyBytes"`
	FromCache     bool          `json:"fromCache"`
	ExtractedText string        `json:"extractedText,omitempty"`
	Links         []string      `json:"links,omitempty"`
	Robots        *RobotsResult `json:"robots,omitempty"`
	FetchedAt     string        `json:"fetchedAt"`
	Error         string        `json:"error,omitempty"`
}

type CrawlSkip struct {
	URL    string        `json:"url"`
	Depth  int           `json:"depth"`
	Reason string        `json:"reason"`
	Robots *RobotsResult `json:"robots,omitempty"`
}

type crawlLimits struct {
	maxPages           int
	maxDepth           int
	maxDurationSeconds int
	maxLinksPerPage    int
	maxTextChars       int
}

type crawlQueueItem struct {
	url   string
	depth int
}

func (s *Service) Crawl(ctx context.Context, input CrawlInput) (CrawlResult, error) {
	started := s.timestamp()
	method := normalizeMethod(input.Method)
	caller := strings.TrimSpace(input.Caller)
	if caller == "" {
		caller = defaultCrawlCaller
	}
	limits := normalizeCrawlLimits(input)

	seedURL, seed, err := normalizeCrawlURL(input.SeedURL, nil)
	if err != nil {
		return CrawlResult{}, err
	}
	allowedDomains := crawlAllowedDomains(input.AllowedDomains, seed.Hostname())
	result := CrawlResult{
		RunID:              crawlRunID(started, seedURL, method),
		SeedURL:            seedURL,
		Method:             method,
		Status:             "running",
		StartedAt:          started,
		AllowedDomains:     allowedDomains,
		MaxPages:           limits.maxPages,
		MaxDepth:           limits.maxDepth,
		MaxDurationSeconds: limits.maxDurationSeconds,
		MaxLinksPerPage:    limits.maxLinksPerPage,
		MaxTextChars:       limits.maxTextChars,
	}

	if !s.Config.Enabled {
		result.Status = "blocked"
		result.FailureReason = "crawler disabled because controlled internet access is disabled"
		result = s.finishCrawlResult(result)
		return result, fmt.Errorf("crawler disabled because controlled internet access is disabled")
	}
	if !input.TaskApproved {
		result.Status = "blocked"
		result.FailureReason = "crawler requires explicit task approval"
		result = s.finishCrawlResult(result)
		return result, fmt.Errorf("crawler requires explicit task approval")
	}
	if method != http.MethodGet && method != http.MethodHead {
		result.Status = "blocked"
		result.FailureReason = "crawler supports GET and HEAD only"
		result = s.finishCrawlResult(result)
		return result, fmt.Errorf("crawler supports GET and HEAD only")
	}

	crawlCtx := ctx
	var cancel context.CancelFunc
	if limits.maxDurationSeconds > 0 {
		crawlCtx, cancel = context.WithTimeout(ctx, time.Duration(limits.maxDurationSeconds)*time.Second)
		defer cancel()
	}

	queue := []crawlQueueItem{{url: seedURL, depth: 0}}
	seen := map[string]bool{seedURL: true}
	for len(queue) > 0 {
		select {
		case <-crawlCtx.Done():
			result.Status = "cancelled"
			result.FailureReason = fmt.Sprintf("crawl stopped: %v", crawlCtx.Err())
			addCrawlSkip(&result, CrawlSkip{Reason: result.FailureReason})
			return s.finishCrawlResult(result), nil
		default:
		}

		if len(result.Pages) >= limits.maxPages {
			result.Status = "max_pages_reached"
			addCrawlSkip(&result, CrawlSkip{URL: queue[0].url, Depth: queue[0].depth, Reason: "max pages limit reached"})
			if len(queue) > 1 {
				result.Skipped += len(queue) - 1
				result.SkipOverflow += len(queue) - 1
			}
			break
		}

		item := queue[0]
		queue = queue[1:]
		result.Visited++

		currentURL, current, err := normalizeCrawlURL(item.url, nil)
		if err != nil {
			addCrawlSkip(&result, CrawlSkip{URL: item.url, Depth: item.depth, Reason: err.Error()})
			continue
		}
		if item.depth > limits.maxDepth {
			addCrawlSkip(&result, CrawlSkip{URL: currentURL, Depth: item.depth, Reason: "exceeds max depth"})
			continue
		}
		if !domainAllowed(current.Hostname(), allowedDomains) {
			addCrawlSkip(&result, CrawlSkip{URL: currentURL, Depth: item.depth, Reason: "outside allowed domains"})
			continue
		}

		robots, allowed := s.crawlRobotsAllowed(crawlCtx, currentURL, input.UserAgent, input.TaskApproved)
		if !allowed {
			addCrawlSkip(&result, CrawlSkip{
				URL:    currentURL,
				Depth:  item.depth,
				Reason: "robots check blocked fetch: " + robots.Reason,
				Robots: &robots,
			})
			continue
		}

		fetch, err := s.Fetch(crawlCtx, FetchInput{
			URL:            currentURL,
			Method:         method,
			ExtractText:    input.ExtractText,
			AllowedDomains: allowedDomains,
			TaskApproved:   input.TaskApproved,
			Caller:         caller,
		})
		if err != nil {
			addCrawlSkip(&result, CrawlSkip{URL: currentURL, Depth: item.depth, Reason: err.Error(), Robots: &robots})
			continue
		}

		page := CrawlPage{
			URL:           currentURL,
			FinalURL:      fetch.FinalURL,
			Depth:         item.depth,
			StatusCode:    fetch.StatusCode,
			ContentType:   fetch.ContentType,
			BodyBytes:     fetch.BodyBytes,
			FromCache:     fetch.FromCache,
			ExtractedText: truncateCrawlText(fetch.ExtractedText, limits.maxTextChars),
			Robots:        &robots,
			FetchedAt:     fetch.FetchedAt,
		}
		if method == http.MethodGet && fetch.StatusCode >= 200 && fetch.StatusCode < 300 {
			base := current
			if finalURL, final, err := normalizeCrawlURL(fetch.FinalURL, nil); err == nil {
				page.FinalURL = finalURL
				base = final
			}
			page.Links = extractCrawlLinks(fetch.Body, fetch.ContentType, base, limits.maxLinksPerPage)
			for _, link := range page.Links {
				linkURL, parsed, err := normalizeCrawlURL(link, nil)
				if err != nil {
					addCrawlSkip(&result, CrawlSkip{URL: link, Depth: item.depth + 1, Reason: err.Error()})
					continue
				}
				if seen[linkURL] {
					continue
				}
				seen[linkURL] = true
				if !domainAllowed(parsed.Hostname(), allowedDomains) {
					addCrawlSkip(&result, CrawlSkip{URL: linkURL, Depth: item.depth + 1, Reason: "outside allowed domains"})
					continue
				}
				if item.depth+1 > limits.maxDepth {
					addCrawlSkip(&result, CrawlSkip{URL: linkURL, Depth: item.depth + 1, Reason: "exceeds max depth"})
					continue
				}
				queue = append(queue, crawlQueueItem{url: linkURL, depth: item.depth + 1})
			}
		}
		result.Pages = append(result.Pages, page)
	}

	return s.finishCrawlResult(result), nil
}

func (s *Service) crawlRobotsAllowed(ctx context.Context, rawURL string, userAgent string, taskApproved bool) (RobotsResult, bool) {
	result, err := s.CheckRobots(ctx, rawURL, firstNonEmpty(userAgent, defaultCrawlRobotsUserAgent), taskApproved)
	if err != nil {
		if result.Reason == "" {
			result.Reason = err.Error()
		}
		return result, false
	}
	return result, result.Allowed
}

func normalizeCrawlLimits(input CrawlInput) crawlLimits {
	return crawlLimits{
		maxPages:           boundedPositive(input.MaxPages, defaultCrawlMaxPages, absoluteCrawlMaxPages),
		maxDepth:           boundedNonNegative(input.MaxDepth, defaultCrawlMaxDepth, absoluteCrawlMaxDepth),
		maxDurationSeconds: boundedPositive(input.MaxDurationSeconds, defaultCrawlDurationSecs, absoluteCrawlDurationSecs),
		maxLinksPerPage:    boundedPositive(input.MaxLinksPerPage, defaultCrawlLinksPerPage, absoluteCrawlLinksPerPage),
		maxTextChars:       boundedPositive(input.MaxTextChars, defaultCrawlMaxTextChars, absoluteCrawlMaxTextChars),
	}
}

func boundedPositive(value int, fallback int, max int) int {
	if value <= 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func boundedNonNegative(value int, fallback int, max int) int {
	if value < 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func crawlAllowedDomains(input []string, seedHost string) []string {
	seen := map[string]bool{}
	var domains []string
	for _, domain := range append([]string{seedHost}, input...) {
		domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
		if domain == "" || seen[domain] {
			continue
		}
		seen[domain] = true
		domains = append(domains, domain)
	}
	return domains
}

func normalizeCrawlURL(raw string, base *url.URL) (string, *url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, fmt.Errorf("url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", nil, fmt.Errorf("parse url: %w", err)
	}
	if base != nil && !parsed.IsAbs() {
		parsed = base.ResolveReference(parsed)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", nil, fmt.Errorf("only http and https URLs are allowed")
	}
	if parsed.Host == "" {
		return "", nil, fmt.Errorf("url must include scheme and host")
	}
	if parsed.User != nil {
		return "", nil, fmt.Errorf("url credentials are not allowed")
	}
	normalized := *parsed
	normalized.Fragment = ""
	normalized.Host = strings.ToLower(normalized.Host)
	if normalized.Path == "" {
		normalized.Path = "/"
	}
	return normalized.String(), &normalized, nil
}

func extractCrawlLinks(body string, contentType string, base *url.URL, maxLinks int) []string {
	if maxLinks <= 0 || base == nil || !crawlBodyLooksHTML(body, contentType) {
		return nil
	}
	document, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var links []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil || len(links) >= maxLinks {
			return
		}
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "a") {
			for _, attr := range node.Attr {
				if !strings.EqualFold(attr.Key, "href") {
					continue
				}
				link, _, err := normalizeCrawlURL(attr.Val, base)
				if err != nil || seen[link] {
					break
				}
				seen[link] = true
				links = append(links, link)
				break
			}
		}
		for child := node.FirstChild; child != nil && len(links) < maxLinks; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	return links
}

func crawlBodyLooksHTML(body string, contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.Contains(contentType, "html") || strings.Contains(strings.ToLower(body), "<a")
}

func truncateCrawlText(text string, maxChars int) string {
	text = strings.TrimSpace(text)
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	for idx := range text {
		if idx > maxChars {
			return strings.TrimSpace(text[:idx]) + "..."
		}
	}
	return text
}

func addCrawlSkip(result *CrawlResult, skip CrawlSkip) {
	result.Skipped++
	if len(result.Skips) >= maxRecordedCrawlSkips {
		result.SkipOverflow++
		return
	}
	result.Skips = append(result.Skips, skip)
}

func (s *Service) finishCrawlResult(result CrawlResult) CrawlResult {
	if result.Status == "" || result.Status == "running" {
		if len(result.Pages) == 0 && len(result.Skips) > 0 {
			result.Status = "blocked"
			result.FailureReason = firstCrawlSkipReason(result.Skips)
		} else {
			result.Status = "completed"
		}
	}
	if strings.TrimSpace(result.FailureReason) == "" && result.Status == "blocked" && len(result.Skips) > 0 {
		result.FailureReason = firstCrawlSkipReason(result.Skips)
	}
	result.Fetched = len(result.Pages)
	result.FinishedAt = s.timestamp()
	_ = s.logCrawlRun(crawlRunRecordFromResult(result))
	return result
}

func firstCrawlSkipReason(skips []CrawlSkip) string {
	for _, skip := range skips {
		if reason := strings.TrimSpace(skip.Reason); reason != "" {
			return reason
		}
	}
	return "crawl completed without fetching any pages"
}

func crawlRunID(started string, seedURL string, method string) string {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(started))
	_, _ = hash.Write([]byte("\x00"))
	_, _ = hash.Write([]byte(seedURL))
	_, _ = hash.Write([]byte("\x00"))
	_, _ = hash.Write([]byte(method))
	return fmt.Sprintf("crawl_%016x", hash.Sum64())
}
