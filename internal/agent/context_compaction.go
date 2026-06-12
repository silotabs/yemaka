package agent

import (
	"strings"

	contextcore "yemaka/internal/context"
	"yemaka/internal/routing"
)

func compactTaskStateForPrompt(plan Plan, input PlanInput) contextcore.TaskStatePackage {
	session := promptSessionContract(plan, input.SessionContract)
	frame := routing.ExtractTaskFrame(routing.Request{
		Content:          input.Content,
		SourceKind:       input.SourceKind,
		WorkspaceContext: input.WorkspaceContext,
		ProfileMemory:    input.ProfileMemory,
		TaskMemory:       input.TaskMemory,
		Sources:          input.Sources,
		MemorySources:    input.MemorySources,
		RouteCorrections: input.RouteCorrections,
		Continuation:     input.Continuation,
		SessionContract:  input.SessionContract,
	})
	if frame.Intent == "" {
		frame.Intent = plan.RouteIntent
	}
	if frame.Domain == "" {
		frame.Domain = plan.RouteDomain
	}
	if frame.Target == "" {
		frame.Target = firstNonEmptyString(plan.RouteTarget, session.ActiveTarget)
	}
	if frame.RiskLevel == "" {
		frame.RiskLevel = plan.RiskLevel
	}
	if frame.RouteCategory == "" {
		frame.RouteCategory = plan.RouteCategory
	}
	if frame.Capability == "" {
		frame.Capability = plan.RouteCapability
	}

	decisions := []string{
		compactStatePair("route", plan.RouteCategory),
		compactStatePair("lane", plan.RouteLane),
		compactStatePair("evidence_policy", plan.EvidencePolicy),
		compactStatePair("evidence_source", plan.EvidenceSource),
		compactStatePair("response_contract", plan.ResponseContract),
	}
	if plan.RoutePreflight != nil {
		decisions = append(decisions,
			compactStatePair("preflight_source", plan.RoutePreflight.SourceOfTruth),
			compactStatePair("freshness_risk", plan.RoutePreflight.FreshnessRisk),
		)
	}
	if plan.RouteContinuation != nil {
		decisions = append(decisions, compactStatePair("continuation", plan.RouteContinuation.Kind))
	}

	outcomes := append([]string{}, session.CompletedSteps...)
	outcomes = append(outcomes,
		compactStatePair("last_outcome", session.LastOutcome),
		compactStatePair("failure_reason", session.FailureReason),
	)
	if !session.LastRecovery.IsZero() {
		outcomes = append(outcomes,
			compactStatePair("recovery_trigger", session.LastRecovery.Trigger),
			compactStatePair("recovery_action", session.LastRecovery.FinalAction),
		)
	}

	return contextcore.BuildTaskStatePackage(contextcore.TaskStateInput{
		ActiveGoal:            firstNonEmptyString(plan.Goal, session.ActiveGoal),
		ActiveRoute:           firstNonEmptyString(plan.RouteCategory, session.ActiveRoute),
		ActiveTaskFrame:       frame,
		PendingApproval:       firstNonEmptyString(session.PendingApproval, pendingApprovalHint(plan)),
		PendingClarification:  firstNonEmptyString(session.PendingClarification, plan.ClarificationQuestion),
		SelectedSources:       compactTaskSelectedSources(plan, input, session),
		ImportantToolOutcomes: outcomes,
		UserConstraints:       compactTaskUserConstraints(input),
		DecisionsMade:         decisions,
		UnfinishedSteps:       compactTaskUnfinishedSteps(plan, session),
		SourceReferences:      compactTaskSourceReferences(plan, input, session),
		LastKnownOutcome:      firstNonEmptyString(session.LastOutcome, plan.EvidencePolicy),
		FreshnessRequired:     compactTaskNeedsFreshness(plan),
	})
}

func promptSessionContract(plan Plan, session routing.SessionContract) routing.SessionContract {
	if session.IsZero() {
		return routing.SessionContract{}
	}
	if strings.TrimSpace(session.PendingApproval) != "" || strings.TrimSpace(session.PendingClarification) != "" {
		return session
	}
	switch strings.TrimSpace(plan.ContinuationMode) {
	case routing.ContinuationModeContinueSameTask,
		routing.ContinuationModeApprovePendingAction,
		routing.ContinuationModeRejectPendingAction,
		routing.ContinuationModeAnswerPendingClarify,
		routing.ContinuationModeRevisePreviousTask,
		routing.ContinuationModeAskFollowupPrevious:
		return session
	default:
		return routing.SessionContract{}
	}
}

func compactTaskSelectedSources(plan Plan, input PlanInput, session routing.SessionContract) []string {
	values := []string{}
	values = append(values, plan.FilesNeeded...)
	values = append(values, plan.RouteTarget)
	values = append(values, input.Sources...)
	values = append(values, session.ContextSources...)
	if session.ActiveTarget != "" {
		values = append(values, session.ActiveTarget)
	}
	if session.ActiveTaskFrame.Target != "" {
		values = append(values, session.ActiveTaskFrame.Target)
	}
	return values
}

func compactTaskSourceReferences(plan Plan, input PlanInput, session routing.SessionContract) []string {
	values := []string{}
	values = append(values, input.Sources...)
	values = append(values, input.MemorySources...)
	if planAllowsKnowledgeContext(plan) {
		values = append(values, input.KnowledgeSources...)
	}
	values = append(values, session.ContextSources...)
	return values
}

func compactTaskUserConstraints(input PlanInput) []string {
	text := strings.Join([]string{input.ProfileMemory, input.TaskMemory}, "\n")
	var constraints []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "prefer") ||
			strings.Contains(lower, "constraint") ||
			strings.Contains(lower, "do not") ||
			strings.Contains(lower, "don't") ||
			strings.Contains(lower, "never") ||
			strings.Contains(lower, "always") ||
			strings.Contains(lower, "only") ||
			strings.Contains(lower, "must") {
			constraints = append(constraints, line)
		}
	}
	return constraints
}

func compactTaskUnfinishedSteps(plan Plan, session routing.SessionContract) []string {
	values := []string{session.PendingStep}
	if session.PendingApproval != "" {
		values = append(values, "waiting for approval: "+session.PendingApproval)
	}
	if session.PendingClarification != "" {
		values = append(values, "waiting for clarification: "+session.PendingClarification)
	}
	if plan.NeedsClarification && plan.ClarificationQuestion != "" {
		values = append(values, "ask clarification: "+plan.ClarificationQuestion)
	}
	if len(values) <= 1 {
		values = append(values, plan.Steps...)
	}
	return values
}

func compactTaskNeedsFreshness(plan Plan) bool {
	if plan.EvidencePolicy == EvidenceFreshInternetRequired || plan.RouteUsesInternet || plan.RouteCategory == routing.RouteInternetSearch {
		return true
	}
	if plan.RoutePreflight != nil {
		return plan.RoutePreflight.FreshnessRisk == routing.PreflightFreshnessHigh || plan.RoutePreflight.SourceOfTruth == routing.PreflightSourceInternet
	}
	return false
}

func pendingApprovalHint(plan Plan) string {
	if !plan.RouteRequiresApproval {
		return ""
	}
	return firstNonEmptyString(plan.RouteCapability, strings.Join(plan.ToolsNeeded, ", "), plan.RouteCategory)
}

func compactStatePair(label string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return label + "=" + value
}
