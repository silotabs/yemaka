package routing

import "strings"

const (
	TaskFrameActionExplainOnly                   = "explain_only"
	TaskFrameActionPassiveCheck                  = "passive_check"
	TaskFrameActionActiveAssessmentRequiresScope = "active_assessment_requires_scope"
	TaskFrameActionApprovalRequired              = "approval_required"
	TaskFrameActionClarifyScope                  = "clarify_scope"
	TaskFrameActionSafeAlternativeOffer          = "safe_alternative_offer"

	TaskFrameAutonomyBounded   = "bounded"
	TaskFrameAutonomyUnbounded = "unbounded"

	TaskFrameAuthorizationNotRequired = "not_required"
	TaskFrameAuthorizationRequired    = "required"
	TaskFrameAuthorizationUnknown     = "unknown"

	TaskFramePolicyAllow                 = "allow"
	TaskFramePolicyClarifyScope          = "clarify_scope"
	TaskFramePolicyAuthorizationRequired = "authorization_required"
	TaskFramePolicyApprovalRequired      = "approval_required"

	TaskFrameTargetNone     = "none"
	TaskFrameTargetText     = "text"
	TaskFrameTargetFile     = "file"
	TaskFrameTargetConfig   = "config"
	TaskFrameTargetURL      = "url"
	TaskFrameTargetDomain   = "domain"
	TaskFrameTargetWebsite  = "website"
	TaskFrameTargetServer   = "server"
	TaskFrameTargetExternal = "external"
)

type TaskFrame struct {
	RawRequest            string
	Intent                string
	Domain                string
	TargetType            string
	Target                string
	ActionType            string
	AutonomyLevel         string
	AuthorizationStatus   string
	RiskLevel             string
	RequiresInternet      bool
	RequiresTools         bool
	RequiresFileWrite     bool
	RequiresShell         bool
	RequiresScheduler     bool
	RequiresExtension     bool
	RequiresApproval      bool
	RequiresClarification bool
	PolicyDecision        string
	SafeAlternative       string
	Explanation           string
	RouteCategory         string
	Capability            string
}

func ExtractTaskFrame(input Request) TaskFrame {
	raw := strings.TrimSpace(input.Content)
	normalized := strings.ToLower(strings.Join(strings.Fields(raw), " "))
	frame := TaskFrame{
		RawRequest:          raw,
		TargetType:          TaskFrameTargetNone,
		ActionType:          "",
		AutonomyLevel:       TaskFrameAutonomyBounded,
		AuthorizationStatus: TaskFrameAuthorizationNotRequired,
		RiskLevel:           "",
		PolicyDecision:      TaskFramePolicyAllow,
	}
	if normalized == "" {
		return frame
	}

	frame.Target, frame.TargetType = taskFrameTarget(raw, normalized)
	frame.AutonomyLevel = taskFrameAutonomy(normalized)

	if LooksInlinePastedExplanationRequest(raw) || taskFrameLooksExplicitExplainOnly(normalized) {
		frame.ActionType = TaskFrameActionExplainOnly
		frame.RiskLevel = RiskLow
		frame.Explanation = "The request asks for explanation or summary, not action."
		return frame
	}

	if LooksFileEditIntent(normalized) {
		frame.ActionType = TaskFrameActionApprovalRequired
		frame.Intent = IntentEdit
		frame.RequiresTools = true
		frame.RequiresFileWrite = true
		frame.RequiresApproval = true
		frame.RiskLevel = RiskMedium
		frame.PolicyDecision = TaskFramePolicyApprovalRequired
		frame.Capability = "filesystem_write"
		frame.Explanation = "File writes require diff, snapshot, and approval."
		return frame
	}

	if LooksExtensionGenerationTask(normalized) {
		frame.ActionType = TaskFrameActionApprovalRequired
		frame.Intent = IntentGenerate
		frame.RequiresTools = true
		frame.RequiresExtension = true
		frame.RequiresApproval = true
		frame.RiskLevel = RiskMedium
		frame.PolicyDecision = TaskFramePolicyApprovalRequired
		frame.Capability = "extension_generation"
		frame.Explanation = "Generated capabilities require approval and tests before registration."
		return frame
	}

	if LooksSchedulerCreateTask(normalized) {
		frame.ActionType = TaskFrameActionApprovalRequired
		frame.Intent = IntentSchedule
		frame.RequiresTools = true
		frame.RequiresScheduler = true
		frame.RequiresApproval = true
		frame.RiskLevel = RiskMedium
		frame.PolicyDecision = TaskFramePolicyApprovalRequired
		frame.Capability = "scheduler"
		frame.Explanation = "Scheduled or persistent jobs require approval before creation."
		return frame
	}

	if LooksShellToolTask(normalized) && !LooksTestToolIntent(normalized) && !LooksGitToolIntent(normalized) {
		frame.ActionType = TaskFrameActionApprovalRequired
		frame.Intent = IntentRunTool
		frame.RequiresTools = true
		frame.RequiresShell = true
		frame.RequiresApproval = true
		frame.RiskLevel = RiskHigh
		frame.PolicyDecision = TaskFramePolicyApprovalRequired
		frame.Capability = "shell"
		frame.Explanation = "Shell commands require explicit approval and scope."
		return frame
	}

	if taskFrameLooksExternalAssessment(normalized, frame) {
		frame.Domain = DomainRecon
		frame.Intent = IntentClarify
		frame.ActionType = TaskFrameActionActiveAssessmentRequiresScope
		frame.AuthorizationStatus = TaskFrameAuthorizationRequired
		frame.RiskLevel = RiskHigh
		frame.RequiresInternet = false
		frame.RequiresTools = false
		frame.RequiresApproval = true
		frame.RequiresClarification = true
		frame.PolicyDecision = TaskFramePolicyAuthorizationRequired
		frame.RouteCategory = RouteActiveAssessmentRequiresScope
		frame.Capability = "scope_authorization"
		frame.SafeAlternative = "I can do a bounded passive check, such as public metadata or security headers, after you provide the target and limits."
		frame.Explanation = "The request implies active security assessment of an external target and needs authorization, scope, limits, and a stop condition before any tool use."
		return frame
	}

	if taskFrameLooksPassiveCheck(normalized, frame) {
		frame.ActionType = TaskFrameActionPassiveCheck
		frame.Intent = IntentResearch
		frame.RiskLevel = RiskLow
		frame.RouteCategory = RoutePassiveCheck
		if taskFrameIsExternalTarget(frame) || taskFrameHasExternalCue(normalized) {
			frame.RequiresInternet = true
			frame.RequiresTools = true
			frame.Capability = "internet"
			frame.RiskLevel = RiskMedium
		}
		if taskFrameMissingConcreteExternalTarget(frame, normalized) {
			frame.Intent = IntentClarify
			frame.RequiresClarification = true
			frame.PolicyDecision = TaskFramePolicyClarifyScope
			frame.SafeAlternative = "Provide the exact public URL or domain and I can keep the check passive and bounded."
			frame.Explanation = "The request is a passive check but the target is not concrete enough to use a tool safely."
		} else {
			frame.Explanation = "The request is a bounded passive check, not active assessment."
		}
		return frame
	}

	return frame
}

func decisionFromTaskFrame(frame TaskFrame) (Decision, bool) {
	switch frame.ActionType {
	case TaskFrameActionActiveAssessmentRequiresScope:
		return Decision{
			TaskType:              TaskChat,
			Confidence:            92,
			Reasons:               []string{"task frame active external assessment requires scope and authorization"},
			NeedsClarification:    true,
			ClarificationQuestion: taskFrameScopeQuestion(frame),
			TaskFrame:             frame,
		}, true
	case TaskFrameActionPassiveCheck:
		if frame.RequiresClarification {
			return Decision{
				TaskType:              TaskChat,
				Confidence:            84,
				Reasons:               []string{"task frame passive check needs target scope"},
				NeedsClarification:    true,
				ClarificationQuestion: taskFramePassiveClarificationQuestion(frame),
				TaskFrame:             frame,
			}, true
		}
		if frame.RequiresTools {
			return Decision{TaskType: TaskTool, Confidence: 86, Reasons: []string{"task frame bounded passive check"}, TaskFrame: frame}, true
		}
		return Decision{TaskType: TaskReasoning, Confidence: 82, Reasons: []string{"task frame local passive check"}, TaskFrame: frame}, true
	default:
		return Decision{}, false
	}
}

func taskFrameOverridesRoute(frame TaskFrame) bool {
	switch frame.ActionType {
	case TaskFrameActionPassiveCheck,
		TaskFrameActionActiveAssessmentRequiresScope,
		TaskFrameActionClarifyScope,
		TaskFrameActionSafeAlternativeOffer:
		return true
	default:
		return false
	}
}

func ToolsForTaskFrame(content string, taskType string, frame TaskFrame) ([]string, bool) {
	if frame.RawRequest == "" {
		return nil, false
	}
	add := func(values []string, tool string) []string {
		for _, existing := range values {
			if existing == tool {
				return values
			}
		}
		return append(values, tool)
	}
	switch frame.ActionType {
	case TaskFrameActionActiveAssessmentRequiresScope:
		return nil, true
	case TaskFrameActionPassiveCheck:
		if frame.RequiresClarification || !frame.RequiresTools {
			return nil, true
		}
		lower := strings.ToLower(strings.Join(strings.Fields(content), " "))
		tools := make([]string, 0, 2)
		if ContainsSearchTerm(lower, "header") || ContainsSearchTerm(lower, "headers") ||
			ContainsSearchTerm(lower, "status code") || ContainsSearchTerm(lower, "reachable") {
			tools = add(tools, "internet_head")
		} else {
			tools = add(tools, "internet_search")
		}
		return tools, true
	default:
		return nil, false
	}
}

func taskFrameTarget(raw string, normalized string) (string, string) {
	if target, ok := InternetURLHint(raw); ok {
		if strings.HasPrefix(strings.ToLower(target), "http://") || strings.HasPrefix(strings.ToLower(target), "https://") {
			host := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(target), "https://"), "http://")
			if slash := strings.Index(host, "/"); slash >= 0 {
				host = host[:slash]
			}
			if ContainsSearchTerm(normalized, "server") {
				return target, TaskFrameTargetServer
			}
			if host != "" {
				return target, TaskFrameTargetURL
			}
		}
		return target, TaskFrameTargetDomain
	}
	if files := FileHints(raw); len(files) > 0 {
		return files[0], TaskFrameTargetFile
	}
	switch {
	case ContainsSearchTerm(normalized, "server"):
		return "", TaskFrameTargetServer
	case ContainsSearchTerm(normalized, "website") || ContainsSearchTerm(normalized, "site") || ContainsSearchTerm(normalized, "url"):
		return "", TaskFrameTargetWebsite
	case ContainsSearchTerm(normalized, "paragraph") || ContainsSearchTerm(normalized, "text"):
		return "", TaskFrameTargetText
	case ContainsSearchTerm(normalized, "config") || ContainsSearchTerm(normalized, "configuration"):
		return "", TaskFrameTargetConfig
	default:
		return "", TaskFrameTargetNone
	}
}

func taskFrameAutonomy(content string) string {
	for _, term := range []string{
		"do not stop", "don't stop", "continue until", "keep going until",
		"find something no matter what", "try everything", "never stop",
		"all possible ways", "all possible methods", "no matter what",
	} {
		if ContainsSearchTerm(content, term) {
			return TaskFrameAutonomyUnbounded
		}
	}
	return TaskFrameAutonomyBounded
}

func taskFrameLooksExplicitExplainOnly(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return false
	}
	for _, prefix := range []string{
		"explain", "explain:", "explain this", "explain how", "explain why",
		"summarize", "summarise", "summarize this", "summarise this",
		"analyze", "analyse", "review", "describe", "clarify", "translate",
		"rewrite", "what does this mean",
	} {
		if content == prefix || strings.HasPrefix(content, prefix+" ") {
			return true
		}
	}
	return false
}

func taskFrameLooksExternalAssessment(content string, frame TaskFrame) bool {
	if !taskFrameIsExternalTarget(frame) && !taskFrameHasExternalCue(content) {
		return false
	}
	if taskFrameLooksPassiveCheck(content, frame) {
		return false
	}
	hasSecurityGoal := taskFrameHasSecurityGoal(content)
	if !hasSecurityGoal {
		return false
	}
	hasActiveGoal := containsAnyTaskFrameTerm(content,
		"find vulnerability", "find vulnerabilities", "test this server", "test server",
		"full assessment", "run assessment", "try all", "try every", "all possible methods",
		"exploit", "penetration test", "pentest",
	) || frame.AutonomyLevel == TaskFrameAutonomyUnbounded
	if hasActiveGoal {
		return true
	}
	return containsAnyTaskFrameTerm(content, "vulnerability", "vulnerabilities") &&
		containsAnyTaskFrameTerm(content, "find", "check", "scan", "test", "assess")
}

func taskFrameLooksPassiveCheck(content string, frame TaskFrame) bool {
	if containsAnyTaskFrameTerm(content, "grammar", "style issue", "style issues", "paragraph", "design", "public metadata", "metadata", "reachable", "status code", "security header", "security headers") {
		return true
	}
	if containsAnyTaskFrameTerm(content, "passive public recon", "passive recon", "public recon") {
		return true
	}
	if containsAnyTaskFrameTerm(content, "check website", "check this website", "review website", "review this website") &&
		!taskFrameHasSecurityGoal(content) {
		return true
	}
	return frame.TargetType == TaskFrameTargetText && ContainsSearchTerm(content, "check")
}

func taskFrameHasExternalCue(content string) bool {
	return containsAnyTaskFrameTerm(content, "website", "site", "url", "domain", "server", "http", "https")
}

func taskFrameIsExternalTarget(frame TaskFrame) bool {
	switch frame.TargetType {
	case TaskFrameTargetURL, TaskFrameTargetDomain, TaskFrameTargetWebsite, TaskFrameTargetServer, TaskFrameTargetExternal:
		return true
	default:
		return false
	}
}

func taskFrameMissingConcreteExternalTarget(frame TaskFrame, content string) bool {
	if !taskFrameHasExternalCue(content) && !taskFrameIsExternalTarget(frame) {
		return false
	}
	return frame.Target == "" && (frame.TargetType == TaskFrameTargetWebsite || frame.TargetType == TaskFrameTargetServer || frame.TargetType == TaskFrameTargetExternal)
}

func taskFrameHasSecurityGoal(content string) bool {
	return containsAnyTaskFrameTerm(content,
		"vulnerability", "vulnerabilities", "vuln", "vulns", "security",
		"pentest", "penetration test", "recon", "assessment",
	)
}

func containsAnyTaskFrameTerm(content string, terms ...string) bool {
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func taskFrameScopeQuestion(frame TaskFrame) string {
	target := strings.TrimSpace(frame.Target)
	if target == "" {
		target = "that external target"
	}
	return "I understand you want an active security assessment of " + target + ", but I need explicit authorization, scope, allowed methods, time/rate limits, exclusions, expected output, and a bounded stop condition before using any tools. I can offer a safer passive alternative such as public metadata or security-header checks."
}

func taskFramePassiveClarificationQuestion(frame TaskFrame) string {
	if frame.SafeAlternative != "" {
		return frame.SafeAlternative
	}
	return "What exact public URL or domain should I check, and should I keep the check passive and bounded?"
}
