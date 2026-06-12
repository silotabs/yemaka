package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"yemaka/internal/attachments"
	"yemaka/internal/config"
	"yemaka/internal/memory"
	"yemaka/internal/modelprofiles"
	"yemaka/internal/models"
	promptcore "yemaka/internal/prompt"
	"yemaka/internal/routing"
)

type Service struct {
	Router           *models.Router
	Runtime          models.Runtime
	RuntimeFactory   func(config.ModelConfig) (models.Runtime, error)
	Memory           *memory.Store
	CloudFallback    *CloudFallback
	ToolExecutor     ToolExecutor
	Retriever        Retriever
	KnowledgeContext KnowledgeContextProvider
	CapabilityGap    *CapabilityGapRouter
	ReplayStore      ReplaySaver
	ModelProfileDir  string
	PromptOptions    promptcore.Options
	PolicyMode       string
	InternetTools    bool
	InternetSearch   bool
}

type CloudFallback struct {
	Config  config.CloudFallbackConfig
	Runtime models.Runtime
}

type appliedModelProfile struct {
	Name   string
	Status string
	System string
}

func (p appliedModelProfile) SystemGuidance() string {
	system := strings.TrimSpace(p.System)
	if strings.TrimSpace(p.Name) == "" || system == "" || p.Status != "applied" {
		return ""
	}
	return " Applied local model profile: " + strings.TrimSpace(p.Name) + ". " +
		"Profile guidance is subordinate to Yemaka safety, policy, route, evidence, and user instructions:\n" + system
}

type ChatInput struct {
	Content        string
	ConversationID string
}

type AskInput struct {
	ConversationID     string
	ParentMessageID    string
	Content            string
	AttachmentIDs      []string
	WorkspaceContext   string
	Sources            []string
	SourceKind         string
	SkillName          string
	SkillVersion       string
	SkillRequiredTools []string
	SkillInstructions  string
}

type executionTrace struct {
	Decision ExecutionDecision
	Result   *ExecutionResult
}

type toolRunMetadata struct {
	SessionID          string
	UserMessageID      string
	AssistantMessageID string
	ParentMessageID    string
	VariantIndex       int
}

func newToolRunSessionID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "sess_" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

func messageSavedEvent(conversationID string, msg memory.Message, content string) Event {
	data := map[string]string{
		"role":            msg.Role,
		"message_id":      msg.ID,
		"conversation_id": conversationID,
	}
	if msg.ParentID != "" {
		data["parent_message_id"] = msg.ParentID
	}
	if msg.VariantIndex > 0 {
		data["variant_index"] = strconv.Itoa(msg.VariantIndex)
	}
	if content != "" {
		data["content"] = content
	}
	return Event{Type: EventMessageSaved, Data: data}
}

func (s *Service) saveAcceptedUserMessage(ctx context.Context, conversationID string, content string, emit EventHandler) (memory.Message, error) {
	userMessage, err := s.Memory.SaveMessage(ctx, memory.Message{
		ConversationID: conversationID,
		Role:           "user",
		Content:        content,
		Model:          "yemaka",
	})
	if err != nil {
		return memory.Message{}, err
	}
	if err := emit(messageSavedEvent(conversationID, userMessage, content)); err != nil {
		return memory.Message{}, err
	}
	return userMessage, nil
}

func (s *Service) Chat(ctx context.Context, input ChatInput, emit EventHandler) error {
	if s == nil {
		return fmt.Errorf("agent service is nil")
	}
	if input.Content == "" {
		return fmt.Errorf("chat message is empty")
	}
	if emit == nil {
		emit = func(Event) error { return nil }
	}

	runMeta := toolRunMetadata{SessionID: newToolRunSessionID()}
	if err := emit(Event{Type: EventAgentStarted, Data: map[string]string{"session_id": runMeta.SessionID}}); err != nil {
		return err
	}

	planInput := PlanInput{Content: input.Content}
	if handled, err := s.handleRouteCorrectionChat(ctx, input.ConversationID, "", input.Content, "", "", runMeta, emit); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	} else if handled {
		return nil
	}
	conversation, err := s.ensureConversation(ctx, input.ConversationID, input.Content)
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	userMessage, err := s.saveAcceptedUserMessage(ctx, conversation.ID, input.Content, emit)
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	planInput.CurrentMessageID = userMessage.ID
	runMeta.UserMessageID = userMessage.ID
	runMeta.ParentMessageID = userMessage.ID
	turn, err := s.startAgentTurn(ctx, conversation.ID, runMeta)
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if turn.ID != "" {
		defer s.markRunningAgentTurnInterrupted(context.WithoutCancel(ctx), turn.ID)
		emit = s.trackAgentTurn(ctx, turn.ID, emit)
	}
	if err := s.attachRoutingSessionContract(ctx, conversation.ID, &planInput); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.attachMemoryContext(ctx, &planInput, emit); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.attachKnowledgeContext(ctx, &planInput, emit); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.attachConversationContext(ctx, conversation.ID, &planInput); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.attachContinuationContext(ctx, conversation.ID, &planInput); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	normalizeFreshStandaloneSessionContext(&planInput)
	plan := BuildPlan(planInput)
	if advisedPlan, err := s.applyRoutingAdvisor(ctx, planInput, plan, emit); err != nil {
		return err
	} else {
		plan = advisedPlan
	}
	if err := emitPlanEvents(emit, plan); err != nil {
		return err
	}
	if plan.NeedsClarification {
		return s.completeWithoutModel(ctx, conversation.ID, runMeta, "", input.Content, "", "", plan, ExecutionDecision{Status: ExecutionNotRequired, RiskLevel: plan.RiskLevel}, nil, nil, ClarificationResponse(plan), &userMessage, emit)
	}
	var extraTraces []executionTrace
	decision, executionResult, editProposal, assistantContent, handled, err := s.resolveExecution(ctx, conversation.ID, runMeta, plan, &planInput, emit)
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if handled {
		return s.completeWithoutModel(ctx, conversation.ID, runMeta, "", input.Content, "", "", plan, decision, executionResult, editProposal, assistantContent, &userMessage, emit)
	}

	selected, selectedRole, err := s.Router.SelectWithRole(plan.ModelTask)
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	selected, profileStatus := s.applyModelProfileToSelection(selected)
	selected.Provider = models.NormalizeProvider(selected.Provider)
	if !models.IsRuntimeProvider(selected.Provider) {
		err := fmt.Errorf("unsupported model provider %q for agent loop", selected.Provider)
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	responseBehavior := s.responseBehavior(plan, selected)

	if err := emit(Event{
		Type: EventModelSelected,
		Data: map[string]string{
			"provider":           selected.Provider,
			"model":              selected.Name,
			"task_type":          plan.TaskType,
			"model_task":         plan.ModelTask,
			"model_role":         selectedRole,
			"model_profile":      profileStatus.Name,
			"profile_status":     profileStatus.Status,
			"risk_level":         plan.RiskLevel,
			"response_mode":      responseBehavior.EffectiveMode,
			"configured_mode":    responseBehavior.ConfiguredMode,
			"thinking_requested": responseBehavior.ThinkingRequested,
		},
	}); err != nil {
		return err
	}

	var systemBuilder strings.Builder
	systemBuilder.WriteString("You are Yemaka, developed by SiloTabs as a local-first AI agent optimized for low-resource systems. ")
	systemBuilder.WriteString("Follow the task plan precisely. Be concise. ")
	systemBuilder.WriteString("Do not claim access to files, tools, memory, or documents unless explicitly provided. ")
	systemBuilder.WriteString("Request confirmation before any destructive action.")
	systemBuilder.WriteString(AnswerContractGuidance(plan))
	systemBuilder.WriteString(ContextGroundingGuidance())
	systemBuilder.WriteString(ToolPolicyGuidance(plan, s.modelToolOptions()))
	systemBuilder.WriteString(FailedToolResultGuidance(executionResult))
	systemBuilder.WriteString(profileStatus.SystemGuidance())
	system := systemBuilder.String()
	useModelToolLoop := s.modelToolLoopEnabled(plan, planInput, decision, executionResult)
	if editProposal != nil && editProposal.NeedsContent {
		system += EditDraftInstructions(*editProposal)
	} else if useModelToolLoop {
		system += ModelToolInstructionsWithOptions(plan, s.modelToolOptions())
	}
	modelToolDefinitions := []models.ToolDefinition(nil)
	if useModelToolLoop {
		modelToolDefinitions = ModelToolDefinitionsWithOptions(plan, s.modelToolOptions())
	}
	promptOptions := responseBehavior.applyPromptOptions(s.promptOptions())
	messages, promptContext := PromptMessagesWithDiagnostics(plan, planInput, system, promptOptions)

	streamFirst := shouldStreamInitialModelOutput(editProposal, executionResult)
	modelTrace := responseBehavior.ModelResponseTrace
	assistantContent, responseModel, err := s.runModelWithFallback(ctx, selected, messages, emit, streamFirst, &modelTrace, responseBehavior, modelToolDefinitions)
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if useModelToolLoop && editProposal == nil {
		var modelDecision ExecutionDecision
		assistantContent, responseModel, modelDecision, executionResult, extraTraces, err = s.resolveModelToolLoop(ctx, conversation.ID, runMeta, selected, plan, planInput, messages, assistantContent, responseModel, emit, streamFirst, &modelTrace, responseBehavior)
		if err != nil {
			_ = emit(Event{Type: EventAgentError, Message: err.Error()})
			return err
		}
		if modelDecision.Status != "" {
			decision = modelDecision
		}
	}
	assistantContent = PrepareAssistantPresentation(assistantContent, decision, executionResult)
	editProposal, assistantContent, err = s.finalizeEditProposalDraftOutput(ctx, conversation.ID, runMeta, emit, decision, editProposal, assistantContent, streamFirst)
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveAndEmitPresentablePermissionRequest(ctx, conversation.ID, runMeta, decision, editProposal, emit); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	assistantContent = PrepareAssistantPresentation(assistantContent, decision, executionResult)
	verification := VerifyResponse(plan, assistantContent)
	verification = VerifyEditProposal(verification, editProposal)
	if err := emitVerification(emit, verification); err != nil {
		return err
	}
	if err := s.saveRoutingSessionContract(ctx, conversation.ID, planInput.SessionContract, plan, decision, executionResult, verification); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}

	assistantMessageID := ""
	assistantVariantIndex := 0
	if assistantContent != "" {
		assistantMessage, err := s.Memory.SaveMessage(ctx, memory.Message{
			ConversationID: conversation.ID,
			Role:           "assistant",
			Content:        assistantContent,
			Model:          responseModel,
			ParentID:       userMessage.ID,
		})
		if err != nil {
			_ = emit(Event{Type: EventAgentError, Message: err.Error()})
			return err
		}
		assistantMessageID = assistantMessage.ID
		assistantVariantIndex = assistantMessage.VariantIndex
		if err := emit(messageSavedEvent(conversation.ID, assistantMessage, assistantContent)); err != nil {
			return err
		}
	}
	runMeta.UserMessageID = userMessage.ID
	runMeta.AssistantMessageID = assistantMessageID
	runMeta.ParentMessageID = userMessage.ID
	runMeta.VariantIndex = assistantVariantIndex
	if err := s.saveVerification(ctx, conversation.ID, runMeta, plan, verification); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveExecutionTraces(ctx, conversation.ID, runMeta, extraTraces); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveExecutionDecision(ctx, conversation.ID, runMeta, decision); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveExecutionResult(ctx, conversation.ID, runMeta, decision, executionResult); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveEditProposal(ctx, conversation.ID, runMeta, editProposal); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.savePresentablePermissionRequestForDecision(ctx, conversation.ID, runMeta, decision, editProposal); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveReplayTrace(agentReplayTraceInput{
		ConversationID:     conversation.ID,
		UserMessageID:      userMessage.ID,
		AssistantMessageID: assistantMessageID,
		Input:              planInput,
		Plan:               plan,
		Decision:           decision,
		Result:             executionResult,
		ExtraTraces:        extraTraces,
		EditProposal:       editProposal,
		AssistantContent:   assistantContent,
		Verification:       verification,
		PromptContext:      promptContext,
		PromptOptions:      promptOptions,
		ModelResponse:      modelTrace,
	}); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.recordWorkflowMemory(ctx, conversation.ID); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.maybeSaveConversationSummary(ctx, conversation.ID); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}

	return emit(Event{
		Type: EventAgentCompleted,
		Data: map[string]string{"conversation_id": conversation.ID},
	})
}

func (s *Service) Ask(ctx context.Context, input AskInput, emit EventHandler) error {
	if s == nil {
		return fmt.Errorf("agent service is nil")
	}
	if input.Content == "" {
		return fmt.Errorf("ask message is empty")
	}
	if emit == nil {
		emit = func(Event) error { return nil }
	}

	runMeta := toolRunMetadata{SessionID: newToolRunSessionID()}
	if err := emit(Event{Type: EventAgentStarted, Data: map[string]string{"session_id": runMeta.SessionID}}); err != nil {
		return err
	}

	planInput := PlanInput{
		Content:            input.Content,
		WorkspaceContext:   input.WorkspaceContext,
		SourceKind:         input.SourceKind,
		Sources:            input.Sources,
		SkillName:          input.SkillName,
		SkillRequiredTools: append([]string{}, input.SkillRequiredTools...),
		SkillInstructions:  input.SkillInstructions,
	}
	if handled, err := s.handleRouteCorrectionChat(ctx, input.ConversationID, input.ParentMessageID, input.Content, input.SkillName, input.SkillVersion, runMeta, emit); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	} else if handled {
		return nil
	}
	conversation, err := s.ensureConversation(ctx, input.ConversationID, input.Content)
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	parentMessageID := strings.TrimSpace(input.ParentMessageID)
	var userMessage memory.Message
	if parentMessageID != "" {
		userMessage, err = s.Memory.GetMessage(ctx, parentMessageID)
		if err != nil {
			_ = emit(Event{Type: EventAgentError, Message: err.Error()})
			return err
		}
		if userMessage.ConversationID != conversation.ID || userMessage.Role != "user" {
			err := fmt.Errorf("parent message does not belong to this conversation")
			_ = emit(Event{Type: EventAgentError, Message: err.Error()})
			return err
		}
	} else {
		userMessage, err = s.saveAcceptedUserMessage(ctx, conversation.ID, input.Content, emit)
		if err != nil {
			_ = emit(Event{Type: EventAgentError, Message: err.Error()})
			return err
		}
	}
	if err := s.attachChatAttachments(ctx, conversation.ID, userMessage.ID, input.AttachmentIDs, &planInput); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	planInput.CurrentMessageID = userMessage.ID
	runMeta.UserMessageID = userMessage.ID
	runMeta.ParentMessageID = userMessage.ID
	turn, err := s.startAgentTurn(ctx, conversation.ID, runMeta)
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if turn.ID != "" {
		defer s.markRunningAgentTurnInterrupted(context.WithoutCancel(ctx), turn.ID)
		emit = s.trackAgentTurn(ctx, turn.ID, emit)
	}
	if err := s.attachRoutingSessionContract(ctx, conversation.ID, &planInput); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.attachRetrievalContext(ctx, &planInput); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.attachMemoryContext(ctx, &planInput, emit); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.attachKnowledgeContext(ctx, &planInput, emit); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.attachConversationContext(ctx, conversation.ID, &planInput); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.attachContinuationContext(ctx, conversation.ID, &planInput); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	normalizeFreshStandaloneSessionContext(&planInput)
	plan := BuildPlan(planInput)
	if advisedPlan, err := s.applyRoutingAdvisor(ctx, planInput, plan, emit); err != nil {
		return err
	} else {
		plan = advisedPlan
	}
	if err := emitPlanEvents(emit, plan); err != nil {
		return err
	}
	if plan.NeedsClarification {
		return s.completeWithoutModel(ctx, conversation.ID, runMeta, input.ParentMessageID, input.Content, input.SkillName, input.SkillVersion, plan, ExecutionDecision{Status: ExecutionNotRequired, RiskLevel: plan.RiskLevel}, nil, nil, ClarificationResponse(plan), &userMessage, emit)
	}
	var extraTraces []executionTrace
	decision, executionResult, editProposal, assistantContent, handled, err := s.resolveExecution(ctx, conversation.ID, runMeta, plan, &planInput, emit)
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if handled {
		return s.completeWithoutModel(ctx, conversation.ID, runMeta, input.ParentMessageID, input.Content, input.SkillName, input.SkillVersion, plan, decision, executionResult, editProposal, assistantContent, &userMessage, emit)
	}

	selected, selectedRole, err := s.Router.SelectWithRole(plan.ModelTask)
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	selected, profileStatus := s.applyModelProfileToSelection(selected)
	selected.Provider = models.NormalizeProvider(selected.Provider)
	if !models.IsRuntimeProvider(selected.Provider) {
		err := fmt.Errorf("unsupported model provider %q for agent loop", selected.Provider)
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	responseBehavior := s.responseBehavior(plan, selected)

	if err := emit(Event{
		Type: EventModelSelected,
		Data: map[string]string{
			"provider":           selected.Provider,
			"model":              selected.Name,
			"task_type":          plan.TaskType,
			"model_task":         plan.ModelTask,
			"model_role":         selectedRole,
			"model_profile":      profileStatus.Name,
			"profile_status":     profileStatus.Status,
			"risk_level":         plan.RiskLevel,
			"response_mode":      responseBehavior.EffectiveMode,
			"configured_mode":    responseBehavior.ConfiguredMode,
			"thinking_requested": responseBehavior.ThinkingRequested,
		},
	}); err != nil {
		return err
	}
	if len(planInput.Sources) > 0 || strings.TrimSpace(planInput.SourceKind) != "" {
		if err := emit(Event{
			Type: EventWorkspaceUsed,
			Data: map[string]string{
				"source_count": fmt.Sprintf("%d", len(planInput.Sources)),
				"sources":      strings.Join(planInput.Sources, ", "),
				"source_kind":  planInput.SourceKind,
			},
		}); err != nil {
			return err
		}
	}
	if input.SkillName != "" {
		if err := emit(Event{
			Type: EventSkillSelected,
			Data: map[string]string{
				"name":    input.SkillName,
				"version": input.SkillVersion,
			},
		}); err != nil {
			return err
		}
	}

	var systemBuilder strings.Builder
	systemBuilder.WriteString("You are Yemaka, an AI agent from SiloTabs optimized for local-first operation on low-resource systems. ")
	systemBuilder.WriteString("Use only the provided workspace context when discussing files. ")
	systemBuilder.WriteString("If the context is insufficient, explicitly request the required file or folder. ")
	systemBuilder.WriteString("Cite source paths when using workspace context and obtain confirmation before any destructive action.")
	systemBuilder.WriteString(AnswerContractGuidance(plan))
	systemBuilder.WriteString(ContextGroundingGuidance())
	systemBuilder.WriteString(ToolPolicyGuidance(plan, s.modelToolOptions()))
	systemBuilder.WriteString(FailedToolResultGuidance(executionResult))
	systemBuilder.WriteString(profileStatus.SystemGuidance())
	system := systemBuilder.String()
	useModelToolLoop := s.modelToolLoopEnabled(plan, planInput, decision, executionResult)
	if editProposal != nil && editProposal.NeedsContent {
		system += EditDraftInstructions(*editProposal)
	} else if useModelToolLoop {
		system += ModelToolInstructionsWithOptions(plan, s.modelToolOptions())
	}
	modelToolDefinitions := []models.ToolDefinition(nil)
	if useModelToolLoop {
		modelToolDefinitions = ModelToolDefinitionsWithOptions(plan, s.modelToolOptions())
	}
	promptOptions := responseBehavior.applyPromptOptions(s.promptOptions())
	messages, promptContext := PromptMessagesWithDiagnostics(plan, planInput, system, promptOptions)

	streamFirst := shouldStreamInitialModelOutput(editProposal, executionResult)
	modelTrace := responseBehavior.ModelResponseTrace
	assistantContent, responseModel, err := s.runModelWithFallback(ctx, selected, messages, emit, streamFirst, &modelTrace, responseBehavior, modelToolDefinitions)
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if useModelToolLoop && editProposal == nil {
		var modelDecision ExecutionDecision
		assistantContent, responseModel, modelDecision, executionResult, extraTraces, err = s.resolveModelToolLoop(ctx, conversation.ID, runMeta, selected, plan, planInput, messages, assistantContent, responseModel, emit, streamFirst, &modelTrace, responseBehavior)
		if err != nil {
			_ = emit(Event{Type: EventAgentError, Message: err.Error()})
			return err
		}
		if modelDecision.Status != "" {
			decision = modelDecision
		}
	}
	assistantContent = PrepareAssistantPresentation(assistantContent, decision, executionResult)
	editProposal, assistantContent, err = s.finalizeEditProposalDraftOutput(ctx, conversation.ID, runMeta, emit, decision, editProposal, assistantContent, streamFirst)
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveAndEmitPresentablePermissionRequest(ctx, conversation.ID, runMeta, decision, editProposal, emit); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	assistantContent = PrepareAssistantPresentation(assistantContent, decision, executionResult)
	verification := VerifyResponse(plan, assistantContent)
	verification = VerifyEditProposal(verification, editProposal)
	if err := emitVerification(emit, verification); err != nil {
		return err
	}
	if err := s.saveRoutingSessionContract(ctx, conversation.ID, planInput.SessionContract, plan, decision, executionResult, verification); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}

	assistantMessageID := ""
	assistantVariantIndex := 0
	if assistantContent != "" {
		assistantMessage, err := s.Memory.SaveMessage(ctx, memory.Message{
			ConversationID: conversation.ID,
			Role:           "assistant",
			Content:        assistantContent,
			Model:          responseModel,
			ParentID:       userMessage.ID,
		})
		if err != nil {
			_ = emit(Event{Type: EventAgentError, Message: err.Error()})
			return err
		}
		assistantMessageID = assistantMessage.ID
		assistantVariantIndex = assistantMessage.VariantIndex
		if err := emit(messageSavedEvent(conversation.ID, assistantMessage, assistantContent)); err != nil {
			return err
		}
	}
	if input.SkillName != "" {
		if _, err := s.Memory.SaveSkillUsed(ctx, memory.SkillUsed{
			ConversationID: conversation.ID,
			SkillName:      input.SkillName,
			SkillVersion:   input.SkillVersion,
		}); err != nil {
			_ = emit(Event{Type: EventAgentError, Message: err.Error()})
			return err
		}
	}
	runMeta.UserMessageID = userMessage.ID
	runMeta.AssistantMessageID = assistantMessageID
	runMeta.ParentMessageID = userMessage.ID
	runMeta.VariantIndex = assistantVariantIndex
	if err := s.saveVerification(ctx, conversation.ID, runMeta, plan, verification); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveExecutionTraces(ctx, conversation.ID, runMeta, extraTraces); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveExecutionDecision(ctx, conversation.ID, runMeta, decision); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveExecutionResult(ctx, conversation.ID, runMeta, decision, executionResult); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveEditProposal(ctx, conversation.ID, runMeta, editProposal); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.savePresentablePermissionRequestForDecision(ctx, conversation.ID, runMeta, decision, editProposal); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveReplayTrace(agentReplayTraceInput{
		ConversationID:     conversation.ID,
		UserMessageID:      userMessage.ID,
		AssistantMessageID: assistantMessageID,
		Input:              planInput,
		Plan:               plan,
		Decision:           decision,
		Result:             executionResult,
		ExtraTraces:        extraTraces,
		EditProposal:       editProposal,
		AssistantContent:   assistantContent,
		Verification:       verification,
		PromptContext:      promptContext,
		PromptOptions:      promptOptions,
		ModelResponse:      modelTrace,
	}); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.recordWorkflowMemory(ctx, conversation.ID); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.maybeSaveConversationSummary(ctx, conversation.ID); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}

	return emit(Event{
		Type: EventAgentCompleted,
		Data: map[string]string{"conversation_id": conversation.ID},
	})
}

func (s *Service) attachRetrievalContext(ctx context.Context, input *PlanInput) error {
	if s == nil || s.Retriever == nil || input == nil {
		return nil
	}
	existingContext := strings.TrimSpace(input.WorkspaceContext)
	existingSourceKind := strings.TrimSpace(input.SourceKind)
	if existingContext != "" && !strings.EqualFold(existingSourceKind, attachments.SourceKind) {
		return nil
	}
	if !shouldRetrieveLocalContext(*input) {
		return nil
	}
	result, err := s.Retriever.Retrieve(ctx, input.Content)
	if err != nil {
		return err
	}
	if strings.TrimSpace(result.Context) == "" {
		return nil
	}
	if existingContext != "" {
		input.WorkspaceContext = existingContext + "\n\n" + result.Context
		input.Sources = appendUniqueStrings(input.Sources, result.Sources)
		input.SourceKind = combinedSourceKind(existingSourceKind, result.SourceKind)
		return nil
	}
	input.WorkspaceContext = result.Context
	input.Sources = result.Sources
	input.SourceKind = result.SourceKind
	return nil
}

func combinedSourceKind(left string, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" {
		return right
	}
	if right == "" || strings.EqualFold(left, right) {
		return left
	}
	return "mixed"
}

func (s *Service) attachChatAttachments(ctx context.Context, conversationID string, userMessageID string, ids []string, input *PlanInput) error {
	if s == nil || s.Memory == nil || input == nil || len(ids) == 0 {
		return nil
	}
	items, err := s.Memory.LinkChatAttachments(ctx, conversationID, userMessageID, ids)
	if err != nil {
		return err
	}
	contextText, sources := attachments.BuildContext(items, 0)
	if strings.TrimSpace(contextText) == "" {
		return nil
	}
	if strings.TrimSpace(input.WorkspaceContext) != "" {
		input.WorkspaceContext += "\n\n"
	}
	input.WorkspaceContext += contextText
	if input.SourceKind == "" {
		input.SourceKind = attachments.SourceKind
	}
	input.Sources = appendUniqueStrings(input.Sources, sources)
	input.MemorySources = appendUniqueStrings(input.MemorySources, attachmentMemorySources(items))
	return nil
}

func attachmentMemorySources(items []memory.ChatAttachment) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ID) != "" {
			out = append(out, "attachment:"+item.ID)
		}
	}
	return out
}

func (s *Service) attachConversationContext(ctx context.Context, conversationID string, input *PlanInput) error {
	if s == nil || s.Memory == nil || input == nil {
		return nil
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil
	}
	if !s.shouldAttachConversationContext(ctx, conversationID, input) {
		return nil
	}
	messages, err := s.Memory.ListConversationMessages(ctx, conversationID, 8)
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		return nil
	}
	lines := []string{"RECENT SESSION MESSAGES (conversation continuity only; not current source evidence):"}
	for _, msg := range messages {
		if input.CurrentMessageID != "" && msg.ID == input.CurrentMessageID {
			continue
		}
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			role = "message"
		}
		content := compactSessionLine(msg.Content, 600)
		if content == "" {
			continue
		}
		lines = append(lines, role+": "+content)
	}
	if len(lines) == 1 {
		return nil
	}
	block := strings.Join(lines, "\n")
	if strings.TrimSpace(input.TaskMemory) != "" {
		input.TaskMemory += "\n\n"
	}
	input.TaskMemory += block
	input.MemorySources = append(input.MemorySources, "conversation:"+conversationID)
	return nil
}

func (s *Service) shouldAttachConversationContext(ctx context.Context, conversationID string, input *PlanInput) bool {
	content := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(input.Content)), " "))
	if content == "" {
		return false
	}
	if looksLikePreviousAssistantArtifactRequest(content) {
		return false
	}
	if looksLikeFreshStandalonePrompt(content) {
		return false
	}
	contract := input.SessionContract
	if strings.TrimSpace(contract.PendingApproval) != "" || strings.TrimSpace(contract.PendingClarification) != "" {
		return true
	}
	frame := input.Continuation
	if frame.IsZero() {
		if prior, ok, err := s.latestContinuationPriorState(ctx, conversationID); err == nil && ok {
			frame = routing.BuildContinuationFrame(routing.ContinuationInput{
				Content:    input.Content,
				TaskMemory: input.TaskMemory,
				Prior:      prior,
			})
		}
	}
	resolution := routing.ResolveContinuation(input.Content, contract, frame)
	if resolution.KeepActiveRoute || resolution.NeedsClarification {
		return true
	}
	switch resolution.Mode {
	case routing.ContinuationModeSocialChat,
		routing.ContinuationModeLastResponseArtifact:
		return false
	case routing.ContinuationModeContinueSameTask,
		routing.ContinuationModeApprovePendingAction,
		routing.ContinuationModeRejectPendingAction,
		routing.ContinuationModeAnswerPendingClarify,
		routing.ContinuationModeRevisePreviousTask,
		routing.ContinuationModeAskFollowupPrevious:
		return true
	default:
		return true
	}
}

func normalizeFreshStandaloneSessionContext(input *PlanInput) {
	if input == nil || input.SessionContract.IsZero() {
		return
	}
	resolution := routing.ResolveContinuation(input.Content, input.SessionContract, input.Continuation)
	if resolution.KeepActiveRoute || resolution.NeedsClarification {
		return
	}
	switch resolution.Mode {
	case routing.ContinuationModeSocialChat,
		routing.ContinuationModeLastResponseArtifact:
		input.SessionContract = routing.SessionContract{}
		input.Continuation = routing.ContinuationFrame{}
	case routing.ContinuationModeNewTask:
		if looksLikeFreshStandalonePrompt(input.Content) {
			input.SessionContract = routing.SessionContract{}
			input.Continuation = routing.ContinuationFrame{}
		}
	}
}

func looksLikePreviousAssistantArtifactRequest(content string) bool {
	if !looksLikePreviousAssistantContentRequest(content) {
		return false
	}
	return strings.Contains(content, "create file") ||
		strings.Contains(content, "write") ||
		strings.Contains(content, "save") ||
		strings.Contains(content, "export") ||
		strings.Contains(content, "insert") ||
		strings.Contains(content, "copy")
}

func looksLikeRecentConversationReference(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(content)), " "))
	if content == "" {
		return false
	}
	if looksLikePreviousAssistantContentRequest(content) {
		return true
	}
	return strings.Contains(content, "previous message") ||
		strings.Contains(content, "earlier message") ||
		strings.Contains(content, "above") ||
		strings.Contains(content, "what did i just") ||
		strings.Contains(content, "what did you just") ||
		strings.Contains(content, "what was the last") ||
		strings.Contains(content, "that answer") ||
		strings.Contains(content, "that result") ||
		strings.Contains(content, "this result") ||
		strings.Contains(content, "the previous result")
}

func looksLikeFreshStandalonePrompt(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(content)), " "))
	if content == "" || looksLikeRecentConversationReference(content) {
		return false
	}
	trimmed := strings.Trim(content, ".!?")
	switch trimmed {
	case "hi", "hello", "hey", "how are you", "thanks", "thank you", "good morning", "good afternoon", "good evening":
		return true
	}
	if strings.Contains(content, " active ") || strings.HasPrefix(content, "active ") || strings.Contains(content, " blocker") {
		return false
	}
	return strings.HasPrefix(content, "what is ") ||
		strings.HasPrefix(content, "what are ") ||
		strings.HasPrefix(content, "who is ") ||
		strings.HasPrefix(content, "where is ") ||
		strings.HasPrefix(content, "when is ") ||
		strings.HasPrefix(content, "why is ") ||
		strings.HasPrefix(content, "how do ") ||
		strings.HasPrefix(content, "how does ") ||
		strings.HasPrefix(content, "explain ") ||
		strings.HasPrefix(content, "define ")
}

func compactSessionLine(input string, maxChars int) string {
	text := strings.Join(strings.Fields(input), " ")
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	if maxChars <= 3 {
		return text[:maxChars]
	}
	return strings.TrimSpace(text[:maxChars-3]) + "..."
}

func (s *Service) ensureConversation(ctx context.Context, conversationID string, title string) (memory.Conversation, error) {
	if s == nil || s.Memory == nil {
		return memory.Conversation{}, fmt.Errorf("memory store is required")
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return s.Memory.CreateConversation(ctx, title)
	}
	return s.Memory.GetConversation(ctx, conversationID)
}

func (s *Service) resolveExecution(ctx context.Context, conversationID string, meta toolRunMetadata, plan Plan, input *PlanInput, emit EventHandler) (ExecutionDecision, *ExecutionResult, *EditProposal, string, bool, error) {
	decision := DecideExecutionWithPolicyMode(plan, *input, s.PolicyMode)
	decision = s.applyToolAvailability(decision)
	decision = ensurePermissionDecisionRequestID(decision)
	if err := emitExecutionDecision(emit, decision); err != nil {
		return decision, nil, nil, "", false, err
	}
	var editProposal *EditProposal
	if decision.Status == ExecutionNeedsConfirmation && decision.ToolName == "edit_file" {
		proposal := BuildEditProposal(plan, *input, decision)
		if err := s.completeAppendLineEditProposal(ctx, conversationID, meta, &proposal, *input, emit); err != nil {
			return decision, nil, nil, "", false, err
		}
		if err := s.completePreviousAssistantEditProposal(ctx, conversationID, meta, &proposal, *input); err != nil {
			return decision, nil, nil, "", false, err
		}
		editProposal = &proposal
		if err := s.saveEditProposal(ctx, conversationID, meta, editProposal); err != nil {
			return decision, nil, nil, "", false, err
		}
		if err := emitEditProposal(emit, proposal); err != nil {
			return decision, nil, nil, "", false, err
		}
	}
	if err := s.saveAndEmitPresentablePermissionRequest(ctx, conversationID, meta, decision, editProposal, emit); err != nil {
		return decision, nil, nil, "", false, err
	}
	if decision.Status == ExecutionReady && s.ToolExecutor != nil {
		result, err := s.ToolExecutor(ctx, decision)
		if err != nil {
			failedDecision := decision
			failedDecision.Status = ExecutionBlocked
			failedDecision.Reason = err.Error()
			failedResult := ExecutionResult{Context: FriendlyToolFailureSummary(decision, err), Status: "failed", SourceKind: "tool"}
			if response, ok, proposalErr := s.schedulerJobProposalResponse(ctx, conversationID, plan, *input, failedDecision, emit); proposalErr != nil {
				return failedDecision, &failedResult, editProposal, "", false, proposalErr
			} else if ok {
				return failedDecision, &failedResult, editProposal, response, true, nil
			}
			if response, ok, emitErr := s.capabilityGapResponse(plan, *input, failedDecision, emit); emitErr != nil {
				return failedDecision, &failedResult, editProposal, "", false, emitErr
			} else if ok {
				return failedDecision, &failedResult, editProposal, response, true, nil
			}
			return failedDecision, &failedResult, editProposal, RouteAwareToolFailureResponse(plan, decision, err), true, nil
		}
		result = annotateEmptyRequiredToolResult(decision, result)
		if err := emitToolCompleted(emit, decision, result); err != nil {
			return decision, nil, editProposal, "", false, err
		}
		if response, ok := EmptyRequiredToolResultResponse(*input, decision, result); ok {
			input.SourceKind = firstNonEmptyString(result.SourceKind, input.SourceKind)
			input.Sources = append(input.Sources, result.Sources...)
			return decision, &result, editProposal, response, true, nil
		}
		if decision.ToolName == "local_time" {
			input.SourceKind = result.SourceKind
			input.Sources = append(input.Sources, result.Sources...)
			return decision, &result, editProposal, LocalTimeResponse(result), true, nil
		}
		if decision.ToolName == "list_files" {
			input.SourceKind = result.SourceKind
			input.Sources = append(input.Sources, result.Sources...)
			return decision, &result, editProposal, DirectoryListResponse(result), true, nil
		}
		if decision.ToolName == "ingest_documents" {
			input.SourceKind = result.SourceKind
			input.Sources = append(input.Sources, result.Sources...)
			return decision, &result, editProposal, DocumentIngestResponse(result), true, nil
		}
		if strings.TrimSpace(result.Context) != "" {
			if result.SourceKind == "" {
				result.SourceKind = "tool"
			}
			if strings.TrimSpace(input.WorkspaceContext) != "" {
				input.WorkspaceContext += "\n\n"
			}
			input.WorkspaceContext += ToolResultPrompt(result)
		}
		if result.SourceKind != "" {
			input.SourceKind = result.SourceKind
		} else {
			input.SourceKind = "tool"
		}
		input.Sources = append(input.Sources, result.Sources...)
		return decision, &result, editProposal, "", false, nil
	}
	if editProposal != nil {
		if editProposal.NeedsContent {
			return decision, nil, editProposal, "", false, nil
		}
		return decision, nil, editProposal, EditProposalResponse(decision, *editProposal), true, nil
	}
	if response, ok, err := s.schedulerJobProposalResponse(ctx, conversationID, plan, *input, decision, emit); err != nil {
		return decision, nil, editProposal, "", false, err
	} else if ok {
		return decision, nil, editProposal, response, true, nil
	}
	if response, ok, err := s.capabilityGapResponse(plan, *input, decision, emit); err != nil {
		return decision, nil, editProposal, "", false, err
	} else if ok {
		return decision, nil, editProposal, response, true, nil
	}
	if response := MissingExecutorResponse(decision); response != "" {
		return decision, nil, editProposal, response, true, nil
	}
	return decision, nil, editProposal, "", false, nil
}

func (s *Service) completeAppendLineEditProposal(ctx context.Context, conversationID string, meta toolRunMetadata, proposal *EditProposal, input PlanInput, emit EventHandler) error {
	if s == nil || s.ToolExecutor == nil || proposal == nil || strings.TrimSpace(proposal.Path) == "" {
		return nil
	}
	line, ok := inlineAppendLine(input.Content, proposal.Path)
	if !ok {
		return nil
	}
	readDecision := ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "read_file",
		Command:   []string{"read_file", proposal.Path},
		RiskLevel: RiskLow,
		Reason:    "read the current file before preparing an append-only diff preview",
	}
	result, err := s.ToolExecutor(ctx, readDecision)
	if err != nil {
		return nil
	}
	if err := emitToolCompleted(emit, readDecision, result); err != nil {
		return err
	}
	if err := s.saveExecutionResult(ctx, conversationID, meta, readDecision, &result); err != nil {
		return err
	}
	current, ok := readFileContentFromToolResult(result)
	if !ok {
		return nil
	}
	proposal.Content = appendLineToContent(current, line)
	proposal.ContentSource = "inline_append"
	proposal.Status = "ready_for_preview"
	proposal.Reason = "A candidate file body is available for diff preview by appending the requested line to the current file content."
	proposal.NeedsContent = false
	proposal.DiffPreview = true
	proposal.SnapshotBeforeWrite = true
	proposal.RollbackSupported = true
	return nil
}

func (s *Service) completePreviousAssistantEditProposal(ctx context.Context, conversationID string, meta toolRunMetadata, proposal *EditProposal, input PlanInput) error {
	if s == nil || s.Memory == nil || proposal == nil || !proposal.NeedsContent || strings.TrimSpace(proposal.Path) == "" {
		return nil
	}
	if !looksLikePreviousAssistantContentRequest(input.Content) {
		return nil
	}
	content, ok, err := s.previousAssistantContentForEdit(ctx, conversationID, input.CurrentMessageID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	proposal.Content = content
	proposal.ContentSource = "conversation_last_response"
	proposal.Status = "ready_for_preview"
	proposal.Reason = "The previous assistant response is available as the requested replacement file body."
	proposal.NeedsContent = false
	proposal.DiffPreview = true
	proposal.SnapshotBeforeWrite = true
	proposal.RollbackSupported = true
	if err := s.saveExecutionResult(ctx, conversationID, meta, ExecutionDecision{
		Status:   ExecutionReady,
		ToolName: "edit_proposal_context",
		Reason:   "resolved requested edit content from the previous assistant response",
	}, &ExecutionResult{
		Context:    "resolved edit proposal content from previous assistant response",
		Status:     "completed",
		SourceKind: "conversation",
	}); err != nil {
		return err
	}
	return nil
}

func (s *Service) previousAssistantContentForEdit(ctx context.Context, conversationID string, currentMessageID string) (string, bool, error) {
	messages, err := s.Memory.ListConversationMessages(ctx, conversationID, 80)
	if err != nil {
		return "", false, err
	}
	seenCurrent := strings.TrimSpace(currentMessageID) == ""
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if !seenCurrent {
			if msg.ID == currentMessageID {
				seenCurrent = true
			}
			continue
		}
		if msg.Role != "assistant" || !msg.ActiveVariant {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if usablePreviousAssistantEditContent(content) {
			return content, true, nil
		}
	}
	return "", false, nil
}

func looksLikePreviousAssistantContentRequest(content string) bool {
	lower := strings.ToLower(strings.Join(strings.Fields(content), " "))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "last response") ||
		strings.Contains(lower, "previous response") ||
		strings.Contains(lower, "last answer") ||
		strings.Contains(lower, "previous answer") ||
		strings.Contains(lower, "last assistant response") ||
		strings.Contains(lower, "previous assistant response")
}

func usablePreviousAssistantEditContent(content string) bool {
	if strings.TrimSpace(content) == "" {
		return false
	}
	lower := strings.ToLower(content)
	if strings.Contains(lower, "needs your approval first") ||
		strings.Contains(lower, "approval recorded") ||
		strings.Contains(lower, "approval received, but") ||
		strings.Contains(lower, "file edits still need") ||
		strings.Contains(lower, "open the file write preview") ||
		strings.Contains(lower, "could not use") ||
		strings.Contains(lower, "could not run") ||
		strings.Contains(lower, "tool did not complete") ||
		strings.Contains(lower, "route correction") ||
		strings.Contains(lower, "i can remember this: when you ask") ||
		strings.Contains(lower, "save this for next time") ||
		strings.Contains(lower, "won't save that routing correction") ||
		strings.Contains(lower, "yemaka_edit_proposal") {
		return false
	}
	return true
}

func readFileContentFromToolResult(result ExecutionResult) (string, bool) {
	context := result.Context
	if strings.Contains(context, "\n[truncated]") || strings.Contains(context, "[truncated]") {
		return "", false
	}
	separator := "\n\n"
	index := strings.Index(context, separator)
	if index < 0 {
		return "", false
	}
	return context[index+len(separator):], true
}

func DirectoryListResponse(result ExecutionResult) string {
	context := strings.TrimSpace(result.Context)
	if context == "" {
		return "I checked the directory, but the listing was empty."
	}
	return "I checked the workspace directory.\n\n" + context
}

func LocalTimeResponse(result ExecutionResult) string {
	context := strings.TrimSpace(result.Context)
	date := localTimeField(context, "date")
	clock := localTimeField(context, "time")
	zone := localTimeField(context, "timezone")
	label := localTimeField(context, "time_of_day")
	location := localTimeField(context, "location")
	iana := localTimeField(context, "iana_timezone")
	if clock == "" {
		if context == "" {
			return "I checked the local system clock, but the time result was empty."
		}
		return "Local time from this machine:\n\n" + context
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "It is %s", clock)
	if zone != "" {
		fmt.Fprintf(&builder, " %s", zone)
	}
	if location != "" {
		fmt.Fprintf(&builder, " in %s", location)
	}
	if date != "" {
		fmt.Fprintf(&builder, " on %s", date)
	}
	if label != "" {
		if location != "" {
			fmt.Fprintf(&builder, ". That makes it %s there.", label)
		} else {
			fmt.Fprintf(&builder, ". That makes it %s here.", label)
		}
	} else {
		builder.WriteString(".")
	}
	if iana != "" {
		if location != "" {
			fmt.Fprintf(&builder, "\n\nSource: local system clock converted with IANA timezone `%s`.", iana)
		} else {
			fmt.Fprintf(&builder, "\n\nSource: local system clock using local IANA timezone `%s`.", iana)
		}
	} else {
		builder.WriteString("\n\nSource: local system clock.")
	}
	return builder.String()
}

func localTimeField(context string, name string) string {
	prefix := strings.ToLower(name) + ":"
	for _, line := range strings.Split(context, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	return ""
}

func ToolResultPrompt(result ExecutionResult) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "TOOL RESULT:\n")
	if strings.TrimSpace(result.SourceKind) != "" {
		fmt.Fprintf(&builder, "source_kind: %s\n", strings.TrimSpace(result.SourceKind))
	}
	if strings.TrimSpace(result.Status) != "" {
		fmt.Fprintf(&builder, "status: %s\n", strings.TrimSpace(result.Status))
	}
	if len(result.Sources) > 0 {
		fmt.Fprintf(&builder, "sources: %s\n", strings.Join(result.Sources, ", "))
	}
	if strings.TrimSpace(result.Context) != "" {
		fmt.Fprintf(&builder, "\n%s", strings.TrimSpace(result.Context))
	}
	return strings.TrimSpace(builder.String())
}

func annotateEmptyRequiredToolResult(decision ExecutionDecision, result ExecutionResult) ExecutionResult {
	if !requiredToolResultLooksEmpty(decision, result) {
		return result
	}
	if strings.TrimSpace(result.Status) == "" || strings.EqualFold(strings.TrimSpace(result.Status), "completed") {
		result.Status = "empty"
	}
	return result
}

func EmptyRequiredToolResultResponse(input PlanInput, decision ExecutionDecision, result ExecutionResult) (string, bool) {
	if !requiredToolResultLooksEmpty(decision, result) {
		return "", false
	}
	query := requiredToolQuery(decision, input)
	switch strings.TrimSpace(decision.ToolName) {
	case "rag_search":
		return fmt.Sprintf("I searched the local documents, but found no matching ingested document context for `%s`.\n\nI cannot answer this from local documents until the relevant files are ingested or the query is narrowed.", query), true
	case "memory_search":
		return fmt.Sprintf("I searched local memory, but found no matching memory for `%s`.\n\nI cannot answer this from memory or prior chat without a matching saved memory.", query), true
	case "internet_search":
		return fmt.Sprintf("I ran web search for `%s`, but the configured provider returned no usable result items.\n\nI cannot answer this as a current public fact from memory, local context, or stale search results. Try a narrower query or configure a stronger search provider.", query), true
	case "search_files", "symbol_search":
		return fmt.Sprintf("I searched the workspace for `%s`, but found no matching local evidence.\n\nI cannot answer from workspace files until there is a matching file, symbol, or narrower query.", query), true
	default:
		return "", false
	}
}

func requiredToolResultLooksEmpty(decision ExecutionDecision, result ExecutionResult) bool {
	context := strings.ToLower(strings.TrimSpace(result.Context))
	switch strings.TrimSpace(decision.ToolName) {
	case "rag_search":
		return len(result.Sources) == 0 || strings.Contains(context, "no rag matches")
	case "memory_search":
		return len(result.Sources) == 0 || strings.Contains(context, "no memory matches")
	case "internet_search":
		return len(result.Sources) == 0 ||
			strings.Contains(context, "result_count: 0") ||
			strings.Contains(context, "no search result items were returned")
	case "search_files":
		return strings.Contains(context, "no file matches")
	case "symbol_search":
		return strings.Contains(context, "no symbols matched")
	default:
		return false
	}
}

func requiredToolQuery(decision ExecutionDecision, input PlanInput) string {
	for _, part := range decision.Command {
		part = strings.TrimSpace(part)
		if part == "" || part == decision.ToolName {
			continue
		}
		return part
	}
	if strings.TrimSpace(input.Content) != "" {
		return strings.TrimSpace(input.Content)
	}
	return "the requested target"
}

func FailedToolResultGuidance(result *ExecutionResult) string {
	if result == nil || strings.TrimSpace(result.Status) != "failed" {
		return ""
	}
	return " A local tool result is marked failed. State that failure plainly before analysis. Do not say the tool still needs to be run. If the failure is a missing local dependency, say which dependency is missing."
}

func GroundFailedToolResponse(content string, decision ExecutionDecision, result *ExecutionResult) string {
	if result == nil || strings.TrimSpace(result.Status) != "failed" {
		return content
	}
	prefix := failedToolDisclosure(decision, result)
	if prefix == "" {
		return content
	}
	content = sanitizeFailedToolFutureClaims(content, decision, result)
	if failedToolAlreadyDisclosed(content, result.Context) {
		return content
	}
	if strings.TrimSpace(content) == "" {
		return prefix
	}
	return prefix + "\n\n" + strings.TrimSpace(content)
}

func failedToolAlreadyDisclosed(content string, context string) bool {
	lowerContent := strings.ToLower(content)
	lowerContext := strings.ToLower(context)
	if strings.Contains(lowerContext, "no module named pytest") {
		if strings.Contains(lowerContent, "pytest") &&
			(strings.Contains(lowerContent, "could not run") ||
				strings.Contains(lowerContent, "couldn't run") ||
				strings.Contains(lowerContent, "tried to run")) &&
			(strings.Contains(lowerContent, "missing") ||
				strings.Contains(lowerContent, "not installed") ||
				strings.Contains(lowerContent, "no module named")) {
			return true
		}
		return false
	}
	lower := lowerContent + "\n" + lowerContext
	disclosureTerms := []string{
		"could not run",
		"couldn't run",
		"failed",
		"not installed",
		"no module named",
		"missing dependency",
		"exit_code: 1",
	}
	for _, term := range disclosureTerms {
		if strings.Contains(lower, term) && strings.Contains(strings.ToLower(content), term) {
			return true
		}
	}
	return false
}

func sanitizeFailedToolFutureClaims(content string, decision ExecutionDecision, result *ExecutionResult) string {
	if strings.TrimSpace(decision.ToolName) != "run_tests" || result == nil {
		return content
	}
	if !hasFutureToolClaim(content) {
		return content
	}
	if strings.Contains(strings.ToLower(result.Context), "no module named pytest") {
		return sanitizeMissingPytestFuturePlan(content)
	}
	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, len(lines))
	skippingNextAction := false
	skipped := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "#") && strings.Contains(lower, "next action") {
			skippingNextAction = true
			skipped = true
			continue
		}
		if skippingNextAction {
			if strings.HasPrefix(lower, "#") && !strings.Contains(lower, "next action") {
				skippingNextAction = false
			} else {
				continue
			}
		}
		if isFutureToolClaimLine(lower) {
			skipped = true
			continue
		}
		filtered = append(filtered, line)
	}
	if !skipped {
		return content
	}
	note := "Next step: resolve the local test dependency or approve a different safe test command, then I can rerun tests. I have not applied any file changes."
	if strings.Contains(strings.ToLower(result.Context), "no module named pytest") {
		note = "Next step: install `pytest` in this local Python environment or approve a different safe test command, then I can rerun tests. I have not applied any file changes."
	}
	joined := strings.TrimSpace(strings.Join(filtered, "\n"))
	if joined == "" {
		return note
	}
	return joined + "\n\n" + note
}

func hasFutureToolClaim(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "i will now execute") ||
		strings.Contains(lower, "i will now run") ||
		strings.Contains(lower, "i will first check") ||
		strings.Contains(lower, "i will check") ||
		strings.Contains(lower, "i will attempt") ||
		strings.Contains(lower, "i will proceed") ||
		strings.Contains(lower, "will now execute") ||
		strings.Contains(lower, "will now run") ||
		strings.Contains(lower, "will i proceed") ||
		strings.Contains(lower, "we can manually execute") ||
		strings.Contains(lower, "should i run the tests now") ||
		strings.Contains(lower, "i'll now execute") ||
		strings.Contains(lower, "i'll now run")
}

func isFutureToolClaimLine(lower string) bool {
	return strings.Contains(lower, "i will now execute") ||
		strings.Contains(lower, "i will now run") ||
		strings.Contains(lower, "i will first check") ||
		strings.Contains(lower, "i will check") ||
		strings.Contains(lower, "i will attempt") ||
		strings.Contains(lower, "i will proceed") ||
		strings.Contains(lower, "will now execute") ||
		strings.Contains(lower, "will now run") ||
		strings.Contains(lower, "will i proceed") ||
		strings.Contains(lower, "we can manually execute") ||
		strings.Contains(lower, "should i run the tests now") ||
		strings.Contains(lower, "i'll now execute") ||
		strings.Contains(lower, "i'll now run")
}

func sanitizeMissingPytestFuturePlan(content string) string {
	lines := strings.Split(content, "\n")
	cut := len(lines)
	for i, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if isFutureToolClaimLine(lower) ||
			(strings.HasPrefix(lower, "#") && (strings.Contains(lower, "proposed action") ||
				strings.Contains(lower, "next action") ||
				strings.Contains(lower, "check for") ||
				strings.Contains(lower, "execute"))) {
			cut = i
			break
		}
	}
	kept := strings.TrimSpace(strings.Join(lines[:cut], "\n"))
	note := "Next step: install `pytest` in this local Python environment or approve a different safe test command. If the static inspection above already points to a likely fix, I will still wait for your approval and a snapshot-backed diff before applying any file change."
	if kept == "" {
		return note
	}
	return kept + "\n\n" + note
}

func failedToolDisclosure(decision ExecutionDecision, result *ExecutionResult) string {
	tool := strings.TrimSpace(decision.ToolName)
	if tool == "" {
		tool = "the requested tool"
	}
	context := strings.ToLower(result.Context)
	switch {
	case tool == "run_tests" && strings.Contains(context, "no module named pytest"):
		return "I tried to run the tests, but this local Python environment is missing `pytest`. I did not apply any file changes."
	case tool == "run_tests":
		return "I ran the test command, but it did not complete successfully. I did not apply any file changes."
	default:
		return fmt.Sprintf("I tried to use `%s`, but the tool did not complete successfully.", tool)
	}
}

func (s *Service) applyToolAvailability(decision ExecutionDecision) ExecutionDecision {
	if decision.Status != ExecutionReady || !isInternetToolName(decision.ToolName) {
		return decision
	}
	if s != nil && s.InternetTools {
		if normalizeToolName(decision.ToolName) != "internet_search" || s.InternetSearch {
			return decision
		}
		decision.Status = ExecutionBlocked
		decision.RiskLevel = RiskMedium
		decision.RequiresConfirmation = false
		decision.Reason = "controlled internet fetch is enabled, but internet search requires a configured search provider; set Search Provider in settings before asking Yemaka to search the web"
		return decision
	}
	decision.Status = ExecutionBlocked
	decision.RiskLevel = RiskMedium
	decision.RequiresConfirmation = false
	decision.Reason = "controlled internet tools exist, but internet access is disabled or not profile-approved for this agent session; enable internet in settings or use full access before asking Yemaka to fetch public web content"
	return decision
}

func (s *Service) capabilityGapResponse(plan Plan, input PlanInput, decision ExecutionDecision, emit EventHandler) (string, bool, error) {
	if s == nil || s.CapabilityGap == nil {
		return "", false, nil
	}
	proposal, ok := s.CapabilityGap.Propose(plan, input, decision)
	if !ok {
		return "", false, nil
	}
	if err := emitCapabilityGap(emit, proposal, input.Content); err != nil {
		return "", false, err
	}
	return proposal.Response(), true, nil
}

func shouldStreamInitialModelOutput(proposal *EditProposal, result *ExecutionResult) bool {
	return result == nil && (proposal == nil || !proposal.NeedsContent)
}

func (s *Service) finalizeEditProposalDraftOutput(ctx context.Context, conversationID string, meta toolRunMetadata, emit EventHandler, decision ExecutionDecision, proposal *EditProposal, assistantContent string, streamed bool) (*EditProposal, string, error) {
	updated, promoted, err := s.promoteModelEditProposal(ctx, conversationID, meta, emit, proposal, assistantContent)
	if err != nil {
		return nil, "", err
	}
	if promoted && updated != nil {
		assistantContent = EditProposalResponse(decision, *updated)
	} else if updated != nil && updated.NeedsContent {
		assistantContent = EditProposalNeedsContentResponse(*updated)
	}
	if !streamed {
		if err := emitBufferedModelContent(emit, assistantContent); err != nil {
			return nil, "", err
		}
	}
	return updated, assistantContent, nil
}

func (s *Service) promoteModelEditProposal(ctx context.Context, conversationID string, meta toolRunMetadata, emit EventHandler, proposal *EditProposal, assistantContent string) (*EditProposal, bool, error) {
	if proposal == nil || !proposal.NeedsContent {
		return proposal, false, nil
	}
	updated, ok := BuildModelEditProposal(*proposal, assistantContent)
	if !ok {
		return proposal, false, nil
	}
	if err := s.saveEditProposal(ctx, conversationID, meta, &updated); err != nil {
		return nil, false, err
	}
	if err := emitEditProposal(emit, updated); err != nil {
		return nil, false, err
	}
	return &updated, true, nil
}

func (s *Service) resolveModelToolLoop(ctx context.Context, conversationID string, meta toolRunMetadata, selected config.ModelConfig, plan Plan, input PlanInput, messages []models.ChatMessage, assistantContent string, responseModel string, emit EventHandler, initialContentStreamed bool, modelTrace *ModelResponseTrace, behavior responseBehavior) (string, string, ExecutionDecision, *ExecutionResult, []executionTrace, error) {
	currentContent := assistantContent
	currentModel := responseModel
	currentMessages := append([]models.ChatMessage{}, messages...)
	currentContentStreamed := initialContentStreamed
	traces := []executionTrace{}
	limit := modelToolStepLimit(plan)
	modelToolDefinitions := ModelToolDefinitionsWithOptions(plan, s.modelToolOptions())

	for step := 0; step < limit; step++ {
		request, ok := ParseModelToolRequest(currentContent)
		if !ok {
			if !currentContentStreamed {
				if err := emitBufferedModelContent(emit, currentContent); err != nil {
					return "", currentModel, ExecutionDecision{}, nil, priorExecutionTraces(traces), err
				}
			}
			return currentContent, currentModel, lastExecutionDecision(traces), lastExecutionResult(traces), priorExecutionTraces(traces), nil
		}
		currentContentStreamed = false

		decision := DecisionFromModelToolRequestWithOptions(plan, request, s.PolicyMode, s.modelToolOptions())
		decision = ensurePermissionDecisionRequestID(decision)
		if err := emitExecutionDecision(emit, decision); err != nil {
			return "", currentModel, decision, nil, priorExecutionTraces(traces), err
		}
		if permission, ok := PermissionForDecision(decision); ok {
			if err := s.savePermissionRequest(ctx, conversationID, meta, permission); err != nil {
				return "", currentModel, decision, nil, priorExecutionTraces(traces), err
			}
			if err := emitPermissionRequest(emit, permission); err != nil {
				return "", currentModel, decision, nil, priorExecutionTraces(traces), err
			}
			content := MissingExecutorResponse(decision)
			if err := emitBufferedModelContent(emit, content); err != nil {
				return "", currentModel, decision, nil, priorExecutionTraces(traces), err
			}
			return content, currentModel, decision, nil, priorExecutionTraces(traces), nil
		}
		if decision.Status != ExecutionReady {
			content, ok, err := s.capabilityGapResponse(plan, input, decision, emit)
			if err != nil {
				return "", currentModel, decision, nil, priorExecutionTraces(traces), err
			}
			if !ok {
				content = MissingExecutorResponse(decision)
			}
			if err := emitBufferedModelContent(emit, content); err != nil {
				return "", currentModel, decision, nil, priorExecutionTraces(traces), err
			}
			return content, currentModel, decision, nil, priorExecutionTraces(traces), nil
		}
		if s.ToolExecutor == nil {
			decision.Status = ExecutionBlocked
			decision.Reason = "model requested a tool but no safe executor is available"
			content, ok, err := s.capabilityGapResponse(plan, input, decision, emit)
			if err != nil {
				return "", currentModel, decision, nil, priorExecutionTraces(traces), err
			}
			if !ok {
				content = MissingExecutorResponse(decision)
			}
			if err := emitBufferedModelContent(emit, content); err != nil {
				return "", currentModel, decision, nil, priorExecutionTraces(traces), err
			}
			return content, currentModel, decision, nil, priorExecutionTraces(traces), nil
		}

		result, err := s.ToolExecutor(ctx, decision)
		if err != nil {
			result = ExecutionResult{Context: FriendlyToolFailureSummary(decision, err), Status: "failed", SourceKind: "tool"}
		}
		result = annotateEmptyRequiredToolResult(decision, result)
		traces = append(traces, executionTrace{Decision: decision, Result: &result})
		if err := emitToolCompleted(emit, decision, result); err != nil {
			return "", currentModel, decision, &result, priorExecutionTraces(traces), err
		}
		if err != nil {
			failedDecision := decision
			failedDecision.Status = ExecutionBlocked
			failedDecision.Reason = err.Error()
			content, ok, gapErr := s.capabilityGapResponse(plan, input, failedDecision, emit)
			if gapErr != nil {
				return "", currentModel, failedDecision, &result, priorExecutionTraces(traces), gapErr
			}
			if !ok {
				content = RouteAwareToolFailureResponse(plan, decision, err)
			}
			if err := emitBufferedModelContent(emit, content); err != nil {
				return "", currentModel, failedDecision, &result, priorExecutionTraces(traces), err
			}
			return content, currentModel, failedDecision, &result, priorExecutionTraces(traces), nil
		}
		if content, ok := EmptyRequiredToolResultResponse(input, decision, result); ok {
			if err := emitBufferedModelContent(emit, content); err != nil {
				return "", currentModel, decision, &result, priorExecutionTraces(traces), err
			}
			return content, currentModel, decision, &result, priorExecutionTraces(traces), nil
		}
		if decision.ToolName == "ingest_documents" {
			content := DocumentIngestResponse(result)
			if err := emitBufferedModelContent(emit, content); err != nil {
				return "", currentModel, decision, &result, priorExecutionTraces(traces), err
			}
			return content, currentModel, decision, &result, priorExecutionTraces(traces), nil
		}

		observation := ToolObservationPrompt(decision, result)
		currentMessages = append(currentMessages,
			models.ChatMessage{Role: "assistant", Content: currentContent},
			models.ChatMessage{Role: "user", Content: observation},
		)
		nextContent, nextModel, err := s.runModelWithFallback(ctx, selected, currentMessages, emit, false, modelTrace, behavior, modelToolDefinitions)
		if err != nil {
			return "", nextModel, decision, &result, priorExecutionTraces(traces), err
		}
		currentContent = nextContent
		currentModel = nextModel
		currentContentStreamed = false
	}

	if _, ok := ParseModelToolRequest(currentContent); ok {
		currentContentStreamed = false
		currentMessages = append(currentMessages,
			models.ChatMessage{Role: "assistant", Content: currentContent},
			models.ChatMessage{Role: "user", Content: fmt.Sprintf("TOOL STEP LIMIT REACHED: Yemaka already ran %d safe local tool observation(s). Answer the original request now using only the collected observations. Do not request another tool.", limit)},
		)
		finalContent, finalModel, err := s.runModelWithFallback(ctx, selected, currentMessages, emit, false, modelTrace, behavior, modelToolDefinitions)
		if err != nil {
			return "", finalModel, lastExecutionDecision(traces), lastExecutionResult(traces), priorExecutionTraces(traces), err
		}
		if _, stillRequest := ParseModelToolRequest(finalContent); stillRequest {
			finalContent = fmt.Sprintf("I stopped after the configured limit of %d safe local tool observation(s). I need a narrower request before running more tools.", limit)
			currentContentStreamed = false
		}
		if !currentContentStreamed {
			finalContent = PrepareAssistantPresentation(finalContent, lastExecutionDecision(traces), lastExecutionResult(traces))
			if err := emitBufferedModelContent(emit, finalContent); err != nil {
				return "", finalModel, lastExecutionDecision(traces), lastExecutionResult(traces), priorExecutionTraces(traces), err
			}
		}
		return finalContent, finalModel, lastExecutionDecision(traces), lastExecutionResult(traces), priorExecutionTraces(traces), nil
	}

	if !currentContentStreamed {
		currentContent = PrepareAssistantPresentation(currentContent, lastExecutionDecision(traces), lastExecutionResult(traces))
		if err := emitBufferedModelContent(emit, currentContent); err != nil {
			return "", currentModel, lastExecutionDecision(traces), lastExecutionResult(traces), priorExecutionTraces(traces), err
		}
	}
	return currentContent, currentModel, lastExecutionDecision(traces), lastExecutionResult(traces), priorExecutionTraces(traces), nil
}

func (s *Service) modelToolLoopEnabled(plan Plan, input PlanInput, decision ExecutionDecision, executionResult *ExecutionResult) bool {
	_ = input
	if s == nil || s.ToolExecutor == nil || plan.RiskLevel == RiskHigh || executionResult != nil {
		return false
	}
	switch decision.Status {
	case ExecutionReady, ExecutionNeedsConfirmation, ExecutionBlocked:
		return false
	}
	return planNeedsModelToolObservation(plan)
}

func (s *Service) modelToolOptions() ModelToolOptions {
	if s == nil {
		return ModelToolOptions{}
	}
	return ModelToolOptions{
		InternetTools:  s.InternetTools,
		InternetSearch: s.InternetSearch,
	}
}

func (s *Service) promptOptions() promptcore.Options {
	if s == nil {
		return promptcore.Options{}
	}
	return s.PromptOptions
}

func PromptOptionsFromConfig(cfg *config.Config) promptcore.Options {
	if cfg == nil {
		return promptcore.Options{}
	}
	maxChars := cfg.Runtime.MaxContextTokens * 3
	if maxChars <= 0 {
		maxChars = promptcore.DefaultMaxChars
	}
	return promptcore.Options{
		MaxChars:  maxChars,
		LowMemory: cfg.Runtime.LowMemoryMode,
	}
}

func InternetToolLoopEnabled(cfg *config.Config, policyMode string) bool {
	if strings.EqualFold(strings.TrimSpace(policyMode), "full_access") {
		return true
	}
	if cfg == nil || !cfg.Internet.Enabled {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(cfg.Internet.DefaultMode), "profile_enabled")
}

func InternetSearchLoopEnabled(cfg *config.Config, policyMode string) bool {
	if cfg == nil || !InternetToolLoopEnabled(cfg, policyMode) {
		return false
	}
	provider := strings.TrimSpace(cfg.Internet.Search.Provider)
	return cfg.Internet.Search.Enabled && provider != "" && !strings.EqualFold(provider, "none")
}

func emitBufferedModelContent(emit EventHandler, content string) error {
	if emit == nil || strings.TrimSpace(content) == "" {
		return nil
	}
	return emit(Event{Type: EventModelToken, Token: content})
}

func lastExecutionDecision(traces []executionTrace) ExecutionDecision {
	if len(traces) == 0 {
		return ExecutionDecision{}
	}
	return traces[len(traces)-1].Decision
}

func lastExecutionResult(traces []executionTrace) *ExecutionResult {
	if len(traces) == 0 {
		return nil
	}
	return traces[len(traces)-1].Result
}

func priorExecutionTraces(traces []executionTrace) []executionTrace {
	if len(traces) <= 1 {
		return nil
	}
	return append([]executionTrace{}, traces[:len(traces)-1]...)
}

func (s *Service) completeWithoutModel(ctx context.Context, conversationID string, meta toolRunMetadata, parentMessageID string, content string, skillName string, skillVersion string, plan Plan, decision ExecutionDecision, result *ExecutionResult, editProposal *EditProposal, assistantContent string, acceptedUserMessage *memory.Message, emit EventHandler) error {
	conversation, err := s.ensureConversation(ctx, conversationID, content)
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	var userMessage memory.Message
	if acceptedUserMessage != nil && acceptedUserMessage.ID != "" {
		userMessage = *acceptedUserMessage
		if userMessage.ConversationID != conversation.ID || userMessage.Role != "user" {
			err := fmt.Errorf("accepted user message does not belong to this conversation")
			_ = emit(Event{Type: EventAgentError, Message: err.Error()})
			return err
		}
	} else if strings.TrimSpace(parentMessageID) != "" {
		userMessage, err = s.Memory.GetMessage(ctx, parentMessageID)
		if err != nil {
			_ = emit(Event{Type: EventAgentError, Message: err.Error()})
			return err
		}
		if userMessage.ConversationID != conversation.ID || userMessage.Role != "user" {
			err := fmt.Errorf("parent message does not belong to this conversation")
			_ = emit(Event{Type: EventAgentError, Message: err.Error()})
			return err
		}
	} else {
		userMessage, err = s.Memory.SaveMessage(ctx, memory.Message{
			ConversationID: conversation.ID,
			Role:           "user",
			Content:        content,
			Model:          "yemaka-executor",
		})
		if err != nil {
			_ = emit(Event{Type: EventAgentError, Message: err.Error()})
			return err
		}
		if err := emit(messageSavedEvent(conversation.ID, userMessage, content)); err != nil {
			return err
		}
	}
	assistantContent = PrepareAssistantPresentation(assistantContent, decision, result)
	verification := VerifyResponse(plan, assistantContent)
	verification = VerifyEditProposal(verification, editProposal)
	if err := emitVerification(emit, verification); err != nil {
		return err
	}
	previousContract := routing.SessionContract{}
	if plan.RouteSession != nil {
		previousContract = *plan.RouteSession
	}
	if err := s.saveRoutingSessionContract(ctx, conversation.ID, previousContract, plan, decision, result, verification); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := emit(Event{
		Type: EventModelSelected,
		Data: map[string]string{
			"provider":   "yemaka",
			"model":      "executor",
			"task_type":  plan.TaskType,
			"model_task": plan.ModelTask,
			"risk_level": plan.RiskLevel,
		},
	}); err != nil {
		return err
	}
	if assistantContent != "" {
		if err := emit(Event{Type: EventModelToken, Token: assistantContent}); err != nil {
			return err
		}
	}
	assistantMessage, err := s.Memory.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        assistantContent,
		Model:          "yemaka-executor",
		ParentID:       userMessage.ID,
	})
	if err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := emit(messageSavedEvent(conversation.ID, assistantMessage, assistantContent)); err != nil {
		return err
	}
	meta.UserMessageID = userMessage.ID
	meta.AssistantMessageID = assistantMessage.ID
	meta.ParentMessageID = userMessage.ID
	meta.VariantIndex = assistantMessage.VariantIndex
	if skillName != "" {
		if _, err := s.Memory.SaveSkillUsed(ctx, memory.SkillUsed{
			ConversationID: conversation.ID,
			SkillName:      skillName,
			SkillVersion:   skillVersion,
		}); err != nil {
			_ = emit(Event{Type: EventAgentError, Message: err.Error()})
			return err
		}
	}
	if err := s.saveExecutionDecision(ctx, conversation.ID, meta, decision); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveExecutionResult(ctx, conversation.ID, meta, decision, result); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveEditProposal(ctx, conversation.ID, meta, editProposal); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.savePresentablePermissionRequestForDecision(ctx, conversation.ID, meta, decision, editProposal); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveVerification(ctx, conversation.ID, meta, plan, verification); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.saveReplayTrace(agentReplayTraceInput{
		ConversationID:     conversation.ID,
		UserMessageID:      userMessage.ID,
		AssistantMessageID: assistantMessage.ID,
		Input:              PlanInput{Content: content, SkillName: skillName},
		Plan:               plan,
		Decision:           decision,
		Result:             result,
		EditProposal:       editProposal,
		AssistantContent:   assistantContent,
		Verification:       verification,
	}); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.recordWorkflowMemory(ctx, conversation.ID); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	if err := s.maybeSaveConversationSummary(ctx, conversation.ID); err != nil {
		_ = emit(Event{Type: EventAgentError, Message: err.Error()})
		return err
	}
	return emit(Event{
		Type: EventAgentCompleted,
		Data: map[string]string{"conversation_id": conversation.ID},
	})
}

func toolRunWithMetadata(conversationID string, meta toolRunMetadata, run memory.ToolRun) memory.ToolRun {
	run.ConversationID = conversationID
	run.SessionID = strings.TrimSpace(meta.SessionID)
	run.UserMessageID = strings.TrimSpace(meta.UserMessageID)
	run.AssistantMessageID = strings.TrimSpace(meta.AssistantMessageID)
	run.ParentMessageID = strings.TrimSpace(meta.ParentMessageID)
	run.VariantIndex = meta.VariantIndex
	return run
}

func (s *Service) saveExecutionDecision(ctx context.Context, conversationID string, meta toolRunMetadata, decision ExecutionDecision) error {
	if s == nil || s.Memory == nil || decision.Status == ExecutionNotRequired {
		return nil
	}
	_, err := s.Memory.SaveToolRun(ctx, toolRunWithMetadata(conversationID, meta, memory.ToolRun{
		ToolName:  "agent_executor",
		Input:     map[string]any{"tool_name": decision.ToolName, "command": decision.Command},
		Output:    decision,
		Status:    decision.Status,
		RiskLevel: decision.RiskLevel,
	}))
	return err
}

func (s *Service) saveExecutionResult(ctx context.Context, conversationID string, meta toolRunMetadata, decision ExecutionDecision, result *ExecutionResult) error {
	if s == nil || s.Memory == nil || result == nil || strings.TrimSpace(decision.ToolName) == "" {
		return nil
	}
	status := result.Status
	if status == "" {
		status = "completed"
	}
	_, err := s.Memory.SaveToolRun(ctx, toolRunWithMetadata(conversationID, meta, memory.ToolRun{
		ToolName:  decision.ToolName,
		Input:     map[string]any{"command": decision.Command, "reason": decision.Reason},
		Output:    result,
		Status:    status,
		RiskLevel: decision.RiskLevel,
	}))
	return err
}

func (s *Service) saveExecutionTraces(ctx context.Context, conversationID string, meta toolRunMetadata, traces []executionTrace) error {
	for _, trace := range traces {
		if err := s.saveExecutionDecision(ctx, conversationID, meta, trace.Decision); err != nil {
			return err
		}
		if err := s.saveExecutionResult(ctx, conversationID, meta, trace.Decision, trace.Result); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) saveReplayTrace(input agentReplayTraceInput) error {
	if s == nil || s.ReplayStore == nil {
		return nil
	}
	trace := replayTraceFromAgentRun(input)
	if strings.TrimSpace(trace.ID) == "" {
		return nil
	}
	_, err := s.ReplayStore.Save(trace)
	return err
}

func (s *Service) savePermissionRequest(ctx context.Context, conversationID string, meta toolRunMetadata, request PermissionRequest) error {
	if s == nil || s.Memory == nil {
		return nil
	}
	request.RequestID = strings.TrimSpace(request.RequestID)
	if request.RequestID == "" {
		return fmt.Errorf("permission request id is required")
	}
	exists, err := s.permissionRequestAlreadyStored(ctx, request.RequestID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = s.Memory.SaveToolRun(ctx, toolRunWithMetadata(conversationID, meta, memory.ToolRun{
		ToolName:  "permission_request",
		Input:     map[string]any{"request_id": request.RequestID, "tool_name": request.ToolName},
		Output:    request,
		Status:    ExecutionNeedsConfirmation,
		RiskLevel: request.RiskLevel,
	}))
	return err
}

func (s *Service) permissionRequestAlreadyStored(ctx context.Context, requestID string) (bool, error) {
	if s == nil || s.Memory == nil {
		return false, nil
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return false, nil
	}
	runs, err := s.Memory.ListToolRuns(ctx, 200)
	if err != nil {
		return false, fmt.Errorf("check pending permission request: %w", err)
	}
	for _, run := range runs {
		if normalizeToolName(run.ToolName) != "permission_request" {
			continue
		}
		stored, ok := permissionRequestFromStoredValue(run.Output)
		if ok && strings.TrimSpace(stored.RequestID) == requestID {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) savePermissionRequestForDecision(ctx context.Context, conversationID string, meta toolRunMetadata, decision ExecutionDecision) error {
	decision = ensurePermissionDecisionRequestID(decision)
	request, ok := PermissionForDecision(decision)
	if !ok {
		return nil
	}
	return s.savePermissionRequest(ctx, conversationID, meta, request)
}

func (s *Service) savePresentablePermissionRequestForDecision(ctx context.Context, conversationID string, meta toolRunMetadata, decision ExecutionDecision, proposal *EditProposal) error {
	if !permissionRequestIsPresentable(decision, proposal) {
		return nil
	}
	return s.savePermissionRequestForDecision(ctx, conversationID, meta, decision)
}

func (s *Service) saveAndEmitPresentablePermissionRequest(ctx context.Context, conversationID string, meta toolRunMetadata, decision ExecutionDecision, proposal *EditProposal, emit EventHandler) error {
	if !permissionRequestIsPresentable(decision, proposal) {
		return nil
	}
	decision = ensurePermissionDecisionRequestID(decision)
	request, ok := PermissionForDecision(decision)
	if !ok {
		return nil
	}
	if err := s.savePermissionRequest(ctx, conversationID, meta, request); err != nil {
		return err
	}
	return emitPermissionRequest(emit, request)
}

func permissionRequestIsPresentable(decision ExecutionDecision, proposal *EditProposal) bool {
	if decision.Status != ExecutionNeedsConfirmation {
		return false
	}
	if decision.ToolName != "edit_file" {
		return true
	}
	return proposal != nil && !proposal.NeedsContent
}

func (s *Service) saveEditProposal(ctx context.Context, conversationID string, meta toolRunMetadata, proposal *EditProposal) error {
	if s == nil || s.Memory == nil || proposal == nil {
		return nil
	}
	exists, err := s.editProposalAlreadyStored(ctx, *proposal)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = s.Memory.SaveToolRun(ctx, toolRunWithMetadata(conversationID, meta, memory.ToolRun{
		ToolName:  "edit_proposal",
		Input:     map[string]any{"request_id": proposal.RequestID, "path": proposal.Path},
		Output:    proposal,
		Status:    proposal.Status,
		RiskLevel: RiskMedium,
	}))
	return err
}

func (s *Service) editProposalAlreadyStored(ctx context.Context, proposal EditProposal) (bool, error) {
	if s == nil || s.Memory == nil || strings.TrimSpace(proposal.RequestID) == "" {
		return false, nil
	}
	runs, err := s.Memory.ListToolRuns(ctx, 200)
	if err != nil {
		return false, fmt.Errorf("check stored edit proposal: %w", err)
	}
	for _, run := range runs {
		if normalizeToolName(run.ToolName) != "edit_proposal" {
			continue
		}
		stored, ok := editProposalFromStoredValue(run.Output)
		if !ok {
			continue
		}
		if strings.TrimSpace(stored.RequestID) == strings.TrimSpace(proposal.RequestID) &&
			stored.Path == proposal.Path &&
			stored.Content == proposal.Content &&
			stored.ContentSource == proposal.ContentSource &&
			stored.Status == proposal.Status &&
			stored.NeedsContent == proposal.NeedsContent {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) saveVerification(ctx context.Context, conversationID string, meta toolRunMetadata, plan Plan, verification VerificationResult) error {
	if s == nil || s.Memory == nil {
		return nil
	}
	_, err := s.Memory.SaveToolRun(ctx, toolRunWithMetadata(conversationID, meta, memory.ToolRun{
		ToolName:  "agent_verifier",
		Input:     plan,
		Output:    verification,
		Status:    verification.Status,
		RiskLevel: plan.RiskLevel,
	}))
	return err
}

func (s *Service) runModelWithFallback(ctx context.Context, selected config.ModelConfig, messages []models.ChatMessage, emit EventHandler, stream bool, modelTrace *ModelResponseTrace, behavior responseBehavior, toolDefinitions ...[]models.ToolDefinition) (string, string, error) {
	selected = s.installedLocalModelOrFallback(ctx, selected, emit)
	runtime, err := s.runtimeForModel(selected)
	if err != nil {
		return "", selected.Name, err
	}
	tools := firstModelToolDefinitions(toolDefinitions)
	var assistantContent string
	streamer := newGuardedTokenEmitter(emit, stream)
	err = runtime.ChatStream(ctx, behavior.applyChatRequest(models.ChatRequest{
		Model:       selected.Name,
		Messages:    messages,
		Temperature: selected.Temperature,
		Tools:       tools,
	}), func(event models.ChatEvent) error {
		if len(event.ToolCalls) > 0 {
			if err := emitModelToolCall(emit, event.ToolCalls); err != nil {
				return err
			}
			if len(tools) > 0 {
				if marker := ModelToolRequestMarkerFromNativeToolCalls(event.ToolCalls); marker != "" {
					assistantContent = appendHiddenModelToolMarker(assistantContent, marker)
				}
			}
		}
		if event.Token != "" {
			assistantContent += event.Token
			return streamer.Emit(event.Token)
		}
		return nil
	})
	if err == nil {
		if modelTrace != nil && containsModelThinking(assistantContent) {
			modelTrace.ThinkingReturned = true
		}
		if err := streamer.Flush(); err != nil {
			return "", selected.Name, err
		}
		return StripModelThinkingBlocks(assistantContent), selected.Name, nil
	}
	if !s.cloudFallbackReady() {
		return "", selected.Name, err
	}

	fallback := s.CloudFallback.Config
	if emitErr := emit(Event{
		Type: EventCloudFallback,
		Data: map[string]string{
			"provider":    fallback.Provider,
			"model":       fallback.Name,
			"local_model": selected.Name,
			"reason":      err.Error(),
		},
	}); emitErr != nil {
		return "", selected.Name, emitErr
	}
	if emitErr := emit(Event{
		Type: EventModelSelected,
		Data: map[string]string{
			"provider": fallback.Provider,
			"model":    fallback.Name,
		},
	}); emitErr != nil {
		return "", selected.Name, emitErr
	}

	assistantContent = ""
	streamer = newGuardedTokenEmitter(emit, stream)
	fallbackErr := s.CloudFallback.Runtime.ChatStream(ctx, models.ChatRequest{
		Model:       fallback.Name,
		Messages:    messages,
		Temperature: fallback.Temperature,
		Tools:       tools,
	}, func(event models.ChatEvent) error {
		if len(event.ToolCalls) > 0 {
			if err := emitModelToolCall(emit, event.ToolCalls); err != nil {
				return err
			}
			if len(tools) > 0 {
				if marker := ModelToolRequestMarkerFromNativeToolCalls(event.ToolCalls); marker != "" {
					assistantContent = appendHiddenModelToolMarker(assistantContent, marker)
				}
			}
		}
		if event.Token != "" {
			assistantContent += event.Token
			return streamer.Emit(event.Token)
		}
		return nil
	})
	if fallbackErr != nil {
		return "", selected.Name, fmt.Errorf("local model failed: %v; cloud fallback failed: %w", err, fallbackErr)
	}
	if modelTrace != nil && containsModelThinking(assistantContent) {
		modelTrace.ThinkingReturned = true
	}
	if err := streamer.Flush(); err != nil {
		return "", fallback.Provider + ":" + fallback.Name, err
	}
	return StripModelThinkingBlocks(assistantContent), fallback.Provider + ":" + fallback.Name, nil
}

func (s *Service) applyModelProfileToSelection(selected config.ModelConfig) (config.ModelConfig, appliedModelProfile) {
	profileName := strings.TrimSpace(selected.Profile)
	if profileName == "" || strings.TrimSpace(s.ModelProfileDir) == "" {
		return selected, appliedModelProfile{Status: "none"}
	}
	profile, err := modelprofiles.NewStore(s.ModelProfileDir).Load(profileName)
	if err != nil {
		return selected, appliedModelProfile{Name: profileName, Status: "missing"}
	}
	selected.Name = profile.BaseModel
	selected.Temperature = profile.Parameters.Temperature
	return selected, appliedModelProfile{
		Name:   profile.Name,
		Status: "applied",
		System: profile.System,
	}
}

func firstModelToolDefinitions(values [][]models.ToolDefinition) []models.ToolDefinition {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func appendHiddenModelToolMarker(content string, marker string) string {
	marker = strings.TrimSpace(marker)
	if marker == "" {
		return content
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return marker
	}
	return content + "\n\n" + marker
}

func (s *Service) installedLocalModelOrFallback(ctx context.Context, selected config.ModelConfig, emit EventHandler) config.ModelConfig {
	if s == nil || s.Router == nil || s.Router.Config == nil || strings.TrimSpace(selected.Name) == "" {
		return selected
	}
	selected.Provider = models.NormalizeProvider(selected.Provider)
	if selected.Provider != models.ProviderOllama && selected.Provider != models.ProviderLlamaCpp && selected.Provider != models.ProviderOpenAICompatible {
		return selected
	}
	runtime, err := s.runtimeForModel(selected)
	if err != nil {
		return selected
	}
	installed, err := runtime.ListModels(ctx)
	if err != nil || len(installed) == 0 || models.ModelInstalled(selected.Name, installed) {
		return selected
	}
	for _, role := range []string{"low_memory", "default", "reasoning", "coding", "stronger_local"} {
		candidate, ok := s.Router.Config.Models[role]
		if !ok || strings.TrimSpace(candidate.Name) == "" {
			continue
		}
		candidate.Provider = models.NormalizeProvider(candidate.Provider)
		if candidate.Provider != selected.Provider || !models.ModelInstalled(candidate.Name, installed) {
			continue
		}
		_ = emit(Event{
			Type: EventModelSelected,
			Data: map[string]string{
				"provider": candidate.Provider,
				"model":    candidate.Name,
				"reason":   fmt.Sprintf("configured model %s is not installed; using installed %s model", selected.Name, role),
			},
		})
		return candidate
	}
	return selected
}

type guardedTokenEmitter struct {
	emit     EventHandler
	enabled  bool
	ready    bool
	pending  string
	thinking modelThinkingFilter
}

func newGuardedTokenEmitter(emit EventHandler, enabled bool) *guardedTokenEmitter {
	return &guardedTokenEmitter{emit: emit, enabled: enabled}
}

func (e *guardedTokenEmitter) Emit(token string) error {
	if e == nil || !e.enabled || e.emit == nil || token == "" {
		return nil
	}
	visible := e.thinking.Filter(token, false)
	if visible == "" {
		return nil
	}
	e.pending += visible
	if e.pendingMightBeToolRequest() {
		return nil
	}
	e.ready = true
	pending := e.pending
	e.pending = ""
	return e.emit(Event{Type: EventModelToken, Token: pending})
}

func (e *guardedTokenEmitter) Flush() error {
	if e == nil || !e.enabled || e.emit == nil {
		return nil
	}
	e.pending += e.thinking.Filter("", true)
	if e.pending == "" {
		return nil
	}
	pending := e.pending
	e.pending = ""
	if _, isToolRequest := ParseModelToolRequest(pending); isToolRequest {
		return nil
	}
	return e.emit(Event{Type: EventModelToken, Token: pending})
}

func (e *guardedTokenEmitter) pendingMightBeToolRequest() bool {
	pending := strings.ToLower(strings.TrimLeft(e.pending, " \t\r\n"))
	if pending == "" {
		return true
	}
	marker := "yemaka_tool_request"
	return strings.HasPrefix(marker, pending) || strings.HasPrefix(pending, marker)
}

func emitModelToolCall(emit EventHandler, calls []models.ToolCall) error {
	if emit == nil || len(calls) == 0 {
		return nil
	}
	names := make([]string, 0, len(calls))
	for _, call := range calls {
		name := strings.TrimSpace(call.Function.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	return emit(Event{
		Type: EventModelToolCall,
		Data: map[string]string{
			"count": fmt.Sprintf("%d", len(calls)),
			"names": strings.Join(names, ", "),
		},
	})
}

func (s *Service) cloudFallbackReady() bool {
	return s != nil &&
		s.CloudFallback != nil &&
		s.CloudFallback.Config.Enabled &&
		strings.TrimSpace(s.CloudFallback.Config.Name) != "" &&
		s.CloudFallback.Runtime != nil
}

func (s *Service) runtimeForModel(selected config.ModelConfig) (models.Runtime, error) {
	if s.RuntimeFactory != nil {
		return s.RuntimeFactory(selected)
	}
	if s.Runtime != nil {
		return s.Runtime, nil
	}
	return nil, fmt.Errorf("agent service has no model runtime")
}
