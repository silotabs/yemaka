package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yemaka/internal/config"
	"yemaka/internal/extensions"
	"yemaka/internal/memory"
	"yemaka/internal/modelprofiles"
	"yemaka/internal/models"
	"yemaka/internal/routing"
)

type failingRuntime struct{}

func (failingRuntime) Health(ctx context.Context) error {
	return nil
}

func (failingRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return nil, nil
}

func (failingRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (failingRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	return fmt.Errorf("local unavailable")
}

type staticRuntime struct {
	text string
}

func (staticRuntime) Health(ctx context.Context) error {
	return nil
}

func (staticRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return nil, nil
}

func (staticRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (r staticRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	if err := emit(models.ChatEvent{Token: r.text}); err != nil {
		return err
	}
	return emit(models.ChatEvent{Done: true})
}

type toolCallRuntime struct{}

func (toolCallRuntime) Health(ctx context.Context) error {
	return nil
}

func (toolCallRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return nil, nil
}

func (toolCallRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (toolCallRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	if err := emit(models.ChatEvent{Token: "before "}); err != nil {
		return err
	}
	if err := emit(models.ChatEvent{ToolCalls: []models.ToolCall{{Function: models.ToolCallFunction{Name: "read_file"}}}}); err != nil {
		return err
	}
	if err := emit(models.ChatEvent{Token: "after"}); err != nil {
		return err
	}
	return emit(models.ChatEvent{Done: true})
}

type nativeToolLoopRuntime struct {
	requests *[]models.ChatRequest
	index    int
}

func (r *nativeToolLoopRuntime) Health(ctx context.Context) error {
	return nil
}

func (r *nativeToolLoopRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return nil, nil
}

func (r *nativeToolLoopRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (r *nativeToolLoopRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	if r.requests != nil {
		*r.requests = append(*r.requests, req)
	}
	r.index++
	if r.index == 1 {
		if err := emit(models.ChatEvent{ToolCalls: []models.ToolCall{{
			Function: models.ToolCallFunction{
				Name:      "read_file",
				Arguments: map[string]any{"path": "README.md"},
			},
		}}}); err != nil {
			return err
		}
		return emit(models.ChatEvent{Done: true})
	}
	if err := emit(models.ChatEvent{Token: "README.md says Yemaka is local-first."}); err != nil {
		return err
	}
	return emit(models.ChatEvent{Done: true})
}

type recordingRuntime struct {
	text     string
	requests *[]models.ChatRequest
}

func (r recordingRuntime) Health(ctx context.Context) error {
	return nil
}

func (r recordingRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return nil, nil
}

func (r recordingRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (r recordingRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	if r.requests != nil {
		*r.requests = append(*r.requests, req)
	}
	if err := emit(models.ChatEvent{Token: r.text}); err != nil {
		return err
	}
	return emit(models.ChatEvent{Done: true})
}

type inspectingRuntime struct {
	text     string
	requests *[]models.ChatRequest
	inspect  func(context.Context) error
}

func (r inspectingRuntime) Health(ctx context.Context) error {
	return nil
}

func (r inspectingRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return nil, nil
}

func (r inspectingRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (r inspectingRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	if r.requests != nil {
		*r.requests = append(*r.requests, req)
	}
	if r.inspect != nil {
		if err := r.inspect(ctx); err != nil {
			return err
		}
	}
	if err := emit(models.ChatEvent{Token: r.text}); err != nil {
		return err
	}
	return emit(models.ChatEvent{Done: true})
}

type installedModelRuntime struct {
	text      string
	installed []models.ModelInfo
	requests  *[]models.ChatRequest
}

func (r installedModelRuntime) Health(ctx context.Context) error {
	return nil
}

func (r installedModelRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return r.installed, nil
}

func (r installedModelRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (r installedModelRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	if r.requests != nil {
		*r.requests = append(*r.requests, req)
	}
	if !models.ModelInstalled(req.Model, r.installed) {
		return fmt.Errorf("model %q not found", req.Model)
	}
	if err := emit(models.ChatEvent{Token: r.text}); err != nil {
		return err
	}
	return emit(models.ChatEvent{Done: true})
}

type sequenceRuntime struct {
	texts    []string
	requests *[]models.ChatRequest
	index    int
}

func (r *sequenceRuntime) Health(ctx context.Context) error {
	return nil
}

func (r *sequenceRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return nil, nil
}

func (r *sequenceRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (r *sequenceRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	if r.requests != nil {
		*r.requests = append(*r.requests, req)
	}
	text := ""
	if r.index < len(r.texts) {
		text = r.texts[r.index]
	}
	r.index++
	if text != "" {
		if err := emit(models.ChatEvent{Token: text}); err != nil {
			return err
		}
	}
	return emit(models.ChatEvent{Done: true})
}

type tokenRuntime struct {
	tokens   []string
	requests *[]models.ChatRequest
}

func (r *tokenRuntime) Health(ctx context.Context) error {
	return nil
}

func (r *tokenRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return nil, nil
}

func (r *tokenRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (r *tokenRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	if r.requests != nil {
		*r.requests = append(*r.requests, req)
	}
	for _, token := range r.tokens {
		if err := emit(models.ChatEvent{Token: token}); err != nil {
			return err
		}
	}
	return emit(models.ChatEvent{Done: true})
}

func TestBuildPlanClassifiesToolRisk(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Run tests and explain failures in main.go"})
	if plan.TaskType != TaskTool {
		t.Fatalf("TaskType = %q, want %q", plan.TaskType, TaskTool)
	}
	if plan.ModelTask != models.TaskCoding {
		t.Fatalf("ModelTask = %q, want coding", plan.ModelTask)
	}
	if plan.RiskLevel != RiskMedium {
		t.Fatalf("RiskLevel = %q, want medium", plan.RiskLevel)
	}
	if len(plan.ToolsNeeded) == 0 {
		t.Fatal("ToolsNeeded is empty")
	}
}

func TestBuildPlanClassifiesInternetFetch(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Check the web for https://example.com"})
	if plan.TaskType != TaskTool {
		t.Fatalf("TaskType = %q, want %q", plan.TaskType, TaskTool)
	}
	if !containsTool(plan.ToolsNeeded, "internet_fetch") {
		t.Fatalf("ToolsNeeded = %#v, want internet_fetch", plan.ToolsNeeded)
	}
	decision := DecideExecution(plan, PlanInput{Content: "Check the web for https://example.com"})
	if decision.Status != ExecutionReady || decision.ToolName != "internet_fetch" {
		t.Fatalf("decision = %+v, want ready internet_fetch", decision)
	}
	if len(decision.Command) < 3 || decision.Command[1] != "https://example.com" || decision.Command[2] != "example.com" {
		t.Fatalf("Command = %#v, want url and allowlisted host", decision.Command)
	}
}

func TestBuildPlanClassifiesGenericSearchAsInternetSearch(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Search SearXNG and tell me what it is for"})
	if plan.TaskType != TaskTool {
		t.Fatalf("TaskType = %q, want %q", plan.TaskType, TaskTool)
	}
	if !containsTool(plan.ToolsNeeded, "internet_search") {
		t.Fatalf("ToolsNeeded = %#v, want internet_search", plan.ToolsNeeded)
	}
	decision := DecideExecution(plan, PlanInput{Content: "Search SearXNG and tell me what it is for"})
	if decision.Status != ExecutionReady || decision.ToolName != "internet_search" {
		t.Fatalf("decision = %+v, want ready internet_search", decision)
	}
}

func TestBuildPlanKeepsLocalSearchLocal(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Search files for config"})
	if containsTool(plan.ToolsNeeded, "internet_search") {
		t.Fatalf("ToolsNeeded = %#v, did not expect internet_search", plan.ToolsNeeded)
	}
	if !containsTool(plan.ToolsNeeded, "search_files") {
		t.Fatalf("ToolsNeeded = %#v, want search_files", plan.ToolsNeeded)
	}
}

func TestDecideExecutionRequiresConfirmationForHighRisk(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Delete build output with rm -rf build"})
	decision := DecideExecution(plan, PlanInput{Content: "Delete build output with rm -rf build"})
	if decision.Status != ExecutionNeedsConfirmation {
		t.Fatalf("Status = %q, want needs_confirmation", decision.Status)
	}
	if !decision.RequiresConfirmation {
		t.Fatal("RequiresConfirmation = false, want true")
	}
}

func TestPermissionRequestForEditFileRequiresSnapshot(t *testing.T) {
	decision := ExecutionDecision{
		Status:               ExecutionNeedsConfirmation,
		ToolName:             "edit_file",
		RiskLevel:            RiskMedium,
		RequiresConfirmation: true,
		Reason:               "file edits require confirmation and snapshot flow",
	}
	request, ok := PermissionForDecision(decision)
	if !ok {
		t.Fatal("PermissionForDecision() ok = false, want true")
	}
	if !request.WorkspaceOnly {
		t.Fatal("WorkspaceOnly = false, want true")
	}
	if !request.DiffPreview {
		t.Fatal("DiffPreview = false, want true")
	}
	if !request.SnapshotBeforeWrite {
		t.Fatal("SnapshotBeforeWrite = false, want true")
	}
	if !request.RollbackSupported {
		t.Fatal("RollbackSupported = false, want true")
	}
	if request.Destructive {
		t.Fatal("Destructive = true, want false for medium-risk edit")
	}
}

func TestPermissionForDecisionUsesStabilizedRequestID(t *testing.T) {
	decision := ensurePermissionDecisionRequestID(ExecutionDecision{
		Status:               ExecutionNeedsConfirmation,
		ToolName:             "project_map",
		RiskLevel:            RiskMedium,
		RequiresConfirmation: true,
		Reason:               "test confirmation",
	})
	if decision.RequestID == "" {
		t.Fatal("stabilized decision request id is empty")
	}
	first, ok := PermissionForDecision(decision)
	if !ok {
		t.Fatal("first PermissionForDecision() ok = false, want true")
	}
	second, ok := PermissionForDecision(decision)
	if !ok {
		t.Fatal("second PermissionForDecision() ok = false, want true")
	}
	if first.RequestID != decision.RequestID || second.RequestID != decision.RequestID {
		t.Fatalf("request ids = %q, %q; want stabilized %q", first.RequestID, second.RequestID, decision.RequestID)
	}
}

func TestAskToolRequestWithoutExecutorStopsBeforeModel(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "model should not run", requests: &requests},
		Memory:  store,
	}

	var output string
	seen := map[string]bool{}
	err = service.Ask(ctx, AskInput{
		Content:          "Run tests and explain failures",
		WorkspaceContext: "workspace context",
		SourceKind:       "workspace",
	}, func(event Event) error {
		seen[event.Type] = true
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0", len(requests))
	}
	if !strings.Contains(output, "run_tests output") {
		t.Fatalf("model output = %q, want executor guidance", output)
	}
	if !seen[EventExecutionDecided] {
		t.Fatal("execution decision event was not emitted")
	}
	results, err := store.SearchMessages(ctx, "run_tests", 5)
	if err != nil {
		t.Fatalf("SearchMessages() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("executor response was not saved to memory")
	}
}

func TestAttachChatAttachmentsAddsContextAndAllowsRetrieval(t *testing.T) {
	ctx := context.Background()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	conversation, err := store.CreateConversation(ctx, "attachment context")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	user, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Use the attachment and workspace notes",
		Model:          "qwen",
	})
	if err != nil {
		t.Fatalf("SaveMessage() error = %v", err)
	}
	attachment, err := store.SaveChatAttachment(ctx, memory.ChatAttachment{
		ID:          "att_agent",
		FileName:    "attached-note.txt",
		ContentType: "text/plain",
		Status:      "ready",
		SourceKind:  "attachment",
		Sources:     []string{"attached-note.txt"},
		Retention:   "conversation",
		Content:     "attached project alpha context",
	})
	if err != nil {
		t.Fatalf("SaveChatAttachment() error = %v", err)
	}

	service := &Service{
		Memory: store,
		Retriever: RetrieverFunc(func(ctx context.Context, content string) (RetrievalResult, error) {
			return RetrievalResult{Context: "workspace beta context", Sources: []string{"README.md"}, SourceKind: "workspace"}, nil
		}),
	}
	input := PlanInput{Content: "Access docs in the workspace and use my attachment"}
	if err := service.attachChatAttachments(ctx, conversation.ID, user.ID, []string{attachment.ID}, &input); err != nil {
		t.Fatalf("attachChatAttachments() error = %v", err)
	}
	if !strings.Contains(input.WorkspaceContext, "attached project alpha context") {
		t.Fatalf("WorkspaceContext = %q, want attachment context", input.WorkspaceContext)
	}
	if input.SourceKind != "attachment" {
		t.Fatalf("SourceKind = %q, want attachment before retrieval", input.SourceKind)
	}
	if err := service.attachRetrievalContext(ctx, &input); err != nil {
		t.Fatalf("attachRetrievalContext() error = %v", err)
	}
	if !strings.Contains(input.WorkspaceContext, "attached project alpha context") || !strings.Contains(input.WorkspaceContext, "workspace beta context") {
		t.Fatalf("WorkspaceContext = %q, want attachment and retrieval context", input.WorkspaceContext)
	}
	if input.SourceKind != "mixed" {
		t.Fatalf("SourceKind = %q, want mixed", input.SourceKind)
	}
	if !containsString(input.Sources, "attached-note.txt") || !containsString(input.Sources, "README.md") {
		t.Fatalf("Sources = %#v, want attachment and workspace sources", input.Sources)
	}
}

func TestCapabilityGapRouterProposesToolExtension(t *testing.T) {
	cfg := config.Default()
	extensionStore := extensions.NewStore(t.TempDir(), t.TempDir())
	router := NewCapabilityGapRouter(cfg, extensionStore)
	input := PlanInput{Content: "Build a tool that converts CSV notes into study flashcards"}
	plan := BuildPlan(input)
	decision := DecideExecution(plan, input)

	proposal, ok := router.Propose(plan, input, decision)
	if !ok {
		t.Fatal("Propose() ok = false, want tool extension proposal")
	}
	if proposal.Kind != CapabilityKindToolExtension {
		t.Fatalf("proposal.Kind = %q, want %q", proposal.Kind, CapabilityKindToolExtension)
	}
	if !proposal.CanGenerate {
		t.Fatal("proposal.CanGenerate = false, want true for generated tool extension")
	}
	if proposal.Extension == nil || proposal.Extension.Type != extensions.TypeTool {
		t.Fatalf("proposal.Extension = %+v, want tool extension proposal", proposal.Extension)
	}
	if !strings.Contains(proposal.Response(), "I will not create or enable this capability without your approval") {
		t.Fatalf("proposal response missing approval guard: %q", proposal.Response())
	}
}

func TestCapabilityGapRouterDoesNotTreatInternetSearchProviderAsExtensionGap(t *testing.T) {
	cfg := config.Default()
	cfg.Internet.Enabled = true
	cfg.Internet.DefaultMode = "profile_enabled"
	cfg.Internet.Search.Enabled = false
	router := NewCapabilityGapRouter(cfg, extensions.NewStore(t.TempDir(), t.TempDir()))
	input := PlanInput{Content: "Search the web for local-first agents"}
	plan := BuildPlan(input)
	decision := DecideExecution(plan, input)

	if proposal, ok := router.Propose(plan, input, decision); ok {
		t.Fatalf("proposal = %+v, want internet setup handled as a friendly tool/config response", proposal)
	}
}

func TestCapabilityGapRouterProposesBrokeredToolGeneration(t *testing.T) {
	cfg := config.Default()
	router := NewCapabilityGapRouter(cfg, extensions.NewStore(t.TempDir(), t.TempDir()))

	proposal, err := router.ProposeRequest("Build a tool that checks https://example.com through the web")
	if err != nil {
		t.Fatalf("ProposeRequest() error = %v", err)
	}
	if proposal.Kind != CapabilityKindToolExtension {
		t.Fatalf("proposal.Kind = %q, want %q", proposal.Kind, CapabilityKindToolExtension)
	}
	if proposal.NetworkMode != "core_broker" {
		t.Fatalf("proposal.NetworkMode = %q, want core_broker", proposal.NetworkMode)
	}
	if strings.Join(proposal.AllowedDomains, ",") != "example.com" {
		t.Fatalf("allowed domains = %+v, want example.com", proposal.AllowedDomains)
	}
	if !strings.Contains(proposal.GenerationCommand, "--brokered-network") || !strings.Contains(proposal.GenerationCommand, "--allow-domain example.com") {
		t.Fatalf("generation command = %q, want brokered-network domain command", proposal.GenerationCommand)
	}
	response := proposal.Response()
	for _, want := range []string{"Safety:", "internet uses the core broker", "GET/HEAD only"} {
		if !strings.Contains(response, want) {
			t.Fatalf("response = %q, want %q", response, want)
		}
	}
	for _, unwanted := range []string{"Network mode", "Generate:"} {
		if strings.Contains(response, unwanted) {
			t.Fatalf("response = %q, did not expect raw generation detail %q", response, unwanted)
		}
	}
}

func TestCapabilityGapRouterProposesBrokeredWorkflowGeneration(t *testing.T) {
	cfg := config.Default()
	router := NewCapabilityGapRouter(cfg, extensions.NewStore(t.TempDir(), t.TempDir()))

	proposal, err := router.ProposeRequest("Monitor https://example.com every hour for changes")
	if err != nil {
		t.Fatalf("ProposeRequest() error = %v", err)
	}
	if proposal.Kind != CapabilityKindWorkflow {
		t.Fatalf("proposal.Kind = %q, want %q", proposal.Kind, CapabilityKindWorkflow)
	}
	if proposal.NetworkMode != "core_broker" {
		t.Fatalf("proposal.NetworkMode = %q, want core_broker", proposal.NetworkMode)
	}
	if !strings.Contains(proposal.GenerationCommand, "--brokered-network") {
		t.Fatalf("generation command = %q, want brokered generation command", proposal.GenerationCommand)
	}
	if !strings.Contains(proposal.SuggestedAction, "scheduler job") {
		t.Fatalf("suggested action = %q, want scheduler job guidance", proposal.SuggestedAction)
	}
}

func TestCapabilityGapRouterBareDomainWorkflowCanGenerate(t *testing.T) {
	cfg := config.Default()
	router := NewCapabilityGapRouter(cfg, extensions.NewStore(t.TempDir(), t.TempDir()))

	proposal, err := router.ProposeRequest("Create a cron job for monitoring example.com every hour and record when it is down")
	if err != nil {
		t.Fatalf("ProposeRequest() error = %v", err)
	}
	if proposal.Kind != CapabilityKindWorkflow {
		t.Fatalf("proposal.Kind = %q, want %q", proposal.Kind, CapabilityKindWorkflow)
	}
	if !proposal.CanGenerate {
		t.Fatalf("proposal.CanGenerate = false, want true for public-domain brokered workflow; suggested action: %s", proposal.SuggestedAction)
	}
	if proposal.NetworkMode != "core_broker" {
		t.Fatalf("proposal.NetworkMode = %q, want core_broker", proposal.NetworkMode)
	}
	if strings.Join(proposal.AllowedDomains, ",") != "example.com" {
		t.Fatalf("allowed domains = %+v, want example.com", proposal.AllowedDomains)
	}
	if !strings.Contains(proposal.GenerationCommand, "--brokered-network") || !strings.Contains(proposal.GenerationCommand, "--allow-domain example.com") {
		t.Fatalf("generation command = %q, want brokered-network domain command", proposal.GenerationCommand)
	}
	if strings.Contains(proposal.SuggestedAction, "Provide the exact public URL or allowed domain") {
		t.Fatalf("suggested action = %q, did not expect missing-domain guidance", proposal.SuggestedAction)
	}
}

func TestCapabilityGapRouterGenerateBareDomainWorkflow(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	cfg := config.Default()
	store := extensions.NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), t.TempDir())
	router := NewCapabilityGapRouter(cfg, store)

	result, err := router.Generate(t.Context(), store, CapabilityGenerationInput{
		Request:           "Create a cron job for monitoring example.com every hour and record when it is down",
		Approved:          true,
		Name:              "example_monitor",
		MaxRuntimeSeconds: 5,
		TestTimeout:       2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Proposal.NetworkMode != "core_broker" || strings.Join(result.Proposal.AllowedDomains, ",") != "example.com" {
		t.Fatalf("proposal = %+v, want brokered network for example.com", result.Proposal)
	}
	if result.Generation.Extension.Name != "example_monitor" || result.Generation.Tests.Status != "passed" {
		t.Fatalf("generation = %+v, want tested brokered monitor extension", result.Generation)
	}
	if result.Schedule != nil {
		t.Fatalf("schedule = %+v, want no scheduler job until explicit schedule attachment approval", result.Schedule)
	}
}

func TestCapabilityGapRouterGenerateUsesApprovedAllowedDomains(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	cfg := config.Default()
	store := extensions.NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), t.TempDir())
	router := NewCapabilityGapRouter(cfg, store)

	result, err := router.Generate(t.Context(), store, CapabilityGenerationInput{
		Request:           "Create a cron job that checks a webpage every hour and records whether it changed",
		Approved:          true,
		Name:              "approved_webpage_monitor",
		AllowedDomains:    []string{"https://x.com/path", "X.com"},
		MaxRuntimeSeconds: 5,
		TestTimeout:       2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Proposal.NetworkMode != "core_broker" || strings.Join(result.Proposal.AllowedDomains, ",") != "x.com" {
		t.Fatalf("proposal = %+v, want brokered network for approved x.com domain", result.Proposal)
	}
	if !strings.Contains(result.Proposal.GenerationCommand, "--allow-domain x.com") {
		t.Fatalf("generation command = %q, want approved domain", result.Proposal.GenerationCommand)
	}
	if result.Generation.Extension.Name != "approved_webpage_monitor" || result.Generation.Tests.Status != "passed" {
		t.Fatalf("generation = %+v, want tested brokered webpage monitor extension", result.Generation)
	}
}

func TestCapabilityGapRouterGenerateRejectsInvalidApprovedAllowedDomains(t *testing.T) {
	cfg := config.Default()
	store := extensions.NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), t.TempDir())
	router := NewCapabilityGapRouter(cfg, store)

	_, err := router.Generate(t.Context(), store, CapabilityGenerationInput{
		Request:        "Create a cron job that checks a webpage every hour and records whether it changed",
		Approved:       true,
		Name:           "invalid_webpage_monitor",
		AllowedDomains: []string{"localhost", "127.0.0.1"},
	})
	if err == nil || !strings.Contains(err.Error(), "public URL or allowed domain") {
		t.Fatalf("Generate() error = %v, want missing valid public domain guard", err)
	}
}

func TestCapabilityGapRouterProposesWebpageWorkflowWithoutDomain(t *testing.T) {
	cfg := config.Default()
	router := NewCapabilityGapRouter(cfg, extensions.NewStore(t.TempDir(), t.TempDir()))

	proposal, err := router.ProposeRequest("I want a reusable tool that checks a webpage every hour and reports if the title changes. If Yemaka does not already have this capability, propose the extension but do not generate it until I approve.")
	if err != nil {
		t.Fatalf("ProposeRequest() error = %v", err)
	}
	if proposal.Kind != CapabilityKindWorkflow {
		t.Fatalf("proposal.Kind = %q, want %q", proposal.Kind, CapabilityKindWorkflow)
	}
	if proposal.CanGenerate {
		t.Fatal("proposal.CanGenerate = true, want false until exact URL/domain is provided")
	}
	if proposal.NetworkMode != "core_broker" {
		t.Fatalf("proposal.NetworkMode = %q, want core_broker", proposal.NetworkMode)
	}
	permissions := strings.Join(proposal.Permissions, ",")
	for _, want := range []string{"scheduler=approval_required", "internet=core_broker", "network=task_scoped", "methods=GET,HEAD"} {
		if !strings.Contains(permissions, want) {
			t.Fatalf("permissions = %q, want %q", permissions, want)
		}
	}
	if strings.Contains(permissions, "network=false") {
		t.Fatalf("permissions = %q, did not expect contradictory network=false", permissions)
	}
	response := proposal.Response()
	if !strings.Contains(response, "Allowed domains") && !strings.Contains(response, "public URL or allowed domain") {
		t.Fatalf("response = %q, want URL/domain guidance", response)
	}
	if !strings.Contains(response, "I will not create or enable this capability without your approval") {
		t.Fatalf("response = %q, want approval guard", response)
	}
}

func TestCapabilityGapRouterGenerateRequiresApprovalAndBuildsExtension(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	cfg := config.Default()
	store := extensions.NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), t.TempDir())
	router := NewCapabilityGapRouter(cfg, store)

	_, err := router.Generate(t.Context(), store, CapabilityGenerationInput{
		Request: "Build a tool that converts CSV notes into study flashcards",
		Name:    "study_flashcards",
	})
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("Generate() error = %v, want approval requirement", err)
	}

	result, err := router.Generate(t.Context(), store, CapabilityGenerationInput{
		Request:     "Build a tool that converts CSV notes into study flashcards",
		Approved:    true,
		Name:        "study_flashcards",
		TestTimeout: 2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Generation.Extension.Name != "study_flashcards" || result.Generation.Tests.Status != "passed" {
		t.Fatalf("generation = %+v, want tested study_flashcards extension", result.Generation)
	}
	if len(result.NextSteps) == 0 || !strings.Contains(strings.Join(result.NextSteps, "\n"), "extension inspect study_flashcards") {
		t.Fatalf("next steps = %+v, want inspect guidance", result.NextSteps)
	}
}

func TestCapabilityGapRouterGenerateCanRunApprovedExtension(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	cfg := config.Default()
	store := extensions.NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), t.TempDir())
	router := NewCapabilityGapRouter(cfg, store)

	result, err := router.Generate(t.Context(), store, CapabilityGenerationInput{
		Request:           "Build a tool that normalizes CSV rows",
		Approved:          true,
		Name:              "csv_rows",
		RunAfterGenerate:  true,
		RunInput:          map[string]any{"task": "normalize rows"},
		MaxRuntimeSeconds: 5,
		TestTimeout:       2 * time.Minute,
		RunOptions: extensions.RunOptions{
			Input:             map[string]any{"task": "normalize rows"},
			MaxRuntimeSeconds: 5,
		},
	})
	if err != nil {
		t.Fatalf("Generate(--run) error = %v", err)
	}
	if result.Run == nil {
		t.Fatal("Generate(--run) Run = nil, want run result")
	}
	if result.Run.Status != "completed" {
		t.Fatalf("run status = %q, error %q", result.Run.Status, result.Run.Error)
	}
	summary, _ := result.Run.Output["summary"].(string)
	if !strings.Contains(summary, "normalize rows") {
		t.Fatalf("run summary = %q, want task", summary)
	}
	if !strings.Contains(result.Message, "run after explicit approval") {
		t.Fatalf("message = %q, want run confirmation", result.Message)
	}
}

func TestCapabilityGapRouterGenerateSynthesizesDraftWithLocalModel(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	cfg := config.Default()
	cfg.Models["coding"] = config.ModelConfig{
		Provider:    "ollama",
		Name:        "local-coder:3b",
		BaseURL:     "http://localhost:11434/api",
		Temperature: 0.1,
	}
	store := extensions.NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), t.TempDir())
	router := NewCapabilityGapRouter(cfg, store)
	var requests []models.ChatRequest
	draftJSON := capabilityDraftJSONForTest(t, "csv_synth", "Normalize CSV rows.")

	result, err := router.Generate(t.Context(), store, CapabilityGenerationInput{
		Request:          "Build a tool that normalizes CSV rows",
		Approved:         true,
		Name:             "csv_synth",
		SynthesizeDraft:  true,
		RunAfterGenerate: true,
		RunInput:         map[string]any{"task": "normalize rows"},
		TestTimeout:      2 * time.Minute,
		DraftModel:       cfg.Models["coding"],
		DraftRuntimeFactory: func(model config.ModelConfig) (models.Runtime, error) {
			if model.Name != "local-coder:3b" {
				t.Fatalf("draft model = %q, want local-coder:3b", model.Name)
			}
			return recordingRuntime{text: draftJSON, requests: &requests}, nil
		},
		RunOptions: extensions.RunOptions{Input: map[string]any{"task": "normalize rows"}},
	})
	if err != nil {
		t.Fatalf("Generate(--synthesize) error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("draft runtime requests = %d, want 1", len(requests))
	}
	if result.Generation.Extension.Name != "csv_synth" || result.Generation.Tests.Status != "passed" {
		t.Fatalf("generation = %+v, want tested csv_synth draft extension", result.Generation)
	}
	if result.Run == nil || result.Run.Status != "completed" {
		t.Fatalf("run = %+v, want completed synthesized draft run", result.Run)
	}
	summary, _ := result.Run.Output["summary"].(string)
	if !strings.Contains(summary, "synth handled normalize rows") {
		t.Fatalf("run summary = %q, want synthesized draft output", summary)
	}
}

func TestCapabilityGapRouterGenerateRepairsSynthesizedDraft(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	cfg := config.Default()
	cfg.Models["coding"] = config.ModelConfig{
		Provider:    "ollama",
		Name:        "local-coder:3b",
		BaseURL:     "http://localhost:11434/api",
		Temperature: 0.1,
	}
	store := extensions.NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), t.TempDir())
	router := NewCapabilityGapRouter(cfg, store)
	var requests []models.ChatRequest
	runtime := &sequenceRuntime{
		texts: []string{
			capabilityDraftJSONForTest(t, "wrong_csv_repair", "Normalize CSV rows."),
			capabilityDraftJSONForTest(t, "csv_repair", "Normalize CSV rows."),
		},
		requests: &requests,
	}

	result, err := router.Generate(t.Context(), store, CapabilityGenerationInput{
		Request:          "Build a tool that normalizes CSV rows",
		Approved:         true,
		Name:             "csv_repair",
		SynthesizeDraft:  true,
		RunAfterGenerate: true,
		RunInput:         map[string]any{"task": "normalize rows"},
		TestTimeout:      2 * time.Minute,
		DraftModel:       cfg.Models["coding"],
		DraftRuntimeFactory: func(model config.ModelConfig) (models.Runtime, error) {
			return runtime, nil
		},
		RunOptions: extensions.RunOptions{Input: map[string]any{"task": "normalize rows"}},
	})
	if err != nil {
		t.Fatalf("Generate(--synthesize repair) error = %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("draft runtime requests = %d, want 2", len(requests))
	}
	if !strings.Contains(requests[1].Messages[len(requests[1].Messages)-1].Content, "Trusted-core failure summary") {
		t.Fatalf("repair prompt = %q, want failure summary", requests[1].Messages[len(requests[1].Messages)-1].Content)
	}
	if result.Repairs != 1 {
		t.Fatalf("repairs = %d, want 1", result.Repairs)
	}
	if result.Generation.Extension.Name != "csv_repair" || result.Generation.Tests.Status != "passed" {
		t.Fatalf("generation = %+v, want repaired csv_repair draft extension", result.Generation)
	}
	if !strings.Contains(result.Message, "repaired 1 time") {
		t.Fatalf("message = %q, want repair disclosure", result.Message)
	}
	if result.Run == nil || result.Run.Status != "completed" {
		t.Fatalf("run = %+v, want completed repaired draft run", result.Run)
	}
}

func TestCapabilityGapRouterGenerateSynthesizeRequiresRuntimeFactory(t *testing.T) {
	cfg := config.Default()
	store := extensions.NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), t.TempDir())
	router := NewCapabilityGapRouter(cfg, store)

	_, err := router.Generate(t.Context(), store, CapabilityGenerationInput{
		Request:         "Build a tool that normalizes CSV rows",
		Approved:        true,
		Name:            "csv_synth_missing_runtime",
		SynthesizeDraft: true,
		DraftModel:      cfg.Models["coding"],
	})
	if err == nil || !strings.Contains(err.Error(), "runtime factory") {
		t.Fatalf("Generate(--synthesize) error = %v, want runtime factory error", err)
	}
}

func TestChatUnsupportedCapabilityEmitsCapabilityGap(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	service := &Service{
		Router:        models.NewRouter(cfg),
		Runtime:       recordingRuntime{text: "model should not run", requests: &requests},
		Memory:        store,
		CapabilityGap: NewCapabilityGapRouter(cfg, extensions.NewStore(t.TempDir(), t.TempDir())),
	}

	var output string
	seen := map[string]bool{}
	var gapRequest string
	err = service.Chat(ctx, ChatInput{Content: "Build a tool that normalizes CSV flashcards"}, func(event Event) error {
		seen[event.Type] = true
		if event.Type == EventCapabilityGap {
			gapRequest = event.Data["request"]
		}
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0", len(requests))
	}
	if !seen[EventCapabilityGap] {
		t.Fatal("capability gap event was not emitted")
	}
	if gapRequest != "Build a tool that normalizes CSV flashcards" {
		t.Fatalf("capability gap request = %q, want original request", gapRequest)
	}
	if !strings.Contains(output, "Proposed next capability") || !strings.Contains(output, "without your approval") {
		t.Fatalf("output = %q, want capability proposal with approval guard", output)
	}
}

func TestAskReusableWebpageMonitorUsesCapabilityHandoffWithoutWorkspace(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	retrieverRan := false
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "model should not run", requests: &requests},
		Memory:  store,
		Retriever: RetrieverFunc(func(ctx context.Context, content string) (RetrievalResult, error) {
			retrieverRan = true
			return RetrievalResult{Context: "README.md workspace context should not be used", Sources: []string{"README.md"}, SourceKind: "workspace"}, nil
		}),
		CapabilityGap: NewCapabilityGapRouter(cfg, extensions.NewStore(t.TempDir(), t.TempDir())),
	}

	content := "I want a reusable tool that checks a webpage every hour and reports if the title changes. If Yemaka does not already have this capability, propose the extension but do not generate it until I approve."
	var output string
	seen := map[string]bool{}
	var gapData map[string]string
	err = service.Ask(ctx, AskInput{Content: content}, func(event Event) error {
		seen[event.Type] = true
		if event.Type == EventCapabilityGap {
			gapData = event.Data
		}
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if retrieverRan {
		t.Fatal("retriever ran for capability handoff request, want no workspace retrieval")
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0", len(requests))
	}
	if !seen[EventCapabilityGap] {
		t.Fatal("capability gap event was not emitted")
	}
	if gapData["kind"] != CapabilityKindWorkflow {
		t.Fatalf("capability kind = %q, want workflow; data=%v", gapData["kind"], gapData)
	}
	for _, want := range []string{"Proposed next capability", "workflow", "scheduler", "internet", "network", "without your approval"} {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(want)) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
	for _, unwanted := range []string{"Evidence from workspace context", "workspace context should not be used"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("output = %q, did not expect %q", output, unwanted)
		}
	}
}

func TestAskWebsiteDownMonitorUsesCapabilityHandoffWithoutModelScript(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	service := &Service{
		Router:        models.NewRouter(cfg),
		Runtime:       recordingRuntime{text: "model should not write a monitoring script", requests: &requests},
		Memory:        store,
		CapabilityGap: NewCapabilityGapRouter(cfg, extensions.NewStore(t.TempDir(), t.TempDir())),
	}

	var output string
	var gapData map[string]string
	err = service.Ask(ctx, AskInput{Content: "Create a sub agent that will monitor https://example.com and tell me when it is down"}, func(event Event) error {
		if event.Type == EventCapabilityGap {
			gapData = event.Data
		}
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0", len(requests))
	}
	if gapData["kind"] != CapabilityKindWorkflow {
		t.Fatalf("capability kind = %q, want workflow; data=%v", gapData["kind"], gapData)
	}
	for _, want := range []string{"Proposed next capability", "workflow", "scheduler", "core internet broker", "without your approval"} {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(want)) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
	for _, unwanted := range []string{"dockerfile", "smtplib", "slack", "schedule.run_pending", "pip install"} {
		if strings.Contains(strings.ToLower(output), unwanted) {
			t.Fatalf("output = %q, did not expect model-authored script term %q", output, unwanted)
		}
	}
}

func TestAskContinuesExistingConversationSession(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "Greeting")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "My name is Yemaka.",
		Model:          "test",
	}); err != nil {
		t.Fatalf("SaveMessage() error = %v", err)
	}

	var requests []models.ChatRequest
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "continued", requests: &requests},
		Memory:  store,
	}
	var completed string
	err = service.Ask(ctx, AskInput{
		ConversationID:   conversation.ID,
		Content:          "What did I say my name was?",
		WorkspaceContext: "README.md: local context",
		SourceKind:       "workspace",
	}, func(event Event) error {
		if event.Type == EventAgentCompleted {
			completed = event.Data["conversation_id"]
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if completed != conversation.ID {
		t.Fatalf("completed conversation = %q, want %q", completed, conversation.ID)
	}
	messages, err := store.ListConversationMessages(ctx, conversation.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("message count = %d, want 3", len(messages))
	}
	if len(requests) == 0 || !strings.Contains(requests[0].Messages[1].Content, "RECENT SESSION MESSAGES") {
		t.Fatalf("model request did not include session context: %+v", requests)
	}
}

func TestAskEditRequestEmitsPermissionRequestAndModelDraft(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "YEMAKA_EDIT_PROPOSAL\npath: README.md\ncontent:\n```md\n# Yemaka\n\nPermission gates are documented.\n```", requests: &requests},
		Memory:  store,
	}

	var output string
	var permission Event
	var proposals []Event
	var eventTypes []string
	err = service.Ask(ctx, AskInput{
		Content:          "Edit README.md to mention permission gates",
		WorkspaceContext: "README.md: # Yemaka",
		Sources:          []string{"README.md"},
		SourceKind:       "workspace",
	}, func(event Event) error {
		eventTypes = append(eventTypes, event.Type)
		if event.Type == EventModelToken {
			output += event.Token
		}
		if event.Type == EventPermissionRequested {
			permission = event
		}
		if event.Type == EventEditProposed {
			proposals = append(proposals, event)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("model requests = %d, want 1", len(requests))
	}
	if !strings.Contains(requests[0].Messages[0].Content, "YEMAKA_EDIT_PROPOSAL") {
		t.Fatalf("system prompt missing edit draft instructions: %s", requests[0].Messages[0].Content)
	}
	if permission.Type != EventPermissionRequested {
		t.Fatal("permission request event was not emitted")
	}
	if permission.Data["tool_name"] != "edit_file" {
		t.Fatalf("permission tool_name = %q, want edit_file", permission.Data["tool_name"])
	}
	if permission.Data["request_id"] == "" {
		t.Fatal("permission request_id is empty")
	}
	if permission.Data["snapshot_before_write"] != "true" {
		t.Fatalf("snapshot_before_write = %q, want true", permission.Data["snapshot_before_write"])
	}
	if len(proposals) != 2 {
		t.Fatalf("edit proposal events = %d, want initial and model draft", len(proposals))
	}
	permissionIndex := -1
	firstProposalIndex := -1
	readyProposalIndex := -1
	proposalCount := 0
	for index, eventType := range eventTypes {
		if eventType == EventPermissionRequested {
			permissionIndex = index
		}
		if eventType == EventEditProposed {
			if proposalCount == 0 {
				firstProposalIndex = index
			} else if proposalCount == 1 {
				readyProposalIndex = index
			}
			proposalCount++
		}
	}
	if firstProposalIndex == -1 || readyProposalIndex == -1 || permissionIndex == -1 {
		t.Fatalf("event order missing proposal/permission: %v", eventTypes)
	}
	if permissionIndex < readyProposalIndex {
		t.Fatalf("permission emitted before ready edit proposal: events=%v", eventTypes)
	}
	initial := proposals[0]
	draft := proposals[1]
	if initial.Data["path"] != "README.md" || draft.Data["path"] != "README.md" {
		t.Fatalf("proposal paths = %q, %q; want README.md", initial.Data["path"], draft.Data["path"])
	}
	if initial.Data["needs_content"] != "true" {
		t.Fatalf("initial needs_content = %q, want true", initial.Data["needs_content"])
	}
	if draft.Data["needs_content"] != "false" {
		t.Fatalf("draft needs_content = %q, want false", draft.Data["needs_content"])
	}
	if draft.Data["content_source"] != "model" {
		t.Fatalf("draft content_source = %q, want model", draft.Data["content_source"])
	}
	if !strings.Contains(draft.Data["content"], "Permission gates are documented.") {
		t.Fatalf("draft content = %q, want model replacement body", draft.Data["content"])
	}
	if strings.Contains(strings.ToLower(draft.Data["content"]), "yemaka_edit_proposal") || strings.Contains(strings.ToLower(draft.Data["content"]), "```yemaka-file:") {
		t.Fatalf("draft content leaked raw edit proposal marker: %q", draft.Data["content"])
	}
	if strings.Contains(output, "YEMAKA_EDIT_PROPOSAL") {
		t.Fatalf("model output leaked structured edit proposal: %q", output)
	}
	if !strings.Contains(output, "I prepared an edit proposal for README.md") {
		t.Fatalf("model output = %q, want friendly edit proposal response", output)
	}
	results, err := store.SearchMessages(ctx, "prepared edit proposal README", 5)
	if err != nil {
		t.Fatalf("SearchMessages() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("friendly edit proposal response was not saved to memory")
	}
	conversations, err := store.ListConversations(ctx, 10)
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}
	if len(conversations) != 1 {
		t.Fatalf("conversations = %d, want 1", len(conversations))
	}
	messages, err := store.ListConversationMessages(ctx, conversations[0].ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	var savedAssistant string
	for _, message := range messages {
		if message.Role == "assistant" {
			savedAssistant = message.Content
		}
	}
	if savedAssistant == "" {
		t.Fatal("assistant edit proposal response was not saved")
	}
	if strings.Contains(strings.ToLower(savedAssistant), "yemaka_edit_proposal") || strings.Contains(strings.ToLower(savedAssistant), "```yemaka-file:") {
		t.Fatalf("saved assistant response leaked raw edit proposal marker: %q", savedAssistant)
	}
	runs, err := store.ListToolRuns(ctx, 10)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	var proposalSaved bool
	var permissionSaved bool
	for _, run := range runs {
		if run.ToolName == "edit_proposal" {
			proposal, ok := editProposalFromStoredValue(run.Output)
			if ok && run.Status == "ready_for_preview" && proposal.Path == "README.md" && !proposal.NeedsContent {
				proposalSaved = true
			}
		}
		if run.ToolName == "permission_request" {
			permissionSaved = true
			if run.Status != ExecutionNeedsConfirmation {
				t.Fatalf("permission_request status = %q, want %q", run.Status, ExecutionNeedsConfirmation)
			}
			stored, ok := permissionRequestFromStoredValue(run.Output)
			if !ok {
				t.Fatalf("permission_request output = %#v, want stored permission request", run.Output)
			}
			if stored.RequestID != permission.Data["request_id"] {
				t.Fatalf("permission request id = %q, want %q", stored.RequestID, permission.Data["request_id"])
			}
			if stored.ToolName != "edit_file" {
				t.Fatalf("permission tool name = %q, want edit_file", stored.ToolName)
			}
		}
	}
	if !proposalSaved {
		t.Fatal("edit proposal was not saved to tool runs")
	}
	if !permissionSaved {
		t.Fatal("permission request was not saved to tool runs")
	}
}

func TestAskEditRequestWithoutModelDraftDoesNotEmitPermissionRequest(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "I need the exact replacement content before I can prepare the edit."},
		Memory:  store,
	}

	var permission Event
	var proposals []Event
	var output strings.Builder
	err = service.Ask(ctx, AskInput{
		Content:          "Edit README.md to mention permission gates",
		WorkspaceContext: "README.md: # Yemaka",
		Sources:          []string{"README.md"},
		SourceKind:       "workspace",
	}, func(event Event) error {
		if event.Type == EventPermissionRequested {
			permission = event
		}
		if event.Type == EventEditProposed {
			proposals = append(proposals, event)
		}
		if event.Type == EventModelToken {
			output.WriteString(event.Token)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if permission.Type == EventPermissionRequested {
		t.Fatalf("permission request was emitted before a ready edit proposal: %+v", permission)
	}
	if len(proposals) != 1 {
		t.Fatalf("edit proposal events = %d, want only initial incomplete proposal", len(proposals))
	}
	if proposals[0].Data["needs_content"] != "true" {
		t.Fatalf("initial needs_content = %q, want true", proposals[0].Data["needs_content"])
	}
	rendered := output.String()
	if strings.Contains(rendered, "exact replacement content before I can prepare the edit") {
		t.Fatalf("output leaked raw model fallback: %q", rendered)
	}
	for _, unwanted := range []string{"Approval Request", "Once you confirm", "I will write the file"} {
		if strings.Contains(rendered, unwanted) {
			t.Fatalf("output = %q, did not expect fake approval prose %q", rendered, unwanted)
		}
	}
	if !strings.Contains(rendered, "I have not changed the file") || !strings.Contains(rendered, "full replacement content") {
		t.Fatalf("output = %q, want grounded missing-content response", rendered)
	}
	runs, err := store.ListToolRuns(ctx, 20)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	for _, run := range runs {
		if run.ToolName == "permission_request" {
			t.Fatalf("permission request was persisted before a ready edit proposal: %+v", run)
		}
	}
}

func TestAskActiveFollowupSkillEmitsMemoryWritePermission(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	service := &Service{
		Router: models.NewRouter(cfg),
		Memory: store,
	}
	var permission Event
	var output string
	err = service.Ask(ctx, AskInput{
		Content:            "capture this follow-up: email Alex tomorrow about the release checklist",
		SkillName:          "followup_capture",
		SkillVersion:       "0.1.0",
		SkillRequiredTools: []string{"memory_search", "memory_write"},
		SkillInstructions:  "Use memory_write only for confirmed local follow-up memory.",
	}, func(event Event) error {
		if event.Type == EventPermissionRequested {
			permission = event
		}
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if permission.Type != EventPermissionRequested {
		t.Fatal("permission request event was not emitted")
	}
	if permission.Data["tool_name"] != "memory_write" {
		t.Fatalf("permission tool_name = %q, want memory_write", permission.Data["tool_name"])
	}
	if permission.Data["workspace_only"] != "false" {
		t.Fatalf("workspace_only = %q, want false for memory write", permission.Data["workspace_only"])
	}
	var command []string
	if err := json.Unmarshal([]byte(permission.Data["command_json"]), &command); err != nil {
		t.Fatalf("parse command_json %q: %v", permission.Data["command_json"], err)
	}
	wantCommand := []string{"memory_write", "follow_up", "email Alex tomorrow about the release checklist"}
	if strings.Join(command, "\x00") != strings.Join(wantCommand, "\x00") {
		t.Fatalf("command = %#v, want %#v", command, wantCommand)
	}
	if !strings.Contains(output, "needs your approval") {
		t.Fatalf("output = %q, want approval summary", output)
	}
	memories, err := store.ListMemories(ctx, 50)
	if err != nil {
		t.Fatalf("ListMemories() error = %v", err)
	}
	for _, item := range memories {
		if item.Kind == "follow_up" {
			t.Fatalf("follow_up memory was saved before approval: %+v", item)
		}
	}
}

func TestAskPersistsPermissionBeforeEmittingApprovalID(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	service := &Service{
		Router: models.NewRouter(cfg),
		Memory: store,
	}
	var emitted PermissionRequest
	var persistedBeforeEmit bool
	err = service.Ask(ctx, AskInput{
		Content:            "capture this follow-up: email Alex tomorrow about the release checklist",
		SkillName:          "followup_capture",
		SkillVersion:       "0.1.0",
		SkillRequiredTools: []string{"memory_search", "memory_write"},
		SkillInstructions:  "Use memory_write only for confirmed local follow-up memory.",
	}, func(event Event) error {
		if event.Type != EventPermissionRequested {
			return nil
		}
		var command []string
		if raw := strings.TrimSpace(event.Data["command_json"]); raw != "" {
			if err := json.Unmarshal([]byte(raw), &command); err != nil {
				return err
			}
		}
		emitted = PermissionRequest{
			RequestID:            event.Data["request_id"],
			ToolName:             event.Data["tool_name"],
			Command:              command,
			RiskLevel:            event.Data["risk_level"],
			Reason:               event.Data["reason"],
			RequiresConfirmation: event.Data["requires_confirmation"] == "true",
		}
		runs, err := store.ListToolRuns(ctx, 10)
		if err != nil {
			return err
		}
		for _, run := range runs {
			if run.ToolName != "permission_request" {
				continue
			}
			stored, ok := permissionRequestFromStoredValue(run.Output)
			if ok && stored.RequestID == emitted.RequestID {
				persistedBeforeEmit = true
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if emitted.RequestID == "" {
		t.Fatal("permission request event was not emitted")
	}
	if !persistedBeforeEmit {
		t.Fatalf("permission request %q was not persisted before emit", emitted.RequestID)
	}
	if err := ValidateStoredPermissionApproval(ctx, store, emitted); err != nil {
		t.Fatalf("ValidateStoredPermissionApproval() error = %v", err)
	}
}

func TestBuildEditProposalParsesInlineContent(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Write notes.md with content: hello local world"})
	decision := DecideExecution(plan, PlanInput{Content: "Write notes.md with content: hello local world"})
	proposal := BuildEditProposal(plan, PlanInput{Content: "Write notes.md with content: hello local world"}, decision)
	if proposal.Path != "notes.md" {
		t.Fatalf("Path = %q, want notes.md", proposal.Path)
	}
	if proposal.Content != "hello local world" {
		t.Fatalf("Content = %q, want inline content", proposal.Content)
	}
	if proposal.Status != "ready_for_preview" {
		t.Fatalf("Status = %q, want ready_for_preview", proposal.Status)
	}
	if proposal.NeedsContent {
		t.Fatal("NeedsContent = true, want false")
	}
}

func TestBuildEditProposalCombinesDirectoryNameAndInlineContent(t *testing.T) {
	content := "Create a file in into path /users/example/downloads, the name should hello.txt and write into it hello World"
	input := PlanInput{Content: content}
	plan := BuildPlan(input)
	decision := DecideExecution(plan, input)
	proposal := BuildEditProposal(plan, input, decision)
	if proposal.Path != "/users/example/downloads/hello.txt" {
		t.Fatalf("Path = %q, want full named target path", proposal.Path)
	}
	if proposal.Content != "hello World" {
		t.Fatalf("Content = %q, want inline content", proposal.Content)
	}
	if proposal.Status != "ready_for_preview" {
		t.Fatalf("Status = %q, want ready_for_preview", proposal.Status)
	}
	if proposal.NeedsContent {
		t.Fatal("NeedsContent = true, want false")
	}
}

func TestBuildEditProposalParsesCreateFileWithTextContent(t *testing.T) {
	content := "Create a file at /Users/example/Desktop/yemaka-qa-file.txt with the text hello from qa."
	input := PlanInput{Content: content}
	plan := BuildPlan(input)
	decision := DecideExecution(plan, input)
	proposal := BuildEditProposal(plan, input, decision)
	if proposal.Path != "/Users/example/Desktop/yemaka-qa-file.txt" {
		t.Fatalf("Path = %q, want explicit target path", proposal.Path)
	}
	if proposal.Content != "hello from qa." {
		t.Fatalf("Content = %q, want inline text content", proposal.Content)
	}
	if proposal.Status != "ready_for_preview" {
		t.Fatalf("Status = %q, want ready_for_preview", proposal.Status)
	}
	if proposal.NeedsContent {
		t.Fatal("NeedsContent = true, want false")
	}
}

func TestBuildEditProposalParsesExactLineContent(t *testing.T) {
	content := strings.Join([]string{
		"Create or update test-files/permission_smoke.md with exactly this line:",
		"",
		"Permission approval works.",
		"",
		"Show me the diff first, create a snapshot, and only apply it after I approve.",
	}, "\n")
	plan := BuildPlan(PlanInput{Content: content})
	decision := DecideExecution(plan, PlanInput{Content: content})
	proposal := BuildEditProposal(plan, PlanInput{Content: content}, decision)
	if proposal.Path != "test-files/permission_smoke.md" {
		t.Fatalf("Path = %q, want test-files/permission_smoke.md", proposal.Path)
	}
	if proposal.Content != "Permission approval works." {
		t.Fatalf("Content = %q, want exact single-line content", proposal.Content)
	}
	if proposal.Status != "ready_for_preview" {
		t.Fatalf("Status = %q, want ready_for_preview", proposal.Status)
	}
	if proposal.NeedsContent {
		t.Fatal("NeedsContent = true, want false")
	}
}

func TestAskInlineFileCreateKeepsExactTargetAndSkipsWorkspaceRetrieval(t *testing.T) {
	ctx := context.Background()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	retrieverRan := false
	service := &Service{
		Router:  models.NewRouter(config.Default()),
		Runtime: recordingRuntime{text: "model should not draft this edit", requests: &requests},
		Memory:  store,
		Retriever: RetrieverFunc(func(ctx context.Context, content string) (RetrievalResult, error) {
			retrieverRan = true
			return RetrievalResult{Context: "README.md: unrelated workspace context", Sources: []string{"README.md"}, SourceKind: "workspace"}, nil
		}),
	}

	content := "Create a file in into path /users/example/downloads, the name should hello.txt and write into it hello World"
	var output strings.Builder
	var proposalData map[string]string
	var workspaceEvents int
	err = service.Ask(ctx, AskInput{Content: content}, func(event Event) error {
		switch event.Type {
		case EventEditProposed:
			proposalData = event.Data
		case EventWorkspaceUsed:
			workspaceEvents++
		case EventModelToken:
			output.WriteString(event.Token)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if retrieverRan {
		t.Fatal("retriever ran for inline file-create request, want no unrelated workspace context")
	}
	if workspaceEvents != 0 {
		t.Fatalf("workspace events = %d, want 0", workspaceEvents)
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0", len(requests))
	}
	if proposalData["path"] != "/users/example/downloads/hello.txt" {
		t.Fatalf("proposal path = %q, want full named target path; data=%v", proposalData["path"], proposalData)
	}
	if proposalData["content"] != "hello World" || proposalData["status"] != "ready_for_preview" || proposalData["needs_content"] != "false" {
		t.Fatalf("proposal data = %v, want ready inline content", proposalData)
	}
	rendered := output.String()
	if !strings.Contains(rendered, "/users/example/downloads/hello.txt") || !strings.Contains(rendered, "I have not changed the file") {
		t.Fatalf("output = %q, want exact path and not-changed approval wording", rendered)
	}
	for _, unwanted := range []string{"README.md", "unrelated workspace context", "/users/example/downloads. Review"} {
		if strings.Contains(rendered, unwanted) {
			t.Fatalf("output = %q, did not expect %q", rendered, unwanted)
		}
	}
}

func TestChatEditProposalUsesPreviousAssistantResponseForLastResponseRequest(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "last response edit")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "search AI latest trends on x.com",
		Model:          "yemaka",
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	const previousAssistant = "# Search Results Summary\n\n| Trend | Source |\n|---|---|\n| Agentic AI | Progress Software Blog |\n"
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        previousAssistant,
		Model:          "local-model",
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}

	var requests []models.ChatRequest
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "model should not be called", requests: &requests},
		Memory:  store,
	}

	var output strings.Builder
	var permission Event
	var proposal Event
	err = service.Chat(ctx, ChatInput{
		ConversationID: conversation.ID,
		Content:        "create file inside /users/example/documents and name it tavily-implementation.md and insert the last response inside it",
	}, func(event Event) error {
		if event.Type == EventModelToken {
			output.WriteString(event.Token)
		}
		if event.Type == EventPermissionRequested {
			permission = event
		}
		if event.Type == EventEditProposed {
			proposal = event
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0", len(requests))
	}
	if proposal.Type != EventEditProposed {
		t.Fatal("edit proposal was not emitted")
	}
	if proposal.Data["path"] != "/users/example/documents/tavily-implementation.md" {
		t.Fatalf("proposal path = %q, want /users/example/documents/tavily-implementation.md; data=%v", proposal.Data["path"], proposal.Data)
	}
	if proposal.Data["needs_content"] != "false" || proposal.Data["status"] != "ready_for_preview" {
		t.Fatalf("proposal data = %v, want ready previous-response content", proposal.Data)
	}
	if proposal.Data["content_source"] != "conversation_last_response" {
		t.Fatalf("content_source = %q, want conversation_last_response", proposal.Data["content_source"])
	}
	if proposal.Data["content"] != strings.TrimSpace(previousAssistant) {
		t.Fatalf("proposal content = %q, want previous assistant response", proposal.Data["content"])
	}
	if permission.Type != EventPermissionRequested {
		t.Fatal("permission request was not emitted for ready previous-response edit")
	}
	rendered := output.String()
	if !strings.Contains(rendered, "I have not changed the file") || !strings.Contains(rendered, "Review the diff preview") {
		t.Fatalf("output = %q, want safe approval wording", rendered)
	}
	if strings.Contains(rendered, "Approval Request") || strings.Contains(rendered, "Once you confirm") {
		t.Fatalf("output = %q, leaked model-style approval prose", rendered)
	}
}

func TestChatEditProposalSkipsOperationalAssistantMessagesForLastResponseRequest(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "last response skips status")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	const wantedAssistant = "# Search Results Summary\n\nThis is the answer that should be saved.\n"
	messages := []memory.Message{
		{ConversationID: conversation.ID, Role: "user", Content: "search AI latest trends on x.com", Model: "yemaka"},
		{ConversationID: conversation.ID, Role: "assistant", Content: wantedAssistant, Model: "local-model"},
		{ConversationID: conversation.ID, Role: "assistant", Content: "Approval recorded. File edits still need the diff and snapshot flow before I apply anything.", Model: "local-model"},
		{ConversationID: conversation.ID, Role: "assistant", Content: "I could not use file_stat: path is outside workspace: /users/example/documents/tavily-implementation.md", Model: "local-model"},
	}
	for _, msg := range messages {
		if _, err := store.SaveMessage(ctx, msg); err != nil {
			t.Fatalf("SaveMessage(%s) error = %v", msg.Role, err)
		}
	}

	var requests []models.ChatRequest
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "model should not be called", requests: &requests},
		Memory:  store,
	}

	var proposal Event
	err = service.Chat(ctx, ChatInput{
		ConversationID: conversation.ID,
		Content:        "create file inside /users/example/documents and name it tavily-implementation.md and insert the last response inside it",
	}, func(event Event) error {
		if event.Type == EventEditProposed {
			proposal = event
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0", len(requests))
	}
	if proposal.Type != EventEditProposed {
		t.Fatal("edit proposal was not emitted")
	}
	if got := proposal.Data["content"]; got != strings.TrimSpace(wantedAssistant) {
		t.Fatalf("proposal content = %q, want real previous assistant answer", got)
	}
}

func TestChatStandalonePromptDoesNotInjectStaleConversationContext(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "stale context")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	stalePath := "/users/example/documents/tavily-implementation.md"
	for _, msg := range []memory.Message{
		{ConversationID: conversation.ID, Role: "user", Content: "check if " + stalePath + " exists"},
		{ConversationID: conversation.ID, Role: "assistant", Content: "I could not use file_stat: path is outside workspace: " + stalePath},
	} {
		if _, err := store.SaveMessage(ctx, msg); err != nil {
			t.Fatalf("SaveMessage(%s) error = %v", msg.Role, err)
		}
	}
	state, err := json.Marshal(routing.SessionContract{
		ActiveGoal:       "check file",
		ActiveRoute:      routing.RouteFileRead,
		ActiveTarget:     stalePath,
		ActiveCapability: "filesystem_read",
		ActiveLane:       routing.ToolLaneResearch,
		TaskStatus:       routing.TaskStatusActive,
		LastOutcome:      routing.LastOutcomeFailed,
		FailureReason:    "path is outside workspace: " + stalePath,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("marshal route state: %v", err)
	}
	if _, err := store.SaveConversationRouteState(ctx, memory.ConversationRouteState{
		ConversationID: conversation.ID,
		StateJSON:      string(state),
	}); err != nil {
		t.Fatalf("SaveConversationRouteState() error = %v", err)
	}

	var requests []models.ChatRequest
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "Hello.", requests: &requests},
		Memory:  store,
	}
	if err := service.Chat(ctx, ChatInput{ConversationID: conversation.ID, Content: "hello"}, func(Event) error { return nil }); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("model requests = %d, want 1", len(requests))
	}
	prompt := requestPromptText(requests[0])
	for _, unwanted := range []string{stalePath, "RECENT SESSION MESSAGES", "path is outside workspace"} {
		if strings.Contains(prompt, unwanted) {
			t.Fatalf("prompt leaked stale context %q:\n%s", unwanted, prompt)
		}
	}
}

func requestPromptText(req models.ChatRequest) string {
	var parts []string
	parts = append(parts, req.System)
	for _, msg := range req.Messages {
		parts = append(parts, msg.Content)
	}
	return strings.Join(parts, "\n")
}

func TestChatEditProposalDoesNotWriteBeforeApproval(t *testing.T) {
	ctx := context.Background()
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "test-files"), 0o755); err != nil {
		t.Fatalf("mkdir test-files: %v", err)
	}
	t.Chdir(workspace)
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	service := &Service{Memory: store}

	var verification Event
	var output strings.Builder
	err = service.Chat(ctx, ChatInput{Content: strings.Join([]string{
		"Create or update test-files/permission_smoke.md with exactly this line:",
		"",
		"Permission approval works.",
		"",
		"Show me the diff first, create a snapshot, and only apply it after I approve.",
	}, "\n")}, func(event Event) error {
		if event.Type == EventVerificationCompleted {
			verification = event
		}
		if event.Type == EventModelToken {
			output.WriteString(event.Token)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "test-files", "permission_smoke.md")); !os.IsNotExist(err) {
		t.Fatalf("permission_smoke.md exists before approval or stat error = %v", err)
	}
	if verification.Data["status"] != "needs_follow_up" {
		t.Fatalf("verification status = %q, want needs_follow_up", verification.Data["status"])
	}
	if !strings.Contains(output.String(), "I have not changed the file") {
		t.Fatalf("output = %q, want explicit not-changed wording", output.String())
	}
	runs, err := store.ListToolRuns(ctx, 20)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	for _, run := range runs {
		if run.ToolName == "write_file" {
			t.Fatalf("write_file ran before approval: %+v", run)
		}
	}
}

func TestChatAppendLineProposalUsesCurrentFileWithoutWriting(t *testing.T) {
	ctx := context.Background()
	workspace := t.TempDir()
	target := filepath.Join(workspace, "test-files", "permission_smoke.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir test-files: %v", err)
	}
	if err := os.WriteFile(target, []byte("Permission approval works."), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	t.Chdir(workspace)
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	cfg := config.Default()
	service := &Service{
		Memory:       store,
		ToolExecutor: NewSafeToolExecutor(SafeToolConfig{WorkspaceRoot: workspace, Config: cfg}),
	}

	var proposalEvent Event
	var sawRead bool
	err = service.Chat(ctx, ChatInput{Content: strings.Join([]string{
		"Append this line to test-files/permission_smoke.md:",
		"",
		"Second approval still works.",
		"",
		"Show me the diff first and wait for approval.",
	}, "\n")}, func(event Event) error {
		if event.Type == EventEditProposed {
			proposalEvent = event
		}
		if event.Type == EventToolCompleted && event.Data["tool_name"] == "read_file" {
			sawRead = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if !sawRead {
		t.Fatal("read_file was not used to ground append proposal")
	}
	if proposalEvent.Type != EventEditProposed {
		t.Fatal("edit proposal was not emitted")
	}
	wantProposal := "Permission approval works.\nSecond approval still works."
	if proposalEvent.Data["content"] != wantProposal {
		t.Fatalf("proposal content = %q, want %q", proposalEvent.Data["content"], wantProposal)
	}
	if proposalEvent.Data["content_source"] != "inline_append" {
		t.Fatalf("content_source = %q, want inline_append", proposalEvent.Data["content_source"])
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != "Permission approval works." {
		t.Fatalf("file changed before approval: %q", string(data))
	}
}

func TestAskToolRequestUsesInjectedExecutor(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var executed bool
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "tests explained", requests: &requests},
		Memory:  store,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			executed = true
			if decision.ToolName != "run_tests" {
				t.Fatalf("ToolName = %q, want run_tests", decision.ToolName)
			}
			return ExecutionResult{
				Context:    "go test ./...\nPASS",
				Sources:    []string{"go test ./..."},
				SourceKind: "tool",
				Status:     "completed",
			}, nil
		},
	}

	seen := map[string]bool{}
	err = service.Ask(ctx, AskInput{
		Content:          "Run tests and explain failures",
		WorkspaceContext: "workspace context",
		SourceKind:       "workspace",
	}, func(event Event) error {
		seen[event.Type] = true
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if !executed {
		t.Fatal("tool executor was not called")
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
	if !strings.Contains(requests[0].Messages[1].Content, "TOOL RESULT") {
		t.Fatalf("prompt missing tool result: %s", requests[0].Messages[1].Content)
	}
	if !strings.Contains(requests[0].Messages[1].Content, "source_kind: tool") {
		t.Fatalf("prompt missing tool source kind: %s", requests[0].Messages[1].Content)
	}
	if !seen[EventToolCompleted] {
		t.Fatal("tool completion event was not emitted")
	}
	runs, err := store.ListToolRuns(ctx, 10)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	var found bool
	for _, run := range runs {
		if run.ToolName == "run_tests" {
			found = true
			if run.Status != "completed" {
				t.Fatalf("run_tests status = %q, want completed", run.Status)
			}
		}
	}
	if !found {
		t.Fatal("run_tests tool result was not saved")
	}
}

func TestAskUsesProvidedToolResultWithoutRerunningTool(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "Tests already passed from the provided tool result.", requests: &requests},
		Memory:  store,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			t.Fatalf("tool executor reran %q even though tool output was already provided", decision.ToolName)
			return ExecutionResult{}, nil
		},
	}

	var toolCompleted bool
	var decidedStatus string
	err = service.Ask(ctx, AskInput{
		Content: "Run tests and summarize the provided result",
		WorkspaceContext: ToolResultPrompt(ExecutionResult{
			Context:    "go test ./internal/agent\nPASS",
			Sources:    []string{"go test ./internal/agent"},
			SourceKind: "tool",
			Status:     "completed",
		}),
		SourceKind: "tool",
	}, func(event Event) error {
		if event.Type == EventToolCompleted {
			toolCompleted = true
		}
		if event.Type == EventExecutionDecided {
			decidedStatus = event.Data["status"]
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if toolCompleted {
		t.Fatal("tool completion event emitted even though the result was already provided")
	}
	if decidedStatus != ExecutionAlreadyProvided {
		t.Fatalf("execution status = %q, want %s", decidedStatus, ExecutionAlreadyProvided)
	}
	if len(requests) != 1 {
		t.Fatalf("model requests = %d, want one request using provided tool result", len(requests))
	}
	userPrompt := requests[0].Messages[len(requests[0].Messages)-1].Content
	if !strings.Contains(userPrompt, "TOOL RESULT") || !strings.Contains(userPrompt, "source_kind: tool") || !strings.Contains(userPrompt, "PASS") {
		t.Fatalf("model prompt missing provided tool result: %s", userPrompt)
	}
	runs, err := store.ListToolRuns(ctx, 20)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	var sawAlreadyProvided bool
	for _, run := range runs {
		if run.ToolName == "run_tests" {
			t.Fatalf("run_tests was logged as rerun: %+v", run)
		}
		if run.ToolName == "agent_executor" && run.Status == ExecutionAlreadyProvided {
			sawAlreadyProvided = true
		}
	}
	if !sawAlreadyProvided {
		t.Fatalf("tool runs missing already-provided execution decision: %+v", runs)
	}
}

func TestAskRunsOneModelRequestedTypedTool(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	runtime := &sequenceRuntime{
		texts: []string{
			"YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"rag_search\",\"query\":\"README Yemaka\"}\n```",
			"README says Yemaka is local-first.",
		},
		requests: &requests,
	}
	var executed bool
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: runtime,
		Memory:  store,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			executed = true
			if decision.ToolName != "rag_search" {
				t.Fatalf("ToolName = %q, want rag_search", decision.ToolName)
			}
			if len(decision.Command) < 2 || decision.Command[1] != "README Yemaka" {
				t.Fatalf("Command = %#v, want README Yemaka query", decision.Command)
			}
			return ExecutionResult{
				Context:    "RAG RESULT:\nsource: README.md\n# Yemaka\nLocal-first.",
				Sources:    []string{"README.md"},
				SourceKind: "rag",
				Status:     "completed",
			}, nil
		},
	}

	var output string
	seen := map[string]bool{}
	err = service.Ask(ctx, AskInput{
		Content: "What does the README say about Yemaka?",
	}, func(event Event) error {
		seen[event.Type] = true
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if !executed {
		t.Fatal("model-requested tool was not executed")
	}
	if len(requests) != 2 {
		t.Fatalf("model requests = %d, want initial request plus final answer", len(requests))
	}
	if strings.Contains(output, "YEMAKA_TOOL_REQUEST") {
		t.Fatalf("output leaked tool request: %q", output)
	}
	if !strings.Contains(output, "local-first") {
		t.Fatalf("output = %q, want final answer", output)
	}
	if strings.Count(output, "README says Yemaka is local-first.") != 1 {
		t.Fatalf("output = %q, want final answer emitted exactly once", output)
	}
	if !strings.Contains(requests[0].Messages[0].Content, "YEMAKA_TOOL_REQUEST") {
		t.Fatalf("initial system prompt missing typed tool instructions: %s", requests[0].Messages[0].Content)
	}
	if !strings.Contains(requests[1].Messages[len(requests[1].Messages)-1].Content, "TOOL OBSERVATION") {
		t.Fatalf("final prompt missing tool observation: %+v", requests[1].Messages)
	}
	if !strings.Contains(requests[1].Messages[len(requests[1].Messages)-1].Content, "source_kind: rag") {
		t.Fatalf("final prompt missing rag source kind: %+v", requests[1].Messages)
	}
	if !seen[EventExecutionDecided] || !seen[EventToolCompleted] {
		t.Fatalf("events = %#v, want execution and tool completion", seen)
	}
	runs, err := store.ListToolRuns(ctx, 10)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	var found bool
	for _, run := range runs {
		if run.ToolName == "rag_search" {
			found = true
			if run.Status != "completed" {
				t.Fatalf("rag_search status = %q, want completed", run.Status)
			}
		}
	}
	if !found {
		t.Fatal("read_file tool result was not saved")
	}
}

func TestAskStreamsNormalAnswerWhenModelToolLoopIsAvailable(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	runtime := &tokenRuntime{
		tokens:   []string{"Yes", ", ", "the ", "README ", "mentions local-first."},
		requests: &requests,
	}
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: runtime,
		Memory:  store,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			t.Fatalf("unexpected tool execution: %s", decision.ToolName)
			return ExecutionResult{}, nil
		},
	}

	var tokens []string
	err = service.Ask(ctx, AskInput{
		Content: "What does the README say about Yemaka?",
	}, func(event Event) error {
		if event.Type == EventModelToken {
			tokens = append(tokens, event.Token)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("model requests = %d, want one normal answer request", len(requests))
	}
	if !strings.Contains(requests[0].Messages[0].Content, "YEMAKA_TOOL_REQUEST") {
		t.Fatalf("initial system prompt missing typed tool instructions: %s", requests[0].Messages[0].Content)
	}
	if len(tokens) != len(runtime.tokens) {
		t.Fatalf("streamed token count = %d, want %d (%#v)", len(tokens), len(runtime.tokens), tokens)
	}
	output := strings.Join(tokens, "")
	if output != "Yes, the README mentions local-first." {
		t.Fatalf("output = %q, want normal streamed answer", output)
	}
}

func TestModelToolInstructionsExposeInternetOnlyWhenEnabled(t *testing.T) {
	plan := Plan{RiskLevel: RiskLow, MaxSteps: 3}
	withoutInternet := ModelToolInstructionsWithOptions(plan, ModelToolOptions{})
	if strings.Contains(withoutInternet, "internet_fetch") {
		t.Fatalf("instructions exposed internet tool while disabled: %s", withoutInternet)
	}
	for _, want := range []string{"file_stat", "file_tree", "git_status", "heartbeat_status"} {
		if !strings.Contains(withoutInternet, want) {
			t.Fatalf("instructions missing %q: %s", want, withoutInternet)
		}
	}
	withInternet := ModelToolInstructionsWithOptions(plan, ModelToolOptions{InternetTools: true})
	if !strings.Contains(withInternet, "internet_fetch") || strings.Contains(withInternet, "internet_search,") || !strings.Contains(withInternet, "allowed_domains") {
		t.Fatalf("instructions should expose fetch/head without search when no provider is configured: %s", withInternet)
	}
	withSearch := ModelToolInstructionsWithOptions(plan, ModelToolOptions{InternetTools: true, InternetSearch: true})
	if !strings.Contains(withSearch, "internet_fetch") || !strings.Contains(withSearch, "internet_search") || !strings.Contains(withSearch, "allowed_domains") {
		t.Fatalf("instructions missing internet search guidance: %s", withSearch)
	}
}

func TestDecisionFromModelToolRequestAllowsReadOnlyPostRCTools(t *testing.T) {
	plan := Plan{RiskLevel: RiskLow, MaxSteps: 3}
	cases := []struct {
		request ModelToolRequest
		want    []string
	}{
		{request: ModelToolRequest{ToolName: "file_stat", Path: "README.md"}, want: []string{"file_stat", "README.md"}},
		{request: ModelToolRequest{ToolName: "file_tree", Path: "internal/agent"}, want: []string{"file_tree", "internal/agent"}},
		{request: ModelToolRequest{ToolName: "git_status"}, want: []string{"git", "status", "--short"}},
		{request: ModelToolRequest{ToolName: "heartbeat_status"}, want: []string{"heartbeat_status"}},
	}
	for _, tc := range cases {
		decision := DecisionFromModelToolRequestWithOptions(plan, tc.request, "", ModelToolOptions{})
		if decision.Status != ExecutionReady || decision.ToolName != tc.request.ToolName {
			t.Fatalf("decision for %s = %+v, want ready", tc.request.ToolName, decision)
		}
		if strings.Join(decision.Command, " ") != strings.Join(tc.want, " ") {
			t.Fatalf("Command for %s = %#v, want %#v", tc.request.ToolName, decision.Command, tc.want)
		}
	}
}

func TestDecisionFromModelToolRequestRequiresInternetOption(t *testing.T) {
	plan := Plan{RiskLevel: RiskLow, MaxSteps: 3}
	request := ModelToolRequest{
		ToolName:       "internet_fetch",
		URL:            "https://example.com",
		AllowedDomains: []string{"example.com"},
	}
	blocked := DecisionFromModelToolRequestWithOptions(plan, request, "", ModelToolOptions{})
	if blocked.Status != ExecutionBlocked {
		t.Fatalf("disabled internet decision status = %q, want blocked", blocked.Status)
	}
	ready := DecisionFromModelToolRequestWithOptions(plan, request, "", ModelToolOptions{InternetTools: true})
	if ready.Status != ExecutionReady || ready.ToolName != "internet_fetch" {
		t.Fatalf("enabled internet decision = %+v, want ready internet_fetch", ready)
	}
	if len(ready.Command) != 3 || ready.Command[1] != "https://example.com" || ready.Command[2] != "example.com" {
		t.Fatalf("Command = %#v, want url and allowlisted domain", ready.Command)
	}

	blockedSearch := DecisionFromModelToolRequestWithOptions(plan, ModelToolRequest{ToolName: "internet_search", Query: "local agents"}, "", ModelToolOptions{InternetTools: true})
	if blockedSearch.Status != ExecutionBlocked || blockedSearch.ToolName != "internet_search" {
		t.Fatalf("internet_search without provider = %+v, want blocked", blockedSearch)
	}
	search := DecisionFromModelToolRequestWithOptions(plan, ModelToolRequest{ToolName: "internet_search", Query: "local agents"}, "", ModelToolOptions{InternetTools: true, InternetSearch: true})
	if search.Status != ExecutionReady || search.ToolName != "internet_search" || search.RiskLevel != RiskMedium {
		t.Fatalf("internet_search decision = %+v, want ready medium-risk search", search)
	}
}

func TestDecisionFromModelToolRequestNormalizesBareInternetDomain(t *testing.T) {
	plan := Plan{RiskLevel: RiskLow, MaxSteps: 3}
	request := ModelToolRequest{
		ToolName: "internet_fetch",
		Query:    "almoayed.com",
	}
	decision := DecisionFromModelToolRequestWithOptions(plan, request, "", ModelToolOptions{InternetTools: true})
	if decision.Status != ExecutionReady {
		t.Fatalf("Status = %q, want ready: %+v", decision.Status, decision)
	}
	if len(decision.Command) != 3 || decision.Command[1] != "https://almoayed.com" || decision.Command[2] != "almoayed.com" {
		t.Fatalf("Command = %#v, want normalized url and inferred allowlist", decision.Command)
	}
}

func TestAskRunsInternetToolBeforeModelWhenProfileEnabled(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var executed ExecutionDecision
	service := &Service{
		Router:        models.NewRouter(cfg),
		Runtime:       recordingRuntime{text: "almoayed.com is reachable.", requests: &requests},
		Memory:        store,
		InternetTools: true,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			executed = decision
			return ExecutionResult{
				Context:    "INTERNET RESULT\nurl: https://almoayed.com\nstatus: 200\ntext:\nPublic website content.",
				Sources:    []string{"https://almoayed.com"},
				SourceKind: "internet",
				Status:     "completed",
			}, nil
		},
	}

	err = service.Ask(ctx, AskInput{Content: "Check the web for almoayed.com"}, func(event Event) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if executed.ToolName != "internet_fetch" {
		t.Fatalf("executed tool = %q, want internet_fetch", executed.ToolName)
	}
	if len(executed.Command) != 3 || executed.Command[1] != "https://almoayed.com" || executed.Command[2] != "almoayed.com" {
		t.Fatalf("executed command = %#v, want normalized internet fetch", executed.Command)
	}
	if len(requests) != 1 {
		t.Fatalf("model requests = %d, want 1 after pre-model tool execution", len(requests))
	}
	if !strings.Contains(requests[0].Messages[1].Content, "INTERNET RESULT") {
		t.Fatalf("model prompt missing internet tool result: %s", requests[0].Messages[1].Content)
	}
	if !strings.Contains(requests[0].Messages[1].Content, "source_kind: internet") {
		t.Fatalf("model prompt missing internet source kind: %s", requests[0].Messages[1].Content)
	}
}

func TestChatRunsInternetToolBeforeModelWhenProfileEnabled(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var executed ExecutionDecision
	service := &Service{
		Router:        models.NewRouter(cfg),
		Runtime:       recordingRuntime{text: "almoayed.com returned public site content.", requests: &requests},
		Memory:        store,
		InternetTools: true,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			executed = decision
			return ExecutionResult{
				Context:    "INTERNET RESULT\nurl: https://almoayed.com\nstatus: 200\ntext:\nPublic website content.",
				Sources:    []string{"https://almoayed.com"},
				SourceKind: "internet",
				Status:     "completed",
			}, nil
		},
	}

	err = service.Chat(ctx, ChatInput{Content: "Check the web for almoayed.com"}, func(event Event) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if executed.ToolName != "internet_fetch" {
		t.Fatalf("executed tool = %q, want internet_fetch", executed.ToolName)
	}
	if len(requests) != 1 {
		t.Fatalf("model requests = %d, want one final model call after tool result", len(requests))
	}
	userPrompt := requests[0].Messages[len(requests[0].Messages)-1].Content
	if !strings.Contains(userPrompt, "INTERNET RESULT") || !strings.Contains(userPrompt, "source_kind: internet") {
		t.Fatalf("chat prompt missing internet result/source kind: %s", userPrompt)
	}
}

func TestAskInternetDisabledReportsPolicyInsteadOfRunningTool(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var toolRan bool
	service := &Service{
		Router:        models.NewRouter(cfg),
		Runtime:       recordingRuntime{text: "should not be called", requests: &requests},
		Memory:        store,
		InternetTools: false,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			toolRan = true
			return ExecutionResult{}, nil
		},
	}

	var output string
	err = service.Ask(ctx, AskInput{Content: "Check the web for almoayed.com"}, func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if toolRan {
		t.Fatal("internet tool ran while InternetTools was disabled")
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0 when disabled policy response is deterministic", len(requests))
	}
	if !strings.Contains(output, "internet access is disabled") {
		t.Fatalf("output = %q, want disabled internet policy message", output)
	}
	if strings.Contains(output, "Tool execution failed") {
		t.Fatalf("output = %q, should not be a tool failure", output)
	}
}

func TestAskInternetSearchWithoutProviderReturnsFriendlySetupMessage(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	cfg.Internet.Enabled = true
	cfg.Internet.DefaultMode = "profile_enabled"
	cfg.Internet.Search.Enabled = false
	cfg.Internet.Search.Provider = "none"
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var toolRan bool
	var gapEvents int
	service := &Service{
		Router:         models.NewRouter(cfg),
		Runtime:        recordingRuntime{text: "should not be called", requests: &requests},
		Memory:         store,
		InternetTools:  true,
		InternetSearch: false,
		CapabilityGap:  NewCapabilityGapRouter(cfg, nil),
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			toolRan = true
			return ExecutionResult{}, nil
		},
	}

	var output string
	err = service.Ask(ctx, AskInput{Content: "What is SearXNG? You can use internet for this task"}, func(event Event) error {
		switch event.Type {
		case EventModelToken:
			output += event.Token
		case EventCapabilityGap:
			gapEvents++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if toolRan {
		t.Fatal("internet_search ran even though no search provider is configured")
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0 for deterministic setup message", len(requests))
	}
	if gapEvents != 0 {
		t.Fatalf("capability gap events = %d, want 0 for internet provider setup", gapEvents)
	}
	if !strings.Contains(output, "web search still needs a ready search provider") || !strings.Contains(output, "Choose a search provider") {
		t.Fatalf("output = %q, want friendly search provider setup message", output)
	}
	if strings.Contains(output, "Tool execution failed") {
		t.Fatalf("output = %q, should not be a raw tool failure", output)
	}
	if strings.Contains(output, "Missing capability") || strings.Contains(output, "Open in Extensions") {
		t.Fatalf("output = %q, should not present search provider setup as an extension capability", output)
	}
}

func TestAskInternetSearchProviderUnreachableReturnsFriendlyHealthMessage(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	cfg.Internet.Enabled = true
	cfg.Internet.DefaultMode = "profile_enabled"
	cfg.Internet.Search.Enabled = true
	cfg.Internet.Search.Provider = "searxng"
	cfg.Internet.Search.Endpoint = "https://priv.au/"
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var gapEvents int
	var toolRan bool
	service := &Service{
		Router:         models.NewRouter(cfg),
		Runtime:        recordingRuntime{text: "model should not be called"},
		Memory:         store,
		InternetTools:  true,
		InternetSearch: true,
		CapabilityGap:  NewCapabilityGapRouter(cfg, nil),
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			toolRan = true
			return ExecutionResult{}, fmt.Errorf(`could not reach SearXNG search provider at https://priv.au/; verify SearXNG is running and the endpoint is reachable: Get "https://priv.au/": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`)
		},
	}

	var output string
	err = service.Ask(ctx, AskInput{Content: "Search the web for SearXNG and explain what it is"}, func(event Event) error {
		switch event.Type {
		case EventModelToken:
			output += event.Token
		case EventCapabilityGap:
			gapEvents++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if !toolRan {
		t.Fatal("internet_search did not run")
	}
	if gapEvents != 0 {
		t.Fatalf("capability gap events = %d, want 0 for provider health failure", gapEvents)
	}
	if !strings.Contains(output, "could not reach it from this session") || !strings.Contains(output, "Check the SearXNG endpoint") {
		t.Fatalf("output = %q, want friendly SearXNG health message", output)
	}
	if strings.Contains(output, "Missing capability") || strings.Contains(output, "Open in Extensions") {
		t.Fatalf("output = %q, should not present provider health as an extension capability", output)
	}
}

func TestAskToolFailureUsesHelpfulChatResponse(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "model should not be called"},
		Memory:  store,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			return ExecutionResult{}, fmt.Errorf("command blocked: rm is blocked by policy")
		},
	}

	var output string
	err = service.Ask(ctx, AskInput{Content: "Run tests"}, func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if !strings.Contains(output, "I could not use `run_tests`") || !strings.Contains(output, "safe command policy") {
		t.Fatalf("output = %q, want helpful tool failure response", output)
	}
	if strings.Contains(output, "Tool execution failed") || strings.Contains(output, "command blocked:") {
		t.Fatalf("output = %q, should not leak raw tool failure", output)
	}
}

func TestAskRunsBoundedModelRequestedToolLoop(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	runtime := &sequenceRuntime{
		texts: []string{
			"YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"read_file\",\"path\":\"README.md\"}\n```",
			"YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"search_files\",\"query\":\"agent\"}\n```",
			"README.md and search results both mention the local agent.",
		},
		requests: &requests,
	}
	var tools []string
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: runtime,
		Memory:  store,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			tools = append(tools, decision.ToolName)
			return ExecutionResult{
				Context:    decision.ToolName + " observation for README.md",
				Sources:    []string{"README.md"},
				SourceKind: "tool",
				Status:     "completed",
			}, nil
		},
	}

	var output string
	err = service.Ask(ctx, AskInput{Content: "What local files mention the agent?"}, func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if strings.Contains(output, "YEMAKA_TOOL_REQUEST") {
		t.Fatalf("output leaked tool marker: %q", output)
	}
	if len(tools) != 2 || tools[0] != "read_file" || tools[1] != "search_files" {
		t.Fatalf("tools = %#v, want read_file then search_files", tools)
	}
	if len(requests) != 3 {
		t.Fatalf("model requests = %d, want two tool requests plus final", len(requests))
	}
	runs, err := store.ListToolRuns(ctx, 20)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	var readLogged, searchLogged bool
	for _, run := range runs {
		if run.ToolName == "read_file" {
			readLogged = true
		}
		if run.ToolName == "search_files" {
			searchLogged = true
		}
	}
	if !readLogged || !searchLogged {
		t.Fatalf("tool logs missing read/search: read=%t search=%t", readLogged, searchLogged)
	}
}

func TestAskRunsNativeToolCallThroughGuardedToolLoop(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	runtime := &nativeToolLoopRuntime{requests: &requests}
	var tools []string
	var toolCallEvents int
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: runtime,
		Memory:  store,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			tools = append(tools, decision.ToolName)
			return ExecutionResult{
				Context:    "README.md says Yemaka is local-first.",
				Sources:    []string{"README.md"},
				SourceKind: "tool",
				Status:     "completed",
			}, nil
		},
	}

	var output string
	err = service.Ask(ctx, AskInput{Content: "What local files mention the agent?"}, func(event Event) error {
		switch event.Type {
		case EventModelToken:
			output += event.Token
		case EventModelToolCall:
			toolCallEvents++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if strings.Contains(output, "YEMAKA_TOOL_REQUEST") || strings.Contains(output, "<think>") {
		t.Fatalf("output leaked hidden control text: %q", output)
	}
	if output != "README.md says Yemaka is local-first." {
		t.Fatalf("output = %q, want final native-tool answer", output)
	}
	if len(tools) != 1 || tools[0] != "read_file" {
		t.Fatalf("tools = %#v, want native read_file through executor", tools)
	}
	if len(requests) < 2 || len(requests[0].Tools) == 0 {
		t.Fatalf("model requests = %d, first tools = %#v; want native tool definitions on first request", len(requests), requests[0].Tools)
	}
	if toolCallEvents != 1 {
		t.Fatalf("toolCallEvents = %d, want 1", toolCallEvents)
	}
}

func TestAskRunsRealWorldPlanActVerifyToolLoop(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	runtime := &sequenceRuntime{
		texts: []string{
			"YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"search_files\",\"query\":\"route request\"}\n```",
			"YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"read_file\",\"path\":\"internal/agent/loop.go\"}\n```",
			"Routing starts with deterministic classification, then uses a bounded plan, typed tools, observations, and verification.",
		},
		requests: &requests,
	}
	var tools []string
	var executionEvents int
	var toolEvents int
	seen := map[string]bool{}
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: runtime,
		Memory:  store,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			tools = append(tools, decision.ToolName)
			switch decision.ToolName {
			case "search_files":
				return ExecutionResult{
					Context:    "internal/agent/loop.go: RouteRequest and BuildPlan shape the request before tools run.",
					Sources:    []string{"internal/agent/loop.go"},
					SourceKind: "tool",
					Status:     "completed",
				}, nil
			case "read_file":
				return ExecutionResult{
					Context:    "FILE: internal/agent/loop.go\nBuildPlan classifies, creates steps, and VerifyResponse checks the final answer.",
					Sources:    []string{"internal/agent/loop.go"},
					SourceKind: "tool",
					Status:     "completed",
				}, nil
			default:
				t.Fatalf("unexpected tool %q", decision.ToolName)
				return ExecutionResult{}, nil
			}
		},
	}

	var output string
	err = service.Ask(ctx, AskInput{Content: "Explain how this project routes requests through planning, tools, and verification."}, func(event Event) error {
		seen[event.Type] = true
		switch event.Type {
		case EventModelToken:
			output += event.Token
		case EventExecutionDecided:
			executionEvents++
		case EventToolCompleted:
			toolEvents++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if strings.Contains(output, "YEMAKA_TOOL_REQUEST") {
		t.Fatalf("output leaked tool marker: %q", output)
	}
	if !strings.Contains(output, "bounded plan") || !strings.Contains(output, "verification") {
		t.Fatalf("output = %q, want final answer grounded in tool observations", output)
	}
	if len(tools) != 2 || tools[0] != "search_files" || tools[1] != "read_file" {
		t.Fatalf("tools = %#v, want search_files then read_file", tools)
	}
	if len(requests) != 3 {
		t.Fatalf("model requests = %d, want initial plus two observation turns", len(requests))
	}
	if !strings.Contains(requests[1].Messages[len(requests[1].Messages)-1].Content, "TOOL OBSERVATION") ||
		!strings.Contains(requests[2].Messages[len(requests[2].Messages)-1].Content, "TOOL OBSERVATION") {
		t.Fatalf("model requests missing tool observations: %+v", requests)
	}
	for _, eventType := range []string{EventTaskClassified, EventPlanCreated, EventModelSelected, EventExecutionDecided, EventToolCompleted, EventVerificationCompleted, EventMessageSaved, EventAgentCompleted} {
		if !seen[eventType] {
			t.Fatalf("event %s was not emitted; seen=%#v", eventType, seen)
		}
	}
	if executionEvents != 3 || toolEvents != 2 {
		t.Fatalf("execution events=%d tool events=%d, want initial decision plus 2 tool decisions and 2 tool completions", executionEvents, toolEvents)
	}
	runs, err := store.ListToolRuns(ctx, 20)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	if len(runs) < 2 {
		t.Fatalf("tool runs = %d, want at least 2", len(runs))
	}
}

func TestAskModelRequestedToolLoopStopsAtStepLimit(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	prompt := "What local files mention the agent?"
	limit := modelToolStepLimit(BuildPlan(PlanInput{Content: prompt}))
	toolRequest := "YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"search_files\",\"query\":\"agent\"}\n```"
	texts := make([]string, 0, limit+2)
	for i := 0; i < limit+2; i++ {
		texts = append(texts, toolRequest)
	}
	var requests []models.ChatRequest
	runtime := &sequenceRuntime{texts: texts, requests: &requests}
	var tools []string
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: runtime,
		Memory:  store,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			tools = append(tools, decision.ToolName)
			return ExecutionResult{
				Context:    fmt.Sprintf("observation %d: agent appears in README.md", len(tools)),
				Sources:    []string{"README.md"},
				SourceKind: "tool",
				Status:     "completed",
			}, nil
		},
	}

	var output string
	err = service.Ask(ctx, AskInput{Content: prompt}, func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(tools) != limit {
		t.Fatalf("tools run = %d, want step limit %d", len(tools), limit)
	}
	if len(requests) != limit+2 {
		t.Fatalf("model requests = %d, want %d", len(requests), limit+2)
	}
	if !strings.Contains(output, fmt.Sprintf("configured limit of %d", limit)) {
		t.Fatalf("output = %q, want step-limit response", output)
	}
	runs, err := store.ListToolRuns(ctx, 20)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	var searchRuns int
	for _, run := range runs {
		if run.ToolName == "search_files" {
			searchRuns++
		}
	}
	if searchRuns != limit {
		t.Fatalf("search_files tool runs = %d, want %d; all runs=%+v", searchRuns, limit, runs)
	}
}

func TestResolveModelToolLoopPersistsPermissionBeforeEmit(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "model permission")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}

	service := &Service{
		Router: models.NewRouter(cfg),
		Memory: store,
	}
	plan := BuildPlan(PlanInput{Content: "Delete build output with rm -rf build"})
	toolRequest := "YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"read_file\",\"path\":\"README.md\"}\n```"
	var emitted PermissionRequest
	var persistedBeforeEmit bool
	behavior := responseBehaviorFromConfig(cfg, plan, cfg.Models["default"])
	trace := behavior.ModelResponseTrace
	content, _, decision, _, _, err := service.resolveModelToolLoop(ctx, conversation.ID, toolRunMetadata{SessionID: "sess_test"}, cfg.Models["default"], plan, PlanInput{Content: "Delete build output with rm -rf build"}, nil, toolRequest, "", func(event Event) error {
		if event.Type != EventPermissionRequested {
			return nil
		}
		var command []string
		if raw := strings.TrimSpace(event.Data["command_json"]); raw != "" {
			if err := json.Unmarshal([]byte(raw), &command); err != nil {
				return err
			}
		}
		emitted = PermissionRequest{
			RequestID:            event.Data["request_id"],
			ToolName:             event.Data["tool_name"],
			Command:              command,
			RiskLevel:            event.Data["risk_level"],
			Reason:               event.Data["reason"],
			RequiresConfirmation: event.Data["requires_confirmation"] == "true",
		}
		runs, err := store.ListToolRuns(ctx, 10)
		if err != nil {
			return err
		}
		for _, run := range runs {
			if run.ToolName != "permission_request" {
				continue
			}
			stored, ok := permissionRequestFromStoredValue(run.Output)
			if ok && stored.RequestID == emitted.RequestID {
				persistedBeforeEmit = true
				break
			}
		}
		return nil
	}, false, &trace, behavior)
	if err != nil {
		t.Fatalf("resolveModelToolLoop() error = %v", err)
	}
	if decision.Status != ExecutionNeedsConfirmation {
		t.Fatalf("decision status = %q, want %q", decision.Status, ExecutionNeedsConfirmation)
	}
	if emitted.RequestID == "" {
		t.Fatal("permission request event was not emitted")
	}
	if decision.RequestID != emitted.RequestID {
		t.Fatalf("decision request id = %q, want emitted %q", decision.RequestID, emitted.RequestID)
	}
	if !persistedBeforeEmit {
		t.Fatalf("permission request %q was not persisted before emit", emitted.RequestID)
	}
	if err := ValidateStoredPermissionApproval(ctx, store, emitted); err != nil {
		t.Fatalf("ValidateStoredPermissionApproval() error = %v", err)
	}
	if !strings.Contains(content, "needs your approval") {
		t.Fatalf("content = %q, want approval response", content)
	}
}

func TestAskModelRequestedToolFailureUsesHelpfulChatResponse(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	toolRequest := "YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"read_file\",\"path\":\"../secret.txt\"}\n```"
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "model should not be called"},
		Memory:  store,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			return ExecutionResult{}, fmt.Errorf("path is outside workspace: ../secret.txt")
		},
	}

	var output string
	plan := BuildPlan(PlanInput{Content: "hello"})
	behavior := responseBehaviorFromConfig(cfg, plan, cfg.Models["default"])
	trace := behavior.ModelResponseTrace
	content, _, _, _, _, err := service.resolveModelToolLoop(ctx, "", toolRunMetadata{SessionID: "sess_test"}, cfg.Models["default"], plan, PlanInput{Content: "hello"}, nil, toolRequest, "", func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	}, false, &trace, behavior)
	if err != nil {
		t.Fatalf("resolveModelToolLoop() error = %v", err)
	}
	if output == "" {
		output = content
	}
	if !strings.Contains(output, "I could not use `read_file`") || !strings.Contains(output, "outside the current workspace boundary") {
		t.Fatalf("output = %q, want helpful read_file failure", output)
	}
	if strings.Contains(output, "Tool execution failed") || strings.Contains(output, "path is outside workspace:") {
		t.Fatalf("output = %q, should not leak raw path error", output)
	}
}

func TestAskUsesCoreRetrieverWhenContextMissing(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "README.md says retrieval is core-owned.", requests: &requests},
		Memory:  store,
		Retriever: RetrieverFunc(func(ctx context.Context, content string) (RetrievalResult, error) {
			return RetrievalResult{
				Context:    "README.md: retrieval is core-owned",
				Sources:    []string{"README.md"},
				SourceKind: "workspace",
			}, nil
		}),
	}

	var sawWorkspace bool
	err = service.Ask(ctx, AskInput{Content: "Explain retrieval in this project"}, func(event Event) error {
		if event.Type == EventWorkspaceUsed {
			sawWorkspace = event.Data["source_kind"] == "workspace" && event.Data["sources"] == "README.md"
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if !sawWorkspace {
		t.Fatal("workspace used event did not reflect core retriever")
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
	if !strings.Contains(requests[0].Messages[1].Content, "README.md: retrieval is core-owned") {
		t.Fatalf("prompt missing retrieved context: %s", requests[0].Messages[1].Content)
	}
}

func TestAskSkipsRetrieverForSmallTalk(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var retrieved bool
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "Hello.", requests: &requests},
		Memory:  store,
		Retriever: RetrieverFunc(func(ctx context.Context, content string) (RetrievalResult, error) {
			retrieved = true
			return RetrievalResult{
				Context:    "roadmap.md: unrelated implementation context",
				Sources:    []string{"roadmap.md"},
				SourceKind: "rag",
			}, nil
		}),
	}

	err = service.Ask(ctx, AskInput{Content: "hello"}, func(event Event) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if retrieved {
		t.Fatal("retriever ran for small talk")
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
	if strings.Contains(requests[0].Messages[1].Content, "RETRIEVED CONTEXT") {
		t.Fatalf("small-talk prompt unexpectedly included retrieved context: %s", requests[0].Messages[1].Content)
	}
}

func TestAskGenericExplanationAllowsGeneralKnowledgeWithoutWorkspaceEvent(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var retrieved bool
	var sawWorkspace bool
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "Cogging torque is an uneven torque ripple in some electric motors.", requests: &requests},
		Memory:  store,
		Retriever: RetrieverFunc(func(ctx context.Context, content string) (RetrievalResult, error) {
			retrieved = true
			return RetrievalResult{
				Context:    "README.md: unrelated workspace context",
				Sources:    []string{"README.md"},
				SourceKind: "workspace",
			}, nil
		}),
	}

	err = service.Ask(ctx, AskInput{Content: "Explain the concept of cogging torque."}, func(event Event) error {
		if event.Type == EventWorkspaceUsed {
			sawWorkspace = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if retrieved {
		t.Fatal("retriever ran for generic explanation")
	}
	if sawWorkspace {
		t.Fatal("workspace.used event emitted without retrieved sources")
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
	userPrompt := requests[0].Messages[1].Content
	if !strings.Contains(userPrompt, "general model knowledge is allowed") {
		t.Fatalf("prompt missing general-knowledge evidence contract:\n%s", userPrompt)
	}
	if strings.Contains(userPrompt, "README.md") || strings.Contains(userPrompt, "RETRIEVED CONTEXT") {
		t.Fatalf("generic explanation prompt unexpectedly included workspace context:\n%s", userPrompt)
	}
}

func TestAskSkipsRetrieverForGenericCodingKnowledge(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var retrieved bool
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "Here is a small example.", requests: &requests},
		Memory:  store,
		Retriever: RetrieverFunc(func(ctx context.Context, content string) (RetrievalResult, error) {
			retrieved = true
			return RetrievalResult{
				Context:    "README.md: unrelated Yemaka project context",
				Sources:    []string{"README.md"},
				SourceKind: "workspace",
			}, nil
		}),
	}

	err = service.Ask(ctx, AskInput{Content: "Write Python code for Fibonacci"}, func(event Event) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if retrieved {
		t.Fatal("retriever ran for generic coding knowledge")
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
	if strings.Contains(requests[0].Messages[1].Content, "README.md") {
		t.Fatalf("generic coding prompt unexpectedly included workspace context: %s", requests[0].Messages[1].Content)
	}
}

func TestAskPersistsAcceptedUserMessageBeforeModelGeneration(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	const prompt = "Explain the current project status."
	var inspected bool
	service := &Service{
		Router: models.NewRouter(cfg),
		Runtime: inspectingRuntime{
			text: "Project status explained.",
			inspect: func(ctx context.Context) error {
				inspected = true
				conversations, err := store.ListConversations(ctx, 10)
				if err != nil {
					return err
				}
				if len(conversations) != 1 {
					return fmt.Errorf("conversations = %d, want 1 accepted conversation before model generation", len(conversations))
				}
				messages, err := store.ListConversationMessages(ctx, conversations[0].ID, 10)
				if err != nil {
					return err
				}
				if len(messages) != 1 {
					return fmt.Errorf("messages = %d, want only the accepted user message before model generation", len(messages))
				}
				if messages[0].Role != "user" || messages[0].Content != prompt {
					return fmt.Errorf("message before model = %+v, want accepted user prompt", messages[0])
				}
				return nil
			},
		},
		Memory: store,
	}

	var events []Event
	err = service.Ask(ctx, AskInput{Content: prompt}, func(event Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if !inspected {
		t.Fatal("runtime did not inspect persisted state")
	}
	userSaveIndex := -1
	tokenIndex := -1
	for index, event := range events {
		if event.Type == EventMessageSaved && event.Data["role"] == "user" {
			userSaveIndex = index
			if event.Data["conversation_id"] == "" {
				t.Fatalf("user message.saved missing conversation_id: %+v", event.Data)
			}
		}
		if event.Type == EventModelToken && tokenIndex == -1 {
			tokenIndex = index
		}
	}
	if userSaveIndex < 0 {
		t.Fatalf("events missing early user message.saved: %+v", events)
	}
	if tokenIndex < 0 {
		t.Fatalf("events missing model token: %+v", events)
	}
	if userSaveIndex > tokenIndex {
		t.Fatalf("user message saved after model token; userSaveIndex=%d tokenIndex=%d", userSaveIndex, tokenIndex)
	}
}

func TestAskPersistsRunningAgentTurnBeforeModelGeneration(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	const prompt = "Explain the local release status."
	var inspected bool
	service := &Service{
		Router: models.NewRouter(cfg),
		Runtime: inspectingRuntime{
			text: "Release status explained.",
			inspect: func(ctx context.Context) error {
				inspected = true
				conversations, err := store.ListConversations(ctx, 10)
				if err != nil {
					return err
				}
				if len(conversations) != 1 {
					return fmt.Errorf("conversations = %d, want 1 accepted conversation before model generation", len(conversations))
				}
				turns, err := store.ListAgentTurnsForConversation(ctx, conversations[0].ID, 10)
				if err != nil {
					return err
				}
				if len(turns) != 1 {
					return fmt.Errorf("agent turns = %d, want one running turn before model generation", len(turns))
				}
				turn := turns[0]
				if turn.Status != AgentTurnRunning || turn.UserMessageID == "" || turn.AssistantMessageID != "" {
					return fmt.Errorf("running turn before model = %+v", turn)
				}
				trace := strings.Join(turn.Trace, "\n")
				for _, want := range []string{"accepted task", "task:", "plan:", "model:"} {
					if !strings.Contains(trace, want) {
						return fmt.Errorf("running turn trace missing %q: %s", want, trace)
					}
				}
				return nil
			},
		},
		Memory: store,
	}

	if err := service.Ask(ctx, AskInput{Content: prompt}, func(Event) error { return nil }); err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if !inspected {
		t.Fatal("runtime did not inspect persisted agent turn")
	}
	conversations, err := store.ListConversations(ctx, 10)
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}
	turns, err := store.ListAgentTurnsForConversation(ctx, conversations[0].ID, 10)
	if err != nil {
		t.Fatalf("ListAgentTurnsForConversation() error = %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("agent turns = %d, want one completed turn", len(turns))
	}
	if turns[0].Status != AgentTurnCompleted || turns[0].AssistantMessageID == "" {
		t.Fatalf("completed agent turn = %+v", turns[0])
	}
}

func TestAskDoesNotTreatEarlySavedPromptAsPriorConversationContext(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	conversation, err := store.CreateConversation(ctx, "Existing project chat")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	previous, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Earlier request about release packaging.",
	})
	if err != nil {
		t.Fatalf("SaveMessage(previous user) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "Earlier answer about release packaging.",
		ParentID:       previous.ID,
	}); err != nil {
		t.Fatalf("SaveMessage(previous assistant) error = %v", err)
	}

	const prompt = "Summarize the active packaging blocker."
	var requests []models.ChatRequest
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: inspectingRuntime{text: "Packaging blocker summarized.", requests: &requests},
		Memory:  store,
	}

	err = service.Ask(ctx, AskInput{ConversationID: conversation.ID, Content: prompt}, func(event Event) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
	if len(requests[0].Messages) < 2 {
		t.Fatalf("model request messages = %+v, want user prompt", requests[0].Messages)
	}
	compiledPrompt := requests[0].Messages[1].Content
	if !strings.Contains(compiledPrompt, "Earlier request about release packaging.") {
		t.Fatalf("compiled prompt missing prior conversation context: %s", compiledPrompt)
	}
	if strings.Contains(compiledPrompt, "RECENT SESSION MESSAGES") && strings.Contains(compiledPrompt, "user: "+prompt) {
		t.Fatalf("compiled prompt treated current prompt as prior context: %s", compiledPrompt)
	}
}

func TestAskGroundsPythonFileInspectionBeforeModelAfterTestFailure(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	root := t.TempDir()
	writeRetrieverFile(t, filepath.Join(root, "README.md"), "# Mixed repo\nThis is not the Python bug.")
	writeRetrieverFile(t, filepath.Join(root, "test-files", "calculator.py"), "def add(a, b):\n    return a - b\n")
	writeRetrieverFile(t, filepath.Join(root, "test-files", "test_calculator.py"), "from calculator import add\n\ndef test_add():\n    assert add(2, 3) == 5\n")
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "calculator.py has an add bug; propose changing `return a - b` to `return a + b` and wait for approval.", requests: &requests},
		Memory:  store,
		Retriever: NewLocalRetriever(LocalRetrieverConfig{
			Config:        cfg,
			WorkspaceRoot: root,
		}),
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			if decision.ToolName != "run_tests" {
				t.Fatalf("ToolName = %q, want run_tests", decision.ToolName)
			}
			return ExecutionResult{
				Context:    "/usr/bin/python3: No module named pytest",
				Sources:    []string{"python3 -m pytest"},
				SourceKind: "tool",
				Status:     "failed",
			}, nil
		},
	}

	var sawWorkspace bool
	err = service.Ask(ctx, AskInput{Content: "Inspect the Python files in test-files, run the tests if safe, find the bug, propose a fix, and only apply it after I approve."}, func(event Event) error {
		if event.Type == EventWorkspaceUsed {
			sawWorkspace = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if !sawWorkspace {
		t.Fatal("expected workspace retrieval before model response")
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
	prompt := requests[0].Messages[1].Content
	for _, want := range []string{
		"test-files/calculator.py",
		"return a - b",
		"test-files/test_calculator.py",
		"No module named pytest",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("model prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestAskSkipsRetrieverForPureInternetTask(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var retrieved bool
	service := &Service{
		Router:        models.NewRouter(cfg),
		Runtime:       recordingRuntime{text: "should not be called"},
		Memory:        store,
		InternetTools: false,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			return ExecutionResult{}, nil
		},
		Retriever: RetrieverFunc(func(ctx context.Context, content string) (RetrievalResult, error) {
			retrieved = true
			return RetrievalResult{Context: "roadmap.md: unrelated local context", SourceKind: "rag"}, nil
		}),
	}

	err = service.Ask(ctx, AskInput{Content: "Check the web for almoayed.com"}, func(event Event) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if retrieved {
		t.Fatal("retriever ran for pure internet task")
	}
}

func TestFileHintsIgnoreInternetTargets(t *testing.T) {
	hints := fileHints("Check the web for https://example.com and almoayed.com, then read README.md")
	if len(hints) != 1 || hints[0] != "README.md" {
		t.Fatalf("fileHints = %#v, want only README.md", hints)
	}
}

func TestVerifyResponseFlagsLeakedToolMarkerAndWeakTests(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Run tests and explain failures"})
	result := VerifyResponse(plan, "YEMAKA_TOOL_REQUEST\n```json\n{}")
	if result.Status != "needs_follow_up" {
		t.Fatalf("Status = %q, want needs_follow_up", result.Status)
	}
	if !testContainsString(result.Reasons, "model tool request marker leaked into the final answer") {
		t.Fatalf("Reasons = %v, want leaked marker reason", result.Reasons)
	}
}

func testContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestAskRoutesCodingTaskAndEmitsLoopEvents(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "coded answer", requests: &requests},
		Memory:  store,
	}

	seen := map[string]bool{}
	err = service.Ask(ctx, AskInput{
		Content:          "Fix the bug in main.go",
		WorkspaceContext: "main.go: package main",
		Sources:          []string{"main.go"},
		SourceKind:       "workspace",
	}, func(event Event) error {
		seen[event.Type] = true
		if event.Type == EventTaskClassified && event.Data["task_type"] != TaskCoding {
			t.Fatalf("task_type = %q, want coding", event.Data["task_type"])
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
	if requests[0].Model != "qwen2.5-coder:3b" {
		t.Fatalf("model = %q, want qwen2.5-coder:3b", requests[0].Model)
	}
	if len(requests[0].Messages) < 2 || !strings.Contains(requests[0].Messages[1].Content, "TASK PLAN:") {
		t.Fatalf("prompt did not include task plan: %+v", requests[0].Messages)
	}
	for _, eventType := range []string{EventTaskClassified, EventPlanCreated, EventVerificationCompleted, EventAgentCompleted} {
		if !seen[eventType] {
			t.Fatalf("event %s was not emitted", eventType)
		}
	}
}

func TestAskFallsBackWhenSpecializedLocalModelIsNotInstalled(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	cfg.Models["low_memory"] = config.ModelConfig{
		Provider:    models.ProviderOllama,
		Name:        "installed-local:3b",
		BaseURL:     "http://localhost:11434/api",
		Temperature: 0.1,
	}
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var fallbackSeen bool
	service := &Service{
		Router: models.NewRouter(cfg),
		Runtime: installedModelRuntime{
			text:      "coded answer",
			installed: []models.ModelInfo{{Name: "installed-local:3b"}},
			requests:  &requests,
		},
		Memory: store,
	}

	err = service.Ask(ctx, AskInput{
		Content:          "Fix the bug in main.go",
		WorkspaceContext: "main.go: package main",
		Sources:          []string{"main.go"},
		SourceKind:       "workspace",
	}, func(event Event) error {
		if event.Type == EventModelSelected && event.Data["model"] == "installed-local:3b" && strings.Contains(event.Data["reason"], "not installed") {
			fallbackSeen = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
	if requests[0].Model != "installed-local:3b" {
		t.Fatalf("model = %q, want installed fallback", requests[0].Model)
	}
	if !fallbackSeen {
		t.Fatal("expected installed fallback model selection event")
	}
}

func TestAskInjectsBoundedMemoryContext(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	cfg.Memory.MaxRelevantMemories = 2
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	if _, err := store.SaveMemory(ctx, memory.Memory{
		Kind:       "preference",
		Content:    "Prefer concise SQLite memory answers.",
		Importance: 5,
		Source:     "user",
	}); err != nil {
		t.Fatalf("SaveMemory(preference) error = %v", err)
	}
	if _, err := store.SaveMemory(ctx, memory.Memory{
		Kind:       "project_note",
		Content:    "Yemaka SQLite memory uses FTS5 retrieval.",
		Importance: 3,
		Source:     "project",
	}); err != nil {
		t.Fatalf("SaveMemory(project_note) error = %v", err)
	}

	var requests []models.ChatRequest
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "memory-aware answer", requests: &requests},
		Memory:  store,
	}

	seen := map[string]bool{}
	err = service.Ask(ctx, AskInput{
		Content:          "SQLite memory",
		WorkspaceContext: "README.md: memory notes",
		SourceKind:       "workspace",
	}, func(event Event) error {
		seen[event.Type] = true
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
	prompt := requests[0].Messages[1].Content
	if !strings.Contains(prompt, "PROFILE MEMORY:") || !strings.Contains(prompt, "Prefer concise SQLite memory answers.") {
		t.Fatalf("prompt missing profile memory: %s", prompt)
	}
	if !strings.Contains(prompt, "TASK MEMORY:") || !strings.Contains(prompt, "Yemaka SQLite memory uses FTS5 retrieval.") {
		t.Fatalf("prompt missing task memory: %s", prompt)
	}
	if !seen[EventMemoryUsed] {
		t.Fatal("memory used event was not emitted")
	}
}

func TestChatSavesConversationSummaryAfterThreshold(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	cfg.Runtime.AutoSummarize = true
	cfg.Memory.SummarizeAfter = 2
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "assistant remembers compact summary", requests: nil},
		Memory:  store,
	}
	err = service.Chat(ctx, ChatInput{Content: "local summary threshold"}, func(event Event) error { return nil })
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	results, err := store.SearchMemories(ctx, "assistant remembers compact summary", 5)
	if err != nil {
		t.Fatalf("SearchMemories() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("summary search len = %d, want 1", len(results))
	}
	if results[0].Kind != "summary" {
		t.Fatalf("summary kind = %q, want summary", results[0].Kind)
	}
}

func TestChatUsesCloudFallbackOnlyAfterLocalFailure(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	cfg.CloudFallback = config.CloudFallbackConfig{
		Enabled:     true,
		Provider:    "openai_compatible",
		Name:        "fallback-model",
		BaseURL:     "https://example.test/v1",
		Temperature: 0.2,
	}
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: failingRuntime{},
		Memory:  store,
		CloudFallback: &CloudFallback{
			Config:  cfg.CloudFallback,
			Runtime: staticRuntime{text: "fallback answer"},
		},
	}

	var sawFallback bool
	var sawCloudSelection bool
	var output string
	err = service.Chat(ctx, ChatInput{Content: "hello"}, func(event Event) error {
		switch event.Type {
		case EventCloudFallback:
			sawFallback = true
		case EventModelSelected:
			if event.Data["provider"] == "openai_compatible" && event.Data["model"] == "fallback-model" {
				sawCloudSelection = true
			}
		case EventModelToken:
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if !sawFallback {
		t.Fatal("cloud fallback event was not emitted")
	}
	if !sawCloudSelection {
		t.Fatal("cloud model selection event was not emitted")
	}
	if output != "fallback answer" {
		t.Fatalf("output = %q, want fallback answer", output)
	}
	results, err := store.SearchMessages(ctx, "fallback answer", 5)
	if err != nil {
		t.Fatalf("SearchMessages() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("fallback answer was not saved to memory")
	}
}

func TestChatReturnsLocalErrorWhenFallbackDisabled(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: failingRuntime{},
		Memory:  store,
	}
	err = service.Chat(ctx, ChatInput{Content: "hello"}, func(event Event) error { return nil })
	if err == nil {
		t.Fatal("Chat() error = nil, want local failure")
	}
}

func TestChatUsesRuntimeFactoryForSelectedLocalProvider(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	model := cfg.Models["low_memory"]
	model.Provider = models.ProviderLlamaCpp
	model.Name = "tiny-local"
	model.BaseURL = "http://127.0.0.1:8080/v1"
	cfg.Models["low_memory"] = model
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var selected config.ModelConfig
	service := &Service{
		Router: models.NewRouter(cfg),
		RuntimeFactory: func(model config.ModelConfig) (models.Runtime, error) {
			selected = model
			return staticRuntime{text: "factory answer"}, nil
		},
		Memory: store,
	}
	var output string
	err = service.Chat(ctx, ChatInput{Content: "hello"}, func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if selected.Provider != models.ProviderLlamaCpp {
		t.Fatalf("selected provider = %q, want llamacpp", selected.Provider)
	}
	if output != "factory answer" {
		t.Fatalf("output = %q, want factory answer", output)
	}
}

func TestChatEmitsModelToolCallEvents(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: toolCallRuntime{},
		Memory:  store,
	}
	var output string
	var toolCall Event
	err = service.Chat(ctx, ChatInput{Content: "hello"}, func(event Event) error {
		switch event.Type {
		case EventModelToken:
			output += event.Token
		case EventModelToolCall:
			toolCall = event
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if output != "before after" {
		t.Fatalf("output = %q, want before after", output)
	}
	if toolCall.Type != EventModelToolCall {
		t.Fatal("model tool-call event was not emitted")
	}
	if toolCall.Data["count"] != "1" || toolCall.Data["names"] != "read_file" {
		t.Fatalf("tool-call data = %#v, want read_file count 1", toolCall.Data)
	}
}

func TestGuardedTokenEmitterStreamsNormalAnswer(t *testing.T) {
	var output string
	emitter := newGuardedTokenEmitter(func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	}, true)

	for _, token := range []string{"Y", "e", "s", ", streaming works."} {
		if err := emitter.Emit(token); err != nil {
			t.Fatalf("Emit() error = %v", err)
		}
	}
	if err := emitter.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if output != "Yes, streaming works." {
		t.Fatalf("output = %q, want normal streamed answer", output)
	}
}

func TestGuardedTokenEmitterHidesToolRequestMarker(t *testing.T) {
	var output string
	emitter := newGuardedTokenEmitter(func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	}, true)

	for _, token := range []string{"YEMAKA", "_TOOL_REQUEST\n", "```json\n", `{"tool_name":"read_file","path":"README.md"}`, "\n```"} {
		if err := emitter.Emit(token); err != nil {
			t.Fatalf("Emit() error = %v", err)
		}
	}
	if err := emitter.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if output != "" {
		t.Fatalf("output = %q, want tool marker hidden", output)
	}
}

func TestGuardedTokenEmitterStripsSplitThinkingBlocks(t *testing.T) {
	var output string
	emitter := newGuardedTokenEmitter(func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	}, true)

	for _, token := range []string{"<thi", "nk>private chain", " of thought</think>", "Final answer."} {
		if err := emitter.Emit(token); err != nil {
			t.Fatalf("Emit() error = %v", err)
		}
	}
	if err := emitter.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if output != "Final answer." {
		t.Fatalf("output = %q, want only final answer", output)
	}
}

func TestStripModelThinkingBlocksDropsUnclosedThinking(t *testing.T) {
	content := StripModelThinkingBlocks("Visible.\n<thinking>private reasoning that never closes")
	if content != "Visible." {
		t.Fatalf("content = %q, want visible answer only", content)
	}
}

func TestAppliedModelProfileUpdatesSelectionAndSystemGuidance(t *testing.T) {
	dir := t.TempDir()
	profile := modelprofiles.Profile{
		Name:      "Careful Helper",
		BaseModel: "small:2b",
		System:    "Prefer concise answers and mention uncertainty.",
		Parameters: modelprofiles.Parameters{
			Temperature: 0.3,
			NumCtx:      2048,
		},
	}
	if _, err := modelprofiles.NewStore(dir).Save(profile); err != nil {
		t.Fatalf("Save profile error = %v", err)
	}
	service := &Service{ModelProfileDir: dir}
	selected, status := service.applyModelProfileToSelection(config.ModelConfig{
		Provider:    models.ProviderOllama,
		Name:        "plain:1b",
		Temperature: 0.9,
		Profile:     "Careful Helper",
	})
	if selected.Name != "small:2b" || selected.Temperature != 0.3 {
		t.Fatalf("selected = %+v, want profile base model and temperature", selected)
	}
	if status.Name != "Careful Helper" || status.Status != "applied" {
		t.Fatalf("status = %+v, want applied profile", status)
	}
	guidance := status.SystemGuidance()
	if !strings.Contains(guidance, "Applied local model profile") || !strings.Contains(guidance, "Prefer concise answers") {
		t.Fatalf("guidance = %q, want profile system guidance", guidance)
	}
}

func capabilityDraftJSONForTest(t *testing.T, name string, description string) string {
	t.Helper()
	manifest := fmt.Sprintf(`name: %s
version: 0.1.0
description: %s
type: tool
entrypoint:
  type: command
  command: ./%s
input_schema:
  type: object
  properties:
    task:
      type: string
  required:
    - task
output_schema:
  type: object
  properties:
    ok:
      type: boolean
    summary:
      type: string
permissions:
  network:
    enabled: false
    allowed_methods: []
    allowed_domains: []
  filesystem:
    read: false
    write: false
    paths: []
  shell: false
  secrets: false
  memory: false
safety:
  requires_user_approval: false
  max_runtime_seconds: 30
  max_response_bytes: 200000
tests:
  - go test ./...
`, name, description, name)
	mainSource := strings.ReplaceAll(`package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const protocolVersion = "yemaka.extension.v1"

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		emit(false, nil, err.Error())
		os.Exit(1)
	}
	input := map[string]any{}
	if len(strings.TrimSpace(string(data))) > 0 {
		var envelope map[string]any
		if err := json.Unmarshal(data, &envelope); err != nil {
			emit(false, nil, err.Error())
			os.Exit(1)
		}
		if value, ok := envelope["input"].(map[string]any); ok {
			input = value
		}
	}
	output, err := handle(input)
	if err != nil {
		emit(false, nil, err.Error())
		os.Exit(1)
	}
	emit(true, output, "")
}

func handle(input map[string]any) (map[string]any, error) {
	task, _ := input["task"].(string)
	task = strings.TrimSpace(task)
	if task == "" {
		return nil, fmt.Errorf("task is required")
	}
	return map[string]any{"ok": true, "summary": "{{NAME}} synth handled " + task}, nil
}

func emit(ok bool, output map[string]any, message string) {
	resp := map[string]any{"protocol": protocolVersion, "ok": ok}
	if output != nil {
		resp["output"] = output
	}
	if message != "" {
		resp["error"] = message
	}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}
`, "{{NAME}}", name)
	testSource := `package main

import (
	"strings"
	"testing"
)

func TestHandle(t *testing.T) {
	output, err := handle(map[string]any{"task": "normalize rows"})
	if err != nil {
		t.Fatalf("handle() error = %v", err)
	}
	summary, _ := output["summary"].(string)
	if !strings.Contains(summary, "normalize rows") {
		t.Fatalf("summary = %q, want task", summary)
	}
}
`
	draft := extensions.PackageDraft{
		Origin: "generated_by=yemaka\nmilestone=4.18\nsource=test_model_draft\n",
		Files: []extensions.DraftFile{
			{Path: extensions.ManifestFile, Content: manifest},
			{Path: "README.md", Content: "# " + name + "\n\n" + description + "\n"},
			{Path: "go.mod", Content: "module " + name + "\n\ngo 1.22\n"},
			{Path: "main.go", Content: mainSource},
			{Path: "main_test.go", Content: testSource},
		},
	}
	data, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshal draft JSON: %v", err)
	}
	return string(data)
}
