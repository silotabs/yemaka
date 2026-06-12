package internet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type FetchInput struct {
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	ExtractText    bool              `json:"extractText"`
	AllowedDomains []string          `json:"allowedDomains"`
	TaskApproved   bool              `json:"taskApproved"`
	Caller         string            `json:"caller"`
	Provider       string            `json:"-"`
	Headers        map[string]string `json:"-"`
	Body           string            `json:"-"`
	AllowVendorAPI bool              `json:"-"`
	SecretQuery    map[string]string `json:"-"`
}

type FetchResult struct {
	URL           string              `json:"url"`
	FinalURL      string              `json:"finalUrl"`
	Method        string              `json:"method"`
	StatusCode    int                 `json:"statusCode"`
	Header        map[string][]string `json:"header,omitempty"`
	ContentType   string              `json:"contentType,omitempty"`
	Body          string              `json:"body,omitempty"`
	ExtractedText string              `json:"extractedText,omitempty"`
	BodyBytes     int                 `json:"bodyBytes"`
	FromCache     bool                `json:"fromCache"`
	CachedAt      string              `json:"cachedAt,omitempty"`
	FetchedAt     string              `json:"fetchedAt"`
}

func (s *Service) Fetch(ctx context.Context, input FetchInput) (FetchResult, error) {
	input.Method = normalizeMethod(input.Method)
	if input.Caller == "" {
		input.Caller = "internet_service"
	}
	started := s.timestamp()
	target, err := parseURL(input.URL)
	if err != nil {
		_ = s.logRequest(RequestRecord{Timestamp: started, Caller: input.Caller, Method: input.Method, URL: input.URL, Allowed: false, Error: err.Error()})
		return FetchResult{}, err
	}
	record := RequestRecord{
		Timestamp: started,
		Caller:    input.Caller,
		Method:    input.Method,
		URL:       target.String(),
		Host:      target.Hostname(),
		Provider:  strings.TrimSpace(input.Provider),
	}
	if err := s.validatePolicy(input, target); err != nil {
		record.Allowed = false
		record.Error = err.Error()
		_ = s.logRequest(record)
		return FetchResult{}, err
	}
	record.Allowed = true
	requestURL := urlWithSecretQuery(target, input.SecretQuery)

	if input.Method == http.MethodGet && s.Config.Cache.Enabled {
		if entry, ok := s.cacheGet(input.Method, target.String()); ok {
			result := cacheEntryResult(entry, input.ExtractText, s.timestamp())
			record.FinalURL = result.FinalURL
			record.StatusCode = result.StatusCode
			record.Bytes = result.BodyBytes
			record.FromCache = true
			_ = s.logRequest(record)
			return result, nil
		}
	}

	client := s.httpClient(input)
	var requestBody io.Reader
	if input.Method != http.MethodGet && input.Method != http.MethodHead && input.Body != "" {
		requestBody = bytes.NewBufferString(input.Body)
	}
	req, err := http.NewRequestWithContext(ctx, input.Method, requestURL.String(), requestBody)
	if err != nil {
		err = sanitizedSecretQueryError(err, input.SecretQuery)
		record.Error = err.Error()
		_ = s.logRequest(record)
		return FetchResult{}, err
	}
	req.Header.Set("User-Agent", "Yemaka/0.1 local-first internet service")
	for key, value := range input.Headers {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		err = sanitizedSecretQueryError(err, input.SecretQuery)
		record.Error = err.Error()
		_ = s.logRequest(record)
		return FetchResult{}, err
	}
	defer resp.Body.Close()

	maxBytes := s.maxResponseBytes()
	var body []byte
	if input.Method != http.MethodHead {
		body, err = readBounded(resp.Body, maxBytes)
		if err != nil {
			record.StatusCode = resp.StatusCode
			record.Error = err.Error()
			_ = s.logRequest(record)
			return FetchResult{}, err
		}
	}
	finalURL := sanitizedSecretQueryURL(resp.Request.URL, input.SecretQuery)
	result := FetchResult{
		URL:         target.String(),
		FinalURL:    finalURL,
		Method:      input.Method,
		StatusCode:  resp.StatusCode,
		Header:      resp.Header,
		ContentType: resp.Header.Get("Content-Type"),
		Body:        string(body),
		BodyBytes:   len(body),
		FetchedAt:   s.timestamp(),
	}
	if input.ExtractText {
		result.ExtractedText = ExtractText(result.Body, result.ContentType)
	}
	record.FinalURL = finalURL
	record.StatusCode = resp.StatusCode
	record.Bytes = len(body)
	_ = s.logRequest(record)
	if input.Method == http.MethodGet && s.Config.Cache.Enabled && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_ = s.cacheSet(input.Method, target.String(), result)
	}
	return result, nil
}

func urlWithSecretQuery(target *url.URL, secretQuery map[string]string) *url.URL {
	requestURL := *target
	if len(secretQuery) == 0 {
		return &requestURL
	}
	values := requestURL.Query()
	for key, value := range secretQuery {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		values.Set(key, value)
	}
	requestURL.RawQuery = values.Encode()
	return &requestURL
}

func sanitizedSecretQueryURL(target *url.URL, secretQuery map[string]string) string {
	if target == nil {
		return ""
	}
	sanitized := *target
	if len(secretQuery) == 0 {
		return sanitized.String()
	}
	values := sanitized.Query()
	for key := range secretQuery {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		values.Del(key)
	}
	sanitized.RawQuery = values.Encode()
	return sanitized.String()
}

func sanitizedSecretQueryError(err error, secretQuery map[string]string) error {
	if err == nil || len(secretQuery) == 0 {
		return err
	}
	message := err.Error()
	for key, value := range secretQuery {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		message = strings.ReplaceAll(message, value, "[redacted]")
		message = strings.ReplaceAll(message, url.QueryEscape(value), "[redacted]")
	}
	return fmt.Errorf("%s", message)
}

func (s *Service) Head(ctx context.Context, input FetchInput) (FetchResult, error) {
	input.Method = http.MethodHead
	return s.Fetch(ctx, input)
}

func (s *Service) httpClient(input FetchInput) *http.Client {
	if s.Client != nil {
		return s.Client
	}
	timeout := time.Duration(s.Config.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	redirectLimit := s.Config.RedirectLimit
	if redirectLimit <= 0 {
		redirectLimit = 3
	}
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= redirectLimit {
				return fmt.Errorf("redirect limit exceeded")
			}
			redirectInput := input
			redirectInput.URL = req.URL.String()
			redirectInput.Method = req.Method
			return s.validatePolicy(redirectInput, req.URL)
		},
	}
}

func (s *Service) maxResponseBytes() int64 {
	if s.Config.MaxResponseBytes > 0 {
		return s.Config.MaxResponseBytes
	}
	return 2000000
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = 2000000
	}
	limited := io.LimitReader(reader, limit+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeded max response bytes")
	}
	return data, nil
}
