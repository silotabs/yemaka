package tui

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"yemaka/internal/config"
	"yemaka/internal/diagnostics"
	"yemaka/internal/extensions"
	"yemaka/internal/memory"
	"yemaka/internal/models"
	"yemaka/internal/profiles"
	"yemaka/internal/rag"
	"yemaka/internal/safety"
	"yemaka/internal/scheduler"
	"yemaka/internal/skills"
)

type fakeRuntime struct {
	models []models.ModelInfo
}

func (f fakeRuntime) Health(ctx context.Context) error {
	return nil
}

func (f fakeRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return f.models, nil
}

func (f fakeRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (f fakeRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	if err := emit(models.ChatEvent{Token: "terminal"}); err != nil {
		return err
	}
	return emit(models.ChatEvent{Token: " answer"})
}

func (f fakeRuntime) Embed(ctx context.Context, req models.EmbeddingRequest) (models.EmbeddingResponse, error) {
	embeddings := make([][]float64, 0, len(req.Input))
	for range req.Input {
		embeddings = append(embeddings, []float64{0.1, 0.2, 0.3})
	}
	return models.EmbeddingResponse{Embeddings: embeddings}, nil
}

func TestRunnerHelpAndQuit(t *testing.T) {
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	var output bytes.Buffer
	err := runner.Run(context.Background(), strings.NewReader("help\nquit\n"), &output)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(output.String(), "Commands:") {
		t.Fatalf("output does not include commands:\n%s", output.String())
	}
	if !strings.Contains(output.String(), "bye") {
		t.Fatalf("output does not include goodbye:\n%s", output.String())
	}
}

func TestRunnerDoesNotReserveBareQ(t *testing.T) {
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	var output bytes.Buffer
	err := runner.Run(context.Background(), strings.NewReader("q\nquit\n"), &output)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(output.String(), "unknown command: q") {
		t.Fatalf("bare q should be regular input, output:\n%s", output.String())
	}
	if !strings.Contains(output.String(), "bye") {
		t.Fatalf("output does not include goodbye:\n%s", output.String())
	}
}

func TestRunnerSlashShowsHelp(t *testing.T) {
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	var output bytes.Buffer
	err := runner.Run(context.Background(), strings.NewReader("/\nquit\n"), &output)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Count(output.String(), "Commands:") < 2 {
		t.Fatalf("slash should show command help, output:\n%s", output.String())
	}
}

func TestBubbleDoesNotReserveBareQKey(t *testing.T) {
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	model := newBubbleModel(context.Background(), runner)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got := updated.(bubbleModel)
	if got.input.Value() != "q" {
		t.Fatalf("input value = %q, want q", got.input.Value())
	}
}

func TestBubbleSlashShowsHelp(t *testing.T) {
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	model := newBubbleModel(context.Background(), runner)
	model.input.SetValue("/")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(bubbleModel)
	if len(got.lines) == 0 || !strings.Contains(got.lines[len(got.lines)-1].Text, "Useful commands:") {
		t.Fatalf("slash did not append help panel: %+v", got.lines)
	}
}

func TestRunnerChatUsesAgentAndSavesMemory(t *testing.T) {
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	var output bytes.Buffer
	err := runner.Run(context.Background(), strings.NewReader("chat hello\nquit\n"), &output)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(output.String(), "terminal answer") {
		t.Fatalf("output does not include model answer:\n%s", output.String())
	}
	results, err := runner.deps.Memory.SearchMessages(context.Background(), "hello", 10)
	if err != nil {
		t.Fatalf("search messages: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("chat did not save user message to memory")
	}
	if !strings.Contains(output.String(), "last_conversation:") || !strings.Contains(output.String(), "resume: conversation use") {
		t.Fatalf("output missing conversation resume hint:\n%s", output.String())
	}
}

func TestRunnerKeepsActiveConversationUntilNewChat(t *testing.T) {
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	var output bytes.Buffer
	input := strings.NewReader("ask first turn\nask second turn\nconversation current\nnew chat\nask third turn\nconversation list\nquit\n")
	err := runner.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	conversations, err := runner.deps.Memory.ListConversations(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}
	if len(conversations) != 2 {
		t.Fatalf("conversation count = %d, want 2\noutput:\n%s", len(conversations), output.String())
	}
	var firstCount, secondCount int
	for _, conversation := range conversations {
		count, err := runner.deps.Memory.CountConversationMessages(context.Background(), conversation.ID)
		if err != nil {
			t.Fatalf("CountConversationMessages(%s) error = %v", conversation.ID, err)
		}
		if strings.Contains(conversation.Title, "first turn") {
			firstCount = count
		}
		if strings.Contains(conversation.Title, "third turn") {
			secondCount = count
		}
	}
	if firstCount != 4 {
		t.Fatalf("first conversation message count = %d, want 4\noutput:\n%s", firstCount, output.String())
	}
	if secondCount != 2 {
		t.Fatalf("second conversation message count = %d, want 2\noutput:\n%s", secondCount, output.String())
	}
}

func TestRunnerPolicyLearningAndHeartbeatCommands(t *testing.T) {
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	conversation, err := runner.deps.Memory.CreateConversation(context.Background(), "TUI learning")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := runner.deps.Memory.SaveMessage(context.Background(), memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Remember terminal parity",
		Model:          "small:2b",
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := runner.deps.Memory.SaveMessage(context.Background(), memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "Terminal parity remembered.",
		Model:          "small:2b",
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}

	var output bytes.Buffer
	input := strings.NewReader("policy status\npolicy mode full_access\nlearn correction " + conversation.ID + " prefer terminal parity\nlearn report\nheartbeat status\nquit\n")
	err = runner.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := output.String()
	if !strings.Contains(got, `"mode": "safe"`) {
		t.Fatalf("output missing initial policy status:\n%s", got)
	}
	if !strings.Contains(got, `"mode": "full_access"`) {
		t.Fatalf("output missing updated policy status:\n%s", got)
	}
	if !strings.Contains(got, `"corrections": 1`) {
		t.Fatalf("output missing learning report:\n%s", got)
	}
	if !strings.Contains(got, `"overall"`) {
		t.Fatalf("output missing heartbeat report:\n%s", got)
	}
}

func TestRunnerInternetSearchProviderCommands(t *testing.T) {
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	var output bytes.Buffer
	input := strings.NewReader("internet search-provider searxng https://search.example/search --max-results 7\ninternet search-provider status\nquit\n")
	err := runner.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if runner.deps.Config.Internet.Enabled {
		t.Fatal("provider setup should not enable internet without --enable-internet")
	}
	if !runner.deps.Config.Internet.Search.Enabled {
		t.Fatal("search provider should be enabled")
	}
	if got := runner.deps.Config.Internet.Search.Provider; got != "searxng" {
		t.Fatalf("provider = %q, want searxng", got)
	}
	if got := runner.deps.Config.Internet.Search.Endpoint; got != "https://search.example/search" {
		t.Fatalf("endpoint = %q", got)
	}
	if got := runner.deps.Config.Internet.Search.MaxResults; got != 7 {
		t.Fatalf("max results = %d, want 7", got)
	}
	got := output.String()
	if !strings.Contains(got, "internet remains disabled") {
		t.Fatalf("output should explain internet remains disabled:\n%s", got)
	}
	if !strings.Contains(got, `"provider": "searxng"`) {
		t.Fatalf("output missing provider status:\n%s", got)
	}
	if !strings.Contains(got, `"ready": false`) {
		t.Fatalf("output should show provider is not ready until internet is enabled:\n%s", got)
	}
}

func TestRunnerInternetSearchProviderCanEnableInternet(t *testing.T) {
	t.Setenv("BRAVE_SEARCH_API_KEY", "test-secret")
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	var output bytes.Buffer
	input := strings.NewReader("internet search-provider brave BRAVE_SEARCH_API_KEY --enable-internet\ninternet search-provider status\nquit\n")
	err := runner.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !runner.deps.Config.Internet.Enabled {
		t.Fatal("expected --enable-internet to enable internet")
	}
	if got := runner.deps.Config.Internet.DefaultMode; got != "profile_enabled" {
		t.Fatalf("internet mode = %q, want profile_enabled", got)
	}
	if got := runner.deps.Config.Internet.Search.Provider; got != "brave" {
		t.Fatalf("provider = %q, want brave", got)
	}
	if got := runner.deps.Config.Internet.Search.APIKeyEnv; got != "BRAVE_SEARCH_API_KEY" {
		t.Fatalf("api key env = %q", got)
	}
	got := output.String()
	if !strings.Contains(got, `"ready": true`) {
		t.Fatalf("output should show configured provider ready:\n%s", got)
	}
}

func TestRunnerInternetSearchProviderConfiguresPostProviders(t *testing.T) {
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	var output bytes.Buffer
	input := strings.NewReader("internet search-provider tavily TAVILY_API_KEY --endpoint https://api.tavily.test/search --enable-internet\ninternet search-provider serper SERPER_API_KEY\nquit\n")
	err := runner.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := runner.deps.Config.Internet.Search.Provider; got != "serper" {
		t.Fatalf("provider = %q, want serper", got)
	}
	if got := runner.deps.Config.Internet.Search.APIKeyEnv; got != "SERPER_API_KEY" {
		t.Fatalf("api key env = %q, want SERPER_API_KEY", got)
	}
	if !strings.Contains(output.String(), "internet search provider configured: tavily") {
		t.Fatalf("output missing Tavily configuration:\n%s", output.String())
	}
}

func TestRunnerInternetSearchProviderConfiguresFallbackProviders(t *testing.T) {
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	var output bytes.Buffer
	input := strings.NewReader("internet search-provider duckduckgo --enable-internet\ninternet search-provider auto\ninternet search-provider status\nquit\n")
	err := runner.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := runner.deps.Config.Internet.Search.Provider; got != "auto" {
		t.Fatalf("provider = %q, want auto", got)
	}
	got := output.String()
	if !strings.Contains(got, `"provider": "auto"`) || !strings.Contains(got, `"fallback_providers"`) {
		t.Fatalf("output missing auto fallback status:\n%s", got)
	}
}

func TestRunnerCapabilityGenerateRequiresApprovalAndCreatesExtension(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	var output bytes.Buffer
	input := strings.NewReader("capability generate build a tool that normalizes CSV --name tui_csv\nquit\n")
	err := runner.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run(capability generate without approval) error = %v", err)
	}
	if !strings.Contains(output.String(), "use --yes") {
		t.Fatalf("output should require explicit approval:\n%s", output.String())
	}

	output.Reset()
	input = strings.NewReader("capability generate build a tool that normalizes CSV --name tui_csv --yes\nextension list\nquit\n")
	err = runner.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run(capability generate) error = %v\noutput:\n%s", err, output.String())
	}
	got := output.String()
	if !strings.Contains(got, `"message": "capability generated, tested, and registered after explicit approval"`) ||
		!strings.Contains(got, `"name": "tui_csv"`) ||
		!strings.Contains(got, `"enabled": true`) {
		t.Fatalf("output missing generated enabled extension:\n%s", got)
	}

	output.Reset()
	input = strings.NewReader("capability generate build a tool that normalizes CSV --name tui_csv_run --yes --run --input-json {\"task\":\"normalize rows\"}\nquit\n")
	err = runner.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run(capability generate --run) error = %v\noutput:\n%s", err, output.String())
	}
	got = output.String()
	if !strings.Contains(got, `"message": "capability generated, tested, registered, and run after explicit approval"`) ||
		!strings.Contains(got, `"extension": "tui_csv_run"`) ||
		!strings.Contains(got, `"status": "completed"`) {
		t.Fatalf("output missing generated run result:\n%s", got)
	}

	output.Reset()
	input = strings.NewReader("capability generate build a tool that cleans CSV --name tui_csv_job --yes --job --every 1h --job-input-json {\"task\":\"scheduled cleanup\"}\nquit\n")
	err = runner.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run(capability generate --job) error = %v\noutput:\n%s", err, output.String())
	}
	got = output.String()
	if !strings.Contains(got, `"message": "capability generated, tested, registered, and attached to an approved disabled job"`) ||
		!strings.Contains(got, `"targetName": "tui_csv_job"`) ||
		!strings.Contains(got, `"scheduleType": "interval"`) ||
		!strings.Contains(got, `"enabled": false`) {
		t.Fatalf("output missing generated job result:\n%s", got)
	}
}

func TestRunnerJobCreateRejectsKnownExtensionInvalidInput(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	runner, cleanup := newTestRunner(t)
	defer cleanup()
	store := extensions.NewStore(runner.deps.Profile.GeneratedExtensions, runner.deps.Profile.Logs)
	if _, err := store.Generate(context.Background(), extensions.GenerateInput{
		Name:        "needs_task",
		Description: "Handle a scheduled task.",
		Approved:    true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var output bytes.Buffer
	err := runner.jobCommand(context.Background(), &output, "create manual needs_task --yes")
	if err == nil || !strings.Contains(err.Error(), "input.task is required") {
		t.Fatalf("jobCommand(create invalid extension input) error = %v, want task requirement", err)
	}
}

func TestRunnerJobEnableRejectsKnownExtensionInvalidStoredInput(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	runner, cleanup := newTestRunner(t)
	defer cleanup()
	extensionStore := extensions.NewStore(runner.deps.Profile.GeneratedExtensions, runner.deps.Profile.Logs)
	if _, err := extensionStore.Generate(context.Background(), extensions.GenerateInput{
		Name:        "needs_task_enable",
		Description: "Handle a scheduled task.",
		Approved:    true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	scheduleStore, err := runner.schedulerStore(context.Background())
	if err != nil {
		t.Fatalf("schedulerStore() error = %v", err)
	}
	defer scheduleStore.Close()
	job, err := scheduleStore.Create(context.Background(), scheduler.CreateInput{
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "needs_task_enable",
		Input:        map[string]any{},
		Approved:     true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var output bytes.Buffer
	err = runner.jobCommand(context.Background(), &output, "enable "+job.ID)
	if err == nil || !strings.Contains(err.Error(), "input.task is required") {
		t.Fatalf("jobCommand(enable invalid extension input) error = %v, want task requirement", err)
	}
}

func TestRunnerJobUpdateInputValidatesAndListsInvalidStoredInput(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	runner, cleanup := newTestRunner(t)
	defer cleanup()
	extensionStore := extensions.NewStore(runner.deps.Profile.GeneratedExtensions, runner.deps.Profile.Logs)
	if _, err := extensionStore.Generate(context.Background(), extensions.GenerateInput{
		Name:        "needs_task_update",
		Description: "Handle a scheduled task.",
		Approved:    true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	scheduleStore, err := runner.schedulerStore(context.Background())
	if err != nil {
		t.Fatalf("schedulerStore() error = %v", err)
	}
	job, err := scheduleStore.Create(context.Background(), scheduler.CreateInput{
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "needs_task_update",
		Input:        map[string]any{},
		Approved:     true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	scheduleStore.Close()

	var output bytes.Buffer
	if err := runner.jobCommand(context.Background(), &output, "list"); err != nil {
		t.Fatalf("jobCommand(list) error = %v", err)
	}
	if !strings.Contains(output.String(), `"inputStatus": "invalid"`) || !strings.Contains(output.String(), "input.task is required") {
		t.Fatalf("list output = %q, want invalid input details", output.String())
	}
	output.Reset()
	if err := runner.jobCommand(context.Background(), &output, "status"); err != nil {
		t.Fatalf("jobCommand(status) error = %v", err)
	}
	if !strings.Contains(output.String(), `"invalidInputJobs": 1`) {
		t.Fatalf("status output = %q, want invalid input count", output.String())
	}
	output.Reset()
	err = runner.jobCommand(context.Background(), &output, "update-input "+job.ID+" --input-json {} --yes")
	if err == nil || !strings.Contains(err.Error(), "input.task is required") {
		t.Fatalf("jobCommand(update-input invalid) error = %v, want task requirement", err)
	}
	output.Reset()
	if err := runner.jobCommand(context.Background(), &output, "update-input "+job.ID+" --input-json {\"task\":\"scheduled cleanup\"} --yes"); err != nil {
		t.Fatalf("jobCommand(update-input valid) error = %v", err)
	}
	if !strings.Contains(output.String(), `"inputStatus": "valid"`) || !strings.Contains(output.String(), `"task": "scheduled cleanup"`) || strings.Contains(output.String(), `"enabled": true`) {
		t.Fatalf("update output = %q, want valid disabled updated job", output.String())
	}
}

func TestParseInternetSearch(t *testing.T) {
	got, err := parseInternetSearch("local agents --yes --limit 3")
	if err != nil {
		t.Fatalf("parseInternetSearch() error = %v", err)
	}
	if got.Query != "local agents" {
		t.Fatalf("query = %q", got.Query)
	}
	if !got.TaskApproved {
		t.Fatal("TaskApproved = false, want true")
	}
	if got.MaxResults != 3 {
		t.Fatalf("MaxResults = %d, want 3", got.MaxResults)
	}
	if got.Caller != "tui" {
		t.Fatalf("Caller = %q, want tui", got.Caller)
	}
}

func TestRunnerRAGEmbeddingsCommands(t *testing.T) {
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	if err := os.WriteFile(filepath.Join(runner.deps.Workspace, "embedding-note.md"), []byte("semantic retrieval helps small local models\n"), 0o644); err != nil {
		t.Fatalf("write embedding fixture: %v", err)
	}
	if _, err := runner.deps.RAG.IngestPath(context.Background(), runner.deps.Profile.Name, runner.deps.Workspace, ragConfig(runner.deps.Config.RAG), workspaceLimits(runner.deps.Config.Workspace)); err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}

	var output bytes.Buffer
	input := strings.NewReader("rag embeddings on small:2b\nrag embeddings index\nrag embeddings status\nrag embeddings off\nquit\n")
	err := runner.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := output.String()
	if !strings.Contains(got, "rag embeddings enabled: small:2b") {
		t.Fatalf("output missing enabled message:\n%s", got)
	}
	if !strings.Contains(got, `"chunks_embedded":`) || !strings.Contains(got, `"dimensions": 3`) {
		t.Fatalf("output missing embedding index result:\n%s", got)
	}
	if !strings.Contains(got, `"enabled": true`) {
		t.Fatalf("output missing enabled status:\n%s", got)
	}
	if runner.deps.Config.RAG.Embeddings.Enabled {
		t.Fatal("embeddings should be disabled after off command")
	}
}

func TestRunnerIngestRequiresWorkspaceGrantForExternalPath(t *testing.T) {
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	externalRoot := t.TempDir()
	docPath := filepath.Join(externalRoot, "external-note.md")
	if err := os.WriteFile(docPath, []byte("external TUI grant content\n"), 0o644); err != nil {
		t.Fatalf("write external doc: %v", err)
	}

	var output bytes.Buffer
	err := runner.ingest(context.Background(), &output, docPath)
	if err == nil || !strings.Contains(err.Error(), "workspace path is not granted") {
		t.Fatalf("ingest without grant error = %v, want workspace grant error", err)
	}

	grantStore := safety.NewWorkspaceGrantStore(safety.WorkspaceGrantsPath(runner.deps.Profile.Permissions))
	if _, err := grantStore.Grant(externalRoot, "external-docs", "test"); err != nil {
		t.Fatalf("grant external root: %v", err)
	}

	output.Reset()
	if err := runner.ingest(context.Background(), &output, docPath); err != nil {
		t.Fatalf("ingest with grant error = %v", err)
	}
	if !strings.Contains(output.String(), "files_indexed: 1") {
		t.Fatalf("ingest output = %q, want indexed file", output.String())
	}
}

func TestBubbleModelViewRendersCorePanels(t *testing.T) {
	runner, cleanup := newTestRunner(t)
	defer cleanup()

	model := newBubbleModel(context.Background(), runner)
	model.width = 110
	model.height = 32
	model.status = diagnosticsRunForTest(context.Background(), runner)
	model.resize()
	view := model.View()
	for _, want := range []string{"Yemaka TUI", "Chat", "Command", "Memory", "Learning"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func newTestRunner(t *testing.T) (*Runner, func()) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	t.Setenv(config.EnvHome, root)
	cfg := config.Default()
	cfg.Path = filepath.Join(root, config.ConfigName)
	cfg.App.SetupComplete = true
	cfg.Models["low_memory"] = config.ModelConfig{
		Provider:    "ollama",
		Name:        "small:2b",
		BaseURL:     "http://localhost:11434/api",
		Temperature: 0.1,
	}
	cfg.Memory.Database = filepath.Join(root, "profiles", "default", "memory.sqlite")
	if err := config.Write(cfg.Path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	profile, err := profiles.Init(cfg)
	if err != nil {
		t.Fatalf("init profile: %v", err)
	}
	store, err := memory.Open(ctx, profile.Database)
	if err != nil {
		t.Fatalf("open memory: %v", err)
	}
	ragStore, err := rag.Open(ctx, profile.Database)
	if err != nil {
		_ = store.Close()
		t.Fatalf("open rag: %v", err)
	}
	runner := New(Dependencies{
		Config:    cfg,
		Profile:   profile,
		Memory:    store,
		RAG:       ragStore,
		Skills:    skills.Registry{Skills: map[string]skills.Skill{}},
		Runtime:   fakeRuntime{models: []models.ModelInfo{{Name: "small:2b", Size: 1_500_000_000}}},
		Router:    models.NewRouter(cfg),
		Workspace: root,
	})
	return runner, func() {
		_ = ragStore.Close()
		_ = store.Close()
	}
}

func diagnosticsRunForTest(ctx context.Context, runner *Runner) diagnostics.Report {
	return diagnostics.Run(ctx, runner.deps.Config, runner.deps.Profile, runner.deps.Memory, runner.deps.Runtime)
}
