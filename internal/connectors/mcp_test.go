package connectors

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"yemaka/internal/config"
)

func TestServeMCPRequiresExplicitEnable(t *testing.T) {
	cfg := config.Default().Connectors
	err := ServeMCP(context.Background(), cfg, strings.NewReader(""), &bytes.Buffer{}, MCPHandlers{})
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("ServeMCP() error = %v, want disabled error", err)
	}
}

func TestServeMCPListsAndCallsTools(t *testing.T) {
	cfg := config.Default()
	if err := EnableMCPServer(cfg); err != nil {
		t.Fatalf("EnableMCPServer() error = %v", err)
	}
	input := strings.Join([]string{
		mcpFrame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`),
		mcpFrame(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`),
		mcpFrame(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"yemaka_chat","arguments":{"content":"hello"}}}`),
	}, "")
	output := &bytes.Buffer{}
	err := ServeMCP(context.Background(), cfg.Connectors, strings.NewReader(input), output, MCPHandlers{
		Chat: func(ctx context.Context, content string) (CoreResult, error) {
			return CoreResult{Text: "ok: " + content, Model: "small:2b"}, nil
		},
		Ask: func(ctx context.Context, content string, skill string) (CoreResult, error) {
			return CoreResult{Text: "ask: " + content, Skill: skill}, nil
		},
		MemorySearch: func(ctx context.Context, query string, limit int) ([]MemoryResult, error) {
			return []MemoryResult{{ID: "mem_1", Snippet: query}}, nil
		},
	})
	if err != nil {
		t.Fatalf("ServeMCP() error = %v", err)
	}
	text := output.String()
	if !strings.Contains(text, `"yemaka_chat"`) {
		t.Fatalf("MCP output missing chat tool: %s", text)
	}
	if !strings.Contains(text, `ok: hello`) {
		t.Fatalf("MCP output missing chat result: %s", text)
	}
}

func mcpFrame(body string) string {
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}
