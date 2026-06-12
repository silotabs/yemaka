package internet

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"yemaka/internal/config"
)

type SearchProviderReadiness struct {
	Provider       string `json:"provider"`
	Label          string `json:"label"`
	Configured     bool   `json:"configured"`
	Ready          bool   `json:"ready"`
	NeedsConfig    bool   `json:"needsConfig"`
	NeedsAuth      bool   `json:"needsAuth"`
	NetworkChecked bool   `json:"networkChecked"`
	Status         string `json:"status"`
}

// SearchProvider finds result URLs through a configured provider. Fetching a
// result page and crawling link graphs remain separate internet primitives.
type SearchProvider interface {
	Name() string
	Label() string
	Readiness(config.InternetConfig) SearchProviderReadiness
	FetchInput(config.InternetConfig, SearchInput, int) (FetchInput, error)
	Parse(string) ([]SearchItem, error)
}

var builtInSearchProviders = map[string]SearchProvider{
	"searxng":       searxngSearchProvider{},
	"brave":         braveSearchProvider{},
	"wikimedia":     wikimediaSearchProvider{},
	"tavily":        tavilySearchProvider{},
	"serper":        serperSearchProvider{},
	"duckduckgo":    duckDuckGoSearchProvider{},
	"firecrawl":     firecrawlSearchProvider{},
	"mojeek":        mojeekSearchProvider{},
	"yemaka_custom": yemakaCustomSearchProvider{},
}

func searchProviderForName(provider string) (SearchProvider, bool) {
	provider = normalizeSearchProvider(provider)
	if provider == "" || provider == "none" {
		return nil, false
	}
	searchProvider, ok := builtInSearchProviders[provider]
	return searchProvider, ok
}

func searchProviderConfiguredForPolicy(cfg config.InternetConfig) bool {
	provider := normalizeSearchProvider(cfg.Search.Provider)
	return cfg.Search.Enabled && provider != "" && provider != "none"
}

func defaultSearchProviderPriority() []string {
	return []string{"tavily", "serper", "brave", "firecrawl", "wikimedia", "duckduckgo", "searxng"}
}

func searchProviderPriority(cfg config.InternetConfig) []string {
	if len(cfg.Search.FallbackProviders) == 0 {
		return defaultSearchProviderPriority()
	}
	providers := make([]string, 0, len(cfg.Search.FallbackProviders))
	seen := map[string]bool{}
	for _, provider := range cfg.Search.FallbackProviders {
		provider = normalizeSearchProvider(provider)
		if provider == "" || provider == "none" || provider == "auto" || seen[provider] {
			continue
		}
		providers = append(providers, provider)
		seen[provider] = true
	}
	if len(providers) == 0 {
		return defaultSearchProviderPriority()
	}
	return providers
}

func searchReadiness(cfg config.InternetConfig) SearchProviderReadiness {
	providerName := normalizeSearchProvider(cfg.Search.Provider)
	readiness := SearchProviderReadiness{
		Provider: providerName,
		Label:    searchProviderLabel(providerName),
	}
	if !cfg.Search.Enabled || providerName == "" || providerName == "none" {
		readiness.NeedsConfig = true
		if !cfg.Enabled {
			readiness.Status = "internet access is disabled"
		} else {
			readiness.Status = "internet search is disabled until a search provider is explicitly configured"
		}
		return readiness
	}
	if providerName == "auto" {
		readiness = autoSearchReadiness(cfg)
		if !cfg.Enabled {
			readiness.Ready = false
			readiness.Status = "internet access is disabled"
		}
		return readiness
	}
	searchProvider, ok := searchProviderForName(providerName)
	if !ok {
		readiness.NeedsConfig = true
		if !cfg.Enabled {
			readiness.Status = "internet access is disabled"
		} else {
			readiness.Status = fmt.Sprintf("internet search provider %q is not supported", cfg.Search.Provider)
		}
		return readiness
	}
	readiness = searchProvider.Readiness(cfg)
	if !cfg.Enabled {
		readiness.Ready = false
		readiness.Status = "internet access is disabled"
	}
	return readiness
}

func autoSearchReadiness(cfg config.InternetConfig) SearchProviderReadiness {
	readiness := SearchProviderReadiness{
		Provider:   "auto",
		Label:      "Auto fallback",
		Configured: true,
	}
	var blocked []string
	for _, providerName := range searchProviderPriority(cfg) {
		searchProvider, ok := searchProviderForName(providerName)
		if !ok {
			continue
		}
		providerReadiness := searchProvider.Readiness(cfg)
		if providerReadiness.Ready {
			readiness.Ready = true
			readiness.Status = fmt.Sprintf("auto search is ready; first available provider is %s", providerReadiness.Label)
			return readiness
		}
		if strings.TrimSpace(providerReadiness.Status) != "" {
			blocked = append(blocked, fmt.Sprintf("%s: %s", providerReadiness.Label, providerReadiness.Status))
		}
	}
	readiness.NeedsConfig = true
	readiness.Status = "auto search has no ready provider"
	if len(blocked) > 0 {
		readiness.Status += "; " + strings.Join(blocked, " | ")
	}
	return readiness
}

func searchProviderLabel(provider string) string {
	if searchProvider, ok := searchProviderForName(provider); ok {
		return searchProvider.Label()
	}
	switch normalizeSearchProvider(provider) {
	case "auto":
		return "Auto fallback"
	case "searxng":
		return "SearXNG"
	case "brave":
		return "Brave"
	case "wikimedia":
		return "Wikimedia"
	case "tavily":
		return "Tavily"
	case "serper":
		return "Serper.dev"
	case "duckduckgo":
		return "DuckDuckGo"
	case "firecrawl":
		return "Firecrawl"
	case "mojeek":
		return "Mojeek"
	case "yemaka_custom":
		return "Yemaka Custom Search"
	case "none", "":
		return ""
	default:
		return provider
	}
}

func ValidateSearchAPIKeyEnvForProvider(provider string, apiKeyEnv string) error {
	return validateSearchAPIKeyEnvName(normalizeSearchProvider(provider), apiKeyEnv)
}

// NormalizeSearchAPIKeyEnvForProvider preserves custom environment variable
// names, while correcting stale provider defaults left behind by the settings UI
// when switching between keyed providers.
func NormalizeSearchAPIKeyEnvForProvider(provider string, apiKeyEnv string) string {
	provider = normalizeSearchProvider(provider)
	apiKeyEnv = strings.TrimSpace(apiKeyEnv)
	if apiKeyEnv == "" {
		return ""
	}
	if !searchProviderUsesAPIKey(provider) {
		return ""
	}
	providerDefault := defaultSearchAPIKeyEnv(provider)
	if providerDefault != "" && apiKeyEnv != providerDefault && isDefaultSearchAPIKeyEnv(apiKeyEnv) {
		return providerDefault
	}
	return apiKeyEnv
}

func isDefaultSearchAPIKeyEnv(apiKeyEnv string) bool {
	apiKeyEnv = strings.TrimSpace(apiKeyEnv)
	if apiKeyEnv == "" {
		return false
	}
	for _, provider := range []string{"firecrawl", "brave", "tavily", "serper", "mojeek"} {
		if apiKeyEnv == defaultSearchAPIKeyEnv(provider) {
			return true
		}
	}
	return false
}

func searchProviderUsesAPIKey(provider string) bool {
	switch normalizeSearchProvider(provider) {
	case "firecrawl", "brave", "tavily", "serper", "mojeek":
		return true
	default:
		return false
	}
}

func searchAPIKeyFromEnv(provider string, apiKeyEnv string) (string, error) {
	provider = normalizeSearchProvider(provider)
	apiKeyEnv = NormalizeSearchAPIKeyEnvForProvider(provider, apiKeyEnv)
	if apiKeyEnv == "" {
		apiKeyEnv = defaultSearchAPIKeyEnv(provider)
	}
	if err := validateSearchAPIKeyEnvName(provider, apiKeyEnv); err != nil {
		return "", err
	}
	apiKey := strings.TrimSpace(os.Getenv(apiKeyEnv))
	if apiKey == "" {
		return "", fmt.Errorf("%s search API key environment variable %s is not set or not visible to Yemaka; set it before launching the app, or restart Yemaka after setting it", searchProviderLabel(provider), apiKeyEnv)
	}
	return apiKey, nil
}

func applySearchAPIKeyReadiness(readiness *SearchProviderReadiness, provider string, apiKeyEnv string) bool {
	if _, err := searchAPIKeyFromEnv(provider, apiKeyEnv); err != nil {
		readiness.NeedsAuth = true
		readiness.Status = err.Error()
		return false
	}
	return true
}

func validateSearchAPIKeyEnvName(provider string, apiKeyEnv string) error {
	apiKeyEnv = strings.TrimSpace(apiKeyEnv)
	if apiKeyEnv == "" {
		return nil
	}
	label := searchProviderLabel(provider)
	if looksLikeRawAPIKey(apiKeyEnv) {
		return fmt.Errorf("%s search API key setting looks like an API key value; enter an environment variable name such as %s. Yemaka will not store raw API keys", label, defaultSearchAPIKeyEnv(provider))
	}
	for index, char := range apiKeyEnv {
		valid := char == '_' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (index > 0 && char >= '0' && char <= '9')
		if !valid {
			return fmt.Errorf("%s search API key environment variable name is invalid; use letters, numbers, and underscores only, for example %s", label, defaultSearchAPIKeyEnv(provider))
		}
	}
	return nil
}

func looksLikeRawAPIKey(value string) bool {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	return strings.HasPrefix(lower, "fc-") ||
		strings.HasPrefix(lower, "sk-") ||
		strings.HasPrefix(lower, "pk-")
}

func defaultSearchAPIKeyEnv(provider string) string {
	switch normalizeSearchProvider(provider) {
	case "firecrawl":
		return "FIRECRAWL_API_KEY"
	case "brave":
		return "BRAVE_SEARCH_API_KEY"
	case "tavily":
		return "TAVILY_API_KEY"
	case "serper":
		return "SERPER_API_KEY"
	case "mojeek":
		return "MOJEEK_API_KEY"
	default:
		return "PROVIDER_API_KEY"
	}
}

type searxngSearchProvider struct{}

func (searxngSearchProvider) Name() string {
	return "searxng"
}

func (searxngSearchProvider) Label() string {
	return "SearXNG"
}

func (provider searxngSearchProvider) Readiness(cfg config.InternetConfig) SearchProviderReadiness {
	readiness := SearchProviderReadiness{
		Provider:   provider.Name(),
		Label:      provider.Label(),
		Configured: true,
	}
	endpoint := strings.TrimSpace(cfg.Search.Endpoint)
	if endpoint == "" {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = "searxng search requires internet.search.endpoint"
		return readiness
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = fmt.Sprintf("searxng endpoint is invalid: %v", err)
		return readiness
	}
	if endpointHasSecretQuery(parsed) {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = "searxng endpoint includes secret-like query parameters; use a secret-safe endpoint without API keys in the URL"
		return readiness
	}
	host := parsed.Hostname()
	if cfg.Policy.BlockLocalNetworkByDefault && isLocalHostname(host) {
		readiness.NeedsConfig = true
		readiness.Status = "searxng endpoint is blocked because local network hosts are blocked by default"
		return readiness
	}
	if cfg.Policy.BlockPrivateIPRanges {
		if ip := net.ParseIP(host); ip != nil && isPrivateOrLocalIP(ip) {
			readiness.NeedsConfig = true
			readiness.Status = fmt.Sprintf("searxng endpoint is blocked because private/local IP ranges are blocked: %s", host)
			return readiness
		}
	}
	readiness.Ready = true
	readiness.Status = "search provider is ready"
	return readiness
}

func (searxngSearchProvider) FetchInput(cfg config.InternetConfig, input SearchInput, limit int) (FetchInput, error) {
	if err := rejectSecretQueryEndpoint("searxng", cfg.Search.Endpoint); err != nil {
		return FetchInput{}, err
	}
	rawURL, err := searxngSearchURL(cfg.Search.Endpoint, input.Query, limit, cfg.Search.SafeSearch)
	if err != nil {
		return FetchInput{}, err
	}
	host, err := hostForURL(rawURL)
	if err != nil {
		return FetchInput{}, err
	}
	return FetchInput{
		URL:            rawURL,
		Method:         http.MethodGet,
		AllowedDomains: []string{host},
		TaskApproved:   input.TaskApproved,
		Caller:         input.Caller,
	}, nil
}

func (searxngSearchProvider) Parse(body string) ([]SearchItem, error) {
	items, err := parseSearXNGItems(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, bodySnippet(body, 180))
	}
	return items, nil
}

type braveSearchProvider struct{}

func (braveSearchProvider) Name() string {
	return "brave"
}

func (braveSearchProvider) Label() string {
	return "Brave"
}

func (provider braveSearchProvider) Readiness(cfg config.InternetConfig) SearchProviderReadiness {
	readiness := SearchProviderReadiness{
		Provider:   provider.Name(),
		Label:      provider.Label(),
		Configured: true,
	}
	if !applySearchAPIKeyReadiness(&readiness, provider.Name(), cfg.Search.APIKeyEnv) {
		return readiness
	}
	if strings.TrimSpace(cfg.Search.Endpoint) != "" {
		parsed, err := parseURL(cfg.Search.Endpoint)
		if err != nil {
			readiness.Configured = false
			readiness.NeedsConfig = true
			readiness.Status = fmt.Sprintf("brave endpoint is invalid: %v", err)
			return readiness
		}
		if endpointHasSecretQuery(parsed) {
			readiness.Configured = false
			readiness.NeedsConfig = true
			readiness.Status = "brave endpoint includes secret-like query parameters; use internet.search.api_key_env instead of API keys in the URL"
			return readiness
		}
		if blockMessage := blockedEndpointMessage("brave", parsed.Hostname(), cfg); blockMessage != "" {
			readiness.NeedsConfig = true
			readiness.Status = blockMessage
			return readiness
		}
	}
	readiness.Ready = true
	readiness.Status = "search provider is ready"
	return readiness
}

func (braveSearchProvider) FetchInput(cfg config.InternetConfig, input SearchInput, limit int) (FetchInput, error) {
	if strings.TrimSpace(cfg.Search.Endpoint) != "" {
		if err := rejectSecretQueryEndpoint("brave", cfg.Search.Endpoint); err != nil {
			return FetchInput{}, err
		}
	}
	rawURL, err := braveSearchURL(cfg.Search.Endpoint, input.Query, limit, cfg.Search.SafeSearch)
	if err != nil {
		return FetchInput{}, err
	}
	apiKey, err := searchAPIKeyFromEnv("brave", cfg.Search.APIKeyEnv)
	if err != nil {
		return FetchInput{}, err
	}
	host, err := hostForURL(rawURL)
	if err != nil {
		return FetchInput{}, err
	}
	return FetchInput{
		URL:            rawURL,
		Method:         http.MethodGet,
		AllowedDomains: []string{host},
		TaskApproved:   input.TaskApproved,
		Caller:         input.Caller,
		Headers: map[string]string{
			"Accept":               "application/json",
			"X-Subscription-Token": apiKey,
		},
	}, nil
}

func (braveSearchProvider) Parse(body string) ([]SearchItem, error) {
	items, err := parseBraveItems(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, bodySnippet(body, 180))
	}
	return items, nil
}

type wikimediaSearchProvider struct{}

func (wikimediaSearchProvider) Name() string {
	return "wikimedia"
}

func (wikimediaSearchProvider) Label() string {
	return "Wikimedia"
}

func (provider wikimediaSearchProvider) Readiness(cfg config.InternetConfig) SearchProviderReadiness {
	readiness := SearchProviderReadiness{
		Provider:   provider.Name(),
		Label:      provider.Label(),
		Configured: true,
		Ready:      true,
		Status:     "search provider is ready",
	}
	endpoint := strings.TrimSpace(cfg.Search.Endpoint)
	if endpoint == "" {
		return readiness
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		readiness.Configured = false
		readiness.Ready = false
		readiness.NeedsConfig = true
		readiness.Status = fmt.Sprintf("wikimedia endpoint is invalid: %v", err)
		return readiness
	}
	if endpointHasSecretQuery(parsed) {
		readiness.Configured = false
		readiness.Ready = false
		readiness.NeedsConfig = true
		readiness.Status = "wikimedia endpoint includes secret-like query parameters; use a secret-safe endpoint without API keys in the URL"
		return readiness
	}
	if blockMessage := blockedEndpointMessage("wikimedia", parsed.Hostname(), cfg); blockMessage != "" {
		readiness.Ready = false
		readiness.NeedsConfig = true
		readiness.Status = blockMessage
	}
	return readiness
}

func (wikimediaSearchProvider) FetchInput(cfg config.InternetConfig, input SearchInput, limit int) (FetchInput, error) {
	if strings.TrimSpace(cfg.Search.Endpoint) != "" {
		if err := rejectSecretQueryEndpoint("wikimedia", cfg.Search.Endpoint); err != nil {
			return FetchInput{}, err
		}
	}
	rawURL, err := wikimediaSearchURL(cfg.Search.Endpoint, input.Query, limit)
	if err != nil {
		return FetchInput{}, err
	}
	host, err := hostForURL(rawURL)
	if err != nil {
		return FetchInput{}, err
	}
	return FetchInput{
		URL:            rawURL,
		Method:         http.MethodGet,
		AllowedDomains: []string{host},
		TaskApproved:   input.TaskApproved,
		Caller:         input.Caller,
	}, nil
}

func (wikimediaSearchProvider) Parse(body string) ([]SearchItem, error) {
	items, err := parseWikimediaItems(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, bodySnippet(body, 180))
	}
	return items, nil
}

type tavilySearchProvider struct{}

func (tavilySearchProvider) Name() string {
	return "tavily"
}

func (tavilySearchProvider) Label() string {
	return "Tavily"
}

func (provider tavilySearchProvider) Readiness(cfg config.InternetConfig) SearchProviderReadiness {
	readiness := SearchProviderReadiness{
		Provider:   provider.Name(),
		Label:      provider.Label(),
		Configured: true,
	}
	if !applySearchAPIKeyReadiness(&readiness, provider.Name(), cfg.Search.APIKeyEnv) {
		return readiness
	}
	endpoint, err := tavilySearchEndpoint(cfg.Search.Endpoint)
	if err != nil {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = fmt.Sprintf("tavily endpoint is invalid: %v", err)
		return readiness
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = fmt.Sprintf("tavily endpoint is invalid: %v", err)
		return readiness
	}
	if endpointHasSecretQuery(parsed) {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = "tavily endpoint includes secret-like query parameters; use internet.search.api_key_env instead of API keys in the URL"
		return readiness
	}
	if blockMessage := blockedEndpointMessage("tavily", parsed.Hostname(), cfg); blockMessage != "" {
		readiness.NeedsConfig = true
		readiness.Status = blockMessage
		return readiness
	}
	readiness.Ready = true
	readiness.Status = "search provider is ready"
	return readiness
}

func (tavilySearchProvider) FetchInput(cfg config.InternetConfig, input SearchInput, limit int) (FetchInput, error) {
	endpoint, err := tavilySearchEndpoint(cfg.Search.Endpoint)
	if err != nil {
		return FetchInput{}, err
	}
	if err := rejectSecretQueryEndpoint("tavily", endpoint); err != nil {
		return FetchInput{}, err
	}
	apiKey, err := searchAPIKeyFromEnv("tavily", cfg.Search.APIKeyEnv)
	if err != nil {
		return FetchInput{}, err
	}
	body, err := tavilySearchBody(input.Query, limit)
	if err != nil {
		return FetchInput{}, err
	}
	host, err := hostForURL(endpoint)
	if err != nil {
		return FetchInput{}, err
	}
	return FetchInput{
		URL:            endpoint,
		Method:         http.MethodPost,
		AllowedDomains: []string{host},
		TaskApproved:   input.TaskApproved,
		Caller:         input.Caller,
		Headers: map[string]string{
			"Accept":        "application/json",
			"Authorization": "Bearer " + apiKey,
			"Content-Type":  "application/json",
		},
		Body:           body,
		AllowVendorAPI: true,
	}, nil
}

func (tavilySearchProvider) Parse(body string) ([]SearchItem, error) {
	items, err := parseTavilyItems(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, bodySnippet(body, 180))
	}
	return items, nil
}

type serperSearchProvider struct{}

func (serperSearchProvider) Name() string {
	return "serper"
}

func (serperSearchProvider) Label() string {
	return "Serper.dev"
}

func (provider serperSearchProvider) Readiness(cfg config.InternetConfig) SearchProviderReadiness {
	readiness := SearchProviderReadiness{
		Provider:   provider.Name(),
		Label:      provider.Label(),
		Configured: true,
	}
	if !applySearchAPIKeyReadiness(&readiness, provider.Name(), cfg.Search.APIKeyEnv) {
		return readiness
	}
	endpoint, err := serperSearchEndpoint(cfg.Search.Endpoint)
	if err != nil {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = fmt.Sprintf("serper endpoint is invalid: %v", err)
		return readiness
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = fmt.Sprintf("serper endpoint is invalid: %v", err)
		return readiness
	}
	if endpointHasSecretQuery(parsed) {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = "serper endpoint includes secret-like query parameters; use internet.search.api_key_env instead of API keys in the URL"
		return readiness
	}
	if blockMessage := blockedEndpointMessage("serper", parsed.Hostname(), cfg); blockMessage != "" {
		readiness.NeedsConfig = true
		readiness.Status = blockMessage
		return readiness
	}
	readiness.Ready = true
	readiness.Status = "search provider is ready"
	return readiness
}

func (serperSearchProvider) FetchInput(cfg config.InternetConfig, input SearchInput, limit int) (FetchInput, error) {
	endpoint, err := serperSearchEndpoint(cfg.Search.Endpoint)
	if err != nil {
		return FetchInput{}, err
	}
	if err := rejectSecretQueryEndpoint("serper", endpoint); err != nil {
		return FetchInput{}, err
	}
	apiKey, err := searchAPIKeyFromEnv("serper", cfg.Search.APIKeyEnv)
	if err != nil {
		return FetchInput{}, err
	}
	body, err := serperSearchBody(input.Query, limit)
	if err != nil {
		return FetchInput{}, err
	}
	host, err := hostForURL(endpoint)
	if err != nil {
		return FetchInput{}, err
	}
	return FetchInput{
		URL:            endpoint,
		Method:         http.MethodPost,
		AllowedDomains: []string{host},
		TaskApproved:   input.TaskApproved,
		Caller:         input.Caller,
		Headers: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
			"X-API-KEY":    apiKey,
		},
		Body:           body,
		AllowVendorAPI: true,
	}, nil
}

func (serperSearchProvider) Parse(body string) ([]SearchItem, error) {
	items, err := parseSerperItems(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, bodySnippet(body, 180))
	}
	return items, nil
}

type duckDuckGoSearchProvider struct{}

func (duckDuckGoSearchProvider) Name() string {
	return "duckduckgo"
}

func (duckDuckGoSearchProvider) Label() string {
	return "DuckDuckGo"
}

func (provider duckDuckGoSearchProvider) Readiness(cfg config.InternetConfig) SearchProviderReadiness {
	readiness := SearchProviderReadiness{
		Provider:   provider.Name(),
		Label:      provider.Label(),
		Configured: true,
		Ready:      true,
		Status:     "search provider is ready",
	}
	endpoint := strings.TrimSpace(cfg.Search.Endpoint)
	if endpoint == "" {
		return readiness
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		readiness.Configured = false
		readiness.Ready = false
		readiness.NeedsConfig = true
		readiness.Status = fmt.Sprintf("duckduckgo endpoint is invalid: %v", err)
		return readiness
	}
	if endpointHasSecretQuery(parsed) {
		readiness.Configured = false
		readiness.Ready = false
		readiness.NeedsConfig = true
		readiness.Status = "duckduckgo endpoint includes secret-like query parameters; use a secret-safe endpoint without API keys in the URL"
		return readiness
	}
	if blockMessage := blockedEndpointMessage("duckduckgo", parsed.Hostname(), cfg); blockMessage != "" {
		readiness.Ready = false
		readiness.NeedsConfig = true
		readiness.Status = blockMessage
	}
	return readiness
}

func (duckDuckGoSearchProvider) FetchInput(cfg config.InternetConfig, input SearchInput, limit int) (FetchInput, error) {
	if strings.TrimSpace(cfg.Search.Endpoint) != "" {
		if err := rejectSecretQueryEndpoint("duckduckgo", cfg.Search.Endpoint); err != nil {
			return FetchInput{}, err
		}
	}
	rawURL, err := duckDuckGoSearchURL(cfg.Search.Endpoint, input.Query, limit, cfg.Search.SafeSearch)
	if err != nil {
		return FetchInput{}, err
	}
	host, err := hostForURL(rawURL)
	if err != nil {
		return FetchInput{}, err
	}
	return FetchInput{
		URL:            rawURL,
		Method:         http.MethodGet,
		AllowedDomains: []string{host},
		TaskApproved:   input.TaskApproved,
		Caller:         input.Caller,
	}, nil
}

func (duckDuckGoSearchProvider) Parse(body string) ([]SearchItem, error) {
	items, err := parseDuckDuckGoItems(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, bodySnippet(body, 180))
	}
	return items, nil
}

type firecrawlSearchProvider struct{}

func (firecrawlSearchProvider) Name() string {
	return "firecrawl"
}

func (firecrawlSearchProvider) Label() string {
	return "Firecrawl"
}

func (provider firecrawlSearchProvider) Readiness(cfg config.InternetConfig) SearchProviderReadiness {
	readiness := SearchProviderReadiness{
		Provider:   provider.Name(),
		Label:      provider.Label(),
		Configured: true,
	}
	if !applySearchAPIKeyReadiness(&readiness, provider.Name(), cfg.Search.APIKeyEnv) {
		return readiness
	}
	endpoint, err := firecrawlSearchEndpoint(cfg.Search.Endpoint)
	if err != nil {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = fmt.Sprintf("firecrawl endpoint is invalid: %v", err)
		return readiness
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = fmt.Sprintf("firecrawl endpoint is invalid: %v", err)
		return readiness
	}
	if endpointHasSecretQuery(parsed) {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = "firecrawl endpoint includes secret-like query parameters; use internet.search.api_key_env instead of API keys in the URL"
		return readiness
	}
	if blockMessage := blockedEndpointMessage("firecrawl", parsed.Hostname(), cfg); blockMessage != "" {
		readiness.NeedsConfig = true
		readiness.Status = blockMessage
		return readiness
	}
	readiness.Ready = true
	readiness.Status = "search provider is ready"
	return readiness
}

func (firecrawlSearchProvider) FetchInput(cfg config.InternetConfig, input SearchInput, limit int) (FetchInput, error) {
	endpoint, err := firecrawlSearchEndpoint(cfg.Search.Endpoint)
	if err != nil {
		return FetchInput{}, err
	}
	if err := rejectSecretQueryEndpoint("firecrawl", endpoint); err != nil {
		return FetchInput{}, err
	}
	apiKey, err := searchAPIKeyFromEnv("firecrawl", cfg.Search.APIKeyEnv)
	if err != nil {
		return FetchInput{}, err
	}
	body, err := firecrawlSearchBody(input.Query, limit)
	if err != nil {
		return FetchInput{}, err
	}
	host, err := hostForURL(endpoint)
	if err != nil {
		return FetchInput{}, err
	}
	return FetchInput{
		URL:            endpoint,
		Method:         http.MethodPost,
		AllowedDomains: []string{host},
		TaskApproved:   input.TaskApproved,
		Caller:         input.Caller,
		Headers: map[string]string{
			"Accept":        "application/json",
			"Authorization": "Bearer " + apiKey,
			"Content-Type":  "application/json",
		},
		Body:           body,
		AllowVendorAPI: true,
	}, nil
}

func (firecrawlSearchProvider) Parse(body string) ([]SearchItem, error) {
	items, err := parseFirecrawlItems(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, bodySnippet(body, 180))
	}
	return items, nil
}

type mojeekSearchProvider struct{}

func (mojeekSearchProvider) Name() string {
	return "mojeek"
}

func (mojeekSearchProvider) Label() string {
	return "Mojeek"
}

func (provider mojeekSearchProvider) Readiness(cfg config.InternetConfig) SearchProviderReadiness {
	readiness := SearchProviderReadiness{
		Provider:   provider.Name(),
		Label:      provider.Label(),
		Configured: true,
	}
	if !applySearchAPIKeyReadiness(&readiness, provider.Name(), cfg.Search.APIKeyEnv) {
		return readiness
	}
	endpoint, err := mojeekSearchURL(cfg.Search.Endpoint, "readiness", 1, cfg.Search.SafeSearch)
	if err != nil {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = fmt.Sprintf("mojeek endpoint is invalid: %v", err)
		return readiness
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = fmt.Sprintf("mojeek endpoint is invalid: %v", err)
		return readiness
	}
	if endpointHasSecretQuery(parsed) {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = "mojeek endpoint includes secret-like query parameters; use internet.search.api_key_env instead of API keys in the URL"
		return readiness
	}
	if blockMessage := blockedEndpointMessage("mojeek", parsed.Hostname(), cfg); blockMessage != "" {
		readiness.NeedsConfig = true
		readiness.Status = blockMessage
		return readiness
	}
	readiness.Ready = true
	readiness.Status = "search provider is ready"
	return readiness
}

func (mojeekSearchProvider) FetchInput(cfg config.InternetConfig, input SearchInput, limit int) (FetchInput, error) {
	rawURL, err := mojeekSearchURL(cfg.Search.Endpoint, input.Query, limit, cfg.Search.SafeSearch)
	if err != nil {
		return FetchInput{}, err
	}
	if err := rejectSecretQueryEndpoint("mojeek", rawURL); err != nil {
		return FetchInput{}, err
	}
	apiKey, err := searchAPIKeyFromEnv("mojeek", cfg.Search.APIKeyEnv)
	if err != nil {
		return FetchInput{}, err
	}
	host, err := hostForURL(rawURL)
	if err != nil {
		return FetchInput{}, err
	}
	return FetchInput{
		URL:            rawURL,
		Method:         http.MethodGet,
		AllowedDomains: []string{host},
		TaskApproved:   input.TaskApproved,
		Caller:         input.Caller,
		SecretQuery: map[string]string{
			"api_key": apiKey,
		},
	}, nil
}

func (mojeekSearchProvider) Parse(body string) ([]SearchItem, error) {
	items, err := parseGenericSearchItems("mojeek", body)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, bodySnippet(body, 180))
	}
	return items, nil
}

type yemakaCustomSearchProvider struct{}

func (yemakaCustomSearchProvider) Name() string {
	return "yemaka_custom"
}

func (yemakaCustomSearchProvider) Label() string {
	return "Yemaka Custom Search"
}

func (provider yemakaCustomSearchProvider) Readiness(cfg config.InternetConfig) SearchProviderReadiness {
	return configuredGETEndpointReadiness(provider.Name(), provider.Label(), "yemaka custom search", cfg)
}

func (yemakaCustomSearchProvider) FetchInput(cfg config.InternetConfig, input SearchInput, limit int) (FetchInput, error) {
	return genericGETEndpointFetchInput("yemaka custom search", cfg, input, limit)
}

func (yemakaCustomSearchProvider) Parse(body string) ([]SearchItem, error) {
	items, err := parseGenericSearchItems("yemaka_custom", body)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, bodySnippet(body, 180))
	}
	return items, nil
}

func configuredGETEndpointReadiness(provider string, label string, messageName string, cfg config.InternetConfig) SearchProviderReadiness {
	readiness := SearchProviderReadiness{
		Provider:   provider,
		Label:      label,
		Configured: true,
	}
	endpoint := strings.TrimSpace(cfg.Search.Endpoint)
	if endpoint == "" {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = fmt.Sprintf("%s requires internet.search.endpoint; direct scraping is not supported", messageName)
		return readiness
	}
	parsed, err := parseURL(endpoint)
	if err != nil {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = fmt.Sprintf("%s endpoint is invalid: %v", messageName, err)
		return readiness
	}
	if endpointHasSecretQuery(parsed) {
		readiness.Configured = false
		readiness.NeedsConfig = true
		readiness.Status = fmt.Sprintf("%s endpoint includes secret-like query parameters; use a secret-safe endpoint without API keys in the URL", messageName)
		return readiness
	}
	if blockMessage := blockedEndpointMessage(messageName, parsed.Hostname(), cfg); blockMessage != "" {
		readiness.NeedsConfig = true
		readiness.Status = blockMessage
		return readiness
	}
	readiness.Ready = true
	readiness.Status = "search provider is ready"
	return readiness
}

func blockedEndpointMessage(providerName string, host string, cfg config.InternetConfig) string {
	if cfg.Policy.BlockLocalNetworkByDefault && isLocalHostname(host) {
		return fmt.Sprintf("%s endpoint is blocked because local network hosts are blocked by default", providerName)
	}
	if cfg.Policy.BlockPrivateIPRanges {
		if ip := net.ParseIP(host); ip != nil && isPrivateOrLocalIP(ip) {
			return fmt.Sprintf("%s endpoint is blocked because private/local IP ranges are blocked: %s", providerName, host)
		}
	}
	return ""
}

func genericGETEndpointFetchInput(providerName string, cfg config.InternetConfig, input SearchInput, limit int) (FetchInput, error) {
	if err := rejectSecretQueryEndpoint(providerName, cfg.Search.Endpoint); err != nil {
		return FetchInput{}, err
	}
	rawURL, err := genericSearchURL(cfg.Search.Endpoint, input.Query, limit, cfg.Search.SafeSearch)
	if err != nil {
		return FetchInput{}, fmt.Errorf("%s requires internet.search.endpoint: %w", providerName, err)
	}
	host, err := hostForURL(rawURL)
	if err != nil {
		return FetchInput{}, err
	}
	return FetchInput{
		URL:            rawURL,
		Method:         http.MethodGet,
		AllowedDomains: []string{host},
		TaskApproved:   input.TaskApproved,
		Caller:         input.Caller,
	}, nil
}

func rejectSecretQueryEndpoint(providerName string, endpoint string) error {
	parsed, err := parseURL(endpoint)
	if err != nil {
		return nil
	}
	if endpointHasSecretQuery(parsed) {
		return fmt.Errorf("%s endpoint includes secret-like query parameters; use a secret-safe endpoint without API keys in the URL", providerName)
	}
	return nil
}

func endpointHasSecretQuery(parsed *url.URL) bool {
	for key := range parsed.Query() {
		if isSecretQueryKey(key) {
			return true
		}
	}
	return false
}

func isSecretQueryKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
	switch normalized {
	case "api_key", "apikey", "access_key", "access_token", "auth_token", "bearer_token", "client_secret", "key", "password", "secret", "token":
		return true
	default:
		return strings.HasSuffix(normalized, "_api_key") ||
			strings.HasSuffix(normalized, "_token") ||
			strings.HasSuffix(normalized, "_secret")
	}
}
