package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"yemaka/internal/agent"
	"yemaka/internal/brand"
	"yemaka/internal/config"
	"yemaka/internal/connectors"
	"yemaka/internal/diagnostics"
	"yemaka/internal/extensions"
	"yemaka/internal/heartbeat"
	"yemaka/internal/internet"
	"yemaka/internal/learning"
	"yemaka/internal/memory"
	"yemaka/internal/models"
	"yemaka/internal/models/modelruntime"
	"yemaka/internal/profiles"
	"yemaka/internal/rag"
	"yemaka/internal/replay"
	"yemaka/internal/safety"
	"yemaka/internal/scheduler"
	"yemaka/internal/skills"
	"yemaka/internal/tools"
	"yemaka/internal/workspace"
)

type Dependencies struct {
	Config         *config.Config
	Profile        *profiles.Profile
	Memory         *memory.Store
	RAG            *rag.Store
	Skills         skills.Registry
	Runtime        models.Runtime
	RuntimeFactory func(config.ModelConfig) (models.Runtime, error)
	Cloud          models.Runtime
	Router         *models.Router
	Workspace      string
}

func modelProfileDir(deps Dependencies) string {
	if deps.Profile == nil || strings.TrimSpace(deps.Profile.Root) == "" {
		return ""
	}
	return filepath.Join(deps.Profile.Root, "model_profiles")
}

type Runner struct {
	deps                 Dependencies
	activeConversationID string
}

func New(deps Dependencies) *Runner {
	if strings.TrimSpace(deps.Workspace) == "" {
		deps.Workspace = "."
	}
	return &Runner{deps: deps}
}

func (r *Runner) Run(ctx context.Context, input io.Reader, output io.Writer) error {
	if err := r.validate(); err != nil {
		return err
	}
	if interactiveIO(input, output) {
		return r.runBubble(ctx, input, output)
	}
	return r.runLine(ctx, input, output)
}

func (r *Runner) runLine(ctx context.Context, input io.Reader, output io.Writer) error {
	reader := bufio.NewReader(input)
	r.printHeader(ctx, output)
	printHelp(output)
	for {
		fmt.Fprint(output, "\nyemaka> ")
		line, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) && strings.TrimSpace(line) == "" {
			fmt.Fprintln(output)
			r.printConversationResume(output)
			return nil
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		command := strings.TrimSpace(line)
		if command == "" {
			if errors.Is(err, io.EOF) {
				return nil
			}
			continue
		}
		if quit(command) {
			r.printConversationResume(output)
			fmt.Fprintln(output, "bye")
			return nil
		}
		if err := r.runCommand(ctx, reader, output, command); err != nil {
			fmt.Fprintf(output, "error: %s\n", err)
		}
		if errors.Is(err, io.EOF) {
			r.printConversationResume(output)
			return nil
		}
	}
}

func interactiveIO(input io.Reader, output io.Writer) bool {
	in, inOK := input.(*os.File)
	out, outOK := output.(*os.File)
	if !inOK || !outOK {
		return false
	}
	inInfo, err := in.Stat()
	if err != nil {
		return false
	}
	outInfo, err := out.Stat()
	if err != nil {
		return false
	}
	return inInfo.Mode()&os.ModeCharDevice != 0 && outInfo.Mode()&os.ModeCharDevice != 0
}

func (r *Runner) validate() error {
	if r == nil {
		return fmt.Errorf("tui runner is nil")
	}
	if r.deps.Config == nil {
		return fmt.Errorf("config is required")
	}
	if r.deps.Profile == nil {
		return fmt.Errorf("profile is required")
	}
	if r.deps.Memory == nil {
		return fmt.Errorf("memory store is required")
	}
	if r.deps.RAG == nil {
		return fmt.Errorf("rag store is required")
	}
	if r.deps.Runtime == nil {
		return fmt.Errorf("model runtime is required")
	}
	if r.deps.Router == nil {
		return fmt.Errorf("model router is required")
	}
	return nil
}

func (r *Runner) printHeader(ctx context.Context, output io.Writer) {
	report := diagnostics.Run(ctx, r.deps.Config, r.deps.Profile, r.deps.Memory, r.deps.Runtime)
	modelReady := "not ready"
	if report.ModelReady {
		modelReady = "ready"
	}
	internet := "net off"
	if r.deps.Config != nil && r.deps.Config.Internet.Enabled {
		internet = "net " + r.deps.Config.Internet.DefaultMode
	}
	fmt.Fprintln(output, brand.Wordmark)
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Yemaka TUI")
	fmt.Fprintln(output, "Local-first agent console")
	fmt.Fprintf(output, "status: %s | model: %s | policy: %s | %s\n", modelReady, report.SelectedModel, policyMode(r.deps.Config), internet)
	fmt.Fprintf(output, "profile: %s\n", report.ProfilePath)
	if report.ModelReady {
		fmt.Fprintln(output, "model_ready: true")
	} else {
		fmt.Fprintln(output, "model_ready: false")
	}
}

func (r *Runner) printConversationResume(output io.Writer) {
	conversationID := strings.TrimSpace(r.activeConversationID)
	if conversationID == "" {
		return
	}
	fmt.Fprintf(output, "last_conversation: %s\n", conversationID)
	fmt.Fprintf(output, "resume: conversation use %s\n", conversationID)
	fmt.Fprintf(output, "fullscreen_resume: /conversation use %s\n", conversationID)
}

func (r *Runner) runCommand(ctx context.Context, reader *bufio.Reader, output io.Writer, command string) error {
	command = strings.TrimSpace(command)
	if command == "/" {
		printHelp(output)
		return nil
	}
	name, rest := splitCommand(command)
	switch name {
	case "help", "?":
		printHelp(output)
	case "status":
		r.printStatus(ctx, output)
	case "chat":
		content, err := r.contentOrPrompt(reader, output, rest, "Chat")
		if err != nil {
			return err
		}
		return r.chat(ctx, output, content)
	case "ask":
		content, err := r.contentOrPrompt(reader, output, rest, "Ask")
		if err != nil {
			return err
		}
		return r.ask(ctx, output, content)
	case "memory":
		query, err := r.contentOrPrompt(reader, output, rest, "Memory search")
		if err != nil {
			return err
		}
		return r.searchMemory(ctx, output, query)
	case "docs":
		query, err := r.contentOrPrompt(reader, output, rest, "Document search")
		if err != nil {
			return err
		}
		return r.searchDocs(ctx, output, query)
	case "rag":
		return r.ragCommand(ctx, output, rest)
	case "models":
		return r.listModels(ctx, output)
	case "model":
		return r.modelCommand(output, rest)
	case "skills":
		return r.skillCommand(ctx, output, rest)
	case "skill":
		return r.skillCommand(ctx, output, rest)
	case "extensions":
		return r.extensionCommand(ctx, output, rest)
	case "extension":
		return r.extensionCommand(ctx, output, rest)
	case "internet":
		return r.internetCommand(ctx, output, rest)
	case "jobs":
		return r.jobCommand(ctx, output, rest)
	case "job":
		return r.jobCommand(ctx, output, rest)
	case "heartbeat":
		return r.heartbeatCommand(ctx, output, rest)
	case "connectors":
		return r.connectorCommand(output, rest)
	case "connector":
		return r.connectorCommand(output, rest)
	case "conversation", "conversations", "session", "sessions":
		return r.conversationCommand(ctx, output, rest)
	case "new":
		if strings.TrimSpace(rest) == "chat" || strings.TrimSpace(rest) == "" {
			r.activeConversationID = ""
			fmt.Fprintln(output, "new conversation will be created on the next message")
			return nil
		}
		fmt.Fprintf(output, "unknown command: %s %s\n", name, rest)
		printHelp(output)
	case "policy":
		return r.policyCommand(output, rest)
	case "learn":
		return r.learnCommand(ctx, output, rest)
	case "capability":
		return r.capabilityCommand(ctx, output, rest)
	case "tools":
		return r.toolRuns(ctx, output)
	case "ingest":
		path := strings.TrimSpace(rest)
		if path == "" {
			path = "."
		}
		return r.ingest(ctx, output, path)
	default:
		fmt.Fprintf(output, "unknown command: %s\n", name)
		printHelp(output)
	}
	return nil
}

func (r *Runner) conversationCommand(ctx context.Context, output io.Writer, rest string) error {
	name, remaining := splitCommand(rest)
	switch name {
	case "", "current":
		if strings.TrimSpace(r.activeConversationID) == "" {
			fmt.Fprintln(output, "conversation: new")
			return nil
		}
		conversation, err := r.deps.Memory.GetConversation(ctx, r.activeConversationID)
		if err != nil {
			return err
		}
		fmt.Fprintf(output, "conversation: %s\t%s\n", conversation.ID, conversation.Title)
		return nil
	case "new":
		r.activeConversationID = ""
		fmt.Fprintln(output, "new conversation will be created on the next message")
		return nil
	case "list":
		conversations, err := r.deps.Memory.ListConversations(ctx, 20)
		if err != nil {
			return err
		}
		if len(conversations) == 0 {
			fmt.Fprintln(output, "No conversations yet.")
			return nil
		}
		for _, conversation := range conversations {
			marker := " "
			if conversation.ID == r.activeConversationID {
				marker = "*"
			}
			star := ""
			if conversation.Starred {
				star = " starred"
			}
			fmt.Fprintf(output, "%s %s\t%s%s\n", marker, conversation.ID, conversation.Title, star)
		}
		return nil
	case "use", "open":
		id := strings.TrimSpace(remaining)
		if id == "" {
			return fmt.Errorf("usage: conversation use <conversation_id>")
		}
		conversation, err := r.deps.Memory.GetConversation(ctx, id)
		if err != nil {
			return err
		}
		r.activeConversationID = conversation.ID
		fmt.Fprintf(output, "conversation: %s\t%s\n", conversation.ID, conversation.Title)
		return nil
	default:
		return fmt.Errorf("usage: conversation current|new|list|use <conversation_id>")
	}
}

func (r *Runner) contentOrPrompt(reader *bufio.Reader, output io.Writer, existing string, label string) (string, error) {
	existing = strings.TrimSpace(existing)
	if existing != "" {
		return existing, nil
	}
	fmt.Fprintf(output, "%s: ", label)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (r *Runner) printStatus(ctx context.Context, output io.Writer) {
	report := diagnostics.Run(ctx, r.deps.Config, r.deps.Profile, r.deps.Memory, r.deps.Runtime)
	fmt.Fprintf(output, "config: %s\n", report.ConfigPath)
	fmt.Fprintf(output, "profile: %s\n", report.ProfilePath)
	fmt.Fprintf(output, "sqlite: %s\n", diagnostics.Status(report.SQLiteOK, report.SQLiteError))
	fmt.Fprintf(output, "ollama: %s\n", diagnostics.Status(report.OllamaOK, report.OllamaError))
	fmt.Fprintf(output, "selected_model: %s\n", report.SelectedModel)
	fmt.Fprintf(output, "model_ready: %t\n", report.ModelReady)
	fmt.Fprintf(output, "low_memory_mode: %t\n", report.LowMemoryMode)
	fmt.Fprintf(output, "rag_enabled: %t\n", report.RAGEnabled)
	fmt.Fprintf(output, "telemetry: %t\n", report.ConfigTelemetry)
}

func (r *Runner) chat(ctx context.Context, output io.Writer, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("chat message is required")
	}
	service := &agent.Service{
		Router:          r.deps.Router,
		Runtime:         r.deps.Runtime,
		RuntimeFactory:  r.deps.RuntimeFactory,
		Memory:          r.deps.Memory,
		CloudFallback:   tuiCloudFallback(r.deps.Config, r.deps.Cloud),
		ToolExecutor:    tuiToolExecutor(r.deps),
		CapabilityGap:   agent.NewCapabilityGapRouter(r.deps.Config, r.extensionStore()),
		ReplayStore:     tuiReplayStore(r.deps),
		ModelProfileDir: modelProfileDir(r.deps),
		PromptOptions:   agent.PromptOptionsFromConfig(r.deps.Config),
		PolicyMode:      policyMode(r.deps.Config),
		InternetTools:   agent.InternetToolLoopEnabled(r.deps.Config, policyMode(r.deps.Config)),
		InternetSearch:  agent.InternetSearchLoopEnabled(r.deps.Config, policyMode(r.deps.Config)),
	}
	return service.Chat(ctx, agent.ChatInput{Content: content, ConversationID: r.activeConversationID}, func(event agent.Event) error {
		switch event.Type {
		case agent.EventModelSelected:
			fmt.Fprintf(output, "model: %s\n\n", event.Data["model"])
		case agent.EventModelToken:
			fmt.Fprint(output, event.Token)
		case agent.EventAgentCompleted:
			r.activeConversationID = event.Data["conversation_id"]
			fmt.Fprintf(output, "\nconversation: %s\n", event.Data["conversation_id"])
		case agent.EventAgentError:
			return errors.New(event.Message)
		}
		return nil
	})
}

func (r *Runner) ask(ctx context.Context, output io.Writer, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("ask message is required")
	}
	selectedSkill, hasSkill := r.deps.Skills.Select(content, availableSkillTools())
	skillInstructions := ""
	if hasSkill {
		skillInstructions = skills.PromptBlock(selectedSkill)
	}

	service := &agent.Service{
		Router:          r.deps.Router,
		Runtime:         r.deps.Runtime,
		RuntimeFactory:  r.deps.RuntimeFactory,
		Memory:          r.deps.Memory,
		CloudFallback:   tuiCloudFallback(r.deps.Config, r.deps.Cloud),
		ToolExecutor:    tuiToolExecutor(r.deps),
		Retriever:       tuiRetriever(r.deps),
		CapabilityGap:   agent.NewCapabilityGapRouter(r.deps.Config, r.extensionStore()),
		ReplayStore:     tuiReplayStore(r.deps),
		ModelProfileDir: modelProfileDir(r.deps),
		PromptOptions:   agent.PromptOptionsFromConfig(r.deps.Config),
		PolicyMode:      policyMode(r.deps.Config),
		InternetTools:   agent.InternetToolLoopEnabled(r.deps.Config, policyMode(r.deps.Config)),
		InternetSearch:  agent.InternetSearchLoopEnabled(r.deps.Config, policyMode(r.deps.Config)),
	}
	return service.Ask(ctx, agent.AskInput{
		Content:            content,
		ConversationID:     r.activeConversationID,
		SkillName:          selectedSkill.Name,
		SkillVersion:       selectedSkill.Version,
		SkillRequiredTools: selectedSkill.RequiredTools,
		SkillInstructions:  skillInstructions,
	}, func(event agent.Event) error {
		switch event.Type {
		case agent.EventModelSelected:
			fmt.Fprintf(output, "model: %s\n", event.Data["model"])
		case agent.EventWorkspaceUsed:
			if event.Data["sources"] != "" {
				fmt.Fprintf(output, "%s_sources: %s\n\n", event.Data["source_kind"], event.Data["sources"])
			} else {
				fmt.Fprintln(output)
			}
		case agent.EventSkillSelected:
			fmt.Fprintf(output, "skill: %s@%s\n", event.Data["name"], event.Data["version"])
		case agent.EventModelToken:
			fmt.Fprint(output, event.Token)
		case agent.EventAgentCompleted:
			r.activeConversationID = event.Data["conversation_id"]
			fmt.Fprintf(output, "\nconversation: %s\n", event.Data["conversation_id"])
		case agent.EventAgentError:
			return errors.New(event.Message)
		}
		return nil
	})
}

func (r *Runner) searchMemory(ctx context.Context, output io.Writer, query string) error {
	results, err := r.deps.Memory.SearchMessages(ctx, query, r.deps.Config.Memory.MaxRelevantMemories)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Fprintln(output, "No memory matches.")
		return nil
	}
	for _, result := range results {
		fmt.Fprintf(output, "%s\t%s\t%s\n", result.CreatedAt, result.Role, result.Snippet)
	}
	return nil
}

func (r *Runner) searchDocs(ctx context.Context, output io.Writer, query string) error {
	results, err := r.deps.RAG.SearchWithConfig(ctx, query, ragConfig(r.deps.Config.RAG), embeddingRuntime(r.deps.Runtime))
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Fprintln(output, "No document matches.")
		return nil
	}
	for _, result := range results {
		fmt.Fprintf(output, "%s\t%s\n", result.Path, oneLine(result.Content))
	}
	return nil
}

func (r *Runner) ragCommand(ctx context.Context, output io.Writer, rest string) error {
	name, rest := splitCommand(rest)
	switch name {
	case "", "search":
		query := strings.TrimSpace(rest)
		if query == "" {
			return fmt.Errorf("usage: rag search <query> | rag embeddings status|on <installed-model>|off|index")
		}
		return r.searchDocs(ctx, output, query)
	case "embeddings":
		return r.ragEmbeddingsCommand(ctx, output, rest)
	default:
		return fmt.Errorf("usage: rag search <query> | rag embeddings status|on <installed-model>|off|index")
	}
}

func (r *Runner) ragEmbeddingsCommand(ctx context.Context, output io.Writer, rest string) error {
	name, rest := splitCommand(rest)
	switch name {
	case "", "status":
		return writeJSON(output, embeddingStatus(r.deps.Config.RAG.Embeddings))
	case "on":
		model := strings.TrimSpace(rest)
		if model == "" {
			return fmt.Errorf("usage: rag embeddings on <installed-model>")
		}
		if err := verifyInstalledLocalModel(ctx, r.deps.Runtime, model); err != nil {
			return err
		}
		r.deps.Config.RAG.Embeddings.Enabled = true
		r.deps.Config.RAG.Embeddings.Provider = "ollama"
		r.deps.Config.RAG.Embeddings.Model = model
		if err := config.Write(r.deps.Config.Path, r.deps.Config); err != nil {
			return err
		}
		fmt.Fprintf(output, "rag embeddings enabled: %s\n", model)
		fmt.Fprintln(output, "run `rag embeddings index` to embed existing chunks")
		return nil
	case "off":
		r.deps.Config.RAG.Embeddings.Enabled = false
		if err := config.Write(r.deps.Config.Path, r.deps.Config); err != nil {
			return err
		}
		fmt.Fprintln(output, "rag embeddings disabled; SQLite FTS5 remains active")
		return nil
	case "index":
		cfg := ragConfig(r.deps.Config.RAG)
		if !cfg.Embeddings.Enabled {
			return fmt.Errorf("embeddings are disabled; run `rag embeddings on <installed-model>` first")
		}
		result, err := r.deps.RAG.EnsureEmbeddings(ctx, cfg, embeddingRuntime(r.deps.Runtime))
		if err != nil {
			return err
		}
		return writeJSON(output, embeddingIndexStatus(result))
	default:
		return fmt.Errorf("usage: rag embeddings status|on <installed-model>|off|index")
	}
}

func (r *Runner) listModels(ctx context.Context, output io.Writer) error {
	models, err := r.deps.Runtime.ListModels(ctx)
	if err != nil {
		return err
	}
	local := localInstalledModels(models)
	if len(local) == 0 {
		fmt.Fprintln(output, "No local Ollama models detected.")
		return nil
	}
	for _, model := range local {
		fmt.Fprintf(output, "%s\t%d MB\n", model.Name, model.Size/1024/1024)
	}
	return nil
}

func (r *Runner) listSkills(output io.Writer) {
	items := r.deps.Skills.List()
	if len(items) == 0 {
		fmt.Fprintln(output, "No skills installed.")
		return
	}
	for _, skill := range items {
		fmt.Fprintf(output, "%s\t%s\t%s\n", skill.Name, skill.Version, skill.Description)
	}
}

func (r *Runner) ingest(ctx context.Context, output io.Writer, path string) error {
	access, err := r.beginWorkspaceAccess(path)
	if err != nil {
		return err
	}
	defer access.Close()
	result, err := r.deps.RAG.IngestPath(ctx, r.deps.Profile.Name, path, ragConfig(r.deps.Config.RAG), workspaceLimits(r.deps.Config.Workspace))
	if err != nil {
		return err
	}
	fmt.Fprintln(output, rag.FormatIngestResult(result))
	return nil
}

func (r *Runner) workspaceGrantStore() safety.WorkspaceGrantStore {
	if r == nil || r.deps.Profile == nil {
		return safety.WorkspaceGrantStore{}
	}
	return safety.NewWorkspaceGrantStore(safety.WorkspaceGrantsPath(r.deps.Profile.Permissions))
}

func (r *Runner) beginWorkspaceAccess(path string) (*safety.WorkspaceAccess, error) {
	root := strings.TrimSpace(r.deps.Workspace)
	if root == "" {
		root = "."
	}
	target := safety.NormalizeUserSuppliedPath(strings.TrimSpace(path))
	if target == "" {
		target = "."
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	return safety.RequireWorkspaceAccessWithBookmark(root, target, r.workspaceGrantStore())
}

func (r *Runner) modelCommand(output io.Writer, rest string) error {
	name, rest := splitCommand(rest)
	switch name {
	case "set":
		parts := strings.Fields(rest)
		if len(parts) < 2 {
			return fmt.Errorf("usage: model set <role> <model> [provider] [base_url]")
		}
		role, modelName := parts[0], parts[1]
		provider, baseURL := "", ""
		if len(parts) > 2 {
			provider = parts[2]
		}
		if len(parts) > 3 {
			baseURL = parts[3]
		}
		if err := r.setModelRole(role, modelName, provider, baseURL); err != nil {
			return err
		}
		fmt.Fprintf(output, "model role updated: %s -> %s\n", role, modelName)
	default:
		return fmt.Errorf("usage: model set <role> <model> [provider] [base_url]")
	}
	return nil
}

func (r *Runner) setModelRole(role string, name string, provider string, baseURL string) error {
	role = strings.TrimSpace(role)
	name = strings.TrimSpace(name)
	provider = strings.TrimSpace(provider)
	baseURL = strings.TrimSpace(baseURL)
	if !knownModelRole(role) {
		return fmt.Errorf("unknown model role %q", role)
	}
	if name == "" {
		return fmt.Errorf("model name is required")
	}
	model := r.deps.Config.Models[role]
	if provider == "" {
		provider = model.Provider
	}
	if provider == "" {
		provider = "ollama"
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(model.BaseURL)
	}
	if baseURL == "" {
		baseURL = modelruntime.DefaultBaseURL(provider)
	}
	model.Provider = provider
	model.BaseURL = baseURL
	model.Name = name
	r.deps.Config.Models[role] = model
	if err := config.Write(r.deps.Config.Path, r.deps.Config); err != nil {
		return err
	}
	r.deps.Router = models.NewRouter(r.deps.Config)
	factory := r.deps.RuntimeFactory
	if factory == nil {
		factory = modelruntime.New
	}
	runtime, err := factory(selectedModel(r.deps.Config))
	if err != nil {
		return err
	}
	r.deps.Runtime = runtime
	return nil
}

func (r *Runner) skillCommand(ctx context.Context, output io.Writer, rest string) error {
	name, rest := splitCommand(rest)
	if name == "" || name == "list" {
		r.listSkills(output)
		return nil
	}
	switch name {
	case "validate":
		skill, ok := r.deps.Skills.Get(strings.TrimSpace(rest))
		if !ok {
			return fmt.Errorf("skill not found: %s", rest)
		}
		if err := skills.ValidateWithOptions(skill, skills.ValidationOptions{AvailableTools: availableSkillTools()}); err != nil {
			return err
		}
		fmt.Fprintf(output, "skill valid: %s\n", skill.Name)
	case "enable", "disable":
		skill, ok := r.deps.Skills.Get(strings.TrimSpace(rest))
		if !ok {
			return fmt.Errorf("skill not found: %s", rest)
		}
		updated, err := skills.SetEnabled(r.deps.Profile.Skills, skill, name == "enable")
		if err != nil {
			return err
		}
		if err := r.reloadSkills(); err != nil {
			return err
		}
		fmt.Fprintf(output, "skill %s: %s\n", name+"d", updated.Name)
	case "create-from-session":
		created, err := r.createSkillFromSession(ctx, strings.TrimSpace(rest))
		if err != nil {
			return err
		}
		if err := r.reloadSkills(); err != nil {
			return err
		}
		return writeJSON(output, created)
	case "improve-from-session":
		skillName, conversationID, ok := splitTwo(rest)
		if !ok {
			return fmt.Errorf("usage: skill improve-from-session <skill> <conversation_id>")
		}
		improved, err := r.improveSkillFromSession(ctx, skillName, conversationID)
		if err != nil {
			return err
		}
		if err := r.reloadSkills(); err != nil {
			return err
		}
		return writeJSON(output, improved)
	case "import":
		imported, err := skills.Import(r.deps.Profile.Skills, strings.TrimSpace(rest))
		if err != nil {
			return err
		}
		if err := r.reloadSkills(); err != nil {
			return err
		}
		return writeJSON(output, imported)
	case "export":
		skillName, path, ok := splitTwo(rest)
		if !ok {
			return fmt.Errorf("usage: skill export <skill> <path>")
		}
		skill, ok := r.deps.Skills.Get(skillName)
		if !ok {
			return fmt.Errorf("skill not found: %s", skillName)
		}
		exported, err := skills.Export(skill, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(output, "skill exported: %s\n", exported)
	default:
		return fmt.Errorf("usage: skills [list] | skill validate|enable|disable <name> | skill create-from-session <conversation_id> | skill improve-from-session <skill> <conversation_id>")
	}
	return nil
}

func (r *Runner) extensionCommand(ctx context.Context, output io.Writer, rest string) error {
	name, rest := splitCommand(rest)
	store := r.extensionStore()
	if name == "" || name == "list" {
		items, err := store.List()
		if err != nil {
			return err
		}
		return writeJSON(output, items)
	}
	switch name {
	case "failures":
		failures, err := store.Failures(20)
		if err != nil {
			return err
		}
		return writeJSON(output, failures)
	case "propose":
		proposal, err := store.Propose(rest)
		if err != nil {
			return err
		}
		return writeJSON(output, proposal)
	case "generate":
		input, err := parseExtensionGenerate(rest, r.deps.Config.Extensions.MaxRuntimeSeconds)
		if err != nil {
			return err
		}
		result, err := store.Generate(ctx, input)
		if err != nil {
			return err
		}
		return writeJSON(output, result)
	case "show":
		detail, err := store.Show(strings.TrimSpace(rest))
		if err != nil {
			return err
		}
		return writeJSON(output, detail)
	case "inspect":
		inspection, err := store.InspectPackage(strings.TrimSpace(rest))
		if err != nil {
			return err
		}
		return writeJSON(output, inspection)
	case "review":
		review, err := store.Review(strings.TrimSpace(rest))
		if err != nil {
			return err
		}
		return writeJSON(output, review)
	case "validate":
		result, err := store.Validate(strings.TrimSpace(rest))
		if err != nil {
			return err
		}
		return writeJSON(output, result)
	case "test":
		result, err := store.Test(ctx, strings.TrimSpace(rest), extensions.TestOptions{})
		if err != nil {
			return err
		}
		return writeJSON(output, result)
	case "register":
		result, err := store.RegisterGenerated(ctx, strings.TrimSpace(rest), extensions.TestOptions{})
		if err != nil {
			return err
		}
		return writeJSON(output, result)
	case "enable", "disable":
		result, err := store.SetEnabled(strings.TrimSpace(rest), name == "enable")
		if err != nil {
			return err
		}
		return writeJSON(output, result)
	case "delete":
		result, err := store.Delete(strings.TrimSpace(rest))
		if err != nil {
			return err
		}
		return writeJSON(output, result)
	case "rollback":
		result, err := store.RollbackGeneration(strings.TrimSpace(rest))
		if err != nil {
			return err
		}
		return writeJSON(output, result)
	case "run":
		extensionName, inputText, ok := splitMaybeTwo(rest)
		if !ok {
			return fmt.Errorf("usage: extension run <name> [input-json]")
		}
		input, err := parseJSONObject(inputText)
		if err != nil {
			return err
		}
		result, err := store.Run(ctx, extensionName, extensions.RunOptions{
			Input:             input,
			MaxRuntimeSeconds: r.deps.Config.Extensions.MaxRuntimeSeconds,
			Internet:          r.internetService(),
		})
		if err != nil {
			return err
		}
		return writeJSON(output, result)
	default:
		return fmt.Errorf("usage: extension list|failures|propose|generate|show|inspect|review|validate|test|register|enable|disable|delete|rollback|run")
	}
}

func (r *Runner) capabilityCommand(ctx context.Context, output io.Writer, rest string) error {
	name, rest := splitCommand(rest)
	switch name {
	case "propose":
		request := strings.TrimSpace(rest)
		if request == "" {
			return fmt.Errorf("usage: capability propose <missing capability>")
		}
		result, err := agent.NewCapabilityGapRouter(r.deps.Config, r.extensionStore()).ProposeRequest(request)
		if err != nil {
			return err
		}
		return writeJSON(output, result)
	case "generate":
		command, err := parseCapabilityGenerate(rest, r.deps.Config.Extensions.MaxRuntimeSeconds)
		if err != nil {
			return err
		}
		input := command.Generation
		if input.SynthesizeDraft {
			draftModel, err := agent.SelectCapabilityDraftModel(r.deps.Config)
			if err != nil {
				return err
			}
			input.DraftModel = draftModel
			input.DraftRuntimeFactory = r.deps.RuntimeFactory
		}
		input.RunOptions = extensions.RunOptions{
			Input:             input.RunInput,
			MaxRuntimeSeconds: r.deps.Config.Extensions.MaxRuntimeSeconds,
			Internet:          r.internetService(),
		}
		store := r.extensionStore()
		result, err := agent.NewCapabilityGapRouter(r.deps.Config, store).Generate(ctx, store, input)
		if err != nil {
			return err
		}
		if command.Schedule.Enabled {
			if err := r.attachGeneratedCapabilityJob(ctx, &result, command.Schedule, input); err != nil {
				return err
			}
		}
		if r.deps.Memory != nil {
			_, _ = r.deps.Memory.SaveMemory(ctx, memory.Memory{
				Kind:       "capability_generation",
				Content:    result.Generation.SkillCandidate,
				Importance: 4,
				Source:     "capability_generator",
			})
		}
		return writeJSON(output, result)
	default:
		return fmt.Errorf("usage: capability propose <missing capability> | generate <missing capability> --yes [--name <name>] [--run --input-json '{}']")
	}
}

func (r *Runner) internetCommand(ctx context.Context, output io.Writer, rest string) error {
	name, rest := splitCommand(rest)
	service := r.internetService()
	switch name {
	case "", "status":
		return writeJSON(output, service.Status())
	case "on":
		r.deps.Config.Internet.Enabled = true
		r.deps.Config.Internet.DefaultMode = "profile_enabled"
		if err := config.Write(r.deps.Config.Path, r.deps.Config); err != nil {
			return err
		}
		fmt.Fprintln(output, "internet enabled for this profile")
	case "off":
		r.deps.Config.Internet.Enabled = false
		if r.deps.Config.Internet.DefaultMode == "profile_enabled" {
			r.deps.Config.Internet.DefaultMode = "ask_each_time"
		}
		if err := config.Write(r.deps.Config.Path, r.deps.Config); err != nil {
			return err
		}
		fmt.Fprintln(output, "internet disabled")
	case "fetch", "head":
		input, err := parseInternetFetch(rest)
		if err != nil {
			return err
		}
		var result internet.FetchResult
		if name == "head" {
			result, err = service.Head(ctx, input)
		} else {
			input.Method = "GET"
			result, err = service.Fetch(ctx, input)
		}
		if err != nil {
			return err
		}
		return writeJSON(output, result)
	case "search":
		input, err := parseInternetSearch(rest)
		if err != nil {
			return err
		}
		result, err := service.Search(ctx, input)
		if err != nil {
			return err
		}
		return writeJSON(output, result)
	case "search-provider", "provider":
		return r.internetSearchProviderCommand(output, rest)
	case "cache":
		items, err := service.CacheList()
		if err != nil {
			return err
		}
		return writeJSON(output, items)
	case "requests":
		limit := parseLimit(rest, 20)
		items, err := service.Requests(limit)
		if err != nil {
			return err
		}
		return writeJSON(output, items)
	default:
		return fmt.Errorf("usage: %s", internetUsage())
	}
	return nil
}

func (r *Runner) internetSearchProviderCommand(output io.Writer, rest string) error {
	name, rest := splitCommand(rest)
	switch name {
	case "", "status":
		return writeJSON(output, internetSearchProviderStatus(r.deps.Config))
	case "off", "disable", "none":
		r.deps.Config.Internet.Search.Enabled = false
		r.deps.Config.Internet.Search.Provider = "none"
		r.deps.Config.Internet.Search.Endpoint = ""
		r.deps.Config.Internet.Search.APIKeyEnv = ""
		if err := config.Write(r.deps.Config.Path, r.deps.Config); err != nil {
			return err
		}
		fmt.Fprintln(output, "internet search provider disabled")
		return nil
	case "auto", "fallback", "auto_fallback":
		_, opts, err := parseSearchProviderRest(rest, false)
		if err != nil {
			return err
		}
		r.deps.Config.Internet.Search.Enabled = true
		r.deps.Config.Internet.Search.Provider = "auto"
		r.deps.Config.Internet.Search.Endpoint = ""
		r.deps.Config.Internet.Search.APIKeyEnv = ""
		applyTUISearchProviderOptions(r.deps.Config, opts)
		return writeTUISearchProviderConfig(r, output, "auto")
	case "searx", "searxng":
		endpoint, opts, err := parseSearchProviderRest(rest, true)
		if err != nil {
			return err
		}
		if endpoint == "" {
			return fmt.Errorf("usage: internet search-provider searxng <endpoint> [--max-results 5] [--enable-internet]")
		}
		if !hasHTTPPrefix(endpoint) {
			return fmt.Errorf("searxng endpoint must start with http:// or https://")
		}
		r.deps.Config.Internet.Search.Enabled = true
		r.deps.Config.Internet.Search.Provider = "searxng"
		r.deps.Config.Internet.Search.Endpoint = endpoint
		r.deps.Config.Internet.Search.APIKeyEnv = ""
		applyTUISearchProviderOptions(r.deps.Config, opts)
		return writeTUISearchProviderConfig(r, output, "searxng")
	case "tavily", "serper", "firecrawl", "mojeek":
		apiKeyEnv, opts, err := parseSearchProviderRest(rest, false)
		if err != nil {
			return err
		}
		if apiKeyEnv == "" {
			return fmt.Errorf("usage: internet search-provider %s <api-key-env> [--endpoint <url>] [--max-results 5] [--enable-internet]", name)
		}
		if opts.endpoint != "" && !hasHTTPPrefix(opts.endpoint) {
			return fmt.Errorf("%s endpoint must start with http:// or https://", name)
		}
		r.deps.Config.Internet.Search.Enabled = true
		r.deps.Config.Internet.Search.Provider = name
		r.deps.Config.Internet.Search.APIKeyEnv = apiKeyEnv
		r.deps.Config.Internet.Search.Endpoint = opts.endpoint
		applyTUISearchProviderOptions(r.deps.Config, opts)
		return writeTUISearchProviderConfig(r, output, name)
	case "brave", "brave_search":
		apiKeyEnv, opts, err := parseSearchProviderRest(rest, false)
		if err != nil {
			return err
		}
		if apiKeyEnv == "" {
			return fmt.Errorf("usage: internet search-provider brave <api-key-env> [--endpoint <url>] [--max-results 5] [--enable-internet]")
		}
		if opts.endpoint != "" && !hasHTTPPrefix(opts.endpoint) {
			return fmt.Errorf("brave endpoint must start with http:// or https://")
		}
		r.deps.Config.Internet.Search.Enabled = true
		r.deps.Config.Internet.Search.Provider = "brave"
		r.deps.Config.Internet.Search.APIKeyEnv = apiKeyEnv
		r.deps.Config.Internet.Search.Endpoint = opts.endpoint
		applyTUISearchProviderOptions(r.deps.Config, opts)
		return writeTUISearchProviderConfig(r, output, "brave")
	case "wikimedia", "wikipedia", "duckduckgo", "ddg":
		provider := name
		if provider == "wikipedia" {
			provider = "wikimedia"
		}
		if provider == "ddg" {
			provider = "duckduckgo"
		}
		_, opts, err := parseSearchProviderRest(rest, false)
		if err != nil {
			return err
		}
		if opts.endpoint != "" && !hasHTTPPrefix(opts.endpoint) {
			return fmt.Errorf("%s endpoint must start with http:// or https://", provider)
		}
		r.deps.Config.Internet.Search.Enabled = true
		r.deps.Config.Internet.Search.Provider = provider
		r.deps.Config.Internet.Search.APIKeyEnv = ""
		r.deps.Config.Internet.Search.Endpoint = opts.endpoint
		applyTUISearchProviderOptions(r.deps.Config, opts)
		return writeTUISearchProviderConfig(r, output, provider)
	case "custom", "custom_search", "yemaka_custom", "yemaka_custom_search":
		endpoint, opts, err := parseSearchProviderRest(rest, true)
		if err != nil {
			return err
		}
		if endpoint == "" {
			return fmt.Errorf("usage: internet search-provider custom <endpoint> [--max-results 5] [--enable-internet]")
		}
		if !hasHTTPPrefix(endpoint) {
			return fmt.Errorf("custom search endpoint must start with http:// or https://")
		}
		r.deps.Config.Internet.Search.Enabled = true
		r.deps.Config.Internet.Search.Provider = "yemaka_custom"
		r.deps.Config.Internet.Search.Endpoint = endpoint
		r.deps.Config.Internet.Search.APIKeyEnv = ""
		applyTUISearchProviderOptions(r.deps.Config, opts)
		return writeTUISearchProviderConfig(r, output, "yemaka_custom")
	default:
		return fmt.Errorf("usage: %s", internetSearchProviderUsage())
	}
}

func (r *Runner) jobCommand(ctx context.Context, output io.Writer, rest string) error {
	name, rest := splitCommand(rest)
	store, err := r.schedulerStore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()
	switch name {
	case "", "list":
		jobs, err := store.List(ctx)
		if err != nil {
			return err
		}
		jobs, _ = r.annotateSchedulerJobs(jobs)
		return writeJSON(output, jobs)
	case "status":
		status, err := store.Status(ctx, r.deps.Config.Scheduler.Enabled, r.schedulerMaxParallel())
		if err != nil {
			return err
		}
		if jobs, listErr := store.List(ctx); listErr == nil {
			_, invalid := r.annotateSchedulerJobs(jobs)
			status.InvalidInputJobs = invalid
		}
		return writeJSON(output, status)
	case "runs":
		runs, err := store.LastRuns(ctx, parseLimit(rest, 20))
		if err != nil {
			return err
		}
		return writeJSON(output, runs)
	case "run":
		id := strings.TrimSpace(rest)
		job, jobErr := store.Get(ctx, id)
		if jobErr == nil && job.TargetType == scheduler.TargetExtension {
			if err := r.extensionStore().ValidateKnownRunInput(job.TargetName, job.Input); err != nil {
				return err
			}
		}
		run, err := store.Run(ctx, id, r.jobRunner(), r.jobRunOptions())
		if jobErr == nil {
			if _, recordErr := agent.RecordJobRunResult(ctx, r.deps.Memory, job, run, r.activeConversationID); recordErr != nil && err == nil {
				return recordErr
			}
		}
		if err != nil {
			return err
		}
		return writeJSON(output, run)
	case "update-input":
		input, err := parseJobUpdateInput(rest)
		if err != nil {
			return err
		}
		existing, err := store.Get(ctx, input.ID)
		if err != nil {
			return err
		}
		if existing.TargetType == scheduler.TargetExtension {
			if err := r.extensionStore().ValidateKnownRunInput(existing.TargetName, input.Input); err != nil {
				return err
			}
		}
		job, err := store.UpdateInput(ctx, input)
		if err != nil {
			return err
		}
		annotated, _ := r.annotateSchedulerJobs([]scheduler.Job{job})
		if len(annotated) == 1 {
			job = annotated[0]
		}
		return writeJSON(output, job)
	case "enable", "disable":
		if name == "enable" {
			existing, err := store.Get(ctx, strings.TrimSpace(rest))
			if err != nil {
				return err
			}
			if existing.TargetType == scheduler.TargetExtension {
				if err := r.extensionStore().ValidateKnownRunInput(existing.TargetName, existing.Input); err != nil {
					return err
				}
			}
		}
		job, err := store.SetEnabled(ctx, strings.TrimSpace(rest), name == "enable")
		if err != nil {
			return err
		}
		return writeJSON(output, job)
	case "create":
		input, err := parseJobCreate(rest)
		if err != nil {
			return err
		}
		if input.TargetType == scheduler.TargetExtension {
			if err := r.extensionStore().ValidateKnownRunInput(input.TargetName, input.Input); err != nil {
				return err
			}
		}
		job, err := store.Create(ctx, input)
		if err != nil {
			return err
		}
		return writeJSON(output, job)
	default:
		return fmt.Errorf("usage: job list|status|runs [limit]|run <id>|enable <id>|disable <id>|update-input <id> --input-json '{}' --yes|create <manual|interval|cron|one_time> <target> --yes")
	}
}

func (r *Runner) heartbeatCommand(ctx context.Context, output io.Writer, rest string) error {
	name, _ := splitCommand(rest)
	if name != "" && name != "status" {
		return fmt.Errorf("usage: heartbeat status")
	}
	report, err := r.heartbeatReport(ctx)
	if err != nil {
		return err
	}
	store, err := heartbeat.Open(ctx, r.deps.Profile.Database)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Record(ctx, report); err != nil {
		return err
	}
	return writeJSON(output, report)
}

func (r *Runner) connectorCommand(output io.Writer, rest string) error {
	name, rest := splitCommand(rest)
	switch name {
	case "", "list":
		return writeJSON(output, connectors.List(r.deps.Config.Connectors))
	case "registry":
		return writeJSON(output, connectors.Registry(r.deps.Config.Connectors))
	case "show":
		for _, entry := range connectors.Registry(r.deps.Config.Connectors) {
			if entry.Status.Name == strings.TrimSpace(rest) {
				return writeJSON(output, entry)
			}
		}
		return fmt.Errorf("connector not found: %s", rest)
	case "enable":
		connectorName, tokenEnv := parseConnectorEnable(rest)
		if connectorName == "" {
			return fmt.Errorf("usage: connector enable <name> [--token-env ENV]")
		}
		if err := enableConnector(r.deps.Config, connectorName, tokenEnv); err != nil {
			return err
		}
		if err := config.Write(r.deps.Config.Path, r.deps.Config); err != nil {
			return err
		}
		fmt.Fprintf(output, "connector enabled: %s\n", connectorName)
	case "disable":
		connectorName := strings.TrimSpace(rest)
		if connectorName == "" {
			return fmt.Errorf("usage: connector disable <name>")
		}
		if err := disableConnector(r.deps.Config, connectorName); err != nil {
			return err
		}
		if err := config.Write(r.deps.Config.Path, r.deps.Config); err != nil {
			return err
		}
		fmt.Fprintf(output, "connector disabled: %s\n", connectorName)
	default:
		return fmt.Errorf("usage: connector list|registry|show <name>|enable <name> [--token-env ENV]|disable <name>")
	}
	return nil
}

func (r *Runner) policyCommand(output io.Writer, rest string) error {
	name, rest := splitCommand(rest)
	if name == "" || name == "status" {
		return writeJSON(output, policyStatus(r.deps.Config))
	}
	if name != "mode" {
		return fmt.Errorf("usage: policy status | policy mode safe | policy mode full_access")
	}
	if err := setPolicyMode(r.deps.Config, rest); err != nil {
		return err
	}
	if err := config.Write(r.deps.Config.Path, r.deps.Config); err != nil {
		return err
	}
	return writeJSON(output, policyStatus(r.deps.Config))
}

func (r *Runner) learnCommand(ctx context.Context, output io.Writer, rest string) error {
	name, rest := splitCommand(rest)
	switch name {
	case "report":
		report, err := learning.BuildReport(ctx, r.deps.Memory, r.extensionStore())
		if err != nil {
			return err
		}
		return writeJSON(output, report)
	case "export":
		conversationID := parseConversationID(rest)
		trajectory, err := learning.ExportTrajectory(ctx, r.deps.Memory, conversationID)
		if err != nil {
			return err
		}
		return writeJSON(output, trajectory)
	case "correction":
		conversationID, correction := parseCorrection(rest)
		item, err := learning.SaveCorrection(ctx, r.deps.Memory, conversationID, correction)
		if err != nil {
			return err
		}
		return writeJSON(output, item)
	default:
		return fmt.Errorf("usage: learn report | learn export <conversation_id> | learn correction <conversation_id> <text>")
	}
}

func (r *Runner) toolRuns(ctx context.Context, output io.Writer) error {
	runs, err := r.deps.Memory.ListToolRuns(ctx, 20)
	if err != nil {
		return err
	}
	return writeJSON(output, runs)
}

func printHelp(output io.Writer) {
	fmt.Fprintln(output, "Commands:")
	fmt.Fprintln(output, "  status              show local readiness")
	fmt.Fprintln(output, "  chat <message>      plain local model chat")
	fmt.Fprintln(output, "  ask <request>       answer with docs/workspace context")
	fmt.Fprintln(output, "  memory <query>      search SQLite message memory")
	fmt.Fprintln(output, "  docs <query>        search SQLite FTS5 RAG chunks")
	fmt.Fprintln(output, "  rag embeddings ...  status/on/off/index optional local embeddings")
	fmt.Fprintln(output, "  models              list installed local Ollama models")
	fmt.Fprintln(output, "  model set <role> <model> [provider] [base_url]")
	fmt.Fprintln(output, "  conversation ...    current/new/list/use chat sessions")
	fmt.Fprintln(output, "  new chat            reset active conversation")
	fmt.Fprintln(output, "  skills              manage local reusable skills")
	fmt.Fprintln(output, "  capability propose  inspect a missing capability route")
	fmt.Fprintln(output, "  capability generate approve, create, and optionally --run a tested extension")
	fmt.Fprintln(output, "  extension ...       manage generated extensions")
	fmt.Fprintln(output, "  internet ...        controlled internet service")
	fmt.Fprintln(output, "  internet search-provider auto|searxng|tavily|serper|brave|firecrawl|wikimedia|duckduckgo|mojeek|custom ...")
	fmt.Fprintln(output, "  internet search <query> [--yes]")
	fmt.Fprintln(output, "  job ...             scheduler jobs")
	fmt.Fprintln(output, "  heartbeat status    core health report")
	fmt.Fprintln(output, "  connector ...       connector registry")
	fmt.Fprintln(output, "  policy ...          policy mode/status")
	fmt.Fprintln(output, "  learn ...           learning report/export/correction")
	fmt.Fprintln(output, "  tools               recent tool runs")
	fmt.Fprintln(output, "  ingest <path>       ingest local docs/code")
	fmt.Fprintln(output, "  help                show commands")
	fmt.Fprintln(output, "  quit or /quit       exit")
}

func splitCommand(input string) (string, string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", ""
	}
	name, rest, found := strings.Cut(input, " ")
	if !found {
		return strings.ToLower(name), ""
	}
	return strings.ToLower(strings.TrimSpace(name)), strings.TrimSpace(rest)
}

func quit(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "/q", "quit", "/quit", "exit", "/exit":
		return true
	default:
		return false
	}
}

func workspaceLimits(cfg config.WorkspaceConfig) workspace.Limits {
	return workspace.Limits{
		MaxFilesScanned:   cfg.MaxFilesScanned,
		MaxFileBytes:      cfg.MaxFileBytes,
		MaxTotalScanBytes: cfg.MaxTotalScanBytes,
		MaxSearchResults:  cfg.MaxSearchResults,
		MaxContextFiles:   cfg.MaxContextFiles,
		MaxContextChars:   cfg.MaxContextChars,
		IncludeHidden:     cfg.IncludeHidden,
		FollowSymlinks:    cfg.FollowSymlinks,
	}
}

func ragConfig(cfg config.RAGConfig) rag.Config {
	return rag.Config{
		Enabled:          cfg.Enabled,
		Mode:             cfg.Mode,
		ChunkSize:        cfg.ChunkSize,
		ChunkOverlap:     cfg.ChunkOverlap,
		TopK:             cfg.TopK,
		MaxFileBytes:     cfg.MaxFileBytes,
		MaxTotalBytes:    cfg.MaxTotalBytes,
		MaxChunksPerFile: cfg.MaxChunksPerFile,
		Embeddings: rag.EmbeddingConfig{
			Enabled:        cfg.Embeddings.Enabled,
			Provider:       cfg.Embeddings.Provider,
			Model:          cfg.Embeddings.Model,
			BatchSize:      cfg.Embeddings.BatchSize,
			MaxTextChars:   cfg.Embeddings.MaxTextChars,
			CandidateLimit: cfg.Embeddings.CandidateLimit,
		},
	}
}

func embeddingRuntime(runtime models.Runtime) models.EmbeddingRuntime {
	embedder, ok := runtime.(models.EmbeddingRuntime)
	if !ok {
		return nil
	}
	return embedder
}

func tuiCloudFallback(cfg *config.Config, runtime models.Runtime) *agent.CloudFallback {
	if cfg == nil || !cfg.CloudFallback.Enabled || runtime == nil {
		return nil
	}
	return &agent.CloudFallback{
		Config:  cfg.CloudFallback,
		Runtime: runtime,
	}
}

func tuiToolExecutor(deps Dependencies) agent.ToolExecutor {
	cfg := deps.Config
	if cfg == nil {
		return nil
	}
	internetService := internet.New(cfg.Internet, deps.Profile.Root, deps.Profile.Logs)
	internetService.PolicyMode = policyMode(cfg)
	return agent.NewSafeToolExecutor(agent.SafeToolConfig{
		WorkspaceRoot:            deps.Workspace,
		TimeoutSeconds:           cfg.Tools.Shell.TimeoutSeconds,
		MaxOutputBytes:           cfg.Tools.Shell.MaxOutputBytes,
		MaxContextChars:          cfg.Workspace.MaxContextChars,
		RequireConfirmationRisky: cfg.Tools.Shell.RequireConfirmationRisky,
		FullAccess:               safety.IsFullAccessMode(cfg.Security.Policy.Mode),
		DisableShellCommands:     !cfg.Tools.Shell.Enabled,
		Config:                   cfg,
		Profile:                  deps.Profile,
		Memory:                   deps.Memory,
		RAG:                      deps.RAG,
		Runtime:                  deps.Runtime,
		Skills:                   deps.Skills,
		Internet:                 internetService,
		UseShellHelper:           true,
		ShellHelperPath:          tools.DefaultShellHelperPath(),
	})
}

func tuiRetriever(deps Dependencies) agent.Retriever {
	if deps.Config == nil {
		return nil
	}
	return agent.NewLocalRetriever(agent.LocalRetrieverConfig{
		Config:        deps.Config,
		WorkspaceRoot: deps.Workspace,
		RAG:           deps.RAG,
		Runtime:       deps.Runtime,
	})
}

func (r *Runner) reloadSkills() error {
	defaultSkillsDir, err := skills.DefaultDir()
	if err != nil {
		return err
	}
	registry, err := skills.LoadRegistry([]string{defaultSkillsDir, r.deps.Profile.Skills})
	if err != nil {
		return err
	}
	r.deps.Skills = registry
	return nil
}

func (r *Runner) createSkillFromSession(ctx context.Context, conversationID string) (skills.Skill, error) {
	messages, err := r.deps.Memory.ListConversationMessages(ctx, strings.TrimSpace(conversationID), r.deps.Config.Memory.SummarizeAfter)
	if err != nil {
		return skills.Skill{}, err
	}
	toolRuns, err := r.deps.Memory.ListToolRunsForConversation(ctx, strings.TrimSpace(conversationID), 50)
	if err != nil {
		return skills.Skill{}, err
	}
	transcript := make([]skills.TranscriptMessage, 0, len(messages))
	for _, msg := range messages {
		transcript = append(transcript, skills.TranscriptMessage{Role: msg.Role, Content: msg.Content})
	}
	runs := make([]skills.ToolRunSummary, 0, len(toolRuns))
	for _, run := range toolRuns {
		runs = append(runs, skills.ToolRunSummary{ToolName: run.ToolName, Status: run.Status})
	}
	return skills.CreateFromSession(skills.GenerateInput{
		ProfileSkillsDir: r.deps.Profile.Skills,
		ConversationID:   strings.TrimSpace(conversationID),
		Messages:         transcript,
		ToolRuns:         runs,
	})
}

func (r *Runner) improveSkillFromSession(ctx context.Context, name string, conversationID string) (skills.Skill, error) {
	skill, ok := r.deps.Skills.Get(strings.TrimSpace(name))
	if !ok {
		return skills.Skill{}, fmt.Errorf("skill not found: %s", name)
	}
	messages, err := r.deps.Memory.ListConversationMessages(ctx, strings.TrimSpace(conversationID), r.deps.Config.Memory.SummarizeAfter)
	if err != nil {
		return skills.Skill{}, err
	}
	toolRuns, err := r.deps.Memory.ListToolRunsForConversation(ctx, strings.TrimSpace(conversationID), 50)
	if err != nil {
		return skills.Skill{}, err
	}
	transcript := make([]skills.TranscriptMessage, 0, len(messages))
	for _, msg := range messages {
		transcript = append(transcript, skills.TranscriptMessage{Role: msg.Role, Content: msg.Content})
	}
	runs := make([]skills.ToolRunSummary, 0, len(toolRuns))
	for _, run := range toolRuns {
		runs = append(runs, skills.ToolRunSummary{ToolName: run.ToolName, Status: run.Status})
	}
	return skills.ImproveFromSession(skills.ImproveInput{
		ProfileSkillsDir: r.deps.Profile.Skills,
		ConversationID:   strings.TrimSpace(conversationID),
		Skill:            skill,
		Messages:         transcript,
		ToolRuns:         runs,
	})
}

func (r *Runner) extensionStore() *extensions.Store {
	store := extensions.NewStore(r.deps.Profile.GeneratedExtensions, r.deps.Profile.Logs)
	store.PolicyMode = policyMode(r.deps.Config)
	return store
}

func tuiReplayStore(deps Dependencies) agent.ReplaySaver {
	if deps.Profile == nil || strings.TrimSpace(deps.Profile.Root) == "" {
		return nil
	}
	return replay.NewStore(filepath.Join(deps.Profile.Root, "replay"))
}

func (r *Runner) internetService() *internet.Service {
	service := internet.New(r.deps.Config.Internet, r.deps.Profile.Root, r.deps.Profile.Logs)
	service.PolicyMode = policyMode(r.deps.Config)
	return service
}

func (r *Runner) schedulerStore(ctx context.Context) (*scheduler.Store, error) {
	return scheduler.Open(ctx, r.deps.Profile.Database)
}

func (r *Runner) jobRunner() scheduler.Runner {
	return scheduler.RunnerFunc(func(ctx context.Context, job scheduler.Job) (any, error) {
		switch job.TargetType {
		case scheduler.TargetExtension:
			if err := r.extensionStore().ValidateRunInput(job.TargetName, job.Input); err != nil {
				return nil, err
			}
			result, err := r.extensionStore().Run(ctx, job.TargetName, extensions.RunOptions{
				Input:             job.Input,
				MaxRuntimeSeconds: r.deps.Config.Scheduler.DefaultJobTimeoutSeconds,
				Internet:          r.internetService(),
			})
			if err != nil {
				return result, err
			}
			return result, nil
		case scheduler.TargetHeartbeat:
			report, err := r.heartbeatReport(ctx)
			if err != nil {
				return report, err
			}
			return report, nil
		default:
			return nil, fmt.Errorf("unsupported job target type: %s", job.TargetType)
		}
	})
}

func (r *Runner) annotateSchedulerJobs(jobs []scheduler.Job) ([]scheduler.Job, int) {
	return scheduler.AnnotateJobInputStatuses(jobs, func(job scheduler.Job) (bool, error) {
		if strings.TrimSpace(job.TargetType) != scheduler.TargetExtension {
			return false, nil
		}
		err := r.extensionStore().ValidateRunInput(job.TargetName, job.Input)
		if errors.Is(err, extensions.ErrExtensionNotFound) {
			return false, nil
		}
		return true, err
	})
}

func (r *Runner) heartbeatReport(ctx context.Context) (heartbeat.Report, error) {
	store, err := r.schedulerStore(ctx)
	if err != nil {
		return heartbeat.Report{}, err
	}
	defer store.Close()
	status, err := store.Status(ctx, r.deps.Config.Scheduler.Enabled, r.schedulerMaxParallel())
	if err != nil {
		return heartbeat.Report{}, err
	}
	return heartbeat.Run(ctx, heartbeat.Input{
		Config:          r.deps.Config,
		Profile:         r.deps.Profile,
		Memory:          r.deps.Memory,
		Runtime:         r.deps.Runtime,
		SchedulerStatus: &status,
		ExtensionStore:  r.extensionStore(),
	}), nil
}

func (r *Runner) jobRunOptions() scheduler.RunOptions {
	return scheduler.RunOptions{
		Enabled:        r.deps.Config.Scheduler.Enabled,
		TimeoutSeconds: r.deps.Config.Scheduler.DefaultJobTimeoutSeconds,
		MaxParallel:    r.schedulerMaxParallel(),
		LowMemoryMode:  r.deps.Config.Runtime.LowMemoryMode,
		PolicyMode:     policyMode(r.deps.Config),
	}
}

func (r *Runner) schedulerMaxParallel() int {
	maxParallel := r.deps.Config.Scheduler.MaxParallelJobs
	if r.deps.Config.Runtime.LowMemoryMode && r.deps.Config.Scheduler.LowMemoryMaxParallelJobs > 0 {
		maxParallel = r.deps.Config.Scheduler.LowMemoryMaxParallelJobs
	}
	if maxParallel <= 0 {
		return 1
	}
	return maxParallel
}

func selectedModel(cfg *config.Config) config.ModelConfig {
	selected := cfg.Models["default"]
	if cfg.Runtime.LowMemoryMode {
		if low, ok := cfg.Models["low_memory"]; ok {
			selected = low
		}
	}
	return selected
}

func knownModelRole(role string) bool {
	switch role {
	case "default", "low_memory", "coding", "reasoning", "stronger_local":
		return true
	default:
		return false
	}
}

func verifyInstalledLocalModel(ctx context.Context, runtime models.Runtime, name string) error {
	installed, err := runtime.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("verify installed local model: %w", err)
	}
	for _, model := range localInstalledModels(installed) {
		if model.Name == name {
			return nil
		}
	}
	return fmt.Errorf("model is not installed locally: %s", name)
}

func localInstalledModels(installed []models.ModelInfo) []models.ModelInfo {
	return models.LocalInstalledModels(installed)
}

func embeddingStatus(cfg config.EmbeddingConfig) map[string]any {
	return map[string]any{
		"enabled":         cfg.Enabled,
		"provider":        cfg.Provider,
		"model":           cfg.Model,
		"batch_size":      cfg.BatchSize,
		"max_text_chars":  cfg.MaxTextChars,
		"candidate_limit": cfg.CandidateLimit,
	}
}

func embeddingIndexStatus(result rag.EmbeddingIndexResult) map[string]any {
	return map[string]any{
		"enabled":         result.Enabled,
		"provider":        result.Provider,
		"model":           result.Model,
		"chunks_embedded": result.ChunksEmbedded,
		"chunks_skipped":  result.ChunksSkipped,
		"dimensions":      result.Dimensions,
	}
}

func availableSkillTools() map[string]bool {
	return tools.ToolAvailabilityForSurface(tools.ToolSurfaceSkill)
}

func oneLine(input string) string {
	input = strings.Join(strings.Fields(input), " ")
	if len(input) > 180 {
		return input[:180] + "..."
	}
	return input
}

func writeJSON(output io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(output, string(data))
	return nil
}

func splitTwo(input string) (string, string, bool) {
	first, rest, ok := splitMaybeTwo(input)
	return first, rest, ok && rest != ""
}

func splitMaybeTwo(input string) (string, string, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", false
	}
	first, rest, found := strings.Cut(input, " ")
	if !found {
		return first, "", true
	}
	return strings.TrimSpace(first), strings.TrimSpace(rest), true
}

func parseJSONObject(input string) (map[string]any, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return map[string]any{}, nil
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(input), &value); err != nil {
		return nil, fmt.Errorf("input json must be an object: %w", err)
	}
	if value == nil {
		value = map[string]any{}
	}
	return value, nil
}

func consumeJSONArgument(parts []string, start int) (string, int, error) {
	if start >= len(parts) {
		return "", start, fmt.Errorf("JSON object is required")
	}
	value := parts[start]
	depth := jsonBraceDepth(value)
	if depth <= 0 {
		return value, start, nil
	}
	for index := start + 1; index < len(parts); index++ {
		value += " " + parts[index]
		depth = jsonBraceDepth(value)
		if depth <= 0 {
			return value, index, nil
		}
	}
	return value, len(parts) - 1, fmt.Errorf("unterminated JSON object")
}

func jsonBraceDepth(value string) int {
	depth := 0
	inString := false
	escaped := false
	for _, char := range value {
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && inString {
			escaped = true
			continue
		}
		if char == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch char {
		case '{':
			depth++
		case '}':
			depth--
		}
	}
	return depth
}

func parseExtensionGenerate(input string, maxRuntimeSeconds int) (extensions.GenerateInput, error) {
	parts := strings.Fields(input)
	if len(parts) < 3 {
		return extensions.GenerateInput{}, fmt.Errorf("usage: extension generate <name> <description> --yes [--brokered-network] [--allow-domain <domain>]")
	}
	approved := false
	brokeredNetwork := false
	allowedDomains := []string{}
	filtered := make([]string, 0, len(parts))
	for index := 0; index < len(parts); index++ {
		part := parts[index]
		if part == "--yes" {
			approved = true
			continue
		}
		if part == "--brokered-network" || part == "--core-broker-network" {
			brokeredNetwork = true
			continue
		}
		if part == "--allow-domain" || part == "--domain" {
			if index+1 >= len(parts) {
				return extensions.GenerateInput{}, fmt.Errorf("%s requires a domain", part)
			}
			index++
			allowedDomains = append(allowedDomains, parts[index])
			continue
		}
		filtered = append(filtered, part)
	}
	if len(filtered) < 2 {
		return extensions.GenerateInput{}, fmt.Errorf("extension description is required")
	}
	if !approved {
		return extensions.GenerateInput{}, fmt.Errorf("use --yes to approve local extension file generation")
	}
	return extensions.GenerateInput{
		Name:              filtered[0],
		Description:       strings.Join(filtered[1:], " "),
		Approved:          true,
		MaxRuntimeSeconds: maxRuntimeSeconds,
		BrokeredNetwork:   brokeredNetwork,
		AllowedDomains:    allowedDomains,
	}, nil
}

type capabilityGenerateCommand struct {
	Generation agent.CapabilityGenerationInput
	Schedule   capabilityGenerateSchedule
}

type capabilityGenerateSchedule struct {
	Enabled      bool
	Name         string
	ScheduleType string
	ScheduleExpr string
	EnableJob    bool
	Input        map[string]any
}

func parseCapabilityGenerate(input string, maxRuntimeSeconds int) (capabilityGenerateCommand, error) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return capabilityGenerateCommand{}, fmt.Errorf("usage: capability generate <missing capability> --yes [--name <name>] [--synthesize] [--repair-attempts <n>] [--run --input-json '{}'] [--job --every 1h]")
	}
	approved := false
	name := ""
	runAfterGenerate := false
	runInput := map[string]any(nil)
	synthesizeDraft := false
	draftRepairAttempts := 0
	schedule := capabilityGenerateSchedule{}
	requestParts := make([]string, 0, len(parts))
	for index := 0; index < len(parts); index++ {
		part := parts[index]
		switch part {
		case "--yes", "--approved":
			approved = true
		case "--run":
			runAfterGenerate = true
		case "--synthesize", "--synthesize-draft":
			synthesizeDraft = true
		case "--no-repair", "--no-draft-repair":
			draftRepairAttempts = -1
		case "--job", "--schedule-job":
			schedule.Enabled = true
		case "--enable-job":
			schedule.Enabled = true
			schedule.EnableJob = true
		case "--name":
			if index+1 >= len(parts) {
				return capabilityGenerateCommand{}, fmt.Errorf("--name requires a value")
			}
			index++
			name = parts[index]
		case "--repair-attempts", "--draft-repair-attempts":
			if index+1 >= len(parts) {
				return capabilityGenerateCommand{}, fmt.Errorf("%s requires a number", part)
			}
			value, err := strconv.Atoi(parts[index+1])
			if err != nil {
				return capabilityGenerateCommand{}, fmt.Errorf("%s must be a number", part)
			}
			index++
			draftRepairAttempts = value
		case "--job-name", "--schedule-name":
			if index+1 >= len(parts) {
				return capabilityGenerateCommand{}, fmt.Errorf("%s requires a value", part)
			}
			schedule.Enabled = true
			index++
			schedule.Name = parts[index]
		case "--schedule", "--schedule-type":
			if index+1 >= len(parts) {
				return capabilityGenerateCommand{}, fmt.Errorf("%s requires a value", part)
			}
			schedule.Enabled = true
			index++
			schedule.ScheduleType = parts[index]
		case "--every":
			if index+1 >= len(parts) {
				return capabilityGenerateCommand{}, fmt.Errorf("--every requires a duration")
			}
			schedule.Enabled = true
			schedule.ScheduleType = scheduler.ScheduleInterval
			index++
			schedule.ScheduleExpr = parts[index]
		case "--cron":
			if index+1 >= len(parts) {
				return capabilityGenerateCommand{}, fmt.Errorf("--cron requires a cron expression")
			}
			schedule.Enabled = true
			schedule.ScheduleType = scheduler.ScheduleCron
			index++
			schedule.ScheduleExpr = parts[index]
		case "--at":
			if index+1 >= len(parts) {
				return capabilityGenerateCommand{}, fmt.Errorf("--at requires an RFC3339 timestamp")
			}
			schedule.Enabled = true
			schedule.ScheduleType = scheduler.ScheduleOneTime
			index++
			schedule.ScheduleExpr = parts[index]
		case "--input-json":
			if index+1 >= len(parts) {
				return capabilityGenerateCommand{}, fmt.Errorf("--input-json requires a JSON object")
			}
			raw, consumed, err := consumeJSONArgument(parts, index+1)
			if err != nil {
				return capabilityGenerateCommand{}, fmt.Errorf("input-json must be a JSON object: %w", err)
			}
			parsed, err := parseJSONObject(raw)
			if err != nil {
				return capabilityGenerateCommand{}, fmt.Errorf("input-json must be a JSON object: %w", err)
			}
			runInput = parsed
			index = consumed
		case "--job-input-json":
			if index+1 >= len(parts) {
				return capabilityGenerateCommand{}, fmt.Errorf("--job-input-json requires a JSON object")
			}
			raw, consumed, err := consumeJSONArgument(parts, index+1)
			if err != nil {
				return capabilityGenerateCommand{}, fmt.Errorf("job-input-json must be a JSON object: %w", err)
			}
			parsed, err := parseJSONObject(raw)
			if err != nil {
				return capabilityGenerateCommand{}, fmt.Errorf("job-input-json must be a JSON object: %w", err)
			}
			schedule.Enabled = true
			schedule.Input = parsed
			index = consumed
		default:
			if strings.HasPrefix(part, "--input-json=") {
				parsed, err := parseJSONObject(strings.TrimPrefix(part, "--input-json="))
				if err != nil {
					return capabilityGenerateCommand{}, fmt.Errorf("input-json must be a JSON object: %w", err)
				}
				runInput = parsed
			} else if strings.HasPrefix(part, "--job-input-json=") {
				parsed, err := parseJSONObject(strings.TrimPrefix(part, "--job-input-json="))
				if err != nil {
					return capabilityGenerateCommand{}, fmt.Errorf("job-input-json must be a JSON object: %w", err)
				}
				schedule.Enabled = true
				schedule.Input = parsed
			} else if strings.HasPrefix(part, "--repair-attempts=") {
				value, err := strconv.Atoi(strings.TrimPrefix(part, "--repair-attempts="))
				if err != nil {
					return capabilityGenerateCommand{}, fmt.Errorf("--repair-attempts must be a number")
				}
				draftRepairAttempts = value
			} else if strings.HasPrefix(part, "--draft-repair-attempts=") {
				value, err := strconv.Atoi(strings.TrimPrefix(part, "--draft-repair-attempts="))
				if err != nil {
					return capabilityGenerateCommand{}, fmt.Errorf("--draft-repair-attempts must be a number")
				}
				draftRepairAttempts = value
			} else if strings.HasPrefix(part, "--job-name=") {
				schedule.Enabled = true
				schedule.Name = strings.TrimPrefix(part, "--job-name=")
			} else if strings.HasPrefix(part, "--schedule-name=") {
				schedule.Enabled = true
				schedule.Name = strings.TrimPrefix(part, "--schedule-name=")
			} else if strings.HasPrefix(part, "--schedule=") {
				schedule.Enabled = true
				schedule.ScheduleType = strings.TrimPrefix(part, "--schedule=")
			} else if strings.HasPrefix(part, "--schedule-type=") {
				schedule.Enabled = true
				schedule.ScheduleType = strings.TrimPrefix(part, "--schedule-type=")
			} else if strings.HasPrefix(part, "--every=") {
				schedule.Enabled = true
				schedule.ScheduleType = scheduler.ScheduleInterval
				schedule.ScheduleExpr = strings.TrimPrefix(part, "--every=")
			} else if strings.HasPrefix(part, "--cron=") {
				schedule.Enabled = true
				schedule.ScheduleType = scheduler.ScheduleCron
				schedule.ScheduleExpr = strings.TrimPrefix(part, "--cron=")
			} else if strings.HasPrefix(part, "--at=") {
				schedule.Enabled = true
				schedule.ScheduleType = scheduler.ScheduleOneTime
				schedule.ScheduleExpr = strings.TrimPrefix(part, "--at=")
			} else {
				requestParts = append(requestParts, part)
			}
		}
	}
	request := strings.TrimSpace(strings.Join(requestParts, " "))
	if request == "" {
		return capabilityGenerateCommand{}, fmt.Errorf("capability request is required")
	}
	if !approved {
		return capabilityGenerateCommand{}, fmt.Errorf("use --yes to approve local capability generation")
	}
	if schedule.Enabled && strings.TrimSpace(schedule.ScheduleType) == "" {
		schedule.ScheduleType = scheduler.ScheduleManual
	}
	return capabilityGenerateCommand{
		Generation: agent.CapabilityGenerationInput{
			Request:             request,
			Approved:            true,
			Name:                name,
			MaxRuntimeSeconds:   maxRuntimeSeconds,
			RunAfterGenerate:    runAfterGenerate,
			RunInput:            runInput,
			SynthesizeDraft:     synthesizeDraft,
			DraftRepairAttempts: draftRepairAttempts,
		},
		Schedule: schedule,
	}, nil
}

func (r *Runner) attachGeneratedCapabilityJob(ctx context.Context, result *agent.CapabilityGenerationResult, schedule capabilityGenerateSchedule, generation agent.CapabilityGenerationInput) error {
	if result == nil {
		return fmt.Errorf("capability result is required")
	}
	extensionName := strings.TrimSpace(result.Generation.Extension.Name)
	if extensionName == "" {
		return fmt.Errorf("generated extension name is required before scheduling")
	}
	store, err := r.schedulerStore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()
	input := capabilityScheduleInputTUI(schedule, generation, result)
	if err := r.extensionStore().ValidateRunInput(extensionName, input); err != nil {
		return err
	}
	job, err := store.Create(ctx, scheduler.CreateInput{
		Name:         strings.TrimSpace(schedule.Name),
		ScheduleType: strings.TrimSpace(schedule.ScheduleType),
		ScheduleExpr: strings.TrimSpace(schedule.ScheduleExpr),
		TargetType:   scheduler.TargetExtension,
		TargetName:   extensionName,
		Input:        input,
		Approved:     true,
		Enabled:      schedule.EnableJob,
	})
	if err != nil {
		return err
	}
	result.Schedule = &agent.CapabilityScheduledJob{
		ID:           job.ID,
		Name:         job.Name,
		ScheduleType: job.ScheduleType,
		ScheduleExpr: job.ScheduleExpr,
		TargetType:   job.TargetType,
		TargetName:   job.TargetName,
		Input:        job.Input,
		Enabled:      job.Enabled,
		Approved:     job.Approved,
		NextDueAt:    job.NextDueAt,
	}
	result.NextSteps = append([]string{capabilityJobNextStepTUI(job)}, result.NextSteps...)
	if job.Enabled {
		result.Message = "capability generated, tested, registered, and scheduled after explicit approval"
	} else {
		result.Message = "capability generated, tested, registered, and attached to an approved disabled job"
	}
	return nil
}

func capabilityScheduleInputTUI(schedule capabilityGenerateSchedule, generation agent.CapabilityGenerationInput, result *agent.CapabilityGenerationResult) map[string]any {
	if len(schedule.Input) > 0 {
		return cloneJobInput(schedule.Input)
	}
	if len(generation.RunInput) > 0 {
		return cloneJobInput(generation.RunInput)
	}
	if result == nil {
		return map[string]any{}
	}
	return agent.SuggestedCapabilityInput(result.Proposal, generation.Request)
}

func cloneJobInput(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	clone := make(map[string]any, len(input))
	for key, value := range input {
		clone[key] = value
	}
	return clone
}

func capabilityJobNextStepTUI(job scheduler.Job) string {
	if job.Enabled {
		return "Scheduler job `" + job.ID + "` is enabled and will run when due."
	}
	if job.ScheduleType == scheduler.ScheduleManual {
		return "Run the approved manual job when ready with `/job run " + job.ID + "`."
	}
	return "Enable the approved job when ready with `/job enable " + job.ID + "`."
}

func parseInternetFetch(input string) (internet.FetchInput, error) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return internet.FetchInput{}, fmt.Errorf("internet fetch requires a URL")
	}
	result := internet.FetchInput{
		URL:    parts[0],
		Caller: "tui",
	}
	for i := 1; i < len(parts); i++ {
		switch parts[i] {
		case "--yes":
			result.TaskApproved = true
		case "--extract-text":
			result.ExtractText = true
		case "--domain":
			i++
			if i >= len(parts) {
				return internet.FetchInput{}, fmt.Errorf("--domain requires a domain")
			}
			result.AllowedDomains = append(result.AllowedDomains, parts[i])
		default:
			return internet.FetchInput{}, fmt.Errorf("unknown internet option: %s", parts[i])
		}
	}
	return result, nil
}

func parseInternetSearch(input string) (internet.SearchInput, error) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return internet.SearchInput{}, fmt.Errorf("usage: internet search <query> [--yes] [--limit 5]")
	}
	result := internet.SearchInput{Caller: "tui"}
	queryParts := make([]string, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		switch parts[i] {
		case "--yes", "--task-approved":
			result.TaskApproved = true
		case "--limit":
			i++
			if i >= len(parts) {
				return internet.SearchInput{}, fmt.Errorf("--limit requires a number")
			}
			limit, err := strconv.Atoi(parts[i])
			if err != nil || limit <= 0 {
				return internet.SearchInput{}, fmt.Errorf("--limit requires a positive number")
			}
			result.MaxResults = limit
		default:
			if strings.HasPrefix(parts[i], "--limit=") {
				limit, err := strconv.Atoi(strings.TrimPrefix(parts[i], "--limit="))
				if err != nil || limit <= 0 {
					return internet.SearchInput{}, fmt.Errorf("--limit requires a positive number")
				}
				result.MaxResults = limit
				continue
			}
			queryParts = append(queryParts, parts[i])
		}
	}
	result.Query = strings.TrimSpace(strings.Join(queryParts, " "))
	if result.Query == "" {
		return internet.SearchInput{}, fmt.Errorf("usage: internet search <query> [--yes] [--limit 5]")
	}
	return result, nil
}

type tuiSearchProviderOptions struct {
	endpoint       string
	maxResults     int
	enableInternet bool
}

func parseSearchProviderRest(input string, firstValueRequired bool) (string, tuiSearchProviderOptions, error) {
	parts := strings.Fields(input)
	var opts tuiSearchProviderOptions
	if len(parts) == 0 {
		return "", opts, nil
	}
	first := ""
	if firstValueRequired || !strings.HasPrefix(parts[0], "--") {
		first = parts[0]
		parts = parts[1:]
	}
	for i := 0; i < len(parts); i++ {
		switch parts[i] {
		case "--enable-internet":
			opts.enableInternet = true
		case "--endpoint":
			i++
			if i >= len(parts) {
				return "", opts, fmt.Errorf("--endpoint requires a URL")
			}
			opts.endpoint = strings.TrimSpace(parts[i])
		case "--max-results":
			i++
			if i >= len(parts) {
				return "", opts, fmt.Errorf("--max-results requires a number")
			}
			limit, err := strconv.Atoi(parts[i])
			if err != nil || limit <= 0 {
				return "", opts, fmt.Errorf("--max-results requires a positive number")
			}
			opts.maxResults = limit
		default:
			switch {
			case strings.HasPrefix(parts[i], "--endpoint="):
				opts.endpoint = strings.TrimSpace(strings.TrimPrefix(parts[i], "--endpoint="))
			case strings.HasPrefix(parts[i], "--max-results="):
				limit, err := strconv.Atoi(strings.TrimPrefix(parts[i], "--max-results="))
				if err != nil || limit <= 0 {
					return "", opts, fmt.Errorf("--max-results requires a positive number")
				}
				opts.maxResults = limit
			default:
				return "", opts, fmt.Errorf("unknown search provider option: %s", parts[i])
			}
		}
	}
	return strings.TrimSpace(first), opts, nil
}

func applyTUISearchProviderOptions(cfg *config.Config, opts tuiSearchProviderOptions) {
	if opts.maxResults > 0 {
		cfg.Internet.Search.MaxResults = opts.maxResults
	}
	cfg.Internet.Search.SafeSearch = true
	if opts.enableInternet {
		cfg.Internet.Enabled = true
		cfg.Internet.DefaultMode = "profile_enabled"
	}
}

func writeTUISearchProviderConfig(r *Runner, output io.Writer, provider string) error {
	if err := config.Write(r.deps.Config.Path, r.deps.Config); err != nil {
		return err
	}
	fmt.Fprintf(output, "internet search provider configured: %s\n", provider)
	if !r.deps.Config.Internet.Enabled {
		fmt.Fprintln(output, "internet remains disabled for this profile; run `internet on` or pass --enable-internet when configuring the provider.")
	}
	return nil
}

func internetSearchProviderStatus(cfg *config.Config) map[string]any {
	status := map[string]any{"ready": false}
	if cfg == nil {
		return status
	}
	internetStatus := internet.New(cfg.Internet, ".", "").Status()
	status["internet_enabled"] = cfg.Internet.Enabled
	status["internet_mode"] = cfg.Internet.DefaultMode
	status["search_enabled"] = cfg.Internet.Search.Enabled
	status["provider"] = cfg.Internet.Search.Provider
	status["provider_label"] = internetStatus.SearchProviderLabel
	status["endpoint"] = cfg.Internet.Search.Endpoint
	status["api_key_env"] = cfg.Internet.Search.APIKeyEnv
	status["max_results"] = cfg.Internet.Search.MaxResults
	status["safe_search"] = cfg.Internet.Search.SafeSearch
	status["ready"] = internetStatus.SearchProviderReady
	status["needs_config"] = internetStatus.SearchProviderNeedsConfig
	status["needs_auth"] = internetStatus.SearchProviderNeedsAuth
	status["status"] = internetStatus.SearchStatus
	status["fallback_providers"] = internetStatus.SearchFallbackProviders
	return status
}

func hasHTTPPrefix(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func internetUsage() string {
	return "internet status|on|off|search-provider status|auto|searxng|tavily|serper|brave|firecrawl|wikimedia|duckduckgo|mojeek|custom|off|fetch <url> [--yes] [--extract-text] [--domain example.com]|head <url> [--yes]|search <query> [--yes] [--limit 5]|cache|requests [limit]"
}

func internetSearchProviderUsage() string {
	return "internet search-provider status | auto [--max-results 5] [--enable-internet] | searxng <endpoint> [--max-results 5] [--enable-internet] | tavily|serper|brave|firecrawl|mojeek <api-key-env> [--endpoint <url>] [--max-results 5] [--enable-internet] | wikimedia|duckduckgo [--endpoint <url>] [--max-results 5] [--enable-internet] | custom <endpoint> [--max-results 5] [--enable-internet] | off"
}

func parseLimit(input string, fallback int) int {
	input = strings.TrimSpace(input)
	if input == "" {
		return fallback
	}
	limit, err := strconv.Atoi(input)
	if err != nil || limit <= 0 {
		return fallback
	}
	return limit
}

func parseJobCreate(input string) (scheduler.CreateInput, error) {
	parts := strings.Fields(input)
	if len(parts) < 3 {
		return scheduler.CreateInput{}, fmt.Errorf("usage: job create <manual|interval|cron|one_time> <target> --yes")
	}
	if parts[0] != "manual" && parts[0] != "interval" && parts[0] != "cron" && parts[0] != "one_time" {
		return scheduler.CreateInput{}, fmt.Errorf("unsupported schedule type: %s", parts[0])
	}
	result := scheduler.CreateInput{
		ScheduleType: parts[0],
		TargetName:   parts[1],
		TargetType:   scheduler.TargetExtension,
		Input:        map[string]any{},
	}
	if result.TargetName == scheduler.TargetHeartbeat {
		result.TargetType = scheduler.TargetHeartbeat
	}
	approved := false
	for i := 2; i < len(parts); i++ {
		switch parts[i] {
		case "--yes":
			approved = true
		case "--enable":
			result.Enabled = true
		case "--every":
			i++
			if i >= len(parts) {
				return scheduler.CreateInput{}, fmt.Errorf("--every requires a duration")
			}
			result.ScheduleExpr = parts[i]
		case "--cron":
			i++
			if i >= len(parts) {
				return scheduler.CreateInput{}, fmt.Errorf("--cron requires an expression")
			}
			result.ScheduleExpr = parts[i]
			result.ScheduleType = "cron"
		case "--at":
			i++
			if i >= len(parts) {
				return scheduler.CreateInput{}, fmt.Errorf("--at requires an RFC3339 timestamp")
			}
			result.ScheduleExpr = parts[i]
			result.ScheduleType = "one_time"
		case "--target-type":
			i++
			if i >= len(parts) {
				return scheduler.CreateInput{}, fmt.Errorf("--target-type requires extension or heartbeat")
			}
			result.TargetType = parts[i]
		case "--input-json":
			i++
			if i >= len(parts) {
				return scheduler.CreateInput{}, fmt.Errorf("--input-json requires JSON")
			}
			parsed, err := parseJSONObject(parts[i])
			if err != nil {
				return scheduler.CreateInput{}, err
			}
			result.Input = parsed
		default:
			return scheduler.CreateInput{}, fmt.Errorf("unknown job create option: %s", parts[i])
		}
	}
	if !approved {
		return scheduler.CreateInput{}, fmt.Errorf("use --yes to approve creating a local scheduled job")
	}
	result.Approved = true
	if result.ScheduleType == "manual" {
		result.ScheduleExpr = ""
	}
	return result, nil
}

func parseJobUpdateInput(input string) (scheduler.UpdateInput, error) {
	parts := strings.Fields(input)
	if len(parts) < 1 {
		return scheduler.UpdateInput{}, fmt.Errorf("usage: job update-input <job_id> --input-json '{}' --yes")
	}
	result := scheduler.UpdateInput{
		ID:    strings.TrimSpace(parts[0]),
		Input: map[string]any{},
	}
	approved := false
	seenInput := false
	for i := 1; i < len(parts); i++ {
		switch parts[i] {
		case "--yes", "--approved":
			approved = true
		case "--input-json":
			start := i + 1
			if start >= len(parts) {
				return scheduler.UpdateInput{}, fmt.Errorf("--input-json requires JSON")
			}
			end := start
			for end < len(parts) && parts[end] != "--yes" && parts[end] != "--approved" {
				end++
			}
			if end == start {
				return scheduler.UpdateInput{}, fmt.Errorf("--input-json requires JSON")
			}
			parsed, err := parseJSONObject(strings.Join(parts[start:end], " "))
			if err != nil {
				return scheduler.UpdateInput{}, err
			}
			result.Input = parsed
			seenInput = true
			i = end - 1
		default:
			return scheduler.UpdateInput{}, fmt.Errorf("unknown job update-input option: %s", parts[i])
		}
	}
	if result.ID == "" {
		return scheduler.UpdateInput{}, fmt.Errorf("job id is required")
	}
	if !seenInput {
		return scheduler.UpdateInput{}, fmt.Errorf("--input-json is required")
	}
	if !approved {
		return scheduler.UpdateInput{}, fmt.Errorf("use --yes to approve updating the local scheduled job input")
	}
	return result, nil
}

func parseConnectorEnable(input string) (string, string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", ""
	}
	name := parts[0]
	tokenEnv := ""
	for i := 1; i < len(parts); i++ {
		if parts[i] == "--token-env" && i+1 < len(parts) {
			tokenEnv = parts[i+1]
			i++
		}
	}
	return name, tokenEnv
}

func enableConnector(cfg *config.Config, name string, tokenEnv string) error {
	if err := connectors.ValidateName(name); err != nil {
		return err
	}
	switch name {
	case connectors.LocalAPI:
		return connectors.EnableLocalAPI(cfg, tokenEnv)
	case connectors.MCPServer:
		return connectors.EnableMCPServer(cfg)
	case connectors.Slack, connectors.Discord, connectors.Telegram, connectors.Email:
		return connectors.EnableAdapter(cfg, name, tokenEnv)
	default:
		return fmt.Errorf("unknown connector: %s", name)
	}
}

func disableConnector(cfg *config.Config, name string) error {
	if err := connectors.ValidateName(name); err != nil {
		return err
	}
	switch name {
	case connectors.LocalAPI:
		return connectors.DisableLocalAPI(cfg)
	case connectors.MCPServer:
		return connectors.DisableMCPServer(cfg)
	case connectors.Slack, connectors.Discord, connectors.Telegram, connectors.Email:
		return connectors.DisableAdapter(cfg, name)
	default:
		return fmt.Errorf("unknown connector: %s", name)
	}
}

func policyMode(cfg *config.Config) string {
	if cfg == nil {
		return safety.PolicyModeSafe
	}
	return safety.NormalizePolicyMode(cfg.Security.Policy.Mode)
}

func policyStatus(cfg *config.Config) map[string]any {
	mode := policyMode(cfg)
	status := map[string]any{
		"mode":       mode,
		"fullAccess": safety.IsFullAccessMode(mode),
	}
	if cfg != nil {
		status["auditEnabled"] = cfg.Security.Policy.AuditEnabled
		status["maxAutonomousLevel"] = cfg.Security.Policy.MaxAutonomousLevel
		status["allowAutonomousPosting"] = cfg.Security.Policy.AllowAutonomousPosting
		status["allowAutonomousDeployment"] = cfg.Security.Policy.AllowAutonomousDeployment
		status["allowLiveTrading"] = cfg.Security.Policy.AllowLiveTrading
		status["generatedCanModifyCorePolicy"] = cfg.Security.Policy.GeneratedCanModifyCorePolicy
	}
	return status
}

func setPolicyMode(cfg *config.Config, mode string) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != safety.PolicyModeSafe && mode != safety.PolicyModeFullAccess {
		return fmt.Errorf("unknown policy mode %q; use safe or full_access", mode)
	}
	cfg.Security.Policy.Mode = safety.NormalizePolicyMode(mode)
	if safety.IsFullAccessMode(cfg.Security.Policy.Mode) {
		cfg.Tools.Shell.RequireConfirmationRisky = false
	}
	return nil
}

func parseConversationID(input string) string {
	parts := strings.Fields(input)
	for i := 0; i < len(parts); i++ {
		if parts[i] == "--conversation" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func parseCorrection(input string) (string, string) {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "--conversation ") {
		parts := strings.Fields(input)
		if len(parts) >= 3 {
			conversationID := parts[1]
			idx := strings.Index(input, conversationID)
			if idx >= 0 {
				return conversationID, strings.TrimSpace(input[idx+len(conversationID):])
			}
		}
	}
	conversationID, correction, _ := splitMaybeTwo(input)
	return conversationID, correction
}
