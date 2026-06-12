package internet

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type RobotsResult struct {
	URL        string `json:"url"`
	RobotsURL  string `json:"robotsUrl"`
	UserAgent  string `json:"userAgent"`
	Allowed    bool   `json:"allowed"`
	StatusCode int    `json:"statusCode"`
	Reason     string `json:"reason"`
	CheckedAt  string `json:"checkedAt"`
}

func (s *Service) CheckRobots(ctx context.Context, rawURL string, userAgent string, taskApproved bool) (RobotsResult, error) {
	target, err := parseURL(rawURL)
	if err != nil {
		return RobotsResult{}, err
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "Yemaka"
	}
	robotsURL := (&url.URL{Scheme: target.Scheme, Host: target.Host, Path: "/robots.txt"}).String()
	result := RobotsResult{
		URL:       target.String(),
		RobotsURL: robotsURL,
		UserAgent: userAgent,
		Allowed:   true,
		Reason:    "robots.txt not checked",
		CheckedAt: s.timestamp(),
	}
	if !s.Config.Policy.RespectRobotsTxt {
		result.Reason = "robots awareness disabled by policy"
		return result, nil
	}
	input := FetchInput{
		URL:          robotsURL,
		Method:       http.MethodGet,
		TaskApproved: taskApproved,
		Caller:       "robots",
	}
	fetch, err := s.Fetch(ctx, input)
	if err != nil {
		result.Allowed = false
		result.Reason = err.Error()
		return result, err
	}
	result.StatusCode = fetch.StatusCode
	if fetch.StatusCode == http.StatusNotFound {
		result.Allowed = true
		result.Reason = "robots.txt not found"
		return result, nil
	}
	if fetch.StatusCode < 200 || fetch.StatusCode >= 300 {
		result.Allowed = false
		result.Reason = fmt.Sprintf("robots.txt returned status %d", fetch.StatusCode)
		return result, nil
	}
	allowed, reason := robotsAllows(fetch.Body, target.EscapedPath(), userAgent)
	result.Allowed = allowed
	result.Reason = reason
	return result, nil
}

func robotsAllows(body string, path string, userAgent string) (bool, string) {
	if path == "" {
		path = "/"
	}
	userAgent = strings.ToLower(strings.TrimSpace(userAgent))
	active := false
	matchedAny := false
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch key {
		case "user-agent":
			agent := strings.ToLower(value)
			active = agent == "*" || strings.Contains(userAgent, agent)
			if active {
				matchedAny = true
			}
		case "disallow":
			if active && value != "" && strings.HasPrefix(path, value) {
				return false, "blocked by robots.txt disallow rule"
			}
		}
	}
	if matchedAny {
		return true, "allowed by robots.txt"
	}
	return true, "no matching robots.txt user-agent rule"
}
