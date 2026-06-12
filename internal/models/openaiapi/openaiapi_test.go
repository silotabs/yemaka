package openaiapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"yemaka/internal/models"
)

func TestListModelsUsesOpenAIModelsEndpoint(t *testing.T) {
	runtime := New(models.ProviderOpenAICompatible, "http://127.0.0.1:4000/v1")
	runtime.client = &http.Client{Transport: openAIRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %q, want /v1/models", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want GET", r.Method)
		}
		return openAIJSONResponse(http.StatusOK, `{"data":[{"id":"tiny-local","created":1770000000}]}`), nil
	})}

	items, err := runtime.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(items) != 1 || items[0].Name != "tiny-local" {
		t.Fatalf("models = %+v, want tiny-local", items)
	}
}

func TestChatStreamUsesChatCompletionsEndpoint(t *testing.T) {
	runtime := New(models.ProviderLlamaCpp, "http://127.0.0.1:8080/v1")
	var seenModel string
	var seenContent string
	runtime.client = &http.Client{Transport: openAIRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		var input struct {
			Model    string               `json:"model"`
			Messages []models.ChatMessage `json:"messages"`
			Stream   bool                 `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		seenModel = input.Model
		if len(input.Messages) == 1 {
			seenContent = input.Messages[0].Content
		}
		if input.Stream {
			t.Fatal("stream = true, want false")
		}
		return openAIJSONResponse(http.StatusOK, `{"choices":[{"message":{"content":"hello local"}}]}`), nil
	})}

	var tokens []string
	err := runtime.ChatStream(context.Background(), models.ChatRequest{
		Model:       "tiny-local",
		Messages:    []models.ChatMessage{{Role: "user", Content: "hello"}},
		Temperature: 0.1,
	}, func(event models.ChatEvent) error {
		if event.Token != "" {
			tokens = append(tokens, event.Token)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream() error = %v", err)
	}
	if seenModel != "tiny-local" {
		t.Fatalf("model = %q, want tiny-local", seenModel)
	}
	if seenContent != "hello" {
		t.Fatalf("message = %q, want hello", seenContent)
	}
	if got := strings.Join(tokens, ""); got != "hello local" {
		t.Fatalf("tokens = %q, want hello local", got)
	}
}

func TestShowModelReadsModelList(t *testing.T) {
	runtime := New(models.ProviderOpenAICompatible, "http://127.0.0.1:4000/v1")
	runtime.client = &http.Client{Transport: openAIRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return openAIJSONResponse(http.StatusOK, `{"data":[{"id":"tiny-local"}]}`), nil
	})}

	details, err := runtime.ShowModel(context.Background(), "tiny-local")
	if err != nil {
		t.Fatalf("ShowModel() error = %v", err)
	}
	if details.Name != "tiny-local" {
		t.Fatalf("Name = %q, want tiny-local", details.Name)
	}
	if details.Family != "openai_compatible" {
		t.Fatalf("Family = %q, want openai_compatible", details.Family)
	}
}

type openAIRoundTripFunc func(*http.Request) (*http.Response, error)

func (f openAIRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func openAIJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
