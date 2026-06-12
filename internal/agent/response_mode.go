package agent

import (
	"strings"

	"yemaka/internal/config"
	"yemaka/internal/models"
	promptcore "yemaka/internal/prompt"
	"yemaka/internal/routing"
)

const (
	responseModeFastNumPredict     = 512
	responseModeBalancedNumPredict = 1024
	responseModeDeepNumPredict     = 1536
	responseModeDefaultNumCtx      = 4096
	responseModeDeepNumCtx         = 8192
	responseModeMaxNumCtx          = 16384
	responseModeFastContextChars   = 6000
	responseModeFastLowMemChars    = 4500
)

type ModelResponseTrace struct {
	ConfiguredMode      string
	EffectiveMode       string
	ThinkingRequested   string
	ThinkingReturned    bool
	ShowThinkingTrace   bool
	NumPredict          int
	NumCtx              int
	UnsupportedThinking bool
}

type responseBehavior struct {
	ModelResponseTrace
	Think         *bool
	ThinkingLevel string
}

func (s *Service) responseBehavior(plan Plan, selected config.ModelConfig) responseBehavior {
	if s == nil || s.Router == nil {
		return responseBehaviorFromConfig(nil, plan, selected)
	}
	return responseBehaviorFromConfig(s.Router.Config, plan, selected)
}

func responseBehaviorFromConfig(cfg *config.Config, plan Plan, selected config.ModelConfig) responseBehavior {
	configured := config.ResponseModeBalanced
	showThinkingTrace := false
	maxContextTokens := responseModeDefaultNumCtx
	lowMemory := true
	if cfg != nil {
		configured = config.NormalizeResponseMode(cfg.Runtime.ResponseMode)
		showThinkingTrace = cfg.Runtime.ShowThinkingTrace
		if cfg.Runtime.MaxContextTokens > 0 {
			maxContextTokens = cfg.Runtime.MaxContextTokens
		}
		lowMemory = cfg.Runtime.LowMemoryMode
	}
	effective := configured
	if configured == config.ResponseModeAuto {
		effective = autoResponseMode(plan)
	}
	trace := ModelResponseTrace{
		ConfiguredMode:    configured,
		EffectiveMode:     effective,
		ShowThinkingTrace: showThinkingTrace,
	}
	behavior := responseBehavior{ModelResponseTrace: trace}
	switch effective {
	case config.ResponseModeFast:
		behavior.NumPredict = responseModeFastNumPredict
		behavior.NumCtx = minPositive(maxContextTokens, responseModeDefaultNumCtx)
		behavior.applyThinking(selected, false, "")
	case config.ResponseModeDeep:
		behavior.NumPredict = responseModeDeepNumPredict
		behavior.NumCtx = maxContextTokens
		if !lowMemory {
			behavior.NumCtx = maxPositive(behavior.NumCtx, responseModeDeepNumCtx)
		}
		behavior.NumCtx = minPositive(behavior.NumCtx, responseModeMaxNumCtx)
		behavior.applyThinking(selected, true, "high")
	default:
		behavior.NumPredict = responseModeBalancedNumPredict
		behavior.NumCtx = maxContextTokens
		behavior.applyBalancedThinkingLevel(selected)
	}
	return behavior
}

func (behavior *responseBehavior) applyThinking(selected config.ModelConfig, boolValue bool, levelValue string) {
	if behavior == nil {
		return
	}
	switch ollamaThinkingSupport(selected) {
	case "bool":
		value := boolValue
		behavior.Think = &value
		behavior.ThinkingRequested = strings.ToLower(strings.TrimSpace(strconvBool(value)))
	case "level":
		behavior.ThinkingLevel = strings.TrimSpace(levelValue)
		if behavior.ThinkingLevel == "" {
			behavior.ThinkingLevel = "medium"
		}
		if !boolValue {
			behavior.ThinkingLevel = "low"
		}
		behavior.ThinkingRequested = behavior.ThinkingLevel
	default:
		behavior.UnsupportedThinking = true
		behavior.ThinkingRequested = "unsupported"
	}
}

func (behavior *responseBehavior) applyBalancedThinkingLevel(selected config.ModelConfig) {
	if behavior == nil || ollamaThinkingSupport(selected) != "level" {
		return
	}
	behavior.ThinkingLevel = "medium"
	behavior.ThinkingRequested = "medium"
}

func autoResponseMode(plan Plan) string {
	if plan.RouteLane == routing.ToolLaneChat || plan.RouteCategory == routing.RouteChatExplanation {
		if plan.RiskLevel == "" || plan.RiskLevel == RiskLow {
			return config.ResponseModeFast
		}
	}
	if plan.TaskType == TaskCoding || plan.ModelTask == models.TaskCoding ||
		plan.RouteUsesInternet || plan.RouteGeneratesExtension || plan.RouteCreatesSchedulerJob ||
		plan.RouteCrawlerTask || plan.RouteConnectorAction || plan.RouteWritesFiles ||
		plan.RouteLane == routing.ToolLaneCoding || plan.RouteLane == routing.ToolLaneWebSearch ||
		plan.RouteLane == routing.ToolLaneWebCrawl || plan.RouteLane == routing.ToolLaneExtension ||
		plan.RouteLane == routing.ToolLaneScheduler || plan.RouteLane == routing.ToolLaneConnector ||
		plan.RiskLevel == RiskHigh {
		return config.ResponseModeDeep
	}
	return config.ResponseModeBalanced
}

func (behavior responseBehavior) applyPromptOptions(options promptcore.Options) promptcore.Options {
	switch behavior.EffectiveMode {
	case config.ResponseModeFast:
		limit := responseModeFastContextChars
		if options.LowMemory {
			limit = responseModeFastLowMemChars
		}
		if options.MaxChars == 0 || options.MaxChars > limit {
			options.MaxChars = limit
		}
	}
	return options
}

func (behavior responseBehavior) applyChatRequest(req models.ChatRequest) models.ChatRequest {
	req.ResponseMode = behavior.EffectiveMode
	req.Think = behavior.Think
	req.ThinkingLevel = behavior.ThinkingLevel
	req.NumPredict = behavior.NumPredict
	req.NumCtx = behavior.NumCtx
	return req
}

func ollamaThinkingSupport(selected config.ModelConfig) string {
	if models.NormalizeProvider(selected.Provider) != models.ProviderOllama {
		return ""
	}
	name := strings.ToLower(strings.TrimSpace(selected.Name))
	switch {
	case strings.Contains(name, "thinking-level"):
		return "level"
	case strings.Contains(name, "qwen3"), strings.Contains(name, "deepseek-r1"), strings.Contains(name, "gpt-oss"):
		return "bool"
	default:
		return ""
	}
}

func strconvBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func minPositive(value int, limit int) int {
	if value <= 0 {
		return limit
	}
	if limit <= 0 || value < limit {
		return value
	}
	return limit
}

func maxPositive(value int, minimum int) int {
	if value <= 0 {
		return minimum
	}
	if value < minimum {
		return minimum
	}
	return value
}

func containsModelThinking(content string) bool {
	lower := strings.ToLower(content)
	for _, tag := range modelThinkingOpenTags {
		if strings.Contains(lower, tag) {
			return true
		}
	}
	return false
}
