package agent

import (
	"context"
	"strings"

	"yemaka/internal/learning"
	"yemaka/internal/routing"
)

func (s *Service) handleRouteCorrectionChat(ctx context.Context, conversationID string, parentMessageID string, content string, skillName string, skillVersion string, meta toolRunMetadata, emit EventHandler) (bool, error) {
	if s == nil || s.Memory == nil {
		return false, nil
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return false, nil
	}
	if strings.TrimSpace(conversationID) != "" {
		if learning.IsRouteCorrectionApproval(content) || learning.IsRouteCorrectionRejection(content) {
			pending, ok, err := learning.LatestPendingRouteCorrection(ctx, s.Memory, conversationID)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
			var response string
			if learning.IsRouteCorrectionApproval(content) {
				approved, err := learning.ApproveRouteCorrection(ctx, s.Memory, pending.ID)
				if err != nil {
					return false, err
				}
				response = learning.RouteCorrectionApprovedResponse(approved)
			} else {
				if _, err := learning.DisableRouteCorrection(ctx, s.Memory, pending.ID); err != nil {
					return false, err
				}
				response = "Okay, I won't save that routing correction."
			}
			return true, s.completeRouteCorrectionTurn(ctx, conversationID, parentMessageID, content, skillName, skillVersion, response, meta, emit, false)
		}
	}

	previousPrompt, err := s.previousUserPrompt(ctx, conversationID)
	if err != nil {
		return false, err
	}
	proposal, ok := learning.ProposeRouteCorrection(learning.RouteCorrectionProposalInput{
		ConversationID:     conversationID,
		Message:            content,
		PreviousUserPrompt: previousPrompt,
	})
	if !ok {
		return false, nil
	}
	if proposal.NeedsClarification {
		return true, s.completeRouteCorrectionTurn(ctx, conversationID, parentMessageID, content, skillName, skillVersion, proposal.Response, meta, emit, true)
	}
	conversation, err := s.ensureConversation(ctx, conversationID, content)
	if err != nil {
		return false, err
	}
	proposal.Correction.SourceConversationID = conversation.ID
	if proposal.Correction.OriginalPrompt == "" {
		proposal.Correction.OriginalPrompt = previousPrompt
	}
	if _, err := learning.SaveRouteCorrection(ctx, s.Memory, proposal.Correction); err != nil {
		return false, err
	}
	return true, s.completeRouteCorrectionTurn(ctx, conversation.ID, parentMessageID, content, skillName, skillVersion, proposal.Response, meta, emit, false)
}

func (s *Service) completeRouteCorrectionTurn(ctx context.Context, conversationID string, parentMessageID string, content string, skillName string, skillVersion string, response string, meta toolRunMetadata, emit EventHandler, clarify bool) error {
	plan := routeCorrectionLearningPlan(content, clarify)
	if err := emitPlanEvents(emit, plan); err != nil {
		return err
	}
	return s.completeWithoutModel(ctx, conversationID, meta, parentMessageID, content, skillName, skillVersion, plan, ExecutionDecision{
		Status:    ExecutionNotRequired,
		RiskLevel: RiskLow,
		Reason:    "route correction learning is handled locally and requires no tool execution",
	}, nil, nil, response, nil, emit)
}

func routeCorrectionLearningPlan(content string, clarify bool) Plan {
	route := routing.RouteLearningAction
	intent := routing.IntentAct
	steps := []string{"Capture the route correction as a pending local learning item.", "Ask for approval before changing future routing."}
	verification := []string{"no route correction is active until approved", "protected routes remain non-overridable"}
	if clarify {
		route = routing.RouteClarify
		intent = routing.IntentClarify
		steps = []string{"Ask what route should be used for similar prompts before saving any correction."}
		verification = []string{"no route correction was saved without a clear intended route"}
	}
	return Plan{
		TaskType:              TaskChat,
		ModelTask:             TaskChat,
		Goal:                  compactGoal(content),
		RiskLevel:             RiskLow,
		MaxSteps:              1,
		RouteConfidence:       90,
		RouteReasons:          []string{"chat-native route correction learning"},
		RouteCategory:         route,
		RouteIntent:           intent,
		RouteDomain:           routing.DomainGeneral,
		RouteCapability:       "route_correction",
		NeedsClarification:    clarify,
		ClarificationQuestion: learningCorrectionClarification(clarify),
		Steps:                 steps,
		Verification:          verification,
	}
}

func learningCorrectionClarification(clarify bool) string {
	if !clarify {
		return ""
	}
	return "What should Yemaka do for similar prompts: search local documents, search memory, use web search, read workspace files, explain only, or ask a follow-up?"
}

func (s *Service) previousUserPrompt(ctx context.Context, conversationID string) (string, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" || s == nil || s.Memory == nil {
		return "", nil
	}
	messages, err := s.Memory.ListConversationMessages(ctx, conversationID, 30)
	if err != nil {
		return "", err
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == "user" && strings.TrimSpace(msg.Content) != "" {
			return strings.TrimSpace(msg.Content), nil
		}
	}
	return "", nil
}
