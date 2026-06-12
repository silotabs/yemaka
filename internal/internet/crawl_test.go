package internet

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yemaka/internal/config"
)

func TestCrawlRequiresTaskApproval(t *testing.T) {
	service := testService(t, enabledTestConfig())
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request to %s", r.URL.String())
		return nil, nil
	})}

	result, err := service.Crawl(context.Background(), CrawlInput{SeedURL: "https://example.com"})
	if err == nil || !strings.Contains(err.Error(), "explicit task approval") {
		t.Fatalf("Crawl() error = %v, want approval error", err)
	}
	if result.Status != "blocked" {
		t.Fatalf("result status = %q, want blocked", result.Status)
	}
}

func TestCrawlRequiresControlledInternetEnabled(t *testing.T) {
	service := testService(t, config.Default().Internet)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request to %s", r.URL.String())
		return nil, nil
	})}

	result, err := service.Crawl(context.Background(), CrawlInput{
		SeedURL:      "https://example.com",
		TaskApproved: true,
	})
	if err == nil || !strings.Contains(err.Error(), "crawler disabled") {
		t.Fatalf("Crawl() error = %v, want disabled error", err)
	}
	if result.Status != "blocked" {
		t.Fatalf("result status = %q, want blocked", result.Status)
	}
}

func TestCrawlDefaultsToSameDomain(t *testing.T) {
	service := testService(t, enabledTestConfig())
	var fetched []string
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() != "example.com" {
			t.Fatalf("unexpected host fetch: %s", r.URL.String())
		}
		fetched = append(fetched, r.URL.Path)
		switch r.URL.Path {
		case "/robots.txt":
			return crawlHTTPResponse(r, http.StatusNotFound, "text/plain", ""), nil
		case "/":
			return crawlHTTPResponse(r, http.StatusOK, "text/html", `<a href="/inside">Inside</a><a href="https://outside.example/page">Outside</a>`), nil
		case "/inside":
			return crawlHTTPResponse(r, http.StatusOK, "text/html", `<p>inside</p>`), nil
		default:
			return crawlHTTPResponse(r, http.StatusNotFound, "text/plain", ""), nil
		}
	})}

	result, err := service.Crawl(context.Background(), CrawlInput{
		SeedURL:      "https://example.com/",
		MaxPages:     5,
		MaxDepth:     1,
		TaskApproved: true,
	})
	if err != nil {
		t.Fatalf("Crawl() error = %v", err)
	}
	if len(result.Pages) != 2 {
		t.Fatalf("pages = %+v, want seed and same-domain child", result.Pages)
	}
	if !crawlHasPage(result, "https://example.com/inside") {
		t.Fatalf("pages = %+v, want /inside fetched", result.Pages)
	}
	if !crawlHasSkip(result, "outside allowed domains") {
		t.Fatalf("skips = %+v, want outside-domain skip", result.Skips)
	}
	for _, path := range fetched {
		if path == "/page" {
			t.Fatalf("fetched outside-domain page: paths=%+v", fetched)
		}
	}
}

func TestCrawlAllowsConfiguredAdditionalDomain(t *testing.T) {
	service := testService(t, enabledTestConfig())
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/robots.txt":
			return crawlHTTPResponse(r, http.StatusNotFound, "text/plain", ""), nil
		case r.URL.Hostname() == "example.com":
			return crawlHTTPResponse(r, http.StatusOK, "text/html", `<a href="https://docs.example.net/page">Docs</a>`), nil
		case r.URL.Hostname() == "docs.example.net":
			return crawlHTTPResponse(r, http.StatusOK, "text/html", `<p>docs</p>`), nil
		default:
			t.Fatalf("unexpected request: %s", r.URL.String())
			return nil, nil
		}
	})}

	result, err := service.Crawl(context.Background(), CrawlInput{
		SeedURL:        "https://example.com/",
		AllowedDomains: []string{"docs.example.net"},
		MaxPages:       5,
		MaxDepth:       1,
		TaskApproved:   true,
	})
	if err != nil {
		t.Fatalf("Crawl() error = %v", err)
	}
	if !crawlHasPage(result, "https://docs.example.net/page") {
		t.Fatalf("pages = %+v, want configured domain page fetched", result.Pages)
	}
}

func TestCrawlHonorsMaxPagesAndDepth(t *testing.T) {
	t.Run("max pages", func(t *testing.T) {
		service := testService(t, enabledTestConfig())
		service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/robots.txt" {
				return crawlHTTPResponse(r, http.StatusNotFound, "text/plain", ""), nil
			}
			return crawlHTTPResponse(r, http.StatusOK, "text/html", `<a href="/a">A</a><a href="/b">B</a>`), nil
		})}

		result, err := service.Crawl(context.Background(), CrawlInput{
			SeedURL:      "https://example.com/",
			MaxPages:     1,
			MaxDepth:     2,
			TaskApproved: true,
		})
		if err != nil {
			t.Fatalf("Crawl() error = %v", err)
		}
		if len(result.Pages) != 1 || result.Status != "max_pages_reached" {
			t.Fatalf("result = %+v, want one page and max_pages_reached", result)
		}
		if !crawlHasSkip(result, "max pages limit reached") {
			t.Fatalf("skips = %+v, want max pages skip", result.Skips)
		}
	})

	t.Run("max depth", func(t *testing.T) {
		service := testService(t, enabledTestConfig())
		service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/robots.txt" {
				return crawlHTTPResponse(r, http.StatusNotFound, "text/plain", ""), nil
			}
			return crawlHTTPResponse(r, http.StatusOK, "text/html", `<a href="/too-deep">Too deep</a>`), nil
		})}

		result, err := service.Crawl(context.Background(), CrawlInput{
			SeedURL:      "https://example.com/",
			MaxPages:     5,
			MaxDepth:     0,
			TaskApproved: true,
		})
		if err != nil {
			t.Fatalf("Crawl() error = %v", err)
		}
		if len(result.Pages) != 1 {
			t.Fatalf("pages = %+v, want only seed page", result.Pages)
		}
		if !crawlHasSkip(result, "exceeds max depth") {
			t.Fatalf("skips = %+v, want depth skip", result.Skips)
		}
	})
}

func TestCrawlRecordsBoundedRunHistory(t *testing.T) {
	service := testService(t, enabledTestConfig())
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/robots.txt" {
			return crawlHTTPResponse(r, http.StatusNotFound, "text/plain", ""), nil
		}
		return crawlHTTPResponse(r, http.StatusOK, "text/html", `<a href="/a">A</a><p>private-ish body text should not be stored in crawl history</p>`), nil
	})}

	result, err := service.Crawl(context.Background(), CrawlInput{
		SeedURL:      "https://example.com/",
		MaxPages:     1,
		MaxDepth:     1,
		ExtractText:  true,
		TaskApproved: true,
	})
	if err != nil {
		t.Fatalf("Crawl() error = %v", err)
	}
	if result.RunID == "" {
		t.Fatal("result run ID is empty")
	}
	runs, err := service.CrawlRuns(10)
	if err != nil {
		t.Fatalf("CrawlRuns() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %+v, want one run", runs)
	}
	run := runs[0]
	if run.RunID != result.RunID || run.Status != "max_pages_reached" || run.Fetched != 1 {
		t.Fatalf("run = %+v, want matching max-pages record", run)
	}
	if len(run.Pages) != 1 || run.Pages[0].LinkCount != 1 || run.Pages[0].TextBytes == 0 {
		t.Fatalf("run page summary = %+v, want bounded page metadata", run.Pages)
	}
	encoded, _ := json.Marshal(run)
	if strings.Contains(string(encoded), "private-ish body text") {
		t.Fatalf("crawl history stored raw page text: %s", string(encoded))
	}
	found, err := service.CrawlRun(result.RunID)
	if err != nil {
		t.Fatalf("CrawlRun() error = %v", err)
	}
	if found.RunID != result.RunID || found.SeedURL != result.SeedURL {
		t.Fatalf("found crawl run = %+v, want run %q", found, result.RunID)
	}
}

func TestCrawlRespectsRobotsDisallow(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.Cache.Enabled = false
	var privateFetched bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("User-agent: *\nDisallow: /private\n"))
		case "/private/page":
			privateFetched = true
			_, _ = w.Write([]byte("<p>private</p>"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	service := testService(t, cfg)

	result, err := service.Crawl(context.Background(), CrawlInput{
		SeedURL:      server.URL + "/private/page",
		MaxPages:     5,
		MaxDepth:     1,
		TaskApproved: true,
	})
	if err != nil {
		t.Fatalf("Crawl() error = %v", err)
	}
	if privateFetched {
		t.Fatal("crawler fetched a robots-disallowed page")
	}
	if len(result.Pages) != 0 {
		t.Fatalf("pages = %+v, want none", result.Pages)
	}
	if result.Status != "blocked" || !strings.Contains(result.FailureReason, "robots") {
		t.Fatalf("result = %+v, want blocked robots failure", result)
	}
	if !crawlHasSkip(result, "robots check blocked fetch") || !crawlHasSkip(result, "disallow") {
		t.Fatalf("skips = %+v, want robots disallow skip", result.Skips)
	}
	requests, err := service.Requests(10)
	if err != nil {
		t.Fatalf("Requests() error = %v", err)
	}
	if !requestLogHasCaller(requests, "robots") {
		t.Fatalf("requests = %+v, want logged robots request", requests)
	}
	runs, err := service.CrawlRuns(10)
	if err != nil {
		t.Fatalf("CrawlRuns() error = %v", err)
	}
	if len(runs) != 1 || runs[0].Status != "blocked" || runs[0].FailureReason == "" {
		t.Fatalf("runs = %+v, want blocked run with failure reason", runs)
	}
}

func TestCrawlUsesExistingFetchPolicyForPrivateLocalBlocks(t *testing.T) {
	cfg := config.Default().Internet
	cfg.Enabled = true
	service := testService(t, cfg)
	service.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request to %s", r.URL.String())
		return nil, nil
	})}

	result, err := service.Crawl(context.Background(), CrawlInput{
		SeedURL:      "http://127.0.0.1:1234/",
		TaskApproved: true,
	})
	if err != nil {
		t.Fatalf("Crawl() error = %v", err)
	}
	if !crawlHasSkip(result, "private/local IP ranges are blocked") {
		t.Fatalf("skips = %+v, want private/local policy skip", result.Skips)
	}
	if result.Status != "blocked" || result.FailureReason == "" {
		t.Fatalf("result = %+v, want blocked result with failure reason", result)
	}
}

func TestCrawlRunFindsOlderThanRecentLimit(t *testing.T) {
	service := testService(t, enabledTestConfig())
	for i := 0; i < 25; i++ {
		if err := service.logCrawlRun(CrawlRunRecord{
			RunID:     "crawl_test_" + string(rune('a'+i)),
			SeedURL:   "https://example.com/",
			Method:    http.MethodGet,
			Status:    "completed",
			StartedAt: "2026-05-08T12:00:00Z",
		}); err != nil {
			t.Fatalf("logCrawlRun(%d) error = %v", i, err)
		}
	}
	found, err := service.CrawlRun("crawl_test_a")
	if err != nil {
		t.Fatalf("CrawlRun(oldest) error = %v", err)
	}
	if found.RunID != "crawl_test_a" {
		t.Fatalf("found = %+v, want oldest crawl record", found)
	}
}

func crawlHTTPResponse(r *http.Request, status int, contentType string, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header: http.Header{
			"Content-Type": []string{contentType},
		},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       r,
	}
}

func crawlHasPage(result CrawlResult, target string) bool {
	for _, page := range result.Pages {
		if page.URL == target || page.FinalURL == target {
			return true
		}
	}
	return false
}

func crawlHasSkip(result CrawlResult, needle string) bool {
	for _, skip := range result.Skips {
		if strings.Contains(skip.Reason, needle) {
			return true
		}
	}
	return false
}

func requestLogHasCaller(records []RequestRecord, caller string) bool {
	for _, record := range records {
		if record.Caller == caller {
			return true
		}
	}
	return false
}
