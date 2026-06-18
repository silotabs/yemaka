package learning

import (
	"strings"

	"yemaka/internal/routing"
)

type RouteCorrectionProposalInput struct {
	ConversationID     string
	Message            string
	PreviousUserPrompt string
}

type RouteCorrectionProposal struct {
	Correction            routing.RouteCorrection
	Reason                string
	Response              string
	NeedsClarification    bool
	ClarificationQuestion string
}

func IsRouteCorrectionApproval(message string) bool {
	normalized := normalizeCorrectionReply(message)
	switch normalized {
	case "yes", "yes please", "approve", "approved", "save it", "save this", "remember this", "remember it", "yes save it", "yes remember this", "yes use that next time", "use that next time":
		return true
	default:
		return false
	}
}

func IsRouteCorrectionRejection(message string) bool {
	normalized := normalizeCorrectionReply(message)
	switch normalized {
	case "no", "no thanks", "reject", "cancel", "do not save", "dont save", "do not remember", "dont remember":
		return true
	default:
		return false
	}
}

func ProposeRouteCorrection(input RouteCorrectionProposalInput) (RouteCorrectionProposal, bool) {
	message := strings.TrimSpace(input.Message)
	if message == "" || IsRouteCorrectionApproval(message) || IsRouteCorrectionRejection(message) {
		return RouteCorrectionProposal{}, false
	}
	normalized := normalizeCorrectionText(message)
	if looksLikeFactualFileStateCorrection(normalized) {
		return RouteCorrectionProposal{}, false
	}
	if looksLikeEditorialRewriteTask(normalized) &&
		!looksExplicitRouteTeachingCue(normalized) &&
		!looksExplicitSourceOrToolPreferenceCorrection(normalized) {
		return RouteCorrectionProposal{}, false
	}
	if !looksLikeRoutingCorrection(normalized) {
		return RouteCorrectionProposal{}, false
	}

	route, tags, requiredTools, forbiddenTools, clarification := inferCorrectedRoute(normalized)
	if route == "" {
		return RouteCorrectionProposal{
			NeedsClarification:    true,
			ClarificationQuestion: routeCorrectionClarificationQuestion(),
			Response:              routeCorrectionClarificationQuestion(),
			Reason:                "correction intent detected but intended route is ambiguous",
		}, true
	}

	pattern := inferRouteCorrectionPattern(message, normalized, input.PreviousUserPrompt, route, tags)
	if strings.TrimSpace(pattern) == "" {
		return RouteCorrectionProposal{
			NeedsClarification:    true,
			ClarificationQuestion: routeCorrectionClarificationQuestion(),
			Response:              routeCorrectionClarificationQuestion(),
			Reason:                "correction intent detected but reusable pattern is ambiguous",
		}, true
	}

	correction := routing.RouteCorrection{
		SourceConversationID:  strings.TrimSpace(input.ConversationID),
		Pattern:               pattern,
		IntendedRouteCategory: route,
		IntendedTaskType:      routeCorrectionTask(route),
		IntendedCapability:    routeCorrectionCapability(route),
		RequiredTools:         requiredTools,
		ForbiddenTools:        forbiddenTools,
		Tags:                  tags,
		OriginalPrompt:        strings.TrimSpace(input.PreviousUserPrompt),
		CorrectionText:        message,
		ClarificationQuestion: clarification,
		ApprovalStatus:        routing.RouteCorrectionStatusPending,
	}
	return RouteCorrectionProposal{
		Correction: correction,
		Reason:     "user corrected Yemaka's routing behavior",
		Response:   routeCorrectionProposalResponse(correction),
	}, true
}

func looksLikeFactualFileStateCorrection(message string) bool {
	if message == "" {
		return false
	}
	hasFileCue := strings.Contains(message, "/") ||
		strings.Contains(message, ".md") ||
		strings.Contains(message, ".txt") ||
		strings.Contains(message, ".json") ||
		containsAnyCorrectionPhrase(message, "file path", "folder", "directory", "the file", "this file", "that file")
	if !hasFileCue {
		return false
	}
	return containsAnyCorrectionPhrase(message,
		"is not in",
		"isn't in",
		"not in the",
		"not there",
		"not listed",
		"not found",
		"can not find",
		"cannot find",
		"can't find",
		"could not find",
		"couldn't find",
		"does not exist",
		"doesn't exist",
		"still not written",
		"still not writen",
		"not written",
		"not saved",
		"not created",
		"was not created",
		"wasn't created",
		"file was created",
		"make sure the file was created",
		"check the folder",
		"check folder",
		"check the directory",
	)
}

func looksLikeRoutingCorrection(message string) bool {
	if message == "" {
		return false
	}
	if looksExplicitSourceOrToolPreferenceCorrection(message) {
		return true
	}
	if looksExplicitClarifyCorrection(message) {
		return true
	}
	return looksExplicitRouteTeachingCue(message) && looksRouteCorrectionSignal(message)
}

func looksExplicitRouteTeachingCue(message string) bool {
	return containsAnyCorrectionPhrase(message,
		"next time",
		"remember that",
		"remember this",
		"save this routing correction",
		"save this route correction",
		"when i ask",
		"when i need",
		"when i say",
		"for prompts like this",
		"for this kind of request",
		"for this type of request",
		"this kind of request",
		"this type of request",
		"route this type of prompt to",
		"route this prompt to",
		"route these prompts to",
		"treat ",
	) || strings.Contains(message, "remember:")
}

func looksExplicitSourceOrToolPreferenceCorrection(message string) bool {
	if looksPastedCodeCorrection(message) &&
		containsAnyCorrectionPhrase(message, "don't fetch", "dont fetch", "do not fetch", "without fetching") {
		return true
	}
	hasPreferredSource := looksLocalDocumentCorrection(message) ||
		looksMemoryCorrection(message) ||
		looksWorkspaceCorrection(message) ||
		looksInternetFreshnessCorrection(message)
	if !hasPreferredSource {
		return false
	}
	if looksExplicitRouteTeachingCue(message) {
		return true
	}
	return containsAnyCorrectionPhrase(message,
		"instead",
		"instead of",
		"not internet",
		"not web",
		"not online",
		"not memory",
		"not workspace",
		"not repo",
		"not repository",
		"not local documents",
		"not local docs",
		"don't use web",
		"dont use web",
		"do not use web",
		"don't search the web",
		"dont search the web",
		"do not search the web",
		"use local docs instead",
		"use local documents instead",
		"use memory instead",
		"use workspace files instead",
		"use repo files instead",
	)
}

func looksExplicitClarifyCorrection(message string) bool {
	return looksClarifyCorrection(message) && containsAnyCorrectionPhrase(message,
		"ask me first",
		"ask what i mean",
		"ask a follow-up",
		"ask follow-up",
		"should have asked",
		"should've asked",
		"target is unclear",
		"unclear target",
		"before acting",
		"not run a tool",
		"do not run a tool",
		"don't run a tool",
		"dont run a tool",
	)
}

func looksLikeEditorialRewriteTask(message string) bool {
	return containsAnyCorrectionPhrase(message,
		"make better",
		"make this better",
		"make it better",
		"make this paragraph better",
		"rewrite",
		"rewrite this",
		"improve",
		"improve this",
		"polish",
		"polish this",
		"refine",
		"refine this",
		"make professional",
		"make it professional",
		"make this professional",
		"make this clearer",
		"make it clearer",
		"make this paragraph clearer",
		"shorten",
		"shorten this",
		"expand",
		"expand this",
		"fix grammar",
		"fix this paragraph",
		"rephrase",
		"rephrase this",
		"summarize this better",
		"summarise this better",
		"phrase it like that",
		"say it like that",
		"word it like that",
		"paragraph better",
	)
}

func looksRouteCorrectionSignal(message string) bool {
	return looksLocalDocumentCorrection(message) ||
		looksMemoryCorrection(message) ||
		looksWorkspaceCorrection(message) ||
		looksPastedCodeCorrection(message) ||
		looksInternetFreshnessCorrection(message) ||
		looksClarifyCorrection(message) ||
		containsAnyCorrectionPhrase(message, "web search", "internet search", "use web", "use internet", "search online")
}

func inferCorrectedRoute(message string) (string, []string, []string, []string, string) {
	tags := []string{}
	requiredTools := []string{}
	forbiddenTools := []string{}

	addTag := func(tag string) {
		tags = appendUniqueString(tags, tag)
	}
	addForbidden := func(tool string) {
		forbiddenTools = appendUniqueString(forbiddenTools, tool)
		addTag("avoid:" + tool)
	}
	negatesInternet := containsAnyCorrectionPhrase(message, "not internet", "not web", "not online", "instead of internet", "instead of web", "instead of online", "don't search the web", "dont search the web", "do not search the web", "don't fetch", "dont fetch", "do not fetch")
	negatesToolUse := containsAnyCorrectionPhrase(message, "don't use a tool", "dont use a tool", "do not use a tool", "should not have used a tool", "without tool")
	if negatesInternet {
		addTag("intent:avoid_tool")
		addForbidden("internet_search")
		addForbidden("internet_fetch")
	}
	if negatesToolUse {
		addTag("intent:avoid_tool")
		addForbidden("internet_search")
		addForbidden("internet_fetch")
		addForbidden("rag_search")
		addForbidden("memory_search")
		addForbidden("workspace_read")
		addTag("avoid:tool_use")
	}

	switch {
	case looksClarifyCorrection(message):
		addTag("intent:clarify_before_acting")
		addTag("intent:do_not_guess")
		addTag("route:clarify")
		return routing.RouteClarify, tags, nil, forbiddenTools, "What should I clarify before acting?"
	case looksPastedCodeCorrection(message):
		addTag("source:pasted_code")
		if containsAnyCorrectionPhrase(message, "svg") {
			addTag("source:pasted_svg")
		}
		addTag("intent:explain_only")
		addTag("route:chat_explanation")
		addForbidden("internet_search")
		addForbidden("internet_fetch")
		return routing.RouteChatExplanation, tags, nil, forbiddenTools, ""
	case looksLocalDocumentCorrection(message):
		addTag("source:local_documents")
		addTag("intent:prefer_route")
		addTag("route:rag_search")
		requiredTools = []string{"rag_search"}
		if negatesInternet {
			addForbidden("internet_search")
			addForbidden("internet_fetch")
		}
		return routing.RouteRAGSearch, tags, requiredTools, forbiddenTools, ""
	case looksMemoryCorrection(message):
		addTag("source:memory")
		addTag("intent:prefer_route")
		addTag("route:memory_search")
		requiredTools = []string{"memory_search"}
		return routing.RouteMemorySearch, tags, requiredTools, forbiddenTools, ""
	case looksWorkspaceCorrection(message):
		addTag("source:workspace")
		addTag("intent:prefer_route")
		addTag("route:workspace_read")
		requiredTools = []string{"search_files"}
		if negatesInternet {
			addForbidden("internet_search")
			addForbidden("internet_fetch")
		}
		return routing.RouteWorkspaceRead, tags, requiredTools, forbiddenTools, ""
	case looksInternetFreshnessCorrection(message):
		addTag("source:internet")
		addTag("intent:freshness_required")
		addTag("route:internet_search")
		requiredTools = []string{"internet_search"}
		return routing.RouteInternetSearch, tags, requiredTools, forbiddenTools, ""
	default:
		return "", tags, nil, forbiddenTools, ""
	}
}

func inferRouteCorrectionPattern(message string, normalized string, previousUserPrompt string, route string, tags []string) string {
	if pattern := explicitRouteCorrectionPattern(message); pattern != "" {
		return pattern
	}
	if !referencesPriorPrompt(normalized) {
		if pattern := patternFromCorrectionText(normalized, route, tags); pattern != "" {
			return pattern
		}
	}
	if referencesPriorPrompt(normalized) && strings.TrimSpace(previousUserPrompt) != "" {
		if pattern := patternFromPreviousPrompt(previousUserPrompt, route, tags); pattern != "" {
			return pattern
		}
	}
	if strings.TrimSpace(previousUserPrompt) != "" && !containsAnyCorrectionPhrase(normalized, "when i say", "treat ") {
		if pattern := patternFromPreviousPrompt(previousUserPrompt, route, tags); pattern != "" {
			return pattern
		}
	}
	return patternFromCorrectionText(normalized, route, tags)
}

func explicitRouteCorrectionPattern(message string) string {
	normalized := strings.TrimSpace(message)
	lower := strings.ToLower(normalized)
	if idx := strings.Index(lower, "when i say "); idx >= 0 {
		rest := strings.TrimSpace(normalized[idx+len("when i say "):])
		return trimPatternBoundary(firstBeforeAny(rest, ",", " use ", " do ", " search ", " should "))
	}
	if idx := strings.Index(lower, "treat "); idx >= 0 {
		rest := strings.TrimSpace(normalized[idx+len("treat "):])
		return trimPatternBoundary(firstBeforeAny(rest, " as ", " like ", ","))
	}
	if idx := strings.Index(lower, "remember:"); idx >= 0 {
		rest := strings.TrimSpace(normalized[idx+len("remember:"):])
		return trimPatternBoundary(firstBeforeAny(rest, " means ", " should ", ","))
	}
	return ""
}

func patternFromPreviousPrompt(prompt string, route string, tags []string) string {
	lower := normalizeCorrectionText(prompt)
	switch route {
	case routing.RouteRAGSearch:
		for _, term := range []string{"indexed document", "indexed documents", "ingested document", "ingested documents", "uploaded document", "uploaded documents", "local document", "local documents", "documents", "docs"} {
			if strings.Contains(lower, term) {
				return term
			}
		}
	case routing.RouteInternetSearch:
		for _, term := range []string{"latest updates", "current updates", "latest", "current", "fresh"} {
			if strings.Contains(lower, term) {
				return term
			}
		}
	case routing.RouteChatExplanation:
		if strings.Contains(lower, "<svg") || strings.Contains(lower, "svg") {
			return "pasted SVG or code"
		}
		if strings.Contains(lower, "make better") || strings.Contains(lower, "improve") {
			return "pasted text or code"
		}
	case routing.RouteMemorySearch:
		if strings.Contains(lower, "memory") {
			return "memory"
		}
	case routing.RouteWorkspaceRead:
		for _, term := range []string{"repo", "repository", "workspace", "local files", "files"} {
			if strings.Contains(lower, term) {
				return term
			}
		}
	case routing.RouteClarify:
		return "unclear target"
	}
	return compactCorrectionPattern(prompt)
}

func patternFromCorrectionText(message string, route string, tags []string) string {
	switch route {
	case routing.RouteRAGSearch:
		for _, term := range []string{"indexed documents", "indexed document", "ingested documents", "ingested document", "uploaded documents", "uploaded document", "local documents", "local docs"} {
			if strings.Contains(message, term) {
				return term
			}
		}
	case routing.RouteInternetSearch:
		for _, term := range []string{"latest updates", "current updates", "latest", "current", "fresh information"} {
			if strings.Contains(message, term) {
				return term
			}
		}
	case routing.RouteMemorySearch:
		return "memory"
	case routing.RouteWorkspaceRead:
		for _, term := range []string{"repo files", "repository files", "workspace files", "local files", "repo", "workspace"} {
			if strings.Contains(message, term) {
				return term
			}
		}
	case routing.RouteChatExplanation:
		if strings.Contains(message, "svg") {
			return "pasted SVG or code"
		}
		if strings.Contains(message, "code") {
			return "pasted code"
		}
		return "pasted text"
	case routing.RouteClarify:
		return "unclear target"
	}
	return ""
}

func routeCorrectionProposalResponse(correction routing.RouteCorrection) string {
	pattern := correction.Pattern
	if pattern == "" {
		pattern = "similar prompts"
	}
	return "I can remember this: when you ask about " + plainPattern(pattern) + ", I should " + plainRouteAction(correction) + ". Save this for next time?"
}

func RouteCorrectionApprovedResponse(correction routing.RouteCorrection) string {
	return "Saved. I'll " + plainRouteAction(correction) + " for similar prompts next time."
}

func routeCorrectionClarificationQuestion() string {
	return "What should Yemaka do for similar prompts: search local documents, search memory, use web search, read workspace files, explain only, or ask a follow-up?"
}

func plainRouteAction(correction routing.RouteCorrection) string {
	switch correction.IntendedRouteCategory {
	case routing.RouteRAGSearch:
		return "search your local or ingested documents instead"
	case routing.RouteMemorySearch:
		return "search memory instead"
	case routing.RouteInternetSearch:
		return "use web search when current information is needed"
	case routing.RouteWorkspaceRead, routing.RouteFileRead:
		return "read your workspace files instead"
	case routing.RouteChatExplanation:
		return "explain or improve the pasted content without using web tools"
	case routing.RouteClarify:
		return "ask a follow-up before acting"
	default:
		return "use the corrected route"
	}
}

func plainPattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "similar prompts"
	}
	return `"` + pattern + `"`
}

func routeCorrectionTask(route string) string {
	switch route {
	case routing.RouteRAGSearch:
		return routing.TaskRAG
	case routing.RouteMemorySearch, routing.RouteInternetSearch, routing.RouteWorkspaceRead, routing.RouteFileRead:
		return routing.TaskTool
	case routing.RouteChatExplanation:
		return routing.TaskReasoning
	case routing.RouteClarify:
		return routing.TaskChat
	default:
		return ""
	}
}

func routeCorrectionCapability(route string) string {
	switch route {
	case routing.RouteRAGSearch:
		return "rag"
	case routing.RouteMemorySearch:
		return "memory"
	case routing.RouteInternetSearch:
		return "internet"
	case routing.RouteWorkspaceRead, routing.RouteFileRead:
		return "filesystem_read"
	case routing.RouteChatExplanation:
		return "chat"
	default:
		return ""
	}
}

func referencesPriorPrompt(message string) bool {
	return containsAnyCorrectionPhrase(message, "this", "that", "like this", "next time", "for prompts like this", "similar")
}

func looksLocalDocumentCorrection(message string) bool {
	return containsAnyCorrectionPhrase(message, "rag", "rag search", "ingested document", "ingested documents", "indexed document", "indexed documents", "uploaded document", "uploaded documents", "local document", "local documents", "local docs", "docs", "documents") &&
		!containsAnyCorrectionPhrase(message, "not document", "not documents", "not docs")
}

func looksMemoryCorrection(message string) bool {
	return containsAnyCorrectionPhrase(message, "memory search", "search memory", "searched memory", "in memory", "conversation history", "belongs in memory")
}

func looksWorkspaceCorrection(message string) bool {
	if containsAnyCorrectionPhrase(message, "not workspace", "not repo", "not repository") {
		return false
	}
	return containsAnyCorrectionPhrase(message, "workspace", "workspace files", "repo", "repo files", "repository", "repository files", "local files", "project files")
}

func looksInternetFreshnessCorrection(message string) bool {
	return containsAnyCorrectionPhrase(message, "latest", "current", "fresh information", "current information", "web search", "search the web", "internet search", "search online", "use web", "use internet")
}

func looksPastedCodeCorrection(message string) bool {
	return containsAnyCorrectionPhrase(message, "pasted code", "pasted svg", "svg", "code", "don't fetch", "dont fetch", "do not fetch") &&
		containsAnyCorrectionPhrase(message, "explain", "improve", "make better", "just", "without fetching", "don't fetch", "dont fetch", "do not fetch")
}

func looksClarifyCorrection(message string) bool {
	return containsAnyCorrectionPhrase(message, "ask me first", "ask what i mean", "ask a follow-up", "ask follow-up", "should have asked", "should've asked", "don't guess", "dont guess", "do not guess", "target is unclear", "unclear target", "followed up")
}

func normalizeCorrectionText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func normalizeCorrectionReply(value string) string {
	value = normalizeCorrectionText(value)
	replacer := strings.NewReplacer(",", "", ".", "", "!", "", "?", "", ":", "", ";", "", "'", "")
	return strings.Join(strings.Fields(replacer.Replace(value)), " ")
}

func containsAnyCorrectionPhrase(content string, phrases ...string) bool {
	content = normalizeCorrectionText(content)
	contentTokens := correctionWordTokens(content)
	contentTokenSet := map[string]bool{}
	for _, token := range contentTokens {
		contentTokenSet[token] = true
	}
	for _, phrase := range phrases {
		phrase = normalizeCorrectionText(phrase)
		if phrase == "" {
			continue
		}
		tokens := correctionWordTokens(phrase)
		switch len(tokens) {
		case 0:
			continue
		case 1:
			if tokens[0] == phrase && contentTokenSet[phrase] {
				return true
			}
			if strings.ContainsAny(phrase, ":;/@#<>=+-") && strings.Contains(content, phrase) {
				return true
			}
		default:
			if containsCorrectionTokenSequence(contentTokens, tokens) {
				return true
			}
			if strings.ContainsAny(phrase, ":;/@#<>=+-") && strings.Contains(content, phrase) {
				return true
			}
		}
	}
	return false
}

func containsCorrectionTokenSequence(contentTokens []string, phraseTokens []string) bool {
	if len(phraseTokens) == 0 || len(phraseTokens) > len(contentTokens) {
		return false
	}
	for i := 0; i <= len(contentTokens)-len(phraseTokens); i++ {
		matched := true
		for j, token := range phraseTokens {
			if contentTokens[i+j] != token {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func correctionWordTokens(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_')
	})
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func firstBeforeAny(value string, separators ...string) string {
	out := value
	lower := strings.ToLower(value)
	best := -1
	for _, separator := range separators {
		separator = strings.ToLower(separator)
		if index := strings.Index(lower, separator); index >= 0 && (best < 0 || index < best) {
			best = index
		}
	}
	if best >= 0 {
		out = value[:best]
	}
	return out
}

func trimPatternBoundary(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\n\r\"'“”‘’.,:;!?")
	return compactCorrectionPattern(value)
}

func compactCorrectionPattern(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= 80 {
		return value
	}
	return strings.TrimSpace(value[:80])
}
