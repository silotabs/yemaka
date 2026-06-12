package models

import (
	"testing"

	"yemaka/internal/config"
)

func TestRouterSelectsLowMemoryModelWhenEnabled(t *testing.T) {
	cfg := config.Default()
	router := NewRouter(cfg)

	model, err := router.Select(TaskChat)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if model.Name != "deepseek-r1:1.5b" {
		t.Fatalf("model = %q, want low-memory model", model.Name)
	}
}

func TestRouterSelectsTaskModelWhenLowMemoryDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.Runtime.LowMemoryMode = false
	router := NewRouter(cfg)

	model, err := router.Select(TaskCoding)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if model.Name != "qwen2.5-coder:3b" {
		t.Fatalf("model = %q, want coding model", model.Name)
	}
}

func TestRouterAllowsSpecializedTaskModelInLowMemoryMode(t *testing.T) {
	cfg := config.Default()
	router := NewRouter(cfg)

	model, err := router.Select(TaskReasoning)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if model.Name != "deepseek-r1:1.5b" {
		t.Fatalf("model = %q, want reasoning model", model.Name)
	}

	model, err = router.Select(TaskCoding)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if model.Name != "qwen2.5-coder:3b" {
		t.Fatalf("model = %q, want coding model", model.Name)
	}
}
