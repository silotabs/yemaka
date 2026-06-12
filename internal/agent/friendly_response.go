package agent

import (
	"fmt"
	"strings"

	"yemaka/internal/routing"
)

func FriendlyToolFailureResponse(decision ExecutionDecision, err error) string {
	tool := strings.TrimSpace(decision.ToolName)
	if tool == "" {
		tool = "the requested tool"
	}
	detail, next := friendlyToolFailureParts(decision, err)
	var builder strings.Builder
	fmt.Fprintf(&builder, "I could not use `%s` for this request yet.", tool)
	if detail != "" {
		fmt.Fprintf(&builder, "\n\n**What happened:** %s", detail)
	}
	if next != "" {
		fmt.Fprintf(&builder, "\n\n**Next step:** %s", next)
	}
	if !strings.Contains(strings.ToLower(builder.String()), "tool logs") {
		fmt.Fprintf(&builder, "\n\nI kept the technical detail in Tool Logs so the chat stays readable.")
	}
	return strings.TrimSpace(builder.String())
}

func RouteAwareToolFailureResponse(plan Plan, decision ExecutionDecision, err error) string {
	tool := strings.TrimSpace(decision.ToolName)
	lower := strings.ToLower(strings.Join([]string{tool, decision.Reason, errorText(err)}, " "))
	recovery := RouteRecoveryForExecution(plan, decision, nil, err)
	if recovery.Trigger == routing.RouteRecoveryTriggerMissingPermission && strings.Contains(strings.ToLower(recovery.Reason), "conversation copy") {
		return "I could not use the attached upload as a filesystem path.\n\n**What happened:** The upload is available as conversation context, but it is not the same as a granted local folder or file path.\n\n**Next step:** I can use the attached content in this chat, or you can grant/select the original folder before I use filesystem tools. I did not treat the upload copy as a writable local path."
	}
	if routeBlocksTool(plan, tool) {
		return "I paused because this tool is outside the selected route lane.\n\n**What happened:** The active route is `" + plan.RouteCategory + "` with lane `" + plan.RouteLane + "`, but the failed tool was `" + tool + "`.\n\n**Next step:** I should clarify the source or stay inside the selected route instead of switching tools."
	}
	if strings.HasPrefix(tool, "internet_") && !plan.RouteUsesInternet && plan.RouteCategory != "" {
		return "I paused because this route was not supposed to use web search or fetch.\n\n**What happened:** The active route is `" + plan.RouteCategory + "`, but the failed tool was `" + tool + "`.\n\n**Next step:** I should clarify the source or stay inside the selected route instead of switching tools."
	}
	if tool == "rag_search" && (strings.Contains(lower, "ollama embed") || strings.Contains(lower, "embedding") || strings.Contains(lower, "embed request")) {
		return "I could not search the local documents yet.\n\n**What happened:** The local document search step needs the local embedding/runtime path, but it did not return a usable result from this session.\n\n**Next step:** Check that Ollama is running and the configured local embedding model is reachable, then retry the same local-document search. I did not use web search or stale memory as a replacement."
	}
	if tool == "rag_search" && plan.RouteCategory == routing.RouteRAGSearch && routeFailureLooksNoEvidence(lower) {
		return "I could not find enough local-document evidence for this route.\n\n**What happened:** The selected route is local documents, but the document search returned no usable evidence for the target.\n\n**Next step:** Ingest or grant the right documents, choose a different local source, or ask me to switch routes explicitly. I did not answer from stale memory or web search as a substitute."
	}
	if tool == "internet_search" && plan.RouteCategory == routing.RouteRAGSearch {
		return "I paused because this should stay in local documents, not web search.\n\n**Next step:** I should search the ingested documents or ask which local source to use."
	}
	if strings.HasPrefix(tool, "internet_") && plan.RouteCategory == routing.RouteInternetSearch && routeFailureLooksProvider(lower) {
		detail, next := friendlyToolFailureParts(decision, err)
		if detail == "" {
			detail = "The selected web-search route needs a configured and reachable provider."
		}
		if next == "" {
			next = "Check Internet/Search settings, then retry the same current-info request."
		}
		return "I stayed in the web-search route, but the provider was not ready.\n\n**What happened:** " + detail + "\n\n**Next step:** " + next
	}
	if routeIsWorkspaceOrFile(plan.RouteCategory) && routeFailureLooksPermission(lower) {
		return "I could not access the requested local path for this route.\n\n**What happened:** The selected route needs workspace or document access, but the OS or Yemaka workspace policy did not allow this path from the current session.\n\n**Next step:** Grant the exact folder in Documents or Workspace settings, then retry. I did not use another source as a workaround."
	}
	if routeFailureLooksMissingCapability(lower) {
		return "This task needs a capability that is not available in the current route lane.\n\n**What happened:** The selected route could not find a safe typed tool or enabled capability for the requested action.\n\n**Next step:** Configure or enable the required local capability, or ask me to propose a small generated extension with manifest, permissions, tests, and approval."
	}
	return FriendlyToolFailureResponse(decision, err)
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func routeBlocksTool(plan Plan, tool string) bool {
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return false
	}
	if plan.RouteCategory == routing.RouteChatExplanation || plan.RouteCategory == routing.RouteClarify {
		return false
	}
	for _, blocked := range plan.RouteBlockedTools {
		if strings.TrimSpace(blocked) == tool {
			return true
		}
	}
	return false
}

func routeFailureLooksProvider(text string) bool {
	return strings.Contains(text, "provider") ||
		strings.Contains(text, "searxng") ||
		strings.Contains(text, "api key") ||
		strings.Contains(text, "connection refused") ||
		strings.Contains(text, "context deadline exceeded") ||
		strings.Contains(text, "no such host")
}

func routeFailureLooksPermission(text string) bool {
	return strings.Contains(text, "operation not permitted") ||
		strings.Contains(text, "permission denied") ||
		strings.Contains(text, "workspace path is not granted") ||
		strings.Contains(text, "outside workspace")
}

func routeFailureLooksNoEvidence(text string) bool {
	return strings.Contains(text, "no retrieved local document context") ||
		strings.Contains(text, "no matching document") ||
		strings.Contains(text, "no local document") ||
		strings.Contains(text, "0 result") ||
		strings.Contains(text, "no usable evidence")
}

func routeFailureLooksMissingCapability(text string) bool {
	return strings.Contains(text, "no safe executor") ||
		strings.Contains(text, "no matching safe executor") ||
		strings.Contains(text, "not allowed in the typed low-resource tool loop") ||
		strings.Contains(text, "unknown tool") ||
		strings.Contains(text, "capability missing")
}

func routeIsWorkspaceOrFile(route string) bool {
	switch strings.TrimSpace(route) {
	case routing.RouteWorkspaceRead, routing.RouteFileRead, routing.RouteFileWrite, routing.RouteRAGSearch, routing.RouteSettingsAction:
		return true
	default:
		return false
	}
}

func FriendlyToolFailureSummary(decision ExecutionDecision, err error) string {
	detail, next := friendlyToolFailureParts(decision, err)
	if detail == "" {
		detail = "The local tool stopped before it could return a safe observation."
	}
	if next == "" {
		return detail
	}
	return detail + " Next step: " + next
}

func DocumentIngestResponse(result ExecutionResult) string {
	target := toolContextField(result.Context, "target")
	root := toolContextField(result.Context, "root")
	indexed := firstNonEmptyString(toolContextField(result.Context, "files_indexed"), "0")
	unchanged := firstNonEmptyString(toolContextField(result.Context, "files_unchanged"), "0")
	skipped := firstNonEmptyString(toolContextField(result.Context, "files_skipped"), "0")
	chunks := firstNonEmptyString(toolContextField(result.Context, "chunks_created"), "0")
	bytes := firstNonEmptyString(toolContextField(result.Context, "bytes_indexed"), "0")
	note := toolContextField(result.Context, "note")
	reasons := toolContextListAfter(result.Context, "skipped_reasons")

	var builder strings.Builder
	builder.WriteString("Document ingest completed.")
	if target != "" {
		fmt.Fprintf(&builder, "\n\nPath: `%s`", target)
	} else if root != "" {
		fmt.Fprintf(&builder, "\n\nPath: `%s`", root)
	}
	fmt.Fprintf(&builder, "\n\nResult: %s indexed, %s unchanged, %s skipped, %s chunks, %s bytes.", indexed, unchanged, skipped, chunks, bytes)
	if note != "" {
		fmt.Fprintf(&builder, "\n\n%s", note)
	} else if indexed == "0" && unchanged == "0" {
		builder.WriteString("\n\nNo new document chunks were created. Yemaka only knows the scan counts; it will not guess the skipped file names or formats unless it lists the folder.")
	}
	if len(reasons) > 0 {
		builder.WriteString("\n\nSkipped reasons:")
		for _, reason := range reasons {
			fmt.Fprintf(&builder, "\n- %s", reason)
		}
	} else if skipped != "0" {
		builder.WriteString("\n\nSkipped file names were not returned by the ingest tool. Ask me to list the folder if you want to inspect what was skipped.")
	}
	return strings.TrimSpace(builder.String())
}

func toolContextField(context string, name string) string {
	prefix := strings.ToLower(strings.TrimSpace(name)) + ":"
	for _, line := range strings.Split(context, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	return ""
}

func toolContextListAfter(context string, name string) []string {
	header := strings.ToLower(strings.TrimSpace(name)) + ":"
	lines := strings.Split(context, "\n")
	var out []string
	inList := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inList {
				break
			}
			continue
		}
		lower := strings.ToLower(trimmed)
		if !inList {
			if lower == header {
				inList = true
			}
			continue
		}
		if !strings.HasPrefix(trimmed, "- ") {
			break
		}
		item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func FriendlyBlockedDecisionResponse(decision ExecutionDecision) string {
	tool := strings.TrimSpace(decision.ToolName)
	if tool == "" {
		tool = "that action"
	}
	reason := strings.TrimSpace(decision.Reason)
	detail, next := friendlyToolFailureParts(decision, fmt.Errorf("%s", reason))
	if detail == "" && reason != "" {
		detail = friendlySentence(reason)
	}
	if detail == "" {
		detail = "The request is outside the currently approved safe local policy."
	}
	if next == "" {
		next = friendlyNextStepForTool(tool)
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "I paused before running `%s`.", tool)
	fmt.Fprintf(&builder, "\n\n**Why:** %s", detail)
	if next != "" {
		fmt.Fprintf(&builder, "\n\n**Next step:** %s", next)
	}
	return strings.TrimSpace(builder.String())
}

func FriendlyErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	raw := strings.TrimSpace(err.Error())
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "local model failed") || strings.Contains(lower, "cloud fallback failed") || strings.Contains(lower, "ollama"):
		return "The selected model did not return a usable response. Check that Ollama is running and the selected model is installed, then try again."
	case strings.Contains(lower, "unsupported model provider"):
		return "The selected model provider is not available for this local agent session. Choose an Ollama model in Models or update the model role in Settings."
	case strings.Contains(lower, "streaming is not supported"):
		return "This connection cannot stream responses. Refresh the app or use the local web/desktop interface again."
	case strings.Contains(lower, "memory store") || strings.Contains(lower, "sqlite") || strings.Contains(lower, "sql logic error"):
		return "Yemaka hit a local memory or index issue. Refresh the app, run the readiness check, and retry after the local database is reachable."
	case strings.Contains(lower, "request is required"):
		return "Send a message first, then I can help."
	case strings.Contains(lower, "tool response was not valid json") ||
		strings.Contains(lower, "tool response did not match the schema"):
		return "The local tool returned a response I couldn't understand. I kept the technical detail in Tool Logs so the chat stays readable."
	case strings.Contains(lower, "tool response was not valid observation") ||
		strings.Contains(lower, "tool response did not match the observation schema"):
		return "The local tool returned a response that didn't fit the expected format. I kept the technical detail in Tool Logs so the chat stays readable."
	default:
		return "Something stopped this request before I could finish it cleanly. The technical detail is in Tool Logs and the local console."
	}
}

func friendlyToolFailureParts(decision ExecutionDecision, err error) (string, string) {
	raw := ""
	if err != nil {
		raw = err.Error()
	}
	lower := strings.ToLower(strings.TrimSpace(strings.Join([]string{decision.ToolName, decision.Reason, raw}, " ")))

	switch {
	case strings.Contains(lower, "could not reach searxng search provider") ||
		strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "client.timeout exceeded") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "no such host"):
		provider := friendlySearchProviderName(lower)
		if provider == "" {
			provider = "The selected search provider"
		}
		return provider + " is configured, but I could not reach it from this session.", fmt.Sprintf("Check the %s endpoint in Settings, confirm it opens in a browser, then retry. If you give me a direct public URL, I can use controlled fetch instead of search.", provider)
	case strings.Contains(lower, "provider returned http") ||
		strings.Contains(lower, "search provider returned http"):
		provider := friendlySearchProviderName(lower)
		if provider == "" {
			provider = "The selected search provider"
		}
		return provider + " responded, but it did not return a successful search response.", fmt.Sprintf("Check the %s endpoint, credentials, rate limits, and response format, then retry.", provider)
	case strings.Contains(lower, "provider \"none\" is not supported") ||
		strings.Contains(lower, "search provider \"none\"") ||
		strings.Contains(lower, "unsupported search provider"):
		return "Web search is not fully configured for this profile yet.", "Choose a supported provider in Settings. Tavily, Serper.dev, Brave, Firecrawl, and Mojeek need API key environment variables; SearXNG needs a reachable endpoint; Wikimedia and DuckDuckGo can be used as limited no-key fallbacks."
	case strings.Contains(lower, "api key environment variable") ||
		strings.Contains(lower, "not visible to yemaka") ||
		strings.Contains(lower, "needs_auth"):
		provider := friendlySearchProviderName(lower)
		envName := friendlySearchProviderEnvName(lower)
		if provider == "" {
			provider = "The selected search provider"
		}
		if envName != "" {
			return provider + " is selected, but Yemaka cannot see its API key environment variable.", fmt.Sprintf("Set %s in the environment visible to Yemaka before launching the app, then restart. On macOS, apps opened from Finder may not inherit shell variables from .zshrc; launch from Terminal or use launchctl setenv for app-visible values.", envName)
		}
		return provider + " is selected, but its API key is not visible to Yemaka.", "Set the provider's API key environment variable before launching the app, then restart Yemaka."
	case strings.Contains(lower, "internet search provider") ||
		strings.Contains(lower, "internet.search.provider") ||
		strings.Contains(lower, "search provider") ||
		strings.Contains(lower, "searxng search requires") ||
		strings.Contains(lower, "brave search requires"):
		provider := friendlySearchProviderName(lower)
		if provider == "" {
			return "Internet access is enabled, but web search still needs a ready search provider.", "Choose a search provider in Settings. Tavily, Serper.dev, Brave, Firecrawl, and Mojeek need API key environment variables; SearXNG needs a reachable endpoint; Wikimedia and DuckDuckGo can be used as limited no-key fallbacks. If you give me a specific public URL, I can use controlled fetch instead of search."
		}
		return provider + " is selected, but it is not ready for search yet.", friendlySearchProviderSetupStep(provider)
	case strings.Contains(lower, "internet access is disabled") ||
		strings.Contains(lower, "internet access is disabled or not profile-approved") ||
		strings.Contains(lower, "not profile-approved"):
		return "Controlled internet access is disabled or not approved for this session.", "Enable internet access in Settings for this profile, or keep the task local."
	case strings.Contains(lower, "requires url") ||
		strings.Contains(lower, "explicit http") ||
		strings.Contains(lower, "explicit public"):
		return "Controlled fetch needs a specific public URL or domain before it can run.", "Send the exact `https://...` URL or domain you want me to inspect."
	case strings.Contains(lower, "workspace path is not granted"):
		if decision.ToolName == "ingest_documents" {
			return "The document path is outside the current workspace and has not been granted.", "Grant that folder from Documents or Workspace settings, then run ingest again. I will not index outside folders without an explicit grant."
		}
		return "The requested path is outside the current workspace and has not been granted.", "Grant that exact folder in Workspace settings, then retry the same read or list request."
	case strings.Contains(lower, "outside workspace"):
		return "The requested path is outside the current workspace boundary.", "Grant that exact folder in Workspace settings, choose a folder inside the current workspace, or switch policy mode only if you intend broader access."
	case strings.Contains(lower, "requires path") ||
		strings.Contains(lower, "workspace path"):
		return "The file tool needs an explicit workspace path.", "Tell me the file path, or ask me to search the workspace first."
	case strings.Contains(lower, "stat file") && strings.Contains(lower, "no such file"):
		if target := failureCommandTarget(decision); target != "" {
			return "I checked `" + target + "`, but that file was not found in the current workspace.", "Check the workspace root and file path, then ask me to read the exact path you see."
		}
		return "That file was not found in the current workspace.", "Check the workspace root and file path, then ask me to read the exact path you see."
	case strings.Contains(lower, "stat directory") && strings.Contains(lower, "no such file"):
		if target := failureCommandTarget(decision); target != "" {
			return "I checked `" + target + "`, but that directory was not found in the current workspace.", "Check the workspace root and folder path, then ask me to list the exact folder you see."
		}
		return "That directory was not found in the current workspace.", "Check the workspace root and folder path, then ask me to list the exact folder you see."
	case strings.Contains(lower, "path is a directory") ||
		strings.Contains(lower, "path is not a directory"):
		return "The path type did not match the requested file operation.", "Ask me to list a folder with `list_files`, or give me a specific file path for `read_file`."
	case strings.Contains(lower, "secret") || strings.Contains(lower, "hidden file"):
		return "The safety policy blocked access to a sensitive or hidden file.", "Use a non-secret project file, or explicitly grant the safe path you want Yemaka to inspect."
	case strings.Contains(lower, "no module named pytest") ||
		strings.Contains(lower, "missing `pytest`") ||
		strings.Contains(lower, "missing pytest"):
		return "The Python test runner started, but this local environment is missing `pytest`.", "Install `pytest` for this Python environment, or point me at a test command that is already available. I did not change any files."
	case strings.Contains(lower, "python tests were requested") &&
		strings.Contains(lower, "no python tests"):
		return "I could not find Python tests or pytest configuration near the requested path.", "Point me at the Python project folder or a `test_*.py` file, then I can rerun the safe test check."
	case strings.Contains(lower, "command blocked"):
		return "The shell command was blocked by the safe command policy.", "Use a safer command, or approve the action explicitly after reviewing the risk."
	case strings.Contains(lower, "no safe executor") ||
		strings.Contains(lower, "no matching safe executor") ||
		strings.Contains(lower, "not allowed in the typed low-resource tool loop"):
		return "This capability is not in the current thin core tool set.", "Ask me to propose a small generated extension, then approve it only after the manifest and tests look right."
	case strings.Contains(lower, "rag_search requires an open rag store") ||
		strings.Contains(lower, "rag store") ||
		strings.Contains(lower, "no retrieved local document context"):
		return "Local document search is not ready for this session.", "Ingest documents or reopen the RAG store, then retry the question."
	case strings.Contains(lower, "sql logic error") ||
		strings.Contains(lower, "no such column") ||
		strings.Contains(lower, "sqlite"):
		return "The local index hit a database query or migration issue while searching.", "Refresh the app or rerun the local readiness check. If it repeats, re-index the affected memory or documents."
	case strings.Contains(lower, "ollama") ||
		strings.Contains(lower, "model failed") ||
		strings.Contains(lower, "chat request failed"):
		return "The local model runtime did not return a usable response.", "Check that Ollama is running and the selected model is installed, then try again."
	default:
		if strings.TrimSpace(raw) != "" {
			return "The local tool stopped before it could return a safe observation.", "Try again with a narrower file, URL, or command. If the task needs a missing capability, I can propose a small generated extension."
		}
		return "", friendlyNextStepForTool(decision.ToolName)
	}
}

func friendlySearchProviderName(text string) string {
	for _, provider := range []struct {
		Name    string
		Needles []string
	}{
		{Name: "Tavily", Needles: []string{"tavily"}},
		{Name: "Serper.dev", Needles: []string{"serper"}},
		{Name: "Firecrawl", Needles: []string{"firecrawl"}},
		{Name: "Brave Search", Needles: []string{"brave"}},
		{Name: "Mojeek", Needles: []string{"mojeek"}},
		{Name: "SearXNG", Needles: []string{"searxng"}},
		{Name: "Wikimedia", Needles: []string{"wikimedia"}},
		{Name: "DuckDuckGo", Needles: []string{"duckduckgo"}},
	} {
		for _, needle := range provider.Needles {
			if strings.Contains(text, needle) {
				return provider.Name
			}
		}
	}
	for _, provider := range []struct {
		Name    string
		Needles []string
	}{
		{Name: "Tavily", Needles: []string{"tavily_api_key"}},
		{Name: "Serper.dev", Needles: []string{"serper_api_key"}},
		{Name: "Firecrawl", Needles: []string{"firecrawl_api_key"}},
		{Name: "Brave Search", Needles: []string{"brave_search_api_key"}},
		{Name: "Mojeek", Needles: []string{"mojeek_api_key"}},
	} {
		for _, needle := range provider.Needles {
			if strings.Contains(text, needle) {
				return provider.Name
			}
		}
	}
	return ""
}

func friendlySearchProviderEnvName(text string) string {
	for _, envName := range []string{"TAVILY_API_KEY", "SERPER_API_KEY", "FIRECRAWL_API_KEY", "BRAVE_SEARCH_API_KEY", "MOJEEK_API_KEY"} {
		if strings.Contains(text, strings.ToLower(envName)) {
			return envName
		}
	}
	return ""
}

func friendlySearchProviderSetupStep(provider string) string {
	switch strings.ToLower(provider) {
	case "tavily":
		return "Set TAVILY_API_KEY in the environment visible to Yemaka, keep the Tavily endpoint blank unless you need an override, then restart the app."
	case "serper.dev":
		return "Set SERPER_API_KEY in the environment visible to Yemaka, keep the Serper.dev endpoint blank unless you need an override, then restart the app."
	case "firecrawl":
		return "Set FIRECRAWL_API_KEY in the environment visible to Yemaka, keep the Firecrawl endpoint blank unless you need an override, then restart the app."
	case "brave search":
		return "Set BRAVE_SEARCH_API_KEY in the environment visible to Yemaka, keep the Brave endpoint blank unless you need an override, then restart the app."
	case "mojeek":
		return "Set MOJEEK_API_KEY in the environment visible to Yemaka, keep the Mojeek endpoint blank unless you need an override, then restart the app."
	case "searxng":
		return "Set a reachable SearXNG-compatible endpoint in Settings, confirm it opens in a browser, then retry."
	default:
		return "Check the selected provider's endpoint/API key settings, then restart Yemaka if you changed environment variables."
	}
}

func failureCommandTarget(decision ExecutionDecision) string {
	for _, part := range decision.Command {
		part = strings.TrimSpace(part)
		if part == "" || part == decision.ToolName {
			continue
		}
		return part
	}
	return ""
}

func friendlyNextStepForTool(tool string) string {
	switch strings.TrimSpace(tool) {
	case "read_file":
		return "Tell me the exact workspace file path to read."
	case "search_files", "symbol_search":
		return "Give me a focused search phrase or symbol name."
	case "rag_search":
		return "Ingest or select documents first, then ask the question again."
	case "internet_fetch", "internet_head":
		return "Give me the public URL or domain to inspect."
	case "internet_search":
		return "Configure a search provider in Settings, then retry the search."
	case "internet_crawl":
		return "Open Automation > Internet Service > Bounded Crawl, set limits, check Task approved, then run the crawl."
	case "run_tests":
		return "Confirm the test command or ask me to inspect the project first."
	case "git_diff":
		return "Run this in a git workspace with local changes."
	case "shell_command":
		return "Use a safer command, or approve the action explicitly after reviewing the risk."
	case "propose_extension":
		return "Review the proposed extension manifest and tests, then approve if it looks safe and useful."
	case "list_files":
		return "Tell me the exact workspace folder path to list, or ask me to search for files first."
	case "code_analysis":
		return "Give me a specific code question, or ask me to analyze a specific file or folder."
	case "python_test_check":
		return "Point me at the Python project folder or a `test_*.py` file, then I can check for testable Python tests."
	case "sql_query":
		return "Give me a more specific question or a narrower dataset to query."
	default:
		return "Ask me to inspect the current capability gap or propose the smallest safe extension."
	}
}

func friendlySentence(input string) string {
	text := strings.TrimSpace(input)
	if text == "" {
		return ""
	}
	text = strings.TrimSuffix(text, ".")
	if text == "" {
		return ""
	}
	return text + "."
}
