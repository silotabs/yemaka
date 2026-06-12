package modelruntime

import (
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/models"
	"yemaka/internal/models/ollama"
	"yemaka/internal/models/openaiapi"
)

func TestNewDefaultsToOllama(t *testing.T) {
	runtime, err := New(config.ModelConfig{Name: "tiny:2b"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, ok := runtime.(*ollama.Runtime); !ok {
		t.Fatalf("runtime type = %T, want *ollama.Runtime", runtime)
	}
}

func TestNewSupportsLlamaCppLocalEndpoint(t *testing.T) {
	runtime, err := New(config.ModelConfig{
		Provider: models.ProviderLlamaCpp,
		Name:     "tiny-local",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, ok := runtime.(*openaiapi.Runtime); !ok {
		t.Fatalf("runtime type = %T, want *openaiapi.Runtime", runtime)
	}
}

func TestNewRejectsRemoteRuntimeEndpoint(t *testing.T) {
	_, err := New(config.ModelConfig{
		Provider: models.ProviderOpenAICompatible,
		Name:     "remote-model",
		BaseURL:  "https://api.example.com/v1",
	})
	if err == nil {
		t.Fatal("New() error = nil, want remote endpoint rejection")
	}
}

func TestIsLocalBaseURL(t *testing.T) {
	local := []string{
		"http://localhost:11434/api",
		"http://127.0.0.1:8080/v1",
		"http://[::1]:4000/v1",
	}
	for _, raw := range local {
		if !IsLocalBaseURL(raw) {
			t.Fatalf("IsLocalBaseURL(%q) = false, want true", raw)
		}
	}
	if IsLocalBaseURL("https://example.com/v1") {
		t.Fatal("IsLocalBaseURL(remote) = true, want false")
	}
}
