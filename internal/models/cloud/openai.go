package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"yemaka/internal/config"
	"yemaka/internal/models"
)

type Runtime struct {
	cfg    config.CloudFallbackConfig
	client *http.Client
}

func New(cfg config.CloudFallbackConfig) *Runtime {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	return &Runtime{
		cfg: cfg,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (r *Runtime) Health(ctx context.Context) error {
	if r == nil {
		return fmt.Errorf("cloud runtime is nil")
	}
	if !r.cfg.Enabled {
		return fmt.Errorf("cloud fallback is disabled")
	}
	if strings.TrimSpace(r.cfg.Name) == "" {
		return fmt.Errorf("cloud fallback model is not configured")
	}
	if strings.TrimSpace(r.cfg.BaseURL) == "" {
		return fmt.Errorf("cloud fallback base_url is not configured")
	}
	if err := r.apiKeyReady(); err != nil {
		return err
	}
	return nil
}

func (r *Runtime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	if err := r.Health(ctx); err != nil {
		return nil, err
	}
	return []models.ModelInfo{{Name: r.cfg.Name}}, nil
}

func (r *Runtime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return fmt.Errorf("cloud fallback does not pull models")
}

func (r *Runtime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	if err := r.Health(ctx); err != nil {
		return err
	}
	if emit == nil {
		emit = func(models.ChatEvent) error { return nil }
	}
	req = models.NormalizeChatRequest(req)
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = r.cfg.Name
	}
	temperature := req.Temperature
	if temperature == 0 {
		temperature = r.cfg.Temperature
	}

	payload := map[string]any{
		"model":       modelName,
		"messages":    req.Messages,
		"temperature": temperature,
		"stream":      false,
	}
	if len(req.Tools) > 0 {
		payload["tools"] = req.Tools
	}
	if strings.TrimSpace(req.ToolChoice) != "" {
		payload["tool_choice"] = req.ToolChoice
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode cloud chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.chatURL(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if key := r.apiKey(); key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("cloud chat request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(data))
		if detail == "" {
			detail = resp.Status
		}
		return fmt.Errorf("cloud chat returned status %s: %s", resp.Status, detail)
	}

	var output struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return fmt.Errorf("decode cloud chat response: %w", err)
	}
	if len(output.Choices) == 0 {
		return fmt.Errorf("cloud chat returned no choices")
	}
	content := output.Choices[0].Message.Content
	if content == "" {
		content = output.Choices[0].Text
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("cloud chat returned empty content")
	}
	if err := emit(models.ChatEvent{Token: content}); err != nil {
		return err
	}
	return emit(models.ChatEvent{Done: true})
}

func (r *Runtime) chatURL() string {
	base := strings.TrimRight(strings.TrimSpace(r.cfg.BaseURL), "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

func (r *Runtime) apiKeyReady() error {
	env := strings.TrimSpace(r.cfg.APIKeyEnv)
	if env == "" {
		return nil
	}
	if strings.TrimSpace(os.Getenv(env)) == "" {
		return fmt.Errorf("cloud fallback API key env %s is not set", env)
	}
	return nil
}

func (r *Runtime) apiKey() string {
	env := strings.TrimSpace(r.cfg.APIKeyEnv)
	if env == "" {
		return ""
	}
	return strings.TrimSpace(os.Getenv(env))
}
