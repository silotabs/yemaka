package learning

import (
	"context"
	"fmt"
	"strings"

	"yemaka/internal/memory"
	"yemaka/internal/modelprofiles"
)

const ModelProfileDraftSchema = "yemaka.model_profile_draft.v1"

type ModelProfileDraftInput struct {
	ConversationID string  `json:"conversationId"`
	Name           string  `json:"name,omitempty"`
	BaseModel      string  `json:"baseModel,omitempty"`
	Temperature    float64 `json:"temperature,omitempty"`
	NumCtx         int     `json:"numCtx,omitempty"`
}

type ModelProfileDraftReport struct {
	Schema            string                        `json:"schema"`
	ConversationID    string                        `json:"conversationId"`
	Eligibility       string                        `json:"eligibility"`
	Reason            string                        `json:"reason"`
	MessageCount      int                           `json:"messageCount"`
	AssistantMessages int                           `json:"assistantMessages"`
	ToolRunCount      int                           `json:"toolRunCount"`
	FailedToolRuns    int                           `json:"failedToolRuns"`
	ObservedTraits    []string                      `json:"observedTraits"`
	PrivacyFilter     PrivacyFilterReport           `json:"privacyFilter"`
	SafetySummary     []string                      `json:"safetySummary"`
	Readiness         modelprofiles.ReadinessReport `json:"readiness"`
}

func DraftModelProfileFromConversation(ctx context.Context, store *memory.Store, input ModelProfileDraftInput) (modelprofiles.Profile, ModelProfileDraftReport, error) {
	if store == nil {
		return modelprofiles.Profile{}, ModelProfileDraftReport{}, fmt.Errorf("memory store is required")
	}
	conversationID := strings.TrimSpace(input.ConversationID)
	if conversationID == "" {
		return modelprofiles.Profile{}, ModelProfileDraftReport{}, fmt.Errorf("conversation id is required")
	}
	conversation, err := store.GetConversation(ctx, conversationID)
	if err != nil {
		return modelprofiles.Profile{}, ModelProfileDraftReport{}, err
	}
	messages, err := store.ListConversationMessages(ctx, conversationID, 500)
	if err != nil {
		return modelprofiles.Profile{}, ModelProfileDraftReport{}, err
	}
	toolRuns, err := store.ListToolRunsForConversation(ctx, conversationID, 500)
	if err != nil {
		return modelprofiles.Profile{}, ModelProfileDraftReport{}, err
	}

	assistantMessages, assistantModel, traits, filter := reviewedAssistantTraits(messages)
	failedToolRuns := countFailedToolRuns(toolRuns)
	report := ModelProfileDraftReport{
		Schema:            ModelProfileDraftSchema,
		ConversationID:    conversationID,
		Eligibility:       "eligible",
		Reason:            "Conversation has reviewed user and assistant turns with no failed tool runs; raw transcript text was not copied into the profile.",
		MessageCount:      len(messages),
		AssistantMessages: assistantMessages,
		ToolRunCount:      len(toolRuns),
		FailedToolRuns:    failedToolRuns,
		ObservedTraits:    traits,
		PrivacyFilter:     filter,
		SafetySummary: []string{
			"draft only; save remains explicit",
			"raw transcript text is not copied",
			"does not train or upload data",
			"does not auto-switch model roles",
			"does not download or pull models",
			"validated by model profile safety rules before preview",
		},
	}
	if len(messages) < 2 || assistantMessages == 0 {
		report.Eligibility = "blocked"
		report.Reason = "A reviewed profile draft requires at least one user turn and one assistant response."
		return modelprofiles.Profile{}, report, fmt.Errorf("%s", report.Reason)
	}
	if failedToolRuns > 0 {
		report.Eligibility = "blocked"
		report.Reason = "A reviewed profile draft requires a successful conversation with no failed tool runs."
		return modelprofiles.Profile{}, report, fmt.Errorf("%s", report.Reason)
	}

	baseModel := strings.TrimSpace(input.BaseModel)
	if baseModel == "" {
		baseModel = assistantModel
	}
	if baseModel == "" {
		report.Eligibility = "blocked"
		report.Reason = "Provide a base local model because the conversation does not record an assistant model."
		return modelprofiles.Profile{}, report, fmt.Errorf("%s", report.Reason)
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "Reviewed " + safeDraftProfileName(conversation.Title, conversationID)
	}
	profile := modelprofiles.Profile{
		Name:        name,
		Description: "Drafted from a reviewed successful Yemaka conversation without copying raw transcript text.",
		BaseModel:   baseModel,
		System:      reviewedConversationSystem(traits),
		Parameters: modelprofiles.Parameters{
			Temperature: input.Temperature,
			NumCtx:      input.NumCtx,
		},
		Metadata: modelprofiles.Metadata{
			Purpose:     "reviewed successful conversation behavior",
			RefinedFrom: conversationID,
			CreatedBy:   "yemaka",
		},
		Tags: append([]string{"reviewed", "local", "trajectory"}, traits...),
	}
	profile = modelprofiles.Normalize(profile)
	if err := modelprofiles.Validate(profile); err != nil {
		return modelprofiles.Profile{}, report, err
	}
	report.Readiness = modelprofiles.AssessReadiness(profile)
	return profile, report, nil
}

func reviewedAssistantTraits(messages []memory.Message) (int, string, []string, PrivacyFilterReport) {
	var assistantMessages int
	var assistantModel string
	var totalChars int
	var structured bool
	var codeAware bool
	var clarifying bool
	var evidenceAware bool
	var localFirst bool
	var filter PrivacyFilterReport
	for _, message := range messages {
		if strings.TrimSpace(strings.ToLower(message.Role)) != "assistant" {
			continue
		}
		content, report := SanitizeText(message.Content)
		filter = mergeReports(filter, report)
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		assistantMessages++
		totalChars += len(content)
		if strings.TrimSpace(message.Model) != "" {
			assistantModel = strings.TrimSpace(message.Model)
		}
		lower := strings.ToLower(content)
		structured = structured || strings.Contains(content, "\n- ") || strings.Contains(content, "\n* ") || strings.Contains(content, "\n1. ")
		codeAware = codeAware || strings.Contains(content, "```") || strings.Contains(lower, "code") || strings.Contains(lower, "function")
		clarifying = clarifying || strings.Contains(content, "?")
		evidenceAware = evidenceAware || strings.Contains(lower, "source") || strings.Contains(lower, "evidence") || strings.Contains(lower, "context")
		localFirst = localFirst || strings.Contains(lower, "local") || strings.Contains(lower, "workspace")
	}
	traits := []string{}
	if assistantMessages > 0 && totalChars/assistantMessages <= 1200 {
		traits = append(traits, "concise")
	}
	if structured {
		traits = append(traits, "structured")
	}
	if codeAware {
		traits = append(traits, "code-aware")
	}
	if clarifying {
		traits = append(traits, "clarifying")
	}
	if evidenceAware {
		traits = append(traits, "evidence-aware")
	}
	if localFirst {
		traits = append(traits, "local-first")
	}
	if len(traits) == 0 && assistantMessages > 0 {
		traits = append(traits, "clear-answer")
	}
	return assistantMessages, assistantModel, traits, filter
}

func countFailedToolRuns(toolRuns []memory.ToolRun) int {
	var failed int
	for _, run := range toolRuns {
		status := strings.TrimSpace(strings.ToLower(run.Status))
		if status == "" || status == "completed" || status == "success" || status == "ok" {
			continue
		}
		failed++
	}
	return failed
}

func reviewedConversationSystem(traits []string) string {
	if len(traits) == 0 {
		traits = []string{"clear-answer"}
	}
	return strings.TrimSpace(fmt.Sprintf(`You are Yemaka, a local-first assistant. This behavior profile was drafted from a reviewed successful conversation without copying raw transcript text.

Follow these constraints:
- Stay concise, careful, and local-first.
- Use tools only when Yemaka exposes them and policy approval is satisfied.
- Ask a clarification when scope, source, target, or permission is unclear.
- Use provided local context or tool results when evidence is required.
- When optional context is absent and the selected route allows general knowledge, answer from general model knowledge with a clear caveat.
- Do not expose hidden reasoning, tool requests, raw tool transcripts, secrets, or private memory.
- Preserve these reviewed response traits: %s.`, strings.Join(traits, ", ")))
}

func safeDraftProfileName(title string, conversationID string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = strings.TrimSpace(conversationID)
	}
	if len(title) > 48 {
		title = strings.TrimSpace(title[:48])
	}
	if title == "" {
		return "Conversation Profile"
	}
	return title
}
