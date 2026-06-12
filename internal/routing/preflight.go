package routing

import (
	"strings"
)

const (
	PreflightTargetExplicitPrompt    = "explicit_prompt"
	PreflightTargetPriorConversation = "prior_conversation"
	PreflightTargetSuppliedContext   = "supplied_context"
	PreflightTargetMissing           = "missing"

	PreflightSourceChat           = "chat"
	PreflightSourceMemory         = "memory"
	PreflightSourceLocalDocuments = "local_documents"
	PreflightSourceWorkspace      = "workspace"
	PreflightSourceInternet       = "internet"
	PreflightSourceTool           = "tool"
	PreflightSourceScheduler      = "scheduler"
	PreflightSourceExtension      = "extension"
	PreflightSourceConnector      = "connector"
	PreflightSourceClarify        = "clarify"

	PreflightFreshnessNone   = "none"
	PreflightFreshnessLow    = "low"
	PreflightFreshnessMedium = "medium"
	PreflightFreshnessHigh   = "high"
)

type PreflightRouteCandidate struct {
	Route         string   `json:"route"`
	SourceOfTruth string   `json:"source_of_truth"`
	Confidence    int      `json:"confidence"`
	Reasons       []string `json:"reasons,omitempty"`
}

type PreflightCard struct {
	RawPrompt                string                    `json:"raw_prompt,omitempty"`
	NormalizedPrompt         string                    `json:"normalized_prompt,omitempty"`
	Intent                   string                    `json:"intent,omitempty"`
	Domain                   string                    `json:"domain,omitempty"`
	Target                   string                    `json:"target,omitempty"`
	TargetSource             string                    `json:"target_source,omitempty"`
	SourceOfTruth            string                    `json:"source_of_truth,omitempty"`
	FreshnessRisk            string                    `json:"freshness_risk,omitempty"`
	RouteCandidates          []PreflightRouteCandidate `json:"route_candidates,omitempty"`
	SelectedRoute            string                    `json:"selected_route,omitempty"`
	Confidence               int                       `json:"confidence,omitempty"`
	MissingSlots             []string                  `json:"missing_slots,omitempty"`
	ConflictSignals          []string                  `json:"conflict_signals,omitempty"`
	ProtectedPolicyDecision  string                    `json:"protected_policy_decision,omitempty"`
	RiskLevel                string                    `json:"risk_level,omitempty"`
	Reasons                  []string                  `json:"reasons,omitempty"`
	AppliedCorrectionIDs     []string                  `json:"applied_correction_ids,omitempty"`
	FollowupTarget           string                    `json:"followup_target,omitempty"`
	ContinuationKind         string                    `json:"continuation_kind,omitempty"`
	ContinuationTargetSource string                    `json:"continuation_target_source,omitempty"`
	PriorTarget              string                    `json:"prior_target,omitempty"`
	RequiredTool             string                    `json:"required_tool,omitempty"`
	BlockedRequirement       string                    `json:"blocked_requirement,omitempty"`
	StopReason               string                    `json:"stop_reason,omitempty"`
}

func (card PreflightCard) IsZero() bool {
	return strings.TrimSpace(card.RawPrompt) == "" &&
		strings.TrimSpace(card.NormalizedPrompt) == "" &&
		strings.TrimSpace(card.SelectedRoute) == ""
}

func BuildPreflightCard(input Request, frame TaskFrame) PreflightCard {
	raw := strings.TrimSpace(input.Content)
	normalized := strings.ToLower(strings.Join(strings.Fields(raw), " "))
	card := PreflightCard{
		RawPrompt:               raw,
		NormalizedPrompt:        normalized,
		TargetSource:            PreflightTargetMissing,
		SourceOfTruth:           PreflightSourceChat,
		FreshnessRisk:           PreflightFreshnessNone,
		ProtectedPolicyDecision: frame.PolicyDecision,
		RiskLevel:               firstNonEmptyString(frame.RiskLevel, RiskLevel(normalized)),
	}
	if normalized == "" {
		card.Intent = IntentClarify
		card.SourceOfTruth = PreflightSourceClarify
		card.SelectedRoute = RouteClarify
		card.Confidence = 100
		card.MissingSlots = appendPreflightUnique(card.MissingSlots, "prompt")
		card.Reasons = appendPreflightUnique(card.Reasons, "empty request")
		return card
	}

	card.Domain = preflightDomain(normalized, frame)
	card.Intent = preflightIntent(normalized, frame)
	card.Target, card.TargetSource = preflightTarget(input, frame)
	if continuationTarget := preflightContinuationTarget(input.Continuation); continuationTarget != "" {
		card.Target = continuationTarget
		card.TargetSource = preflightTargetSourceFromContinuation(input.Continuation.TargetSource)
	}
	if card.TargetSource == PreflightTargetPriorConversation {
		card.FollowupTarget = card.Target
	}
	card.ContinuationKind = input.Continuation.Kind
	card.ContinuationTargetSource = input.Continuation.TargetSource
	card.PriorTarget = input.Continuation.PriorTarget
	if input.Continuation.PriorFailureReason != "" {
		card.BlockedRequirement = input.Continuation.PriorFailureReason
	}
	card.FreshnessRisk = preflightFreshnessRisk(normalized, input, card)
	card.AppliedCorrectionIDs = matchingRouteCorrectionIDs(input, frame)

	if taskFrameProtected(frame) {
		card.SourceOfTruth, card.SelectedRoute = preflightProtectedSourceAndRoute(frame)
		card.Confidence = 94
		card.Reasons = appendPreflightUnique(card.Reasons, "protected task frame has priority")
		if frame.RequiresClarification {
			card.SourceOfTruth = PreflightSourceClarify
			card.MissingSlots = appendPreflightUnique(card.MissingSlots, "scope")
		}
		return preflightFinalize(card)
	}

	preflightAddCandidates(input, frame, &card)
	preflightSelectCandidate(&card)
	return preflightFinalize(card)
}

func preflightAddCandidates(input Request, frame TaskFrame, card *PreflightCard) {
	content := card.NormalizedPrompt
	add := func(route string, source string, confidence int, reasons ...string) {
		card.RouteCandidates = append(card.RouteCandidates, PreflightRouteCandidate{
			Route:         route,
			SourceOfTruth: source,
			Confidence:    confidence,
			Reasons:       append([]string{}, reasons...),
		})
	}

	switch {
	case LooksDocumentIngestAction(content):
		add(RouteSettingsAction, PreflightSourceTool, 88, "document ingestion is an indexing action, not retrieval")
	case LooksInlinePastedExplanationRequest(card.RawPrompt) || preflightLooksPastedCodeOrSVGExplanation(card.RawPrompt):
		add(RouteChatExplanation, PreflightSourceChat, 92, "supplied snippet should be treated as inert text")
		if _, hasURL := InternetURLHint(card.RawPrompt); hasURL || strings.Contains(content, "xmlns=") || strings.Contains(content, "<svg") {
			card.ConflictSignals = appendPreflightUnique(card.ConflictSignals, "url-like text inside pasted content")
		}
	case preflightContinuationRoute(input.Continuation) != "":
		route := preflightContinuationRoute(input.Continuation)
		add(route, preflightContinuationSource(input.Continuation, route), 90, "continuation preserves prior route state")
	case preflightLooksLocalDocumentRetrieval(input, content):
		add(RouteRAGSearch, PreflightSourceLocalDocuments, 92, "user points to supplied, indexed, ingested, or local documents")
	case frame.ActionType == TaskFrameActionExplainOnly:
		add(RouteChatExplanation, PreflightSourceChat, 92, "task frame says this is explanation, not action")
	case LooksBroadCurrentNewsTask(content):
		add(RouteClarify, PreflightSourceClarify, 84, "broad current-news request needs a topic")
		card.MissingSlots = appendPreflightUnique(card.MissingSlots, "current_news_topic")
	case LooksKnownLocalTimeTargetTask(content) || LooksLocalTimeTask(content):
		add(RouteLocalTime, PreflightSourceTool, 86, "current local-time request needs the local time tool")
	case preflightLooksMemoryRetrieval(content):
		add(RouteMemorySearch, PreflightSourceMemory, 90, "user asks about memory or prior conversation")
	case preflightLooksWorkspaceRetrieval(content):
		add(RouteWorkspaceRead, PreflightSourceWorkspace, 88, "user points to repo, project, codebase, or local files")
	case preflightLooksScheduler(content):
		add(RouteSchedulerCreate, PreflightSourceScheduler, 88, "recurring or future job intent stays approval-gated")
	case LooksConnectorActionTask(content):
		add(RouteConnectorAction, PreflightSourceConnector, 84, "external connector action stays disabled or approval-gated")
	case LooksCapabilityActionTask(content):
		add(RouteExtensionGenerate, PreflightSourceExtension, 84, "new reusable capability intent stays approval-gated")
	case preflightLooksPublicMutableLookup(content, *card):
		add(RouteInternetSearch, PreflightSourceInternet, 88, "public factual lookup has freshness risk")
	case LooksSpecificCurrentNewsTask(content) || LooksCurrentWebFactTask(content) || LooksInternetSearchTask(content):
		add(RouteInternetSearch, PreflightSourceInternet, 86, "current or web lookup needs internet source")
	case frame.ActionType == TaskFrameActionPassiveCheck && frame.RequiresTools:
		add(RouteInternetSearch, PreflightSourceInternet, 82, "bounded passive external check")
	default:
		add(RouteChatExplanation, PreflightSourceChat, 72, "stable explanation or general reasoning can use chat")
	}

	if preflightLooksLocalDocumentRetrieval(input, content) && (LooksInternetSearchTask(content) || LooksGenericInternetSearchTask(content)) {
		card.ConflictSignals = appendPreflightUnique(card.ConflictSignals, "search intent constrained by local documents")
	}
	if card.FreshnessRisk == PreflightFreshnessHigh && hasProvidedContext(input) && !preflightLooksLocalDocumentRetrieval(input, content) && !preflightLooksMemoryRetrieval(content) && !preflightLooksWorkspaceRetrieval(content) {
		card.ConflictSignals = appendPreflightUnique(card.ConflictSignals, "current public fact should not be answered from stale local context")
	}
	if preflightHasAmbiguousPriorTargets(input, content) {
		add(RouteClarify, PreflightSourceClarify, 95, "follow-up has multiple possible prior targets")
		card.MissingSlots = appendPreflightUnique(card.MissingSlots, "ambiguous_prior_target")
		card.ConflictSignals = appendPreflightUnique(card.ConflictSignals, "multiple prior targets")
	}
	if preflightVagueReferenceNeedsClarification(content, *card, frame) {
		add(RouteClarify, PreflightSourceClarify, 80, "vague target has no safe prior target")
		card.MissingSlots = appendPreflightUnique(card.MissingSlots, "target")
	}
	if preflightLooksIncompletePublicRoleLookup(content, *card) {
		add(RouteClarify, PreflightSourceClarify, 82, "public role lookup is missing the organization or person target")
		card.MissingSlots = appendPreflightUnique(card.MissingSlots, "public_entity")
	}
}

func preflightSelectCandidate(card *PreflightCard) {
	if len(card.RouteCandidates) == 0 {
		card.SelectedRoute = RouteChatExplanation
		card.SourceOfTruth = PreflightSourceChat
		card.Confidence = 60
		return
	}
	best := card.RouteCandidates[0]
	for _, candidate := range card.RouteCandidates[1:] {
		if candidate.Confidence > best.Confidence {
			best = candidate
		}
	}
	card.SelectedRoute = best.Route
	card.SourceOfTruth = best.SourceOfTruth
	card.Confidence = best.Confidence
	card.Reasons = appendPreflightUnique(card.Reasons, best.Reasons...)
}

func preflightFinalize(card PreflightCard) PreflightCard {
	if card.Intent == "" {
		card.Intent = IntentExplain
	}
	if card.Domain == "" {
		card.Domain = DomainGeneral
	}
	if card.TargetSource == "" {
		card.TargetSource = PreflightTargetMissing
	}
	if card.SourceOfTruth == "" {
		card.SourceOfTruth = PreflightSourceChat
	}
	if card.FreshnessRisk == "" {
		card.FreshnessRisk = PreflightFreshnessNone
	}
	if card.SelectedRoute == "" {
		card.SelectedRoute = RouteChatExplanation
	}
	if card.Confidence == 0 {
		card.Confidence = 60
	}
	return card
}

func decisionFromPreflight(input Request, card PreflightCard, frame TaskFrame) (Decision, bool) {
	if card.IsZero() || taskFrameProtected(frame) || !preflightShouldSelectRoute(input, card) {
		return Decision{}, false
	}
	decision := Decision{
		Confidence:        card.Confidence,
		Reasons:           append([]string{"preflight source-of-truth " + card.SourceOfTruth}, card.Reasons...),
		RouteCategory:     card.SelectedRoute,
		Intent:            card.Intent,
		Domain:            card.Domain,
		Target:            card.Target,
		RiskLevel:         card.RiskLevel,
		Preflight:         card,
		PreflightSelected: true,
	}
	switch card.SelectedRoute {
	case RouteRAGSearch:
		decision.TaskType = TaskRAG
		decision.Tools = []string{"rag_search"}
		decision.Capability = "rag"
		if input.UseRAGConfig && !input.RAGEnabled {
			decision.Tools = nil
		}
	case RouteMemorySearch:
		decision.TaskType = TaskTool
		decision.Tools = []string{"memory_search"}
		decision.Capability = "memory"
	case RouteWorkspaceRead:
		decision.TaskType = TaskTool
		decision.Tools = []string{"read_file", "search_files"}
		decision.Capability = "filesystem_read"
	case RouteInternetSearch:
		decision.TaskType = TaskTool
		decision.Tools = []string{"internet_search"}
		decision.Capability = "internet"
	case RouteInternetFetch:
		decision.TaskType = TaskTool
		decision.Tools = []string{"internet_fetch"}
		decision.Capability = "internet"
	case RouteInternetHead:
		decision.TaskType = TaskTool
		decision.Tools = []string{"internet_head"}
		decision.Capability = "internet"
	case RouteChatExplanation:
		decision.TaskType = TaskReasoning
		decision.Tools = nil
		decision.Capability = "chat"
	case RouteClarify:
		decision.TaskType = TaskChat
		decision.NeedsClarification = true
		decision.ClarificationQuestion = preflightClarificationQuestion(card)
		decision.Tools = nil
		decision.Intent = IntentClarify
	default:
		return Decision{}, false
	}
	return decision, true
}

func preflightShouldSelectRoute(input Request, card PreflightCard) bool {
	content := card.NormalizedPrompt
	switch card.SelectedRoute {
	case RouteInternetSearch, RouteInternetFetch, RouteInternetHead:
		if preflightContinuationRoute(input.Continuation) == card.SelectedRoute {
			return true
		}
		if card.SelectedRoute != RouteInternetSearch {
			return false
		}
		return preflightLooksPublicMutableLookup(content, card) &&
			!LooksCurrentWebFactTask(content) &&
			!LooksSpecificCurrentNewsTask(content) &&
			!LooksInternetSearchTask(content)
	case RouteRAGSearch:
		if preflightContinuationRoute(input.Continuation) == RouteRAGSearch {
			return true
		}
		return preflightLooksExplicitLocalDocumentCorpus(content) ||
			containsAnyPreflightTerm(content, "uploaded pdf", "uploaded document", "attached pdf", "attached document", "supplied document", "provided document") ||
			strings.TrimSpace(input.TaskMemory) != "" &&
				containsAnyPreflightTerm(content, "i meant", "instead", "not internet", "not web") &&
				preflightLooksLocalDocumentRetrieval(input, content)
	case RouteWorkspaceRead:
		if preflightContinuationRoute(input.Continuation) == RouteWorkspaceRead {
			return true
		}
		return preflightLooksWorkspaceRetrieval(content) &&
			containsAnyPreflightTerm(content, "implemented in", "where is", "where are")
	case RouteMemorySearch:
		return preflightContinuationRoute(input.Continuation) == RouteMemorySearch
	case RouteClarify:
		return preflightLooksIncompletePublicRoleLookup(content, card) ||
			containsPreflightValue(card.MissingSlots, "ambiguous_prior_target") ||
			input.Continuation.Kind == ContinuationKindClarify
	case RouteChatExplanation:
		return LooksInlinePastedExplanationRequest(input.Content) ||
			preflightLooksPastedCodeOrSVGExplanation(input.Content)
	default:
		return false
	}
}

func preflightClarificationQuestion(card PreflightCard) string {
	if containsPreflightValue(card.MissingSlots, "ambiguous_prior_target") {
		return "Which previous target should I use for this follow-up?"
	}
	if containsPreflightValue(card.MissingSlots, "public_entity") {
		return "Which public person, company, or organization should I use as the target?"
	}
	if containsPreflightValue(card.MissingSlots, "target") {
		return "What exact target should I use for this request?"
	}
	return "What source should I use for this request: chat, memory, local documents, workspace files, or web search?"
}

func preflightContinuationRoute(frame ContinuationFrame) string {
	switch frame.Kind {
	case ContinuationKindRetry:
		return firstNonEmptyString(frame.PriorRouteCategory, continuationRouteForTool(frame.PriorToolName))
	case ContinuationKindTargetCorrection:
		return firstNonEmptyString(frame.PriorRouteCategory, continuationRouteForTool(frame.PriorToolName))
	case ContinuationKindSourceCorrection:
		return preflightRouteForSource(frame.NewSourceOfTruth)
	case ContinuationKindClarify:
		return RouteClarify
	default:
		return ""
	}
}

func preflightRouteForSource(source string) string {
	switch strings.TrimSpace(source) {
	case PreflightSourceLocalDocuments:
		return RouteRAGSearch
	case PreflightSourceMemory:
		return RouteMemorySearch
	case PreflightSourceWorkspace:
		return RouteWorkspaceRead
	case PreflightSourceInternet:
		return RouteInternetSearch
	default:
		return ""
	}
}

func preflightContinuationSource(frame ContinuationFrame, route string) string {
	if source := strings.TrimSpace(frame.NewSourceOfTruth); source != "" {
		return source
	}
	if source := strings.TrimSpace(frame.PriorSourceOfTruth); source != "" {
		return source
	}
	return continuationSourceForRoute(route, frame.PriorToolName)
}

func preflightContinuationTarget(frame ContinuationFrame) string {
	switch frame.Kind {
	case ContinuationKindTargetCorrection, ContinuationKindSourceCorrection:
		return firstNonEmptyString(frame.NewTarget, frame.PriorTarget)
	case ContinuationKindRetry:
		return frame.PriorTarget
	default:
		return ""
	}
}

func preflightTargetSourceFromContinuation(source string) string {
	switch source {
	case ContinuationTargetExplicitPrompt:
		return PreflightTargetExplicitPrompt
	case ContinuationTargetPriorRoute, ContinuationTargetPriorToolRun, ContinuationTargetPriorConversation:
		return PreflightTargetPriorConversation
	case ContinuationTargetAmbiguous:
		return PreflightTargetMissing
	default:
		return PreflightTargetMissing
	}
}

func preflightDomain(content string, frame TaskFrame) string {
	if frame.Domain != "" {
		return frame.Domain
	}
	tokens := WordTokens(content)
	switch {
	case containsAnyToken(tokens, "ceo", "cfo", "cto", "founder", "chairman", "chairwoman", "president", "minister", "mayor", "director", "company", "business"):
		return DomainRecon
	case LooksCodingTask(content) || containsAnyToken(tokens, "code", "repo", "repository", "codebase", "function", "class"):
		return DomainCode
	case containsAnyToken(tokens, "legal", "contract", "law", "lawyer", "regulation", "regulations"):
		return DomainLegal
	case containsAnyToken(tokens, "medical", "doctor", "health", "diagnosis"):
		return DomainMedical
	case containsAnyToken(tokens, "stock", "stocks", "crypto", "price", "prices", "finance", "financial"):
		return DomainFinance
	default:
		return DomainGeneral
	}
}

func preflightIntent(content string, frame TaskFrame) string {
	if frame.Intent != "" {
		return frame.Intent
	}
	switch {
	case preflightLooksScheduler(content):
		return IntentSchedule
	case LooksConnectorActionTask(content):
		return IntentAct
	case LooksCapabilityActionTask(content):
		return IntentGenerate
	case preflightLooksMemoryRetrieval(content), preflightLooksLocalDocumentRetrieval(Request{Content: content}, content), preflightLooksWorkspaceRetrieval(content), LooksInternetSearchTask(content), preflightLooksPublicMutableLookup(content, PreflightCard{}):
		return IntentResearch
	case strings.HasPrefix(content, "summarize ") || strings.HasPrefix(content, "summarise "):
		return IntentSummarize
	default:
		return IntentExplain
	}
}

func preflightTarget(input Request, frame TaskFrame) (string, string) {
	if frame.Target != "" {
		return frame.Target, PreflightTargetExplicitPrompt
	}
	if target := preflightExplicitTarget(input.Content); target != "" {
		return target, PreflightTargetExplicitPrompt
	}
	if preflightHasVagueReference(strings.ToLower(input.Content)) ||
		input.Continuation.Kind == ContinuationKindRetry {
		targets := preflightPriorTargets(input.TaskMemory)
		if len(targets) == 1 {
			return targets[0], PreflightTargetPriorConversation
		}
	}
	if hasProvidedContext(input) && strings.TrimSpace(input.TaskMemory) == "" {
		return "supplied_context", PreflightTargetSuppliedContext
	}
	return "", PreflightTargetMissing
}

func preflightExplicitTarget(content string) string {
	raw := strings.TrimSpace(content)
	normalized := strings.ToLower(strings.Join(strings.Fields(raw), " "))
	if target, ok := InternetURLHint(raw); ok {
		return target
	}
	if files := FileHints(raw); len(files) > 0 {
		return files[0]
	}
	if target := preflightPublicRoleTarget(raw, normalized); target != "" {
		return target
	}
	for _, marker := range []string{" latest updates on ", " updates on ", " news on ", " information on ", " background on ", " about "} {
		if idx := strings.Index(" "+normalized, marker); idx >= 0 {
			start := idx + len(marker) - 1
			return cleanPreflightTarget(raw[start:])
		}
	}
	return ""
}

func preflightPublicRoleTarget(raw string, normalized string) string {
	if !preflightHasPublicRole(normalized) {
		return ""
	}
	for _, marker := range []string{" of ", " at ", " for "} {
		if idx := strings.LastIndex(normalized, marker); idx >= 0 {
			role := preflightFirstRole(normalized[:idx])
			target := cleanPreflightTarget(raw[idx+len(marker):])
			if role != "" && target != "" {
				return strings.TrimSpace(target + " " + strings.ToUpper(role))
			}
		}
	}
	if target := preflightTrailingRoleTarget(raw); target != "" {
		return target
	}
	return ""
}

func preflightPriorTargets(taskMemory string) []string {
	userTargets := preflightCollectPriorTargets(taskMemory, "user:")
	if len(userTargets) > 0 {
		return userTargets
	}
	return preflightCollectPriorTargets(taskMemory, "assistant:")
}

func preflightCollectPriorTargets(taskMemory string, prefix string) []string {
	targets := []string{}
	seen := map[string]bool{}
	lines := strings.Split(taskMemory, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, prefix) {
			continue
		}
		if target := preflightExplicitTarget(strings.TrimSpace(line[strings.Index(line, ":")+1:])); target != "" {
			key := strings.ToLower(target)
			if seen[key] {
				continue
			}
			seen[key] = true
			targets = append(targets, target)
		}
	}
	return targets
}

func preflightTrailingRoleTarget(raw string) string {
	fields := strings.Fields(raw)
	cleaned := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, " \t\r\n:;,.!?()[]{}\"'`")
		if field != "" {
			cleaned = append(cleaned, field)
		}
	}
	for i, field := range cleaned {
		role := strings.ToLower(field)
		if !preflightIsPublicRoleToken(role) {
			continue
		}
		start := i
		for start > 0 && !preflightTargetBoundary(cleaned[start-1]) {
			start--
		}
		if i-start >= 1 {
			return cleanPreflightTarget(strings.Join(cleaned[start:i+1], " "))
		}
	}
	return ""
}

func preflightTargetBoundary(field string) bool {
	switch strings.ToLower(strings.Trim(field, " \t\r\n:;,.!?()[]{}\"'`")) {
	case "", "user", "assistant", "find", "search", "look", "lookup", "about", "for", "on", "the", "a", "an",
		"i", "need", "current", "latest", "public", "business", "background", "and", "or", "to", "of", "with",
		"tell", "me", "what", "who", "is", "are", "explain", "compare":
		return true
	default:
		return false
	}
}

func preflightFreshnessRisk(content string, input Request, card PreflightCard) string {
	switch {
	case preflightLooksMemoryRetrieval(content), preflightLooksLocalDocumentRetrieval(input, content), preflightLooksWorkspaceRetrieval(content):
		return PreflightFreshnessNone
	case LooksSpecificCurrentNewsTask(content), LooksCurrentWebFactTask(content), preflightLooksPublicMutableLookup(content, card):
		return PreflightFreshnessHigh
	case LooksInternetSearchTask(content):
		return PreflightFreshnessMedium
	default:
		return PreflightFreshnessNone
	}
}

func matchingRouteCorrectionIDs(input Request, frame TaskFrame) []string {
	if !routeCorrectionsMayApply(input.Content, frame) {
		return nil
	}
	ids := []string{}
	for _, correction := range input.RouteCorrections {
		if !routeCorrectionUsable(correction) || !routeCorrectionMatches(input.Content, correction) {
			continue
		}
		ids = appendPreflightUnique(ids, strings.TrimSpace(correction.ID))
	}
	return ids
}

func preflightLooksMemoryRetrieval(content string) bool {
	return LooksMemorySearchTask(content) ||
		ContainsSearchTerm(content, "what did i say") ||
		ContainsSearchTerm(content, "what did we discuss") ||
		ContainsSearchTerm(content, "previous conversation") ||
		ContainsSearchTerm(content, "conversation history") ||
		ContainsSearchTerm(content, "saved memory") ||
		ContainsSearchTerm(content, "remembered preference") ||
		ContainsSearchTerm(content, "what did you remember")
}

func preflightLooksLocalDocumentRetrieval(input Request, content string) bool {
	if LooksDocumentIngestAction(content) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(input.SourceKind), "rag") ||
		LooksRAGTask(content) ||
		LooksDocumentSearchTask(content) ||
		LooksDocumentContextTask(content) ||
		containsAnyPreflightTerm(content, "indexed documents", "ingested documents", "uploaded pdf", "uploaded document", "uploaded file", "attached pdf", "attached document", "supplied document", "provided document")
}

func preflightLooksExplicitLocalDocumentCorpus(content string) bool {
	if LooksDocumentIngestAction(content) || LooksFileEditIntent(content) ||
		containsAnyPreflightTerm(content, "preview patch", "patch preview", "apply patch", "create patch") {
		return false
	}
	return LooksRAGTask(content) ||
		containsAnyPreflightTerm(content,
			"indexed document",
			"indexed documents",
			"indexed file",
			"indexed files",
			"ingested document",
			"ingested documents",
			"ingested file",
			"ingested files",
			"local document",
			"local documents",
			"local docs",
			"my documents",
			"my docs",
			"retrieved document",
			"retrieved documents",
			"retrieved docs",
			"answer from my docs",
			"search my docs",
		)
}

func preflightLooksWorkspaceRetrieval(content string) bool {
	return LooksLocalFileToolTask(content) ||
		LooksLocalContextToolTask(content) ||
		containsAnyPreflightTerm(content, "repo", "repository", "codebase", "workspace", "project files", "local files", "implemented in") ||
		(containsAnyPreflightTerm(content, "project") && containsAnyPreflightTerm(content, "inspect", "tests", "test", "files", "code"))
}

func preflightLooksScheduler(content string) bool {
	return LooksSchedulerCreateTask(content)
}

func preflightLooksPublicMutableLookup(content string, card PreflightCard) bool {
	if preflightLooksMemoryRetrieval(content) || preflightLooksLocalDocumentRetrieval(Request{Content: content}, content) || preflightLooksWorkspaceRetrieval(content) {
		return false
	}
	if preflightLooksIncompletePublicRoleLookup(content, card) {
		return false
	}
	if preflightHasPublicRole(content) && (strings.Contains(content, " of ") || strings.Contains(content, " at ") || strings.Contains(content, " for ") || card.TargetSource == PreflightTargetPriorConversation) {
		return true
	}
	return containsAnyPreflightTerm(content,
		"stock price", "share price", "crypto price", "exchange rate",
		"current price", "latest price", "release date", "latest version",
		"product specs", "current regulation", "latest regulation",
	)
}

func preflightLooksIncompletePublicRoleLookup(content string, card PreflightCard) bool {
	if !preflightHasPublicRole(content) || card.TargetSource != PreflightTargetMissing {
		return false
	}
	return strings.HasPrefix(content, "who is ") ||
		strings.HasPrefix(content, "who's ") ||
		strings.HasPrefix(content, "what is the name of ") ||
		strings.HasPrefix(content, "what's the name of ")
}

func preflightHasPublicRole(content string) bool {
	return containsAnyToken(WordTokens(content),
		"ceo", "cfo", "cto", "coo", "founder", "owner", "chair", "chairman", "chairwoman",
		"president", "minister", "mayor", "director", "leader", "head",
	)
}

func preflightIsPublicRoleToken(token string) bool {
	switch token {
	case "ceo", "cfo", "cto", "coo", "founder", "owner", "chair", "chairman", "chairwoman", "president", "minister", "mayor", "director", "leader", "head":
		return true
	default:
		return false
	}
}

func preflightFirstRole(content string) string {
	for _, token := range WordTokens(content) {
		if preflightIsPublicRoleToken(token) {
			return token
		}
	}
	return ""
}

func preflightHasVagueReference(content string) bool {
	return containsAnyPreflightTerm(content, "it", "this", "that", "there", "them", "him", "her", "that company", "the company", "the ceo", "that person")
}

func preflightHasAmbiguousPriorTargets(input Request, content string) bool {
	if LooksInlinePastedExplanationRequest(input.Content) ||
		preflightLooksPastedCodeOrSVGExplanation(input.Content) ||
		LooksExplanationOnlyRequest(content) ||
		strings.TrimSpace(input.TaskMemory) == "" {
		return false
	}
	if strings.TrimSpace(input.Continuation.PriorTarget) != "" ||
		strings.TrimSpace(input.Continuation.NewTarget) != "" {
		return false
	}
	if !preflightHasVagueReference(content) && input.Continuation.Kind != ContinuationKindRetry {
		return false
	}
	return len(preflightPriorTargets(input.TaskMemory)) > 1
}

func preflightLooksPastedCodeOrSVGExplanation(content string) bool {
	lower := strings.ToLower(content)
	if !strings.Contains(lower, "<svg") && !strings.Contains(lower, "xmlns=") && !strings.Contains(lower, "```") {
		return false
	}
	return containsAnyPreflightTerm(lower,
		"make this",
		"make better",
		"make cleaner",
		"cleaner",
		"improve",
		"explain",
		"summarize",
		"summarise",
		"fix this",
	)
}

func preflightVagueReferenceNeedsClarification(content string, card PreflightCard, frame TaskFrame) bool {
	if card.TargetSource != PreflightTargetMissing || !preflightHasVagueReference(content) {
		return false
	}
	if frame.ActionType == TaskFrameActionExplainOnly ||
		LooksExplanationOnlyRequest(content) ||
		LooksInlinePastedExplanationRequest(content) ||
		LooksKnownLocalTimeTargetTask(content) ||
		LooksLocalTimeTask(content) {
		return false
	}
	for _, candidate := range card.RouteCandidates {
		if candidate.Route != RouteChatExplanation && candidate.Confidence >= 80 {
			return false
		}
	}
	return true
}

func taskFrameProtected(frame TaskFrame) bool {
	return frame.PolicyDecision == TaskFramePolicyAuthorizationRequired ||
		frame.PolicyDecision == TaskFramePolicyApprovalRequired ||
		frame.ActionType == TaskFrameActionActiveAssessmentRequiresScope ||
		frame.ActionType == TaskFrameActionApprovalRequired ||
		frame.RequiresFileWrite ||
		frame.RequiresShell ||
		frame.RequiresScheduler ||
		frame.RequiresExtension ||
		frame.RequiresApproval
}

func preflightProtectedSourceAndRoute(frame TaskFrame) (string, string) {
	switch {
	case frame.RequiresScheduler:
		return PreflightSourceScheduler, RouteSchedulerCreate
	case frame.RequiresExtension:
		return PreflightSourceExtension, RouteExtensionGenerate
	case frame.RequiresShell || frame.RequiresFileWrite:
		return PreflightSourceTool, firstNonEmptyString(frame.RouteCategory, RoutePermissionRequired)
	case frame.RequiresClarification:
		return PreflightSourceClarify, firstNonEmptyString(frame.RouteCategory, RouteClarify)
	default:
		return PreflightSourceTool, firstNonEmptyString(frame.RouteCategory, RoutePermissionRequired)
	}
}

func containsAnyPreflightTerm(content string, terms ...string) bool {
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func appendPreflightUnique(values []string, additions ...string) []string {
	for _, addition := range additions {
		addition = strings.TrimSpace(addition)
		if addition == "" || containsPreflightValue(values, addition) {
			continue
		}
		values = append(values, addition)
	}
	return values
}

func containsPreflightValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func cleanPreflightTarget(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	value = strings.Trim(value, " \t\r\n:;,.!?()[]{}\"'`")
	if len(value) > 120 {
		value = strings.TrimSpace(value[:120])
	}
	return value
}
