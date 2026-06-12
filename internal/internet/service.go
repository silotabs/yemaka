package internet

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yemaka/internal/config"
)

const (
	requestsFile  = "internet_requests.jsonl"
	crawlRunsFile = "internet_crawls.jsonl"
	cacheDirName  = "internet_cache"
)

type Service struct {
	Config       config.InternetConfig
	PolicyMode   string
	CacheDir     string
	LogPath      string
	CrawlLogPath string
	Client       *http.Client
	Now          func() time.Time
}

type Status struct {
	Enabled                   bool                      `json:"enabled"`
	DefaultMode               string                    `json:"defaultMode"`
	AllowMethods              []string                  `json:"allowMethods"`
	MaxResponseBytes          int64                     `json:"maxResponseBytes"`
	TimeoutSeconds            int                       `json:"timeoutSeconds"`
	RedirectLimit             int                       `json:"redirectLimit"`
	CacheEnabled              bool                      `json:"cacheEnabled"`
	CacheTTLSeconds           int                       `json:"cacheTtlSeconds"`
	SearchEnabled             bool                      `json:"searchEnabled"`
	SearchProvider            string                    `json:"searchProvider"`
	SearchProviderLabel       string                    `json:"searchProviderLabel,omitempty"`
	SearchEndpoint            string                    `json:"searchEndpoint,omitempty"`
	SearchAPIKeyEnv           string                    `json:"searchApiKeyEnv,omitempty"`
	SearchMaxResults          int                       `json:"searchMaxResults"`
	SearchSafeSearch          bool                      `json:"searchSafeSearch"`
	SearchProviderReady       bool                      `json:"searchProviderReady"`
	SearchProviderConfigured  bool                      `json:"searchProviderConfigured"`
	SearchProviderNeedsConfig bool                      `json:"searchProviderNeedsConfig"`
	SearchProviderNeedsAuth   bool                      `json:"searchProviderNeedsAuth"`
	SearchNetworkChecked      bool                      `json:"searchNetworkChecked"`
	SearchStatus              string                    `json:"searchStatus"`
	SearchFallbackProviders   []string                  `json:"searchFallbackProviders,omitempty"`
	SearchFallbackReadiness   []SearchProviderReadiness `json:"searchFallbackReadiness,omitempty"`
	SearchState               string                    `json:"searchState"`
	SearchHealthScore         int                       `json:"searchHealthScore"`
	SearchNextAction          string                    `json:"searchNextAction,omitempty"`
	CacheFreshEntries         int                       `json:"cacheFreshEntries"`
	CacheStaleEntries         int                       `json:"cacheStaleEntries"`
	CacheLatestCachedAt       string                    `json:"cacheLatestCachedAt,omitempty"`
	LogPath                   string                    `json:"logPath"`
	CacheDir                  string                    `json:"cacheDir"`
	RobotsAware               bool                      `json:"robotsAware"`
	CrawlerAvailable          bool                      `json:"crawlerAvailable"`
	CrawlerManualOnly         bool                      `json:"crawlerManualOnly"`
	CrawlerRequiresApproval   bool                      `json:"crawlerRequiresApproval"`
	CrawlerMaxPages           int                       `json:"crawlerMaxPages"`
	CrawlerMaxDepth           int                       `json:"crawlerMaxDepth"`
	CrawlerMaxDurationSeconds int                       `json:"crawlerMaxDurationSeconds"`
	CrawlerMaxLinksPerPage    int                       `json:"crawlerMaxLinksPerPage"`
	CrawlerStatus             string                    `json:"crawlerStatus"`
}

type RequestRecord struct {
	Timestamp  string `json:"timestamp"`
	Caller     string `json:"caller"`
	Method     string `json:"method"`
	URL        string `json:"url"`
	FinalURL   string `json:"finalUrl,omitempty"`
	Host       string `json:"host"`
	Provider   string `json:"provider,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
	Bytes      int    `json:"bytes,omitempty"`
	FromCache  bool   `json:"fromCache"`
	Allowed    bool   `json:"allowed"`
	Error      string `json:"error,omitempty"`
}

type searchHealthStatus struct {
	State      string
	Score      int
	NextAction string
}

type cacheStatusStats struct {
	Fresh          int
	Stale          int
	LatestCachedAt string
}

func New(cfg config.InternetConfig, profileRoot string, logsDir string) *Service {
	if strings.TrimSpace(profileRoot) == "" {
		profileRoot = "."
	}
	if strings.TrimSpace(logsDir) == "" {
		logsDir = filepath.Join(profileRoot, "logs")
	}
	return &Service{
		Config:       cfg,
		CacheDir:     filepath.Join(profileRoot, cacheDirName),
		LogPath:      filepath.Join(logsDir, requestsFile),
		CrawlLogPath: filepath.Join(logsDir, crawlRunsFile),
		Now:          time.Now,
	}
}

func (s *Service) Status() Status {
	cfg := s.Config
	searchReadiness := searchReadiness(cfg)
	searchAPIKeyEnv := NormalizeSearchAPIKeyEnvForProvider(cfg.Search.Provider, cfg.Search.APIKeyEnv)
	searchHealth := s.searchHealth(cfg, searchReadiness)
	cacheStats := s.cacheStatusStats()
	return Status{
		Enabled:                   cfg.Enabled,
		DefaultMode:               cfg.DefaultMode,
		AllowMethods:              append([]string{}, cfg.AllowMethods...),
		MaxResponseBytes:          cfg.MaxResponseBytes,
		TimeoutSeconds:            cfg.TimeoutSeconds,
		RedirectLimit:             cfg.RedirectLimit,
		CacheEnabled:              cfg.Cache.Enabled,
		CacheTTLSeconds:           cfg.Cache.TTLSeconds,
		SearchEnabled:             cfg.Search.Enabled,
		SearchProvider:            cfg.Search.Provider,
		SearchProviderLabel:       searchReadiness.Label,
		SearchEndpoint:            searchEndpointForStatus(cfg.Search.Endpoint),
		SearchAPIKeyEnv:           searchAPIKeyEnv,
		SearchMaxResults:          cfg.Search.MaxResults,
		SearchSafeSearch:          cfg.Search.SafeSearch,
		SearchProviderReady:       searchReadiness.Ready,
		SearchProviderConfigured:  searchReadiness.Configured,
		SearchProviderNeedsConfig: searchReadiness.NeedsConfig,
		SearchProviderNeedsAuth:   searchReadiness.NeedsAuth,
		SearchNetworkChecked:      searchReadiness.NetworkChecked,
		SearchStatus:              searchReadiness.Status,
		SearchFallbackProviders:   searchProviderPriority(cfg),
		SearchFallbackReadiness:   searchFallbackReadiness(cfg),
		SearchState:               searchHealth.State,
		SearchHealthScore:         searchHealth.Score,
		SearchNextAction:          searchHealth.NextAction,
		CacheFreshEntries:         cacheStats.Fresh,
		CacheStaleEntries:         cacheStats.Stale,
		CacheLatestCachedAt:       cacheStats.LatestCachedAt,
		LogPath:                   s.LogPath,
		CacheDir:                  s.CacheDir,
		RobotsAware:               cfg.Policy.RespectRobotsTxt,
		CrawlerAvailable:          cfg.Enabled,
		CrawlerManualOnly:         true,
		CrawlerRequiresApproval:   true,
		CrawlerMaxPages:           absoluteCrawlMaxPages,
		CrawlerMaxDepth:           absoluteCrawlMaxDepth,
		CrawlerMaxDurationSeconds: absoluteCrawlDurationSecs,
		CrawlerMaxLinksPerPage:    absoluteCrawlLinksPerPage,
		CrawlerStatus:             crawlerStatusMessage(cfg),
	}
}

func crawlerStatusMessage(cfg config.InternetConfig) string {
	if !cfg.Enabled {
		return "crawler disabled because controlled internet access is disabled"
	}
	if !cfg.Policy.RespectRobotsTxt {
		return "manual bounded crawler is available; robots awareness is disabled by policy"
	}
	return "manual bounded crawler is available; explicit approval required"
}

func (s *Service) searchStatus(cfg config.InternetConfig) (bool, string) {
	readiness := searchReadiness(cfg)
	return readiness.Ready, readiness.Status
}

func (s *Service) searchHealth(cfg config.InternetConfig, readiness SearchProviderReadiness) searchHealthStatus {
	state := searchStateForReadiness(cfg, readiness)
	health := searchHealthStatus{
		State:      state,
		Score:      searchHealthScore(state),
		NextAction: searchHealthNextAction(state, readiness),
	}
	if state != "configured" && state != "healthy" {
		return health
	}
	record, ok := s.latestSearchRequest(readiness.Provider)
	if !ok {
		return health
	}
	if searchRequestLooksFailed(record) {
		health.State = "degraded"
		health.Score = searchHealthScore(health.State)
		health.NextAction = "Inspect recent internet request logs and provider settings before relying on current search."
		return health
	}
	health.State = "healthy"
	health.Score = searchHealthScore(health.State)
	health.NextAction = "Search provider has recent successful evidence."
	return health
}

func searchStateForReadiness(cfg config.InternetConfig, readiness SearchProviderReadiness) string {
	if !cfg.Enabled || !cfg.Search.Enabled || normalizeSearchProvider(cfg.Search.Provider) == "none" {
		return "disabled"
	}
	if readiness.NeedsAuth {
		return "needs_auth"
	}
	if readiness.NeedsConfig || !readiness.Configured {
		return "unconfigured"
	}
	if readiness.Ready {
		return "configured"
	}
	return "broken"
}

func searchHealthScore(state string) int {
	switch strings.TrimSpace(state) {
	case "healthy":
		return 100
	case "configured":
		return 70
	case "degraded":
		return 45
	case "needs_auth":
		return 25
	case "broken":
		return 20
	case "unconfigured":
		return 15
	case "disabled":
		return 0
	default:
		return 0
	}
}

func searchHealthNextAction(state string, readiness SearchProviderReadiness) string {
	switch strings.TrimSpace(state) {
	case "disabled":
		return "Enable internet search and configure a provider before asking for current public facts."
	case "unconfigured":
		return firstNonEmpty(readiness.Status, "Configure an explicit search provider before using web search.")
	case "needs_auth":
		return firstNonEmpty(readiness.Status, "Set the provider API key environment variable, then restart Yemaka if needed.")
	case "configured":
		return "Provider is configured. Run an approved search to confirm live runtime health."
	case "healthy":
		return "Search provider is ready."
	case "degraded":
		return "Inspect recent failed search requests and provider settings."
	case "broken":
		return firstNonEmpty(readiness.Status, "Search provider is not ready.")
	default:
		return ""
	}
}

func (s *Service) latestSearchRequest(provider string) (RequestRecord, bool) {
	records, err := s.Requests(50)
	if err != nil {
		return RequestRecord{}, false
	}
	provider = normalizeSearchProvider(provider)
	for _, record := range records {
		recordProvider := normalizeSearchProvider(record.Provider)
		if recordProvider != "" && recordProvider != "none" {
			if provider == "auto" || provider == "" || recordProvider == provider {
				return record, true
			}
			continue
		}
		if strings.TrimSpace(record.Caller) == "internet_search" {
			return record, true
		}
	}
	return RequestRecord{}, false
}

func searchRequestLooksFailed(record RequestRecord) bool {
	if !record.Allowed || strings.TrimSpace(record.Error) != "" {
		return true
	}
	return record.StatusCode >= 400
}

func searchFallbackReadiness(cfg config.InternetConfig) []SearchProviderReadiness {
	providers := searchProviderPriority(cfg)
	out := make([]SearchProviderReadiness, 0, len(providers))
	for _, providerName := range providers {
		provider, ok := searchProviderForName(providerName)
		if !ok {
			continue
		}
		readiness := provider.Readiness(cfg)
		if !cfg.Enabled {
			readiness.Ready = false
			readiness.Status = "internet access is disabled"
		}
		out = append(out, readiness)
	}
	return out
}

func (s *Service) cacheStatusStats() cacheStatusStats {
	items, err := s.CacheList()
	if err != nil {
		return cacheStatusStats{}
	}
	var stats cacheStatusStats
	for _, item := range items {
		if item.Expired {
			stats.Stale++
		} else {
			stats.Fresh++
		}
		if item.CachedAt > stats.LatestCachedAt {
			stats.LatestCachedAt = item.CachedAt
		}
	}
	return stats
}

func (s *Service) Requests(limit int) ([]RequestRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	data, err := os.ReadFile(s.LogPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	result := []RequestRecord{}
	for i := len(lines) - 1; i >= 0 && len(result) < limit; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var record RequestRecord
		if err := json.Unmarshal([]byte(line), &record); err == nil {
			result = append(result, record)
		}
	}
	return result, nil
}

func (s *Service) logRequest(record RequestRecord) error {
	if !s.Config.Policy.LogRequests || strings.TrimSpace(s.LogPath) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.LogPath), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(s.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}

func (s *Service) timestamp() string {
	return s.now().UTC().Format(time.RFC3339)
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
