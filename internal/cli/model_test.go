package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/models"
)

type modelCommandRuntime struct {
	showName     string
	generateName string
	prompt       string
}

func (r *modelCommandRuntime) Health(ctx context.Context) error {
	return nil
}

func (r *modelCommandRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return []models.ModelInfo{{Name: "small:2b", Size: 123}}, nil
}

func (r *modelCommandRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (r *modelCommandRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	return nil
}

func (r *modelCommandRuntime) ShowModel(ctx context.Context, name string) (models.ModelDetails, error) {
	r.showName = name
	return models.ModelDetails{
		Name:              name,
		Family:            "qwen2",
		ParameterSize:     "2B",
		QuantizationLevel: "Q4_K_M",
		ContextLength:     4096,
	}, nil
}

func (r *modelCommandRuntime) GenerateStream(ctx context.Context, req models.GenerateRequest, emit func(models.GenerateEvent) error) error {
	r.generateName = req.Model
	r.prompt = req.Prompt
	if err := emit(models.GenerateEvent{Token: "hello"}); err != nil {
		return err
	}
	return emit(models.GenerateEvent{Done: true})
}

func TestRunModelShowUsesDetailCapability(t *testing.T) {
	runtime := &modelCommandRuntime{}
	app := &appContext{
		config:  testModelConfig(),
		runtime: runtime,
	}
	var out bytes.Buffer

	if err := runModel(context.Background(), app, []string{"show", "small:2b"}, &out); err != nil {
		t.Fatalf("runModel(show) error = %v", err)
	}
	if runtime.showName != "small:2b" {
		t.Fatalf("showName = %q, want small:2b", runtime.showName)
	}
	if !bytes.Contains(out.Bytes(), []byte("parameter_size: 2B")) {
		t.Fatalf("output missing model details: %s", out.String())
	}
}

func TestRunModelGenerateUsesGenerateCapability(t *testing.T) {
	runtime := &modelCommandRuntime{}
	app := &appContext{
		config:  testModelConfig(),
		runtime: runtime,
	}
	var out bytes.Buffer

	if err := runModel(context.Background(), app, []string{"generate", "small:2b", "Say", "hello"}, &out); err != nil {
		t.Fatalf("runModel(generate) error = %v", err)
	}
	if runtime.generateName != "small:2b" {
		t.Fatalf("generateName = %q, want small:2b", runtime.generateName)
	}
	if runtime.prompt != "Say hello" {
		t.Fatalf("prompt = %q, want Say hello", runtime.prompt)
	}
	if out.String() != "hello\n" {
		t.Fatalf("output = %q, want hello newline", out.String())
	}
}

func TestRunModelProvidersListsLocalRuntimeProviders(t *testing.T) {
	app := &appContext{
		config:  testModelConfig(),
		runtime: &modelCommandRuntime{},
	}
	var out bytes.Buffer

	if err := runModel(context.Background(), app, []string{"providers"}, &out); err != nil {
		t.Fatalf("runModel(providers) error = %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("ollama")) || !bytes.Contains(out.Bytes(), []byte("llamacpp")) {
		t.Fatalf("output missing local providers: %s", out.String())
	}
}

func TestRunModelSetSupportsExplicitLocalProvider(t *testing.T) {
	cfg := testModelConfig()
	cfg.Path = filepath.Join(t.TempDir(), "config.yaml")
	app := &appContext{
		config:  cfg,
		runtime: &modelCommandRuntime{},
	}
	var out bytes.Buffer

	err := runModel(context.Background(), app, []string{"set", "coding", "tiny-local", "--provider", "llama.cpp", "--base-url", "http://127.0.0.1:8080/v1"}, &out)
	if err != nil {
		t.Fatalf("runModel(set) error = %v", err)
	}
	model := app.config.Models["coding"]
	if model.Provider != models.ProviderLlamaCpp {
		t.Fatalf("Provider = %q, want %q", model.Provider, models.ProviderLlamaCpp)
	}
	if model.BaseURL != "http://127.0.0.1:8080/v1" {
		t.Fatalf("BaseURL = %q", model.BaseURL)
	}
	if !bytes.Contains(out.Bytes(), []byte("llamacpp")) {
		t.Fatalf("output missing provider: %s", out.String())
	}
}

func TestRunModelSetRejectsRemoteRuntimeBaseURL(t *testing.T) {
	cfg := testModelConfig()
	cfg.Path = filepath.Join(t.TempDir(), "config.yaml")
	app := &appContext{
		config:  cfg,
		runtime: &modelCommandRuntime{},
	}
	var out bytes.Buffer

	err := runModel(context.Background(), app, []string{"set", "coding", "remote", "--provider", "openai_compatible", "--base-url", "https://api.example.com/v1"}, &out)
	if err == nil {
		t.Fatal("runModel(set) error = nil, want remote endpoint rejection")
	}
}

func testModelConfig() *config.Config {
	return &config.Config{
		Models: map[string]config.ModelConfig{
			"default":    {Provider: "ollama", Name: "small:2b", BaseURL: "http://localhost:11434/api", Temperature: 0.2},
			"low_memory": {Provider: "ollama", Name: "small:2b", BaseURL: "http://localhost:11434/api", Temperature: 0.1},
			"coding":     {Provider: "ollama", Name: "small:2b", BaseURL: "http://localhost:11434/api", Temperature: 0.1},
		},
	}
}
