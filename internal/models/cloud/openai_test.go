package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"yemaka/internal/config"
	"yemaka/internal/models"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestChatStreamOpenAICompatibleResponse(t *testing.T) {
	t.Setenv("YEMAKA_TEST_KEY", "test-key")
	var sawAuth bool
	runtime := New(config.CloudFallbackConfig{
		Enabled:        true,
		Provider:       "openai_compatible",
		Name:           "fallback-model",
		BaseURL:        "https://cloud.test/v1",
		APIKeyEnv:      "YEMAKA_TEST_KEY",
		Temperature:    0.2,
		TimeoutSeconds: 5,
	})
	runtime.client = &http.Client{
		Timeout: 5 * time.Second,
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/chat/completions" {
				t.Fatalf("path = %s, want /v1/chat/completions", r.URL.Path)
			}
			if r.Header.Get("Authorization") == "Bearer test-key" {
				sawAuth = true
			}
			var payload struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if payload.Model != "fallback-model" {
				t.Fatalf("model = %q, want fallback-model", payload.Model)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewBufferString(`{"choices":[{"message":{"content":"cloud answer"}}]}`)),
			}, nil
		}),
	}

	var output strings.Builder
	err := runtime.ChatStream(context.Background(), models.ChatRequest{
		Model: "fallback-model",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "hello"},
		},
	}, func(event models.ChatEvent) error {
		output.WriteString(event.Token)
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream() error = %v", err)
	}
	if output.String() != "cloud answer" {
		t.Fatalf("output = %q, want cloud answer", output.String())
	}
	if !sawAuth {
		t.Fatal("Authorization header was not sent")
	}
}

func TestHealthRequiresConfiguredAPIKeyEnvWhenProvided(t *testing.T) {
	runtime := New(config.CloudFallbackConfig{
		Enabled:   true,
		Name:      "fallback-model",
		BaseURL:   "https://example.test/v1",
		APIKeyEnv: "YEMAKA_MISSING_KEY",
	})
	if err := runtime.Health(context.Background()); err == nil {
		t.Fatal("Health() error = nil, want missing key error")
	}
}
