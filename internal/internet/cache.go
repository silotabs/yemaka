package internet

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type CacheEntry struct {
	Key         string              `json:"key"`
	URL         string              `json:"url"`
	FinalURL    string              `json:"finalUrl"`
	Method      string              `json:"method"`
	StatusCode  int                 `json:"statusCode"`
	Header      map[string][]string `json:"header,omitempty"`
	ContentType string              `json:"contentType,omitempty"`
	Body        string              `json:"body,omitempty"`
	BodyBytes   int                 `json:"bodyBytes"`
	CachedAt    string              `json:"cachedAt"`
	ExpiresAt   string              `json:"expiresAt"`
}

type CacheSummary struct {
	Key              string `json:"key"`
	URL              string `json:"url"`
	Method           string `json:"method"`
	BodyBytes        int    `json:"bodyBytes"`
	CachedAt         string `json:"cachedAt"`
	ExpiresAt        string `json:"expiresAt"`
	Expired          bool   `json:"expired"`
	State            string `json:"state"`
	AgeSeconds       int64  `json:"ageSeconds"`
	ExpiresInSeconds int64  `json:"expiresInSeconds"`
}

func (s *Service) CacheList() ([]CacheSummary, error) {
	entries, err := os.ReadDir(s.CacheDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read internet cache: %w", err)
	}
	now := s.now()
	result := []CacheSummary{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.CacheDir, entry.Name()))
		if err != nil {
			continue
		}
		var cache CacheEntry
		if err := json.Unmarshal(data, &cache); err != nil {
			continue
		}
		expiresAt, _ := time.Parse(time.RFC3339, cache.ExpiresAt)
		cachedAt, _ := time.Parse(time.RFC3339, cache.CachedAt)
		expired := !expiresAt.IsZero() && now.After(expiresAt)
		state := "fresh"
		if expired {
			state = "stale"
		}
		var ageSeconds int64
		if !cachedAt.IsZero() {
			ageSeconds = int64(now.Sub(cachedAt).Seconds())
			if ageSeconds < 0 {
				ageSeconds = 0
			}
		}
		var expiresInSeconds int64
		if !expiresAt.IsZero() {
			expiresInSeconds = int64(expiresAt.Sub(now).Seconds())
		}
		result = append(result, CacheSummary{
			Key:              cache.Key,
			URL:              cache.URL,
			Method:           cache.Method,
			BodyBytes:        cache.BodyBytes,
			CachedAt:         cache.CachedAt,
			ExpiresAt:        cache.ExpiresAt,
			Expired:          expired,
			State:            state,
			AgeSeconds:       ageSeconds,
			ExpiresInSeconds: expiresInSeconds,
		})
	}
	sort.Slice(result, func(i int, j int) bool {
		return result[i].CachedAt > result[j].CachedAt
	})
	return result, nil
}

func (s *Service) cacheGet(method string, rawURL string) (CacheEntry, bool) {
	path := s.cachePath(method, rawURL)
	data, err := os.ReadFile(path)
	if err != nil {
		return CacheEntry{}, false
	}
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return CacheEntry{}, false
	}
	expiresAt, err := time.Parse(time.RFC3339, entry.ExpiresAt)
	if err != nil || s.now().After(expiresAt) {
		return CacheEntry{}, false
	}
	return entry, true
}

func (s *Service) cacheSet(method string, rawURL string, result FetchResult) error {
	if err := os.MkdirAll(s.CacheDir, 0o755); err != nil {
		return err
	}
	cachedAt := s.timestamp()
	expiresAt := s.now().Add(time.Duration(s.cacheTTLSeconds()) * time.Second).UTC().Format(time.RFC3339)
	entry := CacheEntry{
		Key:         cacheKey(method, rawURL),
		URL:         rawURL,
		FinalURL:    result.FinalURL,
		Method:      method,
		StatusCode:  result.StatusCode,
		Header:      result.Header,
		ContentType: result.ContentType,
		Body:        result.Body,
		BodyBytes:   result.BodyBytes,
		CachedAt:    cachedAt,
		ExpiresAt:   expiresAt,
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.cachePath(method, rawURL), append(data, '\n'), 0o644)
}

func cacheEntryResult(entry CacheEntry, extractText bool, fetchedAt string) FetchResult {
	result := FetchResult{
		URL:         entry.URL,
		FinalURL:    entry.FinalURL,
		Method:      entry.Method,
		StatusCode:  entry.StatusCode,
		Header:      entry.Header,
		ContentType: entry.ContentType,
		Body:        entry.Body,
		BodyBytes:   entry.BodyBytes,
		FromCache:   true,
		CachedAt:    entry.CachedAt,
		FetchedAt:   fetchedAt,
	}
	if extractText {
		result.ExtractedText = ExtractText(result.Body, result.ContentType)
	}
	return result
}

func (s *Service) cacheTTLSeconds() int {
	if s.Config.Cache.TTLSeconds > 0 {
		return s.Config.Cache.TTLSeconds
	}
	return 3600
}

func (s *Service) cachePath(method string, rawURL string) string {
	return filepath.Join(s.CacheDir, cacheKey(method, rawURL)+".json")
}

func cacheKey(method string, rawURL string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(method) + "\n" + rawURL))
	return hex.EncodeToString(sum[:])
}
