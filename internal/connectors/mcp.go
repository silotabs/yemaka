package connectors

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"yemaka/internal/config"
)

type CoreResult struct {
	ConversationID string   `json:"conversationId,omitempty"`
	Text           string   `json:"text"`
	Model          string   `json:"model,omitempty"`
	Skill          string   `json:"skill,omitempty"`
	Sources        []string `json:"sources,omitempty"`
	SourceKind     string   `json:"sourceKind,omitempty"`
}

type MemoryResult struct {
	ID         string `json:"id,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Content    string `json:"content,omitempty"`
	Snippet    string `json:"snippet,omitempty"`
	Importance int    `json:"importance,omitempty"`
	Source     string `json:"source,omitempty"`
	Pinned     bool   `json:"pinned,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
}

type MCPHandlers struct {
	Chat         func(context.Context, string) (CoreResult, error)
	Ask          func(context.Context, string, string) (CoreResult, error)
	MemorySearch func(context.Context, string, int) ([]MemoryResult, error)
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *mcpError `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func ServeMCP(ctx context.Context, cfg config.ConnectorsConfig, input io.Reader, output io.Writer, handlers MCPHandlers) error {
	status := MCPServerStatus(cfg)
	if !status.Enabled {
		return fmt.Errorf("mcp_server connector is disabled")
	}
	if handlers.Chat == nil || handlers.Ask == nil || handlers.MemorySearch == nil {
		return fmt.Errorf("mcp_server handlers are incomplete")
	}
	reader := bufio.NewReader(input)
	limit := int(status.MaxBodyBytes)
	if limit <= 0 {
		limit = 65536
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		body, err := readMCPFrame(reader, limit)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var request mcpRequest
		if err := json.Unmarshal(body, &request); err != nil {
			if err := writeMCPResponse(output, mcpResponse{
				JSONRPC: "2.0",
				Error:   &mcpError{Code: -32700, Message: "parse error"},
			}); err != nil {
				return err
			}
			continue
		}
		if request.ID == nil {
			continue
		}
		response := handleMCPRequest(ctx, request, handlers)
		if err := writeMCPResponse(output, response); err != nil {
			return err
		}
	}
}

func handleMCPRequest(ctx context.Context, request mcpRequest, handlers MCPHandlers) mcpResponse {
	response := mcpResponse{JSONRPC: "2.0", ID: request.ID}
	switch request.Method {
	case "initialize":
		response.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "yemaka",
				"version": "1.0-rc",
			},
		}
	case "ping":
		response.Result = map[string]any{}
	case "tools/list":
		response.Result = map[string]any{"tools": MCPTools()}
	case "tools/call":
		result, err := callMCPTool(ctx, request.Params, handlers)
		if err != nil {
			response.Result = toolCallResult(err.Error(), true)
			return response
		}
		response.Result = toolCallResult(result, false)
	default:
		response.Error = &mcpError{Code: -32601, Message: "method not found: " + request.Method}
	}
	return response
}

func callMCPTool(ctx context.Context, raw json.RawMessage, handlers MCPHandlers) (string, error) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return "", fmt.Errorf("invalid tools/call params")
	}
	switch params.Name {
	case "yemaka_chat":
		content := stringArgument(params.Arguments, "content")
		if content == "" {
			return "", fmt.Errorf("content is required")
		}
		result, err := handlers.Chat(ctx, content)
		if err != nil {
			return "", err
		}
		return result.Text, nil
	case "yemaka_ask":
		content := stringArgument(params.Arguments, "content")
		if content == "" {
			return "", fmt.Errorf("content is required")
		}
		result, err := handlers.Ask(ctx, content, stringArgument(params.Arguments, "skill"))
		if err != nil {
			return "", err
		}
		return result.Text, nil
	case "yemaka_memory_search":
		query := stringArgument(params.Arguments, "query")
		if query == "" {
			return "", fmt.Errorf("query is required")
		}
		limit := intArgument(params.Arguments, "limit", 5)
		items, err := handlers.MemorySearch(ctx, query, limit)
		if err != nil {
			return "", err
		}
		data, err := json.Marshal(items)
		if err != nil {
			return "", err
		}
		return string(data), nil
	default:
		return "", fmt.Errorf("unknown tool: %s", params.Name)
	}
}

func toolCallResult(text string, isError bool) map[string]any {
	return map[string]any{
		"content": []map[string]string{
			{"type": "text", "text": text},
		},
		"isError": isError,
	}
}

func readMCPFrame(reader *bufio.Reader, limit int) ([]byte, error) {
	contentLength := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(key), "content-length") {
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("invalid MCP content length")
			}
			contentLength = parsed
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("MCP content length is missing")
	}
	if contentLength > limit {
		return nil, fmt.Errorf("MCP message exceeds max frame bytes")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return body, nil
}

func writeMCPResponse(output io.Writer, response mcpResponse) error {
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	_, err = output.Write(data)
	return err
}

func stringArgument(args map[string]any, name string) string {
	value, ok := args[name]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func intArgument(args map[string]any, name string, fallback int) int {
	value, ok := args[name]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		if typed >= 1 && typed <= 20 {
			return int(typed)
		}
	case int:
		if typed >= 1 && typed <= 20 {
			return typed
		}
	}
	return fallback
}
