package internet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxCrawlRunHistoryPages = 12
	maxCrawlRunHistorySkips = 24
)

type CrawlRunRecord struct {
	RunID              string                `json:"runId"`
	SeedURL            string                `json:"seedUrl"`
	Method             string                `json:"method"`
	Status             string                `json:"status"`
	FailureReason      string                `json:"failureReason,omitempty"`
	StartedAt          string                `json:"startedAt"`
	FinishedAt         string                `json:"finishedAt"`
	AllowedDomains     []string              `json:"allowedDomains"`
	MaxPages           int                   `json:"maxPages"`
	MaxDepth           int                   `json:"maxDepth"`
	MaxDurationSeconds int                   `json:"maxDurationSeconds"`
	MaxLinksPerPage    int                   `json:"maxLinksPerPage"`
	MaxTextChars       int                   `json:"maxTextChars"`
	Visited            int                   `json:"visited"`
	Fetched            int                   `json:"fetched"`
	Skipped            int                   `json:"skipped"`
	SkipOverflow       int                   `json:"skipOverflow,omitempty"`
	Pages              []CrawlRunPageSummary `json:"pages,omitempty"`
	Skips              []CrawlSkip           `json:"skips,omitempty"`
}

type CrawlRunPageSummary struct {
	URL           string `json:"url"`
	FinalURL      string `json:"finalUrl"`
	Depth         int    `json:"depth"`
	StatusCode    int    `json:"statusCode"`
	ContentType   string `json:"contentType,omitempty"`
	BodyBytes     int    `json:"bodyBytes"`
	FromCache     bool   `json:"fromCache"`
	LinkCount     int    `json:"linkCount"`
	TextBytes     int    `json:"textBytes"`
	RobotsAllowed bool   `json:"robotsAllowed"`
	FetchedAt     string `json:"fetchedAt"`
	Error         string `json:"error,omitempty"`
}

func (s *Service) CrawlRuns(limit int) ([]CrawlRunRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	data, err := os.ReadFile(s.CrawlLogPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	result := []CrawlRunRecord{}
	for i := len(lines) - 1; i >= 0 && len(result) < limit; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var record CrawlRunRecord
		if err := json.Unmarshal([]byte(line), &record); err == nil {
			result = append(result, record)
		}
	}
	return result, nil
}

func (s *Service) CrawlRun(id string) (CrawlRunRecord, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return CrawlRunRecord{}, os.ErrNotExist
	}
	data, err := os.ReadFile(s.CrawlLogPath)
	if os.IsNotExist(err) {
		return CrawlRunRecord{}, os.ErrNotExist
	}
	if err != nil {
		return CrawlRunRecord{}, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var run CrawlRunRecord
		if err := json.Unmarshal([]byte(line), &run); err != nil {
			continue
		}
		if run.RunID == id {
			return run, nil
		}
	}
	return CrawlRunRecord{}, os.ErrNotExist
}

func (s *Service) logCrawlRun(record CrawlRunRecord) error {
	if !s.Config.Policy.LogRequests || strings.TrimSpace(s.CrawlLogPath) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.CrawlLogPath), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(s.CrawlLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}

func crawlRunRecordFromResult(result CrawlResult) CrawlRunRecord {
	record := CrawlRunRecord{
		RunID:              result.RunID,
		SeedURL:            result.SeedURL,
		Method:             result.Method,
		Status:             result.Status,
		FailureReason:      result.FailureReason,
		StartedAt:          result.StartedAt,
		FinishedAt:         result.FinishedAt,
		AllowedDomains:     append([]string{}, result.AllowedDomains...),
		MaxPages:           result.MaxPages,
		MaxDepth:           result.MaxDepth,
		MaxDurationSeconds: result.MaxDurationSeconds,
		MaxLinksPerPage:    result.MaxLinksPerPage,
		MaxTextChars:       result.MaxTextChars,
		Visited:            result.Visited,
		Fetched:            result.Fetched,
		Skipped:            result.Skipped,
		SkipOverflow:       result.SkipOverflow,
	}
	for i, page := range result.Pages {
		if i >= maxCrawlRunHistoryPages {
			break
		}
		robotsAllowed := false
		if page.Robots != nil {
			robotsAllowed = page.Robots.Allowed
		}
		record.Pages = append(record.Pages, CrawlRunPageSummary{
			URL:           page.URL,
			FinalURL:      page.FinalURL,
			Depth:         page.Depth,
			StatusCode:    page.StatusCode,
			ContentType:   page.ContentType,
			BodyBytes:     page.BodyBytes,
			FromCache:     page.FromCache,
			LinkCount:     len(page.Links),
			TextBytes:     len(page.ExtractedText),
			RobotsAllowed: robotsAllowed,
			FetchedAt:     page.FetchedAt,
			Error:         page.Error,
		})
	}
	for i, skip := range result.Skips {
		if i >= maxCrawlRunHistorySkips {
			break
		}
		record.Skips = append(record.Skips, skip)
	}
	return record
}
