package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunInternetStatusAndToggle(t *testing.T) {
	app := newExtensionTestApp(t)

	var out bytes.Buffer
	if err := runInternet(context.Background(), app, []string{"status"}, &out); err != nil {
		t.Fatalf("runInternet(status) error = %v", err)
	}
	if !strings.Contains(out.String(), `"enabled": false`) || !strings.Contains(out.String(), "ask_each_time") {
		t.Fatalf("status output = %q", out.String())
	}

	out.Reset()
	if err := runInternet(context.Background(), app, []string{"on"}, &out); err != nil {
		t.Fatalf("runInternet(on) error = %v", err)
	}
	if !app.config.Internet.Enabled || app.config.Internet.DefaultMode != "profile_enabled" {
		t.Fatalf("internet config = %+v, want profile enabled", app.config.Internet)
	}

	out.Reset()
	if err := runInternet(context.Background(), app, []string{"off"}, &out); err != nil {
		t.Fatalf("runInternet(off) error = %v", err)
	}
	if app.config.Internet.Enabled {
		t.Fatal("internet should be disabled")
	}
}

func TestRunInternetFetchRequiresApprovalWhenDisabled(t *testing.T) {
	app := newExtensionTestApp(t)
	var out bytes.Buffer
	err := runInternet(context.Background(), app, []string{"fetch", "https://example.com", "--domain", "example.com"}, &out)
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("runInternet(fetch) error = %v, want disabled error", err)
	}
}

func TestRunInternetSearchProviderConfiguresSearXNG(t *testing.T) {
	app := newExtensionTestApp(t)
	var out bytes.Buffer

	err := runInternet(context.Background(), app, []string{"search-provider", "searxng", "https://search.example/search", "--max-results", "7"}, &out)
	if err != nil {
		t.Fatalf("runInternet(search-provider searxng) error = %v", err)
	}
	if !app.config.Internet.Search.Enabled || app.config.Internet.Search.Provider != "searxng" {
		t.Fatalf("search config = %+v, want enabled searxng", app.config.Internet.Search)
	}
	if app.config.Internet.Search.Endpoint != "https://search.example/search" || app.config.Internet.Search.MaxResults != 7 {
		t.Fatalf("search config = %+v, want endpoint and max results", app.config.Internet.Search)
	}
	if app.config.Internet.Enabled {
		t.Fatal("provider setup should not enable internet unless explicitly requested")
	}
	if !strings.Contains(out.String(), "internet remains disabled") {
		t.Fatalf("output = %q, want disabled reminder", out.String())
	}
}

func TestRunInternetSearchProviderCanEnableInternet(t *testing.T) {
	t.Setenv("BRAVE_SEARCH_API_KEY", "test-secret")
	app := newExtensionTestApp(t)
	var out bytes.Buffer

	err := runInternet(context.Background(), app, []string{"search-provider", "brave", "BRAVE_SEARCH_API_KEY", "--enable-internet"}, &out)
	if err != nil {
		t.Fatalf("runInternet(search-provider brave) error = %v", err)
	}
	if !app.config.Internet.Enabled || app.config.Internet.DefaultMode != "profile_enabled" {
		t.Fatalf("internet config = %+v, want profile enabled", app.config.Internet)
	}
	if !app.config.Internet.Search.Enabled || app.config.Internet.Search.Provider != "brave" || app.config.Internet.Search.APIKeyEnv != "BRAVE_SEARCH_API_KEY" {
		t.Fatalf("search config = %+v, want brave api key env", app.config.Internet.Search)
	}

	out.Reset()
	if err := runInternet(context.Background(), app, []string{"search-provider", "status"}, &out); err != nil {
		t.Fatalf("runInternet(search-provider status) error = %v", err)
	}
	if !strings.Contains(out.String(), "ready: true") {
		t.Fatalf("status output = %q, want ready true", out.String())
	}
}

func TestRunInternetSearchProviderConfiguresPostProviders(t *testing.T) {
	app := newExtensionTestApp(t)
	var out bytes.Buffer

	err := runInternet(context.Background(), app, []string{"search-provider", "tavily", "TAVILY_API_KEY", "--endpoint", "https://api.tavily.test/search", "--enable-internet"}, &out)
	if err != nil {
		t.Fatalf("runInternet(search-provider tavily) error = %v", err)
	}
	if !app.config.Internet.Search.Enabled || app.config.Internet.Search.Provider != "tavily" || app.config.Internet.Search.APIKeyEnv != "TAVILY_API_KEY" {
		t.Fatalf("search config = %+v, want tavily api key env", app.config.Internet.Search)
	}
	if got := app.config.Internet.Search.Endpoint; got != "https://api.tavily.test/search" {
		t.Fatalf("endpoint = %q, want Tavily override", got)
	}

	out.Reset()
	err = runInternet(context.Background(), app, []string{"search-provider", "serper", "SERPER_API_KEY"}, &out)
	if err != nil {
		t.Fatalf("runInternet(search-provider serper) error = %v", err)
	}
	if app.config.Internet.Search.Provider != "serper" || app.config.Internet.Search.APIKeyEnv != "SERPER_API_KEY" {
		t.Fatalf("search config = %+v, want serper api key env", app.config.Internet.Search)
	}
}

func TestRunInternetSearchProviderConfiguresFallbackProviders(t *testing.T) {
	app := newExtensionTestApp(t)
	var out bytes.Buffer

	err := runInternet(context.Background(), app, []string{"search-provider", "duckduckgo", "--enable-internet"}, &out)
	if err != nil {
		t.Fatalf("runInternet(search-provider duckduckgo) error = %v", err)
	}
	if app.config.Internet.Search.Provider != "duckduckgo" || app.config.Internet.Search.APIKeyEnv != "" {
		t.Fatalf("search config = %+v, want DuckDuckGo no-key provider", app.config.Internet.Search)
	}

	out.Reset()
	err = runInternet(context.Background(), app, []string{"search-provider", "auto"}, &out)
	if err != nil {
		t.Fatalf("runInternet(search-provider auto) error = %v", err)
	}
	if app.config.Internet.Search.Provider != "auto" || app.config.Internet.Search.Endpoint != "" || app.config.Internet.Search.APIKeyEnv != "" {
		t.Fatalf("search config = %+v, want auto fallback provider", app.config.Internet.Search)
	}
}

func TestRunInternetSearchProviderDisable(t *testing.T) {
	app := newExtensionTestApp(t)
	app.config.Internet.Search.Enabled = true
	app.config.Internet.Search.Provider = "searxng"
	app.config.Internet.Search.Endpoint = "https://search.example/search"

	var out bytes.Buffer
	if err := runInternet(context.Background(), app, []string{"search-provider", "off"}, &out); err != nil {
		t.Fatalf("runInternet(search-provider off) error = %v", err)
	}
	if app.config.Internet.Search.Enabled || app.config.Internet.Search.Provider != "none" || app.config.Internet.Search.Endpoint != "" {
		t.Fatalf("search config = %+v, want disabled none", app.config.Internet.Search)
	}
}

func TestRunInternetSearchProviderMissingValueReturnsUsage(t *testing.T) {
	app := newExtensionTestApp(t)
	var out bytes.Buffer
	err := runInternet(context.Background(), app, []string{"search-provider", "searxng"}, &out)
	if err == nil || !strings.Contains(err.Error(), "search-provider searxng") {
		t.Fatalf("runInternet(search-provider missing endpoint) error = %v, want usage", err)
	}
}
