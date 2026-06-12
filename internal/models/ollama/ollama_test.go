package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"yemaka/internal/models"
)

func TestGenerateStreamUsesOllamaGenerateAPI(t *testing.T) {
	var seenModel string
	var seenPrompt string
	runtime := New("http://ollama.test/api")
	runtime.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/generate" {
			t.Fatalf("path = %q, want /api/generate", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		var input struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
			Stream bool   `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		seenModel = input.Model
		seenPrompt = input.Prompt
		if !input.Stream {
			t.Fatal("stream = false, want true")
		}
		body := `{"response":"hel","done":false}` + "\n" +
			`{"response":"lo","done":false}` + "\n" +
			`{"done":true}` + "\n"
		return jsonResponse(http.StatusOK, "application/x-ndjson", body), nil
	})}

	var tokens []string
	err := runtime.GenerateStream(context.Background(), models.GenerateRequest{
		Model:       "small:2b",
		Prompt:      "Say hello briefly",
		Temperature: 0.1,
	}, func(event models.GenerateEvent) error {
		if event.Token != "" {
			tokens = append(tokens, event.Token)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("GenerateStream() error = %v", err)
	}
	if seenModel != "small:2b" {
		t.Fatalf("model = %q, want small:2b", seenModel)
	}
	if seenPrompt != "Say hello briefly" {
		t.Fatalf("prompt = %q, want Say hello briefly", seenPrompt)
	}
	if got := strings.Join(tokens, ""); got != "hello" {
		t.Fatalf("tokens = %q, want hello", got)
	}
}

func TestChatStreamNormalizesMessagesAndDecodesToolCalls(t *testing.T) {
	var request struct {
		Model      string                  `json:"model"`
		Messages   []models.ChatMessage    `json:"messages"`
		Tools      []models.ToolDefinition `json:"tools"`
		ToolChoice string                  `json:"tool_choice"`
		Stream     bool                    `json:"stream"`
	}
	runtime := New("http://ollama.test/api")
	runtime.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("path = %q, want /api/chat", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		body := `{"message":{"role":"assistant","content":"checking "},"done":false}` + "\n" +
			`{"message":{"role":"assistant","tool_calls":[{"function":{"name":"read_file","arguments":"{\"path\":\"README.md\"}"}}]},"done":false}` + "\n" +
			`{"message":{"role":"assistant","content":"done"},"done":false}` + "\n" +
			`{"done":true}` + "\n"
		return jsonResponse(http.StatusOK, "application/x-ndjson", body), nil
	})}

	var tokens []string
	var toolCalls []models.ToolCall
	err := runtime.ChatStream(context.Background(), models.ChatRequest{
		Model:  " small:2b ",
		System: "Use tools safely.",
		Messages: []models.ChatMessage{
			{Role: "system", Content: "Cite sources."},
			{Role: "user", Content: "read README"},
		},
		Tools:      []models.ToolDefinition{{Function: models.ToolFunctionDefinition{Name: "read_file"}}},
		ToolChoice: "auto",
	}, func(event models.ChatEvent) error {
		if event.Token != "" {
			tokens = append(tokens, event.Token)
		}
		if len(event.ToolCalls) > 0 {
			toolCalls = append(toolCalls, event.ToolCalls...)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream() error = %v", err)
	}
	if request.Model != "small:2b" {
		t.Fatalf("model = %q, want small:2b", request.Model)
	}
	if !request.Stream {
		t.Fatal("stream = false, want true")
	}
	if len(request.Messages) != 2 || request.Messages[0].Role != "system" || request.Messages[0].Content != "Use tools safely.\n\nCite sources." {
		t.Fatalf("messages = %#v, want normalized system and user", request.Messages)
	}
	if len(request.Tools) != 1 || request.Tools[0].Type != "function" || request.Tools[0].Function.Name != "read_file" {
		t.Fatalf("tools = %#v, want normalized read_file tool", request.Tools)
	}
	if request.ToolChoice != "auto" {
		t.Fatalf("tool_choice = %q, want auto", request.ToolChoice)
	}
	if got := strings.Join(tokens, ""); got != "checking done" {
		t.Fatalf("tokens = %q, want checking done", got)
	}
	if len(toolCalls) != 1 || toolCalls[0].Function.Name != "read_file" || toolCalls[0].Function.Arguments["path"] != "README.md" {
		t.Fatalf("toolCalls = %#v, want read_file README.md", toolCalls)
	}
}

func TestChatStreamAllowsNilEmitter(t *testing.T) {
	runtime := New("http://ollama.test/api")
	runtime.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, "application/x-ndjson", `{"message":{"role":"assistant","content":"ok"},"done":false}`+"\n"+`{"done":true}`+"\n"), nil
	})}
	if err := runtime.ChatStream(context.Background(), models.ChatRequest{
		Model:    "small:2b",
		Messages: []models.ChatMessage{{Role: "user", Content: "hello"}},
	}, nil); err != nil {
		t.Fatalf("ChatStream(nil emit) error = %v", err)
	}
}

func TestChatStreamSendsThinkingAndGenerationOptions(t *testing.T) {
	think := false
	var request struct {
		Model   string         `json:"model"`
		Think   any            `json:"think"`
		Options map[string]any `json:"options"`
	}
	runtime := New("http://ollama.test/api")
	runtime.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("path = %q, want /api/chat", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		return jsonResponse(http.StatusOK, "application/x-ndjson", `{"message":{"role":"assistant","content":"ok"},"done":false}`+"\n"+`{"done":true}`+"\n"), nil
	})}

	err := runtime.ChatStream(context.Background(), models.ChatRequest{
		Model:       "qwen3:4b",
		Messages:    []models.ChatMessage{{Role: "user", Content: "hello"}},
		Think:       &think,
		NumPredict:  512,
		NumCtx:      4096,
		Temperature: 0.2,
	}, nil)
	if err != nil {
		t.Fatalf("ChatStream() error = %v", err)
	}
	if request.Think != false {
		t.Fatalf("think = %#v, want false", request.Think)
	}
	if got := request.Options["num_predict"]; got != float64(512) {
		t.Fatalf("num_predict = %#v, want 512", got)
	}
	if got := request.Options["num_ctx"]; got != float64(4096) {
		t.Fatalf("num_ctx = %#v, want 4096", got)
	}
}

func TestChatStreamSendsThinkingLevel(t *testing.T) {
	var request struct {
		Think any `json:"think"`
	}
	runtime := New("http://ollama.test/api")
	runtime.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		return jsonResponse(http.StatusOK, "application/x-ndjson", `{"message":{"role":"assistant","content":"ok"},"done":false}`+"\n"+`{"done":true}`+"\n"), nil
	})}

	err := runtime.ChatStream(context.Background(), models.ChatRequest{
		Model:         "thinking-level:test",
		Messages:      []models.ChatMessage{{Role: "user", Content: "hello"}},
		ThinkingLevel: "high",
	}, nil)
	if err != nil {
		t.Fatalf("ChatStream() error = %v", err)
	}
	if request.Think != "high" {
		t.Fatalf("think = %#v, want high", request.Think)
	}
}

func TestShowModelDecodesDetails(t *testing.T) {
	var seenModel string
	runtime := New("http://ollama.test/api")
	runtime.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/show" {
			t.Fatalf("path = %q, want /api/show", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		var input struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		seenModel = input.Model
		return jsonResponse(http.StatusOK, "application/json", `{
			"model": "small:2b",
			"modified_at": "2026-05-07T12:00:00Z",
			"size": 123456789,
			"digest": "sha256:abc",
			"modelfile": "FROM small",
			"parameters": "temperature 0.2",
			"template": "{{ .Prompt }}",
			"details": {
				"family": "qwen2",
				"format": "gguf",
				"parameter_size": "2B",
				"quantization_level": "Q4_K_M"
			},
			"model_info": {
				"qwen2.context_length": 32768
			}
		}`), nil
	})}

	details, err := runtime.ShowModel(context.Background(), "small:2b")
	if err != nil {
		t.Fatalf("ShowModel() error = %v", err)
	}
	if seenModel != "small:2b" {
		t.Fatalf("model = %q, want small:2b", seenModel)
	}
	if details.Name != "small:2b" {
		t.Fatalf("Name = %q, want small:2b", details.Name)
	}
	if details.Family != "qwen2" {
		t.Fatalf("Family = %q, want qwen2", details.Family)
	}
	if details.ParameterSize != "2B" {
		t.Fatalf("ParameterSize = %q, want 2B", details.ParameterSize)
	}
	if details.QuantizationLevel != "Q4_K_M" {
		t.Fatalf("QuantizationLevel = %q, want Q4_K_M", details.QuantizationLevel)
	}
	if details.ContextLength != 32768 {
		t.Fatalf("ContextLength = %d, want 32768", details.ContextLength)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func jsonResponse(status int, contentType string, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
