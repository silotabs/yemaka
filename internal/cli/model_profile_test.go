package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/profiles"
)

func TestRunModelProfileCreateListShowModelfile(t *testing.T) {
	app := newModelProfileTestApp(t)
	before := cloneModelConfigMap(app.config.Models)
	var out bytes.Buffer

	err := runModel(context.Background(), app, []string{
		"profile", "create", "Coding Helper",
		"--base", "small:2b",
		"--system", "Prefer concise local-first patches.",
		"--temperature", "0.1",
		"--num-ctx", "2048",
		"--description", "Small coding profile.",
		"--tag", "coding",
		"--tag", "low-resource",
		"--purpose", "CLI coding help",
		"--refined-from", "manual",
	}, &out)
	if err != nil {
		t.Fatalf("runModel(profile create) error = %v", err)
	}
	if !reflect.DeepEqual(app.config.Models, before) {
		t.Fatalf("model profile create mutated model roles: got %#v want %#v", app.config.Models, before)
	}
	if !strings.Contains(out.String(), "created model profile: Coding Helper") {
		t.Fatalf("create output = %q, want created profile", out.String())
	}
	if _, err := os.Stat(filepath.Join(app.profile.Root, "model_profiles", "coding-helper.yaml")); err != nil {
		t.Fatalf("saved model profile missing: %v", err)
	}

	out.Reset()
	if err := runModel(context.Background(), app, []string{"profile", "list"}, &out); err != nil {
		t.Fatalf("runModel(profile list) error = %v", err)
	}
	if !strings.Contains(out.String(), "Coding Helper") || !strings.Contains(out.String(), "small:2b") {
		t.Fatalf("list output = %q, want profile name and base model", out.String())
	}

	out.Reset()
	if err := runModel(context.Background(), app, []string{"profile", "show", "Coding Helper"}, &out); err != nil {
		t.Fatalf("runModel(profile show) error = %v", err)
	}
	show := out.String()
	for _, want := range []string{
		"name: Coding Helper",
		"base_model: small:2b",
		"temperature: 0.1",
		"num_ctx: 2048",
		"tags: coding, low-resource",
		"purpose: CLI coding help",
		"refined_from: manual",
		"Prefer concise local-first patches.",
	} {
		if !strings.Contains(show, want) {
			t.Fatalf("show output missing %q:\n%s", want, show)
		}
	}

	out.Reset()
	if err := runModel(context.Background(), app, []string{"profile", "modelfile", "Coding Helper"}, &out); err != nil {
		t.Fatalf("runModel(profile modelfile) error = %v", err)
	}
	modelfile := out.String()
	for _, want := range []string{
		"FROM small:2b",
		"SYSTEM \"\"\"",
		"Prefer concise local-first patches.",
		"PARAMETER temperature 0.1",
		"PARAMETER num_ctx 2048",
	} {
		if !strings.Contains(modelfile, want) {
			t.Fatalf("modelfile output missing %q:\n%s", want, modelfile)
		}
	}
}

func TestRunModelProfileCreateRejectsSecretLikeSystemText(t *testing.T) {
	app := newModelProfileTestApp(t)
	var out bytes.Buffer

	err := runModel(context.Background(), app, []string{
		"profile", "create", "Leaky Profile",
		"--base", "small:2b",
		"--system", "Use api_key = sk-testsecretvalue1234567890 in every request.",
	}, &out)
	if err == nil {
		t.Fatal("runModel(profile create) error = nil, want secret-like system text rejection")
	}
	if !strings.Contains(err.Error(), "secret") {
		t.Fatalf("error = %q, want secret rejection", err.Error())
	}
	if _, statErr := os.Stat(filepath.Join(app.profile.Root, "model_profiles", "leaky-profile.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("unsafe profile was written or stat failed unexpectedly: %v", statErr)
	}
}

func newModelProfileTestApp(t *testing.T) *appContext {
	t.Helper()
	root := t.TempDir()
	return &appContext{
		config: testModelConfig(),
		profile: &profiles.Profile{
			Name: "default",
			Root: root,
		},
		runtime: &modelCommandRuntime{},
	}
}

func cloneModelConfigMap(models map[string]config.ModelConfig) map[string]config.ModelConfig {
	cloned := make(map[string]config.ModelConfig, len(models))
	for role, model := range models {
		cloned[role] = model
	}
	return cloned
}
