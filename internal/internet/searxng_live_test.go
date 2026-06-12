package internet

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"yemaka/internal/config"
)

func TestLiveSearXNGReadiness(t *testing.T) {
	endpoint := strings.TrimSpace(os.Getenv("YEMAKA_LIVE_SEARXNG_ENDPOINT"))
	if endpoint == "" {
		t.Skip("set YEMAKA_LIVE_SEARXNG_ENDPOINT to run live SearXNG readiness smoke")
	}
	query := strings.TrimSpace(os.Getenv("YEMAKA_LIVE_SEARXNG_QUERY"))
	if query == "" {
		query = "Yemaka local agent"
	}

	cfg := config.Default().Internet
	cfg.Enabled = true
	cfg.TimeoutSeconds = 10
	cfg.Search.Enabled = true
	cfg.Search.Provider = "searxng"
	cfg.Search.Endpoint = endpoint
	cfg.Search.MaxResults = 1

	service := testService(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := service.Search(ctx, SearchInput{Query: query, TaskApproved: true, MaxResults: 1})
	if err != nil {
		t.Fatalf("live SearXNG readiness failed: %v", err)
	}
	if result.Provider != "searxng" {
		t.Fatalf("provider = %q, want searxng", result.Provider)
	}
}
