package models

import (
	"encoding/json"
	"testing"
)

func TestNormalizeChatRequestMergesSystemAndKeepsToolMetadata(t *testing.T) {
	req := NormalizeChatRequest(ChatRequest{
		Model:  " small:2b ",
		System: "Core system",
		Messages: []ChatMessage{
			{Role: "system", Content: "Session system"},
			{Role: "USER", Content: "  hello  "},
			{Role: "assistant", Content: ""},
			{Role: "assistant", ToolCalls: []ToolCall{{Function: ToolCallFunction{Name: " read_file ", Arguments: map[string]any{"path": "README.md"}}}}},
			{Role: "weird", Content: "fallback role"},
		},
		Tools: []ToolDefinition{
			{Function: ToolFunctionDefinition{Name: " internet_fetch "}},
			{Function: ToolFunctionDefinition{Name: ""}},
		},
	})

	if req.Model != "small:2b" {
		t.Fatalf("Model = %q, want small:2b", req.Model)
	}
	if len(req.Messages) != 4 {
		t.Fatalf("message count = %d, want 4: %#v", len(req.Messages), req.Messages)
	}
	if req.Messages[0].Role != "system" || req.Messages[0].Content != "Core system\n\nSession system" {
		t.Fatalf("system message = %#v", req.Messages[0])
	}
	if req.Messages[1].Role != "user" || req.Messages[1].Content != "hello" {
		t.Fatalf("user message = %#v", req.Messages[1])
	}
	if req.Messages[2].Role != "assistant" || len(req.Messages[2].ToolCalls) != 1 {
		t.Fatalf("tool-call assistant message = %#v", req.Messages[2])
	}
	if req.Messages[2].ToolCalls[0].Type != "function" || req.Messages[2].ToolCalls[0].Function.Name != "read_file" {
		t.Fatalf("normalized tool call = %#v", req.Messages[2].ToolCalls[0])
	}
	if req.Messages[3].Role != "user" {
		t.Fatalf("unknown role normalized to %q, want user", req.Messages[3].Role)
	}
	if len(req.Tools) != 1 || req.Tools[0].Type != "function" || req.Tools[0].Function.Name != "internet_fetch" {
		t.Fatalf("tools = %#v, want one normalized function", req.Tools)
	}
}

func TestToolCallFunctionDecodesObjectAndStringArguments(t *testing.T) {
	var objectCall ToolCall
	if err := json.Unmarshal([]byte(`{"function":{"name":"read_file","arguments":{"path":"README.md"}}}`), &objectCall); err != nil {
		t.Fatalf("decode object arguments: %v", err)
	}
	if objectCall.Function.Arguments["path"] != "README.md" {
		t.Fatalf("object arguments = %#v, want path", objectCall.Function.Arguments)
	}

	var stringCall ToolCall
	if err := json.Unmarshal([]byte(`{"function":{"name":"read_file","arguments":"{\"path\":\"README.md\"}"}}`), &stringCall); err != nil {
		t.Fatalf("decode string arguments: %v", err)
	}
	if stringCall.Function.Arguments["path"] != "README.md" {
		t.Fatalf("string arguments = %#v, want path", stringCall.Function.Arguments)
	}
}

func TestLocalInstalledModelsKeepsUnknownSizeLocalModels(t *testing.T) {
	local := LocalInstalledModels([]ModelInfo{
		{Name: "tiny-local:2b", Size: 0},
		{Name: "remote:cloud", Size: 0},
		{Name: "", Size: 123},
	})
	if len(local) != 1 {
		t.Fatalf("local count = %d, want 1", len(local))
	}
	if local[0].Name != "tiny-local:2b" {
		t.Fatalf("local model = %q, want tiny-local:2b", local[0].Name)
	}
}

func TestModelInstalledNormalizesName(t *testing.T) {
	installed := []ModelInfo{{Name: "tiny-local:2b"}}
	if !ModelInstalled(" TINY-LOCAL:2B ", installed) {
		t.Fatal("expected installed model lookup to trim and ignore case")
	}
	if ModelInstalled("", installed) {
		t.Fatal("empty model name should not be installed")
	}
}
