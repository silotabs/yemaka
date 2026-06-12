package openaiapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"yemaka/internal/models"
)

type Runtime struct {
	provider string
	baseURL  string
	client   *http.Client
}

func New(provider string, baseURL string) *Runtime {
	return &Runtime{
		provider: models.NormalizeProvider(provider),
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:   &http.Client{Timeout: 45 * time.Second},
	}
}

func (r *Runtime) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.modelsURL(), nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s runtime not reachable at %s: %w", r.providerName(), r.baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s runtime returned status %s", r.providerName(), resp.Status)
	}
	return nil
}

func (r *Runtime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.modelsURL(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list %s models: %w", r.providerName(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = resp.Status
		}
		return nil, fmt.Errorf("list %s models: %s", r.providerName(), detail)
	}

	var payload struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode %s model list: %w", r.providerName(), err)
	}
	infos := make([]models.ModelInfo, 0, len(payload.Data))
	for _, item := range payload.Data {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		info := models.ModelInfo{Name: item.ID}
		if item.Created > 0 {
			info.ModifiedAt = time.Unix(item.Created, 0).UTC().Format(time.RFC3339)
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (r *Runtime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return fmt.Errorf("%s runtime does not pull models; start the local server with an installed model", r.providerName())
}

func (r *Runtime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	if emit == nil {
		emit = func(models.ChatEvent) error { return nil }
	}
	req = models.NormalizeChatRequest(req)
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		return fmt.Errorf("model is required")
	}
	payload := map[string]any{
		"model":       modelName,
		"messages":    req.Messages,
		"temperature": req.Temperature,
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
		return fmt.Errorf("encode %s chat request: %w", r.providerName(), err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.chatURL(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%s chat request failed: %w", r.providerName(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = resp.Status
		}
		return fmt.Errorf("%s chat returned status %s: %s", r.providerName(), resp.Status, detail)
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
		return fmt.Errorf("decode %s chat response: %w", r.providerName(), err)
	}
	if len(output.Choices) == 0 {
		return fmt.Errorf("%s chat returned no choices", r.providerName())
	}
	content := output.Choices[0].Message.Content
	if content == "" {
		content = output.Choices[0].Text
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("%s chat returned empty content", r.providerName())
	}
	if err := emit(models.ChatEvent{Token: content}); err != nil {
		return err
	}
	return emit(models.ChatEvent{Done: true})
}

func (r *Runtime) GenerateStream(ctx context.Context, req models.GenerateRequest, emit func(models.GenerateEvent) error) error {
	var output strings.Builder
	err := r.ChatStream(ctx, models.ChatRequest{
		Model: req.Model,
		Messages: []models.ChatMessage{
			{Role: "user", Content: req.Prompt},
		},
		Temperature: req.Temperature,
	}, func(event models.ChatEvent) error {
		if event.Token != "" {
			output.WriteString(event.Token)
			if emit != nil {
				return emit(models.GenerateEvent{Token: event.Token})
			}
		}
		if event.Done && emit != nil {
			return emit(models.GenerateEvent{Done: true})
		}
		return nil
	})
	if err != nil {
		return err
	}
	if emit == nil {
		_ = output.String()
	}
	return nil
}

func (r *Runtime) ShowModel(ctx context.Context, name string) (models.ModelDetails, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return models.ModelDetails{}, fmt.Errorf("model is required")
	}
	installed, err := r.ListModels(ctx)
	if err != nil {
		return models.ModelDetails{}, err
	}
	for _, model := range installed {
		if strings.EqualFold(model.Name, name) {
			return models.ModelDetails{
				Name:       model.Name,
				ModifiedAt: model.ModifiedAt,
				Size:       model.Size,
				Digest:     model.Digest,
				Family:     r.providerName(),
				Details: map[string]any{
					"provider": r.provider,
					"base_url": r.baseURL,
				},
			}, nil
		}
	}
	return models.ModelDetails{}, fmt.Errorf("model is not listed by %s runtime: %s", r.providerName(), name)
}

func (r *Runtime) chatURL() string {
	base := r.base()
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

func (r *Runtime) modelsURL() string {
	base := r.base()
	if strings.HasSuffix(base, "/models") {
		return base
	}
	return base + "/models"
}

func (r *Runtime) base() string {
	base := strings.TrimRight(strings.TrimSpace(r.baseURL), "/")
	if base == "" {
		base = "http://127.0.0.1:8080/v1"
	}
	if strings.HasSuffix(base, "/v1") {
		return base
	}
	if strings.HasSuffix(base, "/v1/chat/completions") || strings.HasSuffix(base, "/v1/models") {
		return strings.TrimSuffix(strings.TrimSuffix(base, "/chat/completions"), "/models")
	}
	return base + "/v1"
}

func (r *Runtime) providerName() string {
	if r.provider == models.ProviderLlamaCpp {
		return "llama.cpp"
	}
	if r.provider == "" {
		return "openai-compatible"
	}
	return r.provider
}
