package modelruntime

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"yemaka/internal/config"
	"yemaka/internal/models"
	"yemaka/internal/models/ollama"
	"yemaka/internal/models/openaiapi"
)

const (
	DefaultOllamaBaseURL           = "http://localhost:11434/api"
	DefaultLlamaCppBaseURL         = "http://127.0.0.1:8080/v1"
	DefaultOpenAICompatibleBaseURL = "http://127.0.0.1:4000/v1"
)

func New(cfg config.ModelConfig) (models.Runtime, error) {
	provider := models.NormalizeProvider(cfg.Provider)
	baseURL := cfg.BaseURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL(provider)
	}
	if !IsLocalBaseURL(baseURL) {
		return nil, fmt.Errorf("%s runtime base_url must be local: %s", providerName(provider), baseURL)
	}

	switch provider {
	case models.ProviderOllama:
		return ollama.New(baseURL), nil
	case models.ProviderLlamaCpp, models.ProviderOpenAICompatible:
		return openaiapi.New(provider, baseURL), nil
	default:
		return nil, fmt.Errorf("unsupported model provider %q", cfg.Provider)
	}
}

func DefaultBaseURL(provider string) string {
	switch models.NormalizeProvider(provider) {
	case models.ProviderLlamaCpp:
		return DefaultLlamaCppBaseURL
	case models.ProviderOpenAICompatible:
		return DefaultOpenAICompatibleBaseURL
	default:
		return DefaultOllamaBaseURL
	}
}

func NormalizeConfig(cfg config.ModelConfig) config.ModelConfig {
	cfg.Provider = models.NormalizeProvider(cfg.Provider)
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = DefaultBaseURL(cfg.Provider)
	}
	return cfg
}

func IsLocalBaseURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return false
	}
	host := parsed.Hostname()
	if host == "" {
		return false
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

func providerName(provider string) string {
	if models.NormalizeProvider(provider) == models.ProviderLlamaCpp {
		return "llama.cpp"
	}
	return models.NormalizeProvider(provider)
}
