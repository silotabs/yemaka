package models

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"yemaka/internal/config"
)

const (
	TaskChat      = "chat"
	TaskLowMemory = "low_memory"
	TaskCoding    = "coding"
	TaskReasoning = "reasoning"
	TaskRAG       = "rag"
	TaskTool      = "tool"

	ProviderOllama           = "ollama"
	ProviderLlamaCpp         = "llamacpp"
	ProviderOpenAICompatible = "openai_compatible"
	ProviderLiteLLM          = "litellm"
)

type ChatMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolName  string     `json:"name,omitempty"`
}

type ChatRequest struct {
	Model         string
	System        string
	Messages      []ChatMessage
	Temperature   float64
	Tools         []ToolDefinition
	ToolChoice    string
	ResponseMode  string
	Think         *bool
	ThinkingLevel string
	NumPredict    int
	NumCtx        int
}

type ChatEvent struct {
	Token     string
	Done      bool
	ToolCalls []ToolCall
}

type ToolDefinition struct {
	Type     string                 `json:"type,omitempty"`
	Function ToolFunctionDefinition `json:"function,omitempty"`
}

type ToolFunctionDefinition struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function ToolCallFunction `json:"function,omitempty"`
}

type ToolCallFunction struct {
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

func (f *ToolCallFunction) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	f.Name = raw.Name
	f.Arguments = nil
	if len(raw.Arguments) == 0 || string(raw.Arguments) == "null" {
		return nil
	}
	var args map[string]any
	if err := json.Unmarshal(raw.Arguments, &args); err == nil {
		f.Arguments = args
		return nil
	}
	var encoded string
	if err := json.Unmarshal(raw.Arguments, &encoded); err != nil {
		return err
	}
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(encoded), &args); err == nil {
		f.Arguments = args
		return nil
	}
	f.Arguments = map[string]any{"_raw": encoded}
	return nil
}

type GenerateRequest struct {
	Model       string
	Prompt      string
	Temperature float64
}

type GenerateEvent struct {
	Token string
	Done  bool
}

type PullEvent struct {
	Status    string
	Digest    string
	Total     int64
	Completed int64
	Done      bool
}

type EmbeddingRequest struct {
	Model string
	Input []string
}

type EmbeddingResponse struct {
	Embeddings [][]float64
}

type ModelInfo struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modifiedAt"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
}

type ModelDetails struct {
	Name              string         `json:"name"`
	ModifiedAt        string         `json:"modifiedAt"`
	Size              int64          `json:"size"`
	Digest            string         `json:"digest"`
	Family            string         `json:"family"`
	Format            string         `json:"format"`
	ParameterSize     string         `json:"parameterSize"`
	QuantizationLevel string         `json:"quantizationLevel"`
	ContextLength     int            `json:"contextLength"`
	Template          string         `json:"template"`
	System            string         `json:"system"`
	License           string         `json:"license"`
	Parameters        string         `json:"parameters"`
	Modelfile         string         `json:"modelfile"`
	Details           map[string]any `json:"details"`
	ModelInfo         map[string]any `json:"modelInfo"`
}

type Runtime interface {
	Health(ctx context.Context) error
	ListModels(ctx context.Context) ([]ModelInfo, error)
	PullModel(ctx context.Context, name string, emit func(PullEvent) error) error
	ChatStream(ctx context.Context, req ChatRequest, emit func(ChatEvent) error) error
}

type GenerateRuntime interface {
	GenerateStream(ctx context.Context, req GenerateRequest, emit func(GenerateEvent) error) error
}

type ModelDetailRuntime interface {
	ShowModel(ctx context.Context, name string) (ModelDetails, error)
}

type EmbeddingRuntime interface {
	Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
}

type Router struct {
	Config *config.Config
}

func NewRouter(cfg *config.Config) *Router {
	return &Router{Config: cfg}
}

func NormalizeChatRequest(req ChatRequest) ChatRequest {
	normalized := req
	normalized.Model = strings.TrimSpace(req.Model)
	normalized.System = strings.TrimSpace(req.System)
	normalized.ToolChoice = strings.TrimSpace(req.ToolChoice)
	normalized.ResponseMode = strings.TrimSpace(req.ResponseMode)
	normalized.ThinkingLevel = strings.TrimSpace(req.ThinkingLevel)
	normalized.Tools = normalizeToolDefinitions(req.Tools)

	systemParts := make([]string, 0, 1)
	if normalized.System != "" {
		systemParts = append(systemParts, normalized.System)
	}
	messages := make([]ChatMessage, 0, len(req.Messages)+1)
	for _, message := range req.Messages {
		role := normalizeChatRole(message.Role)
		content := strings.TrimSpace(message.Content)
		toolName := strings.TrimSpace(message.ToolName)
		toolCalls := normalizeToolCalls(message.ToolCalls)
		if role == "system" {
			if content != "" {
				systemParts = append(systemParts, content)
			}
			continue
		}
		if content == "" && len(toolCalls) == 0 {
			continue
		}
		messages = append(messages, ChatMessage{
			Role:      role,
			Content:   content,
			ToolCalls: toolCalls,
			ToolName:  toolName,
		})
	}
	if len(systemParts) > 0 {
		system := strings.Join(systemParts, "\n\n")
		messages = append([]ChatMessage{{Role: "system", Content: system}}, messages...)
		normalized.System = system
	}
	normalized.Messages = messages
	return normalized
}

func normalizeChatRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "system":
		return "system"
	case "assistant", "model":
		return "assistant"
	case "tool", "function":
		return "tool"
	case "user", "":
		return "user"
	default:
		return "user"
	}
}

func normalizeToolDefinitions(tools []ToolDefinition) []ToolDefinition {
	normalized := make([]ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		tool.Type = strings.TrimSpace(tool.Type)
		if tool.Type == "" {
			tool.Type = "function"
		}
		tool.Function.Name = strings.TrimSpace(tool.Function.Name)
		tool.Function.Description = strings.TrimSpace(tool.Function.Description)
		if tool.Function.Name == "" {
			continue
		}
		normalized = append(normalized, tool)
	}
	return normalized
}

func normalizeToolCalls(calls []ToolCall) []ToolCall {
	normalized := make([]ToolCall, 0, len(calls))
	for _, call := range calls {
		call.ID = strings.TrimSpace(call.ID)
		call.Type = strings.TrimSpace(call.Type)
		if call.Type == "" {
			call.Type = "function"
		}
		call.Function.Name = strings.TrimSpace(call.Function.Name)
		if call.Function.Name == "" {
			continue
		}
		normalized = append(normalized, call)
	}
	return normalized
}

func (r *Router) Select(taskType string) (config.ModelConfig, error) {
	model, _, err := r.SelectWithRole(taskType)
	return model, err
}

func (r *Router) SelectWithRole(taskType string) (config.ModelConfig, string, error) {
	if r == nil || r.Config == nil {
		return config.ModelConfig{}, "", fmt.Errorf("model router has no config")
	}
	taskType = strings.TrimSpace(taskType)
	if taskType == "" {
		taskType = TaskChat
	}

	if r.Config.Runtime.LowMemoryMode && lowMemoryTask(taskType) {
		if model, ok := r.Config.Models["low_memory"]; ok && model.Name != "" {
			return model, "low_memory", nil
		}
	}

	if model, ok := r.Config.Models[taskType]; ok && model.Name != "" {
		return model, taskType, nil
	}

	if model, ok := r.Config.Models["default"]; ok && model.Name != "" {
		return model, "default", nil
	}

	return config.ModelConfig{}, "", fmt.Errorf("no default model configured")
}

func lowMemoryTask(taskType string) bool {
	switch taskType {
	case TaskChat, TaskLowMemory, TaskRAG, TaskTool:
		return true
	default:
		return false
	}
}

func NormalizeProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", ProviderOllama:
		return ProviderOllama
	case "llama.cpp", "llama_cpp", ProviderLlamaCpp:
		return ProviderLlamaCpp
	case "openai-compatible", "openai", ProviderOpenAICompatible:
		return ProviderOpenAICompatible
	case "lite_llm", ProviderLiteLLM:
		return ProviderLiteLLM
	default:
		return strings.ToLower(strings.TrimSpace(provider))
	}
}

func IsRuntimeProvider(provider string) bool {
	switch NormalizeProvider(provider) {
	case ProviderOllama, ProviderLlamaCpp, ProviderOpenAICompatible:
		return true
	default:
		return false
	}
}

func SupportedRuntimeProviders() []string {
	return []string{ProviderOllama, ProviderLlamaCpp, ProviderOpenAICompatible}
}

func ModelInstalled(name string, installed []ModelInfo) bool {
	target := strings.TrimSpace(name)
	if target == "" {
		return false
	}
	for _, model := range installed {
		if strings.EqualFold(strings.TrimSpace(model.Name), target) {
			return true
		}
	}
	return false
}

func LocalInstalledModels(installed []ModelInfo) []ModelInfo {
	local := make([]ModelInfo, 0, len(installed))
	for _, model := range installed {
		name := strings.TrimSpace(model.Name)
		if name == "" || isCloudPlaceholder(name) {
			continue
		}
		local = append(local, model)
	}
	return local
}

func isCloudPlaceholder(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return lower == "" || strings.Contains(lower, ":cloud")
}
