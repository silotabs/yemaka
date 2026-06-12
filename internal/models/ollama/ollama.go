package ollama

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
	baseURL string
	client  *http.Client
}

func New(baseURL string) *Runtime {
	return &Runtime{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (r *Runtime) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url("/tags"), nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama not reachable at %s: %w", r.baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ollama returned status %s", resp.Status)
	}
	return nil
}

func (r *Runtime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url("/tags"), nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list ollama models: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list ollama models: status %s", resp.Status)
	}

	var payload struct {
		Models []struct {
			Name       string `json:"name"`
			ModifiedAt string `json:"modified_at"`
			Size       int64  `json:"size"`
			Digest     string `json:"digest"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode ollama model list: %w", err)
	}

	infos := make([]models.ModelInfo, 0, len(payload.Models))
	for _, model := range payload.Models {
		infos = append(infos, models.ModelInfo{
			Name:       model.Name,
			ModifiedAt: model.ModifiedAt,
			Size:       model.Size,
			Digest:     model.Digest,
		})
	}
	return infos, nil
}

func (r *Runtime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	payload := map[string]any{
		"model":  name,
		"stream": true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode ollama pull request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url("/pull"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := *r.client
	client.Timeout = 0
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ollama pull request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = resp.Status
		}
		return fmt.Errorf("ollama pull returned status %s: %s", resp.Status, detail)
	}

	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk struct {
			Status    string `json:"status"`
			Digest    string `json:"digest"`
			Total     int64  `json:"total"`
			Completed int64  `json:"completed"`
			Error     string `json:"error"`
		}
		if err := decoder.Decode(&chunk); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("decode ollama pull stream: %w", err)
		}
		if chunk.Error != "" {
			return fmt.Errorf("ollama pull error: %s", chunk.Error)
		}

		event := models.PullEvent{
			Status:    chunk.Status,
			Digest:    chunk.Digest,
			Total:     chunk.Total,
			Completed: chunk.Completed,
			Done:      isPullDone(chunk.Status),
		}
		if emit != nil {
			if err := emit(event); err != nil {
				return err
			}
		}
		if event.Done {
			return nil
		}
	}
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
	options := map[string]any{
		"temperature": req.Temperature,
	}
	if req.NumPredict > 0 {
		options["num_predict"] = req.NumPredict
	}
	if req.NumCtx > 0 {
		options["num_ctx"] = req.NumCtx
	}
	payload := map[string]any{
		"model":    modelName,
		"messages": req.Messages,
		"stream":   true,
		"options":  options,
	}
	if req.ThinkingLevel != "" {
		payload["think"] = req.ThinkingLevel
	} else if req.Think != nil {
		payload["think"] = *req.Think
	}
	if len(req.Tools) > 0 {
		payload["tools"] = req.Tools
	}
	if strings.TrimSpace(req.ToolChoice) != "" {
		payload["tool_choice"] = req.ToolChoice
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode ollama chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url("/chat"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := *r.client
	client.Timeout = 0
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ollama chat request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = resp.Status
		}
		return fmt.Errorf("ollama chat returned status %s: %s\nhint: configure an installed low-end model with `yemaka model set low_memory <installed-model>`", resp.Status, detail)
	}

	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk struct {
			Message struct {
				Role      string            `json:"role"`
				Content   string            `json:"content"`
				ToolCalls []models.ToolCall `json:"tool_calls"`
			} `json:"message"`
			Done  bool   `json:"done"`
			Error string `json:"error"`
		}
		if err := decoder.Decode(&chunk); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("decode ollama stream: %w", err)
		}
		if chunk.Error != "" {
			return fmt.Errorf("ollama stream error: %s", chunk.Error)
		}
		if chunk.Message.Content != "" {
			if err := emit(models.ChatEvent{Token: chunk.Message.Content}); err != nil {
				return err
			}
		}
		if len(chunk.Message.ToolCalls) > 0 {
			if err := emit(models.ChatEvent{ToolCalls: chunk.Message.ToolCalls}); err != nil {
				return err
			}
		}
		if chunk.Done {
			return emit(models.ChatEvent{Done: true})
		}
	}
}

func (r *Runtime) GenerateStream(ctx context.Context, req models.GenerateRequest, emit func(models.GenerateEvent) error) error {
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		return fmt.Errorf("model is required")
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return fmt.Errorf("prompt is required")
	}
	payload := map[string]any{
		"model":  modelName,
		"prompt": prompt,
		"stream": true,
		"options": map[string]any{
			"temperature": req.Temperature,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode ollama generate request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url("/generate"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := *r.client
	client.Timeout = 0
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ollama generate request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = resp.Status
		}
		return fmt.Errorf("ollama generate returned status %s: %s\nhint: choose an installed local model with `yemaka model list`", resp.Status, detail)
	}

	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk struct {
			Response string `json:"response"`
			Done     bool   `json:"done"`
			Error    string `json:"error"`
		}
		if err := decoder.Decode(&chunk); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("decode ollama generate stream: %w", err)
		}
		if chunk.Error != "" {
			return fmt.Errorf("ollama generate error: %s", chunk.Error)
		}
		if chunk.Response != "" && emit != nil {
			if err := emit(models.GenerateEvent{Token: chunk.Response}); err != nil {
				return err
			}
		}
		if chunk.Done {
			if emit == nil {
				return nil
			}
			return emit(models.GenerateEvent{Done: true})
		}
	}
}

func (r *Runtime) ShowModel(ctx context.Context, name string) (models.ModelDetails, error) {
	modelName := strings.TrimSpace(name)
	if modelName == "" {
		return models.ModelDetails{}, fmt.Errorf("model is required")
	}
	body, err := json.Marshal(map[string]any{"model": modelName})
	if err != nil {
		return models.ModelDetails{}, fmt.Errorf("encode ollama show request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url("/show"), bytes.NewReader(body))
	if err != nil {
		return models.ModelDetails{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return models.ModelDetails{}, fmt.Errorf("ollama show request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = resp.Status
		}
		return models.ModelDetails{}, fmt.Errorf("ollama show returned status %s: %s\nhint: choose an installed local model with `yemaka model list`", resp.Status, detail)
	}

	var payload map[string]any
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return models.ModelDetails{}, fmt.Errorf("decode ollama show response: %w", err)
	}

	details := mapField(payload, "details")
	modelInfo := mapField(payload, "model_info")
	return models.ModelDetails{
		Name:              firstNonEmpty(stringField(payload, "name"), stringField(payload, "model"), modelName),
		ModifiedAt:        stringField(payload, "modified_at"),
		Size:              int64Field(payload, "size"),
		Digest:            stringField(payload, "digest"),
		Family:            stringField(details, "family"),
		Format:            stringField(details, "format"),
		ParameterSize:     stringField(details, "parameter_size"),
		QuantizationLevel: stringField(details, "quantization_level"),
		ContextLength:     contextLength(modelInfo),
		Template:          stringField(payload, "template"),
		System:            stringField(payload, "system"),
		License:           stringField(payload, "license"),
		Parameters:        stringField(payload, "parameters"),
		Modelfile:         stringField(payload, "modelfile"),
		Details:           details,
		ModelInfo:         modelInfo,
	}, nil
}

func (r *Runtime) Embed(ctx context.Context, req models.EmbeddingRequest) (models.EmbeddingResponse, error) {
	if strings.TrimSpace(req.Model) == "" {
		return models.EmbeddingResponse{}, fmt.Errorf("embedding model is required")
	}
	if len(req.Input) == 0 {
		return models.EmbeddingResponse{}, nil
	}
	payload := map[string]any{
		"model": req.Model,
		"input": req.Input,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return models.EmbeddingResponse{}, fmt.Errorf("encode ollama embed request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url("/embed"), bytes.NewReader(body))
	if err != nil {
		return models.EmbeddingResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return models.EmbeddingResponse{}, fmt.Errorf("ollama embed request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = resp.Status
		}
		return models.EmbeddingResponse{}, fmt.Errorf("ollama embed returned status %s: %s\nhint: configure an installed embedding model with `yemaka rag embeddings on <installed-model>`", resp.Status, detail)
	}

	var payloadOut struct {
		Embeddings [][]float64 `json:"embeddings"`
		Embedding  []float64   `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payloadOut); err != nil {
		return models.EmbeddingResponse{}, fmt.Errorf("decode ollama embed response: %w", err)
	}
	if len(payloadOut.Embeddings) == 0 && len(payloadOut.Embedding) > 0 {
		payloadOut.Embeddings = [][]float64{payloadOut.Embedding}
	}
	if len(payloadOut.Embeddings) != len(req.Input) {
		return models.EmbeddingResponse{}, fmt.Errorf("ollama embed returned %d embedding(s) for %d input(s)", len(payloadOut.Embeddings), len(req.Input))
	}
	return models.EmbeddingResponse{Embeddings: payloadOut.Embeddings}, nil
}

func (r *Runtime) url(path string) string {
	return r.baseURL + path
}

func isPullDone(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "success" || status == "up to date"
}

func mapField(values map[string]any, key string) map[string]any {
	if values == nil {
		return nil
	}
	if nested, ok := values[key].(map[string]any); ok {
		return nested
	}
	return nil
}

func stringField(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	switch value := values[key].(type) {
	case string:
		return value
	case json.Number:
		return value.String()
	case float64:
		return fmt.Sprintf("%g", value)
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func int64Field(values map[string]any, key string) int64 {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case json.Number:
		result, _ := value.Int64()
		return result
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	default:
		return 0
	}
}

func contextLength(modelInfo map[string]any) int {
	if modelInfo == nil {
		return 0
	}
	for key := range modelInfo {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized == "context_length" || normalized == "num_ctx" || strings.HasSuffix(normalized, ".context_length") {
			return int(int64Field(modelInfo, key))
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func DefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}
