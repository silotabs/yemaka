package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"yemaka/internal/models"
	"yemaka/internal/routing"
)

const routingAdvisorTimeout = 8 * time.Second

type routingAdvisory struct {
	RouteCategory         string `json:"route_category"`
	SourceOfTruth         string `json:"source_of_truth"`
	Target                string `json:"target"`
	NeedsClarification    bool   `json:"needs_clarification"`
	ClarificationQuestion string `json:"clarification_question"`
	Confidence            int    `json:"confidence"`
	Reason                string `json:"reason"`
}

func (s *Service) applyRoutingAdvisor(ctx context.Context, input PlanInput, plan Plan, emit EventHandler) (Plan, error) {
	if !shouldRunRoutingAdvisor(plan) {
		return plan, nil
	}
	selected, runtime, ok := s.routingAdvisorRuntime()
	if !ok {
		return plan, nil
	}
	advice, raw, err := runRoutingAdvisor(ctx, runtime, selected.Name, input, plan)
	if err != nil {
		if emitErr := emitRoutingAdvisoryEvent(emit, selected.Provider, selected.Name, routingAdvisory{
			Reason: err.Error(),
		}, "failed", false); emitErr != nil {
			return plan, emitErr
		}
		return plan, nil
	}
	if raw != "" {
		advice.Reason = firstNonEmptyAdvisorString(advice.Reason, "model returned routing advice")
	}

	advice = normalizeRoutingAdvisory(advice)
	accepted := false
	if plan.NeedsClarification && routingAdvisorAsksClarification(advice) {
		if question := sanitizeRoutingAdvisorQuestion(advice.ClarificationQuestion); question != "" {
			plan.ClarificationQuestion = question
			accepted = true
		}
		plan.RouteReasons = appendNonDuplicate(plan.RouteReasons, "routing advisor confirmed clarification")
		accepted = true
	} else if advice.RouteCategory != "" {
		plan.RouteReasons = appendNonDuplicate(plan.RouteReasons, "routing advisor observed "+advice.RouteCategory)
	}

	status := "observed"
	if accepted {
		status = "accepted"
	}
	if err := emitRoutingAdvisoryEvent(emit, selected.Provider, selected.Name, advice, status, accepted); err != nil {
		return plan, err
	}
	return plan, nil
}

func shouldRunRoutingAdvisor(plan Plan) bool {
	if plan.RiskLevel == RiskHigh {
		return false
	}
	if plan.RouteRequiresApproval ||
		plan.RouteWritesFiles ||
		plan.RouteRunsShell ||
		plan.RouteGeneratesExtension ||
		plan.RouteCreatesSchedulerJob ||
		plan.RouteConnectorAction ||
		plan.RouteCrawlerTask {
		return false
	}
	switch plan.RouteCategory {
	case routing.RouteAuthorizationRequired,
		routing.RouteActiveAssessmentRequiresScope,
		routing.RouteClarifyScope,
		routing.RoutePermissionRequired,
		routing.RouteApprovalRequired,
		routing.RouteSafeAlternativeOffer,
		routing.RoutePassiveCheck,
		routing.RouteFileWrite,
		routing.RouteShellTool,
		routing.RouteExtensionGenerate,
		routing.RouteSchedulerCreate,
		routing.RouteConnectorAction,
		routing.RouteCrawlerTask:
		return false
	}
	if plan.NeedsClarification && plan.RouteConfidence <= 55 {
		return true
	}
	return false
}

func (s *Service) routingAdvisorRuntime() (providerModel, models.Runtime, bool) {
	if s == nil || s.Router == nil {
		return providerModel{}, nil, false
	}
	selected, err := s.Router.Select(models.TaskLowMemory)
	if err != nil {
		return providerModel{}, nil, false
	}
	selected.Provider = models.NormalizeProvider(selected.Provider)
	if !models.IsRuntimeProvider(selected.Provider) {
		return providerModel{}, nil, false
	}
	runtime, err := s.runtimeForModel(selected)
	if err != nil || runtime == nil {
		return providerModel{}, nil, false
	}
	return providerModel{Provider: selected.Provider, Name: selected.Name}, runtime, true
}

type providerModel struct {
	Provider string
	Name     string
}

func runRoutingAdvisor(ctx context.Context, runtime models.Runtime, model string, input PlanInput, plan Plan) (routingAdvisory, string, error) {
	if runtime == nil {
		return routingAdvisory{}, "", fmt.Errorf("routing advisor runtime is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, routingAdvisorTimeout)
	defer cancel()

	var content string
	err := runtime.ChatStream(ctx, models.ChatRequest{
		Model:       model,
		Messages:    routingAdvisorMessages(input, plan),
		Temperature: 0,
	}, func(event models.ChatEvent) error {
		if event.Token != "" {
			content += event.Token
		}
		return nil
	})
	if err != nil {
		return routingAdvisory{}, content, err
	}
	content = StripModelThinkingBlocks(content)
	advice, ok := parseRoutingAdvisory(content)
	if !ok {
		return routingAdvisory{}, content, fmt.Errorf("routing advisor returned no valid JSON")
	}
	return advice, content, nil
}

func routingAdvisorMessages(input PlanInput, plan Plan) []models.ChatMessage {
	return []models.ChatMessage{
		{
			Role: "system",
			Content: strings.Join([]string{
				"You are Yemaka's routing advisor.",
				"Output exactly one JSON object and no prose.",
				"Do not execute tools. Do not request tools. Do not override safety policy.",
				"Allowed route_category values: clarify, chat_explanation, rag_search, memory_search, workspace_read, internet_search, local_time.",
				"If the target, source, or action is unclear, choose clarify and write one short user-facing question.",
			}, " "),
		},
		{
			Role:    "user",
			Content: routingAdvisorPrompt(input, plan),
		},
	}
}

func routingAdvisorPrompt(input PlanInput, plan Plan) string {
	lines := []string{
		"Prompt:",
		strings.TrimSpace(input.Content),
		"",
		"Deterministic route:",
		"task_type: " + plan.TaskType,
		"route_category: " + plan.RouteCategory,
		fmt.Sprintf("route_confidence: %d", plan.RouteConfidence),
		"needs_clarification: " + fmt.Sprintf("%t", plan.NeedsClarification),
		"clarification_question: " + plan.ClarificationQuestion,
		"route_reasons: " + strings.Join(plan.RouteReasons, ", "),
	}
	if plan.RoutePreflight != nil {
		lines = append(lines,
			"",
			"Preflight:",
			"source_of_truth: "+plan.RoutePreflight.SourceOfTruth,
			"target: "+plan.RoutePreflight.Target,
			"target_source: "+plan.RoutePreflight.TargetSource,
			"missing_slots: "+strings.Join(plan.RoutePreflight.MissingSlots, ", "),
			"conflict_signals: "+strings.Join(plan.RoutePreflight.ConflictSignals, ", "),
		)
	}
	lines = append(lines,
		"",
		"Return JSON fields: route_category, source_of_truth, target, needs_clarification, clarification_question, confidence, reason.",
	)
	return strings.Join(lines, "\n")
}

func parseRoutingAdvisory(content string) (routingAdvisory, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return routingAdvisory{}, false
	}
	if strings.HasPrefix(content, "```") {
		content = strings.TrimSpace(strings.Trim(content, "`"))
		if strings.HasPrefix(strings.ToLower(content), "json") {
			content = strings.TrimSpace(content[4:])
		}
	}
	jsonText := firstJSONObject(content)
	if jsonText == "" {
		return routingAdvisory{}, false
	}
	var advice routingAdvisory
	if err := json.Unmarshal([]byte(jsonText), &advice); err != nil {
		return routingAdvisory{}, false
	}
	return advice, true
}

func firstJSONObject(content string) string {
	start := strings.Index(content, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(content); i++ {
		ch := content[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[start : i+1]
			}
		}
	}
	return ""
}

func normalizeRoutingAdvisory(advice routingAdvisory) routingAdvisory {
	advice.RouteCategory = strings.ToLower(strings.TrimSpace(advice.RouteCategory))
	advice.SourceOfTruth = strings.ToLower(strings.TrimSpace(advice.SourceOfTruth))
	advice.Target = strings.TrimSpace(advice.Target)
	advice.ClarificationQuestion = sanitizeRoutingAdvisorQuestion(advice.ClarificationQuestion)
	advice.Reason = strings.Join(strings.Fields(advice.Reason), " ")
	if advice.Confidence < 0 {
		advice.Confidence = 0
	}
	if advice.Confidence > 100 {
		advice.Confidence = 100
	}
	if !routingAdvisorAllowedRoute(advice.RouteCategory) {
		advice.RouteCategory = ""
	}
	if advice.RouteCategory == routing.RouteClarify {
		advice.NeedsClarification = true
	}
	return advice
}

func routingAdvisorAllowedRoute(routeCategory string) bool {
	switch routeCategory {
	case routing.RouteClarify,
		routing.RouteChatExplanation,
		routing.RouteRAGSearch,
		routing.RouteMemorySearch,
		routing.RouteWorkspaceRead,
		routing.RouteInternetSearch,
		routing.RouteLocalTime:
		return true
	default:
		return false
	}
}

func routingAdvisorAsksClarification(advice routingAdvisory) bool {
	return advice.NeedsClarification || advice.RouteCategory == routing.RouteClarify
}

func sanitizeRoutingAdvisorQuestion(question string) string {
	question = strings.Join(strings.Fields(question), " ")
	if question == "" {
		return ""
	}
	lower := strings.ToLower(question)
	if strings.Contains(lower, "yemaka_tool_request") || strings.Contains(lower, "```") {
		return ""
	}
	runes := []rune(question)
	if len(runes) > 240 {
		question = string(runes[:240])
	}
	return strings.TrimSpace(question)
}

func emitRoutingAdvisoryEvent(emit EventHandler, provider string, model string, advice routingAdvisory, status string, accepted bool) error {
	if emit == nil {
		return nil
	}
	return emit(Event{
		Type: EventRoutingAdvisory,
		Data: map[string]string{
			"status":                 status,
			"accepted":               fmt.Sprintf("%t", accepted),
			"provider":               provider,
			"model":                  model,
			"suggested_route":        advice.RouteCategory,
			"source_of_truth":        advice.SourceOfTruth,
			"target":                 advice.Target,
			"needs_clarification":    fmt.Sprintf("%t", advice.NeedsClarification),
			"clarification_question": advice.ClarificationQuestion,
			"confidence":             fmt.Sprintf("%d", advice.Confidence),
			"reason":                 advice.Reason,
		},
	})
}

func firstNonEmptyAdvisorString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
