package replay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreSaveLoadList(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	trace := NewTrace("  summarize replay foundation  ")
	trace.ID = " trace one "
	trace.ToolsCalled = []ToolCall{{
		Name:          " shell ",
		InputSummary:  " Authorization: Bearer abcdefghijklmnop ",
		OutputSummary: " ok ",
		Status:        " completed ",
		Attributes: []Attribute{
			{Key: "api_key", Value: "secret-value"},
			{Key: "provider", Value: "local"},
		},
	}}

	saved, err := store.Save(trace)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if saved.ID != "trace one" || saved.Filename != "trace-one.json" {
		t.Fatalf("saved file = %+v, want sanitized filename with normalized id", saved)
	}
	if filepath.Dir(saved.Path) != root {
		t.Fatalf("saved path = %q, want inside %q", saved.Path, root)
	}

	loaded, err := store.Load("trace one")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.ID != "trace one" || loaded.UserRequest != "summarize replay foundation" {
		t.Fatalf("loaded trace identifiers = %+v", loaded)
	}
	if got := loaded.ToolsCalled[0].Attributes[0]; got.Key != "api_key" || got.Value != "[redacted]" {
		t.Fatalf("secret attribute = %+v, want redacted value", got)
	}
	if strings.Contains(loaded.ToolsCalled[0].InputSummary, "abcdefghijklmnop") {
		t.Fatalf("InputSummary = %q, want bearer token redacted", loaded.ToolsCalled[0].InputSummary)
	}

	files, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(files) != 1 || files[0].ID != "trace one" || files[0].Filename != "trace-one.json" {
		t.Fatalf("List() = %+v, want saved trace file", files)
	}
}

func TestStoreRejectsPathTraversal(t *testing.T) {
	store := NewStore(t.TempDir())
	for _, id := range []string{"../escape", "..", "nested/trace", `nested\trace`, "trace..escape"} {
		trace := NewTrace("path safety")
		trace.ID = id
		if _, err := store.Save(trace); err == nil {
			t.Fatalf("Save(%q) error = nil, want unsafe id rejection", id)
		}
		if _, err := store.Load(id); err == nil {
			t.Fatalf("Load(%q) error = nil, want unsafe id rejection", id)
		}
	}
}

func TestStoreWritesStableJSON(t *testing.T) {
	store := NewStore(t.TempDir())
	trace := NewTrace("stable json")
	trace.ID = "stable"
	trace.Attributes = []Attribute{
		{Key: "z", Value: "last"},
		{Key: "a", Value: "first"},
	}
	trace.Plan = PlanSnapshot{Steps: []string{"two", "one"}}

	saved, err := store.Save(trace)
	if err != nil {
		t.Fatalf("first Save() error = %v", err)
	}
	first, err := os.ReadFile(saved.Path)
	if err != nil {
		t.Fatalf("ReadFile(first) error = %v", err)
	}
	if _, err := store.Save(trace); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	second, err := os.ReadFile(saved.Path)
	if err != nil {
		t.Fatalf("ReadFile(second) error = %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("stable JSON mismatch\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if !strings.HasSuffix(string(first), "\n") || !strings.Contains(string(first), "\n  \"schema\":") {
		t.Fatalf("stored JSON = %q, want indented JSON with trailing newline", string(first))
	}
}

func TestStoreDoesNotWriteOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "store")
	store := NewStore(root)
	trace := NewTrace("do not escape")
	trace.ID = "../outside"

	if _, err := store.Save(trace); err == nil {
		t.Fatal("Save() error = nil, want traversal rejection")
	}
	if _, err := os.Stat(filepath.Join(parent, "outside.json")); !os.IsNotExist(err) {
		t.Fatalf("outside file stat error = %v, want not exist", err)
	}
	if entries, err := os.ReadDir(parent); err != nil {
		t.Fatalf("ReadDir(parent) error = %v", err)
	} else if len(entries) != 0 {
		t.Fatalf("parent entries = %v, want no writes after rejected save", entries)
	}
}

func TestStoreLoadRedactsExistingTrace(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	trace := NewTrace("Search docs with token=legacy-secret-value")
	trace.ID = "token=legacy-secret-value"
	trace.ToolsCalled = []ToolCall{{
		Name:         "internet_search",
		InputSummary: "Authorization: Bearer abcdefghijklmnop",
		Status:       "failed",
		Error:        "provider token=legacy-secret-value rejected",
		Attributes: []Attribute{{
			Key:   "api_key",
			Value: "legacy-secret-value",
		}},
	}}
	data, err := json.Marshal(trace)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "legacy.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	loaded, err := store.Load("legacy")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	encoded, err := json.Marshal(loaded)
	if err != nil {
		t.Fatalf("Marshal(loaded) error = %v", err)
	}
	text := string(encoded)
	for _, leaked := range []string{"legacy-secret-value", "abcdefghijklmnop"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("Load() leaked %q in %s", leaked, text)
		}
	}
	if !strings.Contains(text, "[redacted]") {
		t.Fatalf("Load() = %s, want redaction marker", text)
	}
	files, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("List() returned %d files, want 1", len(files))
	}
	if strings.Contains(files[0].ID, "legacy-secret-value") {
		t.Fatalf("List() leaked trace id secret: %+v", files[0])
	}
}
