package agent

import (
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/models"
	promptcore "yemaka/internal/prompt"
	"yemaka/internal/replay"
	"yemaka/internal/routing"
)

func TestResponseBehaviorFastRequestsThinkingFalseForSupportedOllama(t *testing.T) {
	cfg := config.Default()
	cfg.Runtime.ResponseMode = config.ResponseModeFast
	selected := config.ModelConfig{Provider: models.ProviderOllama, Name: "qwen3:4b"}

	behavior := responseBehaviorFromConfig(cfg, Plan{}, selected)
	if behavior.EffectiveMode != config.ResponseModeFast {
		t.Fatalf("EffectiveMode = %q, want fast", behavior.EffectiveMode)
	}
	if behavior.Think == nil || *behavior.Think {
		t.Fatalf("Think = %v, want false", behavior.Think)
	}
	if behavior.ThinkingRequested != "false" {
		t.Fatalf("ThinkingRequested = %q, want false", behavior.ThinkingRequested)
	}
	if behavior.NumPredict != responseModeFastNumPredict {
		t.Fatalf("NumPredict = %d, want %d", behavior.NumPredict, responseModeFastNumPredict)
	}
}

func TestResponseBehaviorDeepRequestsThinkingTrueForSupportedOllama(t *testing.T) {
	cfg := config.Default()
	cfg.Runtime.ResponseMode = config.ResponseModeDeep
	selected := config.ModelConfig{Provider: models.ProviderOllama, Name: "deepseek-r1:7b"}

	behavior := responseBehaviorFromConfig(cfg, Plan{}, selected)
	if behavior.EffectiveMode != config.ResponseModeDeep {
		t.Fatalf("EffectiveMode = %q, want deep", behavior.EffectiveMode)
	}
	if behavior.Think == nil || !*behavior.Think {
		t.Fatalf("Think = %v, want true", behavior.Think)
	}
	if behavior.ThinkingRequested != "true" {
		t.Fatalf("ThinkingRequested = %q, want true", behavior.ThinkingRequested)
	}
	if behavior.NumPredict != responseModeDeepNumPredict {
		t.Fatalf("NumPredict = %d, want %d", behavior.NumPredict, responseModeDeepNumPredict)
	}
}

func TestResponseBehaviorAutoMapsSimpleChatToFast(t *testing.T) {
	cfg := config.Default()
	cfg.Runtime.ResponseMode = config.ResponseModeAuto
	selected := config.ModelConfig{Provider: models.ProviderOllama, Name: "qwen3:4b"}

	behavior := responseBehaviorFromConfig(cfg, Plan{
		RouteCategory: routing.RouteChatExplanation,
		RouteLane:     routing.ToolLaneChat,
		RiskLevel:     RiskLow,
	}, selected)
	if behavior.ConfiguredMode != config.ResponseModeAuto {
		t.Fatalf("ConfiguredMode = %q, want auto", behavior.ConfiguredMode)
	}
	if behavior.EffectiveMode != config.ResponseModeFast {
		t.Fatalf("EffectiveMode = %q, want fast", behavior.EffectiveMode)
	}
}

func TestResponseBehaviorAutoMapsExtensionRouteToDeep(t *testing.T) {
	cfg := config.Default()
	cfg.Runtime.ResponseMode = config.ResponseModeAuto
	selected := config.ModelConfig{Provider: models.ProviderOllama, Name: "qwen3:4b"}

	behavior := responseBehaviorFromConfig(cfg, Plan{
		RouteCategory:           routing.RouteExtensionGenerate,
		RouteLane:               routing.ToolLaneExtension,
		RouteGeneratesExtension: true,
		RiskLevel:               RiskMedium,
	}, selected)
	if behavior.EffectiveMode != config.ResponseModeDeep {
		t.Fatalf("EffectiveMode = %q, want deep", behavior.EffectiveMode)
	}
}

func TestResponseBehaviorUnsupportedThinkingModelDoesNotSetThinkField(t *testing.T) {
	cfg := config.Default()
	cfg.Runtime.ResponseMode = config.ResponseModeDeep
	selected := config.ModelConfig{Provider: models.ProviderOllama, Name: "llama3.2:3b"}

	behavior := responseBehaviorFromConfig(cfg, Plan{}, selected)
	if behavior.Think != nil {
		t.Fatalf("Think = %v, want nil for unsupported thinking model", behavior.Think)
	}
	if behavior.ThinkingLevel != "" {
		t.Fatalf("ThinkingLevel = %q, want empty", behavior.ThinkingLevel)
	}
	if !behavior.UnsupportedThinking {
		t.Fatal("UnsupportedThinking = false, want true")
	}

	req := behavior.applyChatRequest(models.ChatRequest{Model: selected.Name})
	if req.Think != nil || req.ThinkingLevel != "" {
		t.Fatalf("request thinking fields = %v/%q, want unset", req.Think, req.ThinkingLevel)
	}
}

func TestResponseBehaviorBalancedUsesMediumForThinkingLevelModels(t *testing.T) {
	cfg := config.Default()
	cfg.Runtime.ResponseMode = config.ResponseModeBalanced
	selected := config.ModelConfig{Provider: models.ProviderOllama, Name: "thinking-level:test"}

	behavior := responseBehaviorFromConfig(cfg, Plan{}, selected)
	if behavior.Think != nil {
		t.Fatalf("Think = %v, want nil for level-based thinking model", behavior.Think)
	}
	if behavior.ThinkingLevel != "medium" {
		t.Fatalf("ThinkingLevel = %q, want medium", behavior.ThinkingLevel)
	}
	if behavior.ThinkingRequested != "medium" {
		t.Fatalf("ThinkingRequested = %q, want medium", behavior.ThinkingRequested)
	}
}

func TestResponseBehaviorFastTightensPromptBudget(t *testing.T) {
	cfg := config.Default()
	cfg.Runtime.ResponseMode = config.ResponseModeFast
	cfg.Runtime.LowMemoryMode = true

	behavior := responseBehaviorFromConfig(cfg, Plan{}, config.ModelConfig{Provider: models.ProviderOllama, Name: "qwen3:4b"})
	options := behavior.applyPromptOptions(promptcore.Options{MaxChars: 12000, LowMemory: true})
	if options.MaxChars != responseModeFastLowMemChars {
		t.Fatalf("MaxChars = %d, want %d", options.MaxChars, responseModeFastLowMemChars)
	}
}

func TestReplayTraceRecordsResponseModeMetadata(t *testing.T) {
	trace := replayTraceFromAgentRun(agentReplayTraceInput{
		ConversationID:     "conv_response_mode",
		UserMessageID:      "msg_user",
		AssistantMessageID: "msg_assistant",
		Input:              PlanInput{Content: "hello"},
		Plan: Plan{
			RouteCategory: routing.RouteChatExplanation,
			RouteLane:     routing.ToolLaneChat,
			RiskLevel:     RiskLow,
		},
		AssistantContent: "hello",
		ModelResponse: ModelResponseTrace{
			ConfiguredMode:      config.ResponseModeAuto,
			EffectiveMode:       config.ResponseModeFast,
			ThinkingRequested:   "false",
			ThinkingReturned:    true,
			ShowThinkingTrace:   false,
			NumPredict:          responseModeFastNumPredict,
			UnsupportedThinking: false,
		},
	})

	assertReplayAttribute(t, trace.Attributes, "response_mode", config.ResponseModeFast)
	assertReplayAttribute(t, trace.Attributes, "response_mode_configured", config.ResponseModeAuto)
	assertReplayAttribute(t, trace.Attributes, "thinking_requested", "false")
	assertReplayAttribute(t, trace.Attributes, "thinking_returned", "true")
	assertReplayAttribute(t, trace.Attributes, "show_thinking_trace", "false")
}

func assertReplayAttribute(t *testing.T, attributes []replay.Attribute, key string, want string) {
	t.Helper()
	for _, attribute := range attributes {
		if attribute.Key == key {
			if attribute.Value != want {
				t.Fatalf("%s = %q, want %q", key, attribute.Value, want)
			}
			return
		}
	}
	t.Fatalf("missing replay attribute %q in %+v", key, attributes)
}
