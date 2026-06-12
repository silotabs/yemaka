package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"yemaka/internal/agent"
	"yemaka/internal/config"
	"yemaka/internal/connectors"
	"yemaka/internal/diagnostics"
	"yemaka/internal/domainpacks"
	"yemaka/internal/evaluation"
	"yemaka/internal/extensions"
	"yemaka/internal/heartbeat"
	"yemaka/internal/internet"
	"yemaka/internal/learning"
	"yemaka/internal/memory"
	"yemaka/internal/modelprofiles"
	"yemaka/internal/models"
	"yemaka/internal/models/cloud"
	"yemaka/internal/models/modelruntime"
	"yemaka/internal/notifications"
	"yemaka/internal/profiles"
	"yemaka/internal/rag"
	"yemaka/internal/release"
	"yemaka/internal/replay"
	"yemaka/internal/routing"
	"yemaka/internal/safety"
	"yemaka/internal/scheduler"
	"yemaka/internal/secrets"
	"yemaka/internal/server"
	"yemaka/internal/skills"
	"yemaka/internal/tools"
	"yemaka/internal/tui"
	"yemaka/internal/workflows"
	"yemaka/internal/workspace"
)

type appContext struct {
	config         *config.Config
	profile        *profiles.Profile
	store          *memory.Store
	rag            *rag.Store
	skills         skills.Registry
	runtime        models.Runtime
	runtimeFactory func(config.ModelConfig) (models.Runtime, error)
	cloud          models.Runtime
	router         *models.Router
}

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printHelp(stdout)
		return nil
	}
	if args[0] == "helper" {
		return runHelper(ctx, args[1:], os.Stdin, stdout)
	}

	app, err := loadApp(ctx)
	if err != nil {
		return err
	}
	defer app.store.Close()
	defer app.rag.Close()

	switch args[0] {
	case "doctor":
		return runDoctor(ctx, app, stdout)
	case "model":
		return runModel(ctx, app, args[1:], stdout)
	case "cloud":
		return runCloud(ctx, app, args[1:], stdout)
	case "connector":
		return runConnector(ctx, app, args[1:], os.Stdin, stdout)
	case "domain-pack", "domainpack":
		return runDomainPack(app, args[1:], stdout)
	case "workflow":
		return runWorkflow(app, args[1:], stdout)
	case "policy":
		return runPolicy(app, args[1:], stdout)
	case "learn":
		return runLearn(ctx, app, args[1:], stdout)
	case "replay":
		return runReplay(app, args[1:], stdout)
	case "capability":
		return runCapability(app, args[1:], stdout)
	case "extension":
		return runExtension(ctx, app, args[1:], stdout)
	case "internet":
		return runInternet(ctx, app, args[1:], stdout)
	case "job":
		return runJob(ctx, app, args[1:], stdout)
	case "heartbeat":
		return runHeartbeat(ctx, app, args[1:], stdout)
	case "notification", "notifications":
		return runNotification(app, args[1:], stdout)
	case "chat":
		return runChat(ctx, app, args[1:], stdout)
	case "memory":
		return runMemory(ctx, app, args[1:], stdout)
	case "workspace":
		return runWorkspace(ctx, app, args[1:], stdout)
	case "file":
		return runFile(ctx, app, args[1:], stdout, stderr)
	case "tool":
		return runCoreTool(ctx, app, args[1:], stdout)
	case "ask":
		return runAsk(ctx, app, args[1:], stdout)
	case "ingest":
		return runIngest(ctx, app, args[1:], stdout)
	case "rag":
		return runRAG(ctx, app, args[1:], stdout)
	case "snapshot":
		return runSnapshot(ctx, app, args[1:], stdout)
	case "rollback":
		return runRollback(ctx, app, args[1:], stdout)
	case "git":
		return runGit(ctx, app, args[1:], stdout)
	case "test":
		return runTest(ctx, app, stdout)
	case "shell":
		return runShell(ctx, app, args[1:], stdout, stderr)
	case "skill":
		return runSkill(ctx, app, args[1:], stdout)
	case "eval":
		return runEval(ctx, app, args[1:], stdout)
	case "release":
		return runRelease(ctx, app, args[1:], stdout)
	case "serve":
		return runServe(ctx, app, args[1:], stdout)
	case "tui":
		return runTUI(ctx, app, args[1:], os.Stdin, stdout)
	default:
		printHelp(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runHelper(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) != 1 || args[0] != "shell" {
		return fmt.Errorf("usage: yemaka helper shell")
	}
	return tools.RunShellHelper(ctx, stdin, stdout)
}

func runTUI(ctx context.Context, app *appContext, args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("usage: yemaka tui")
	}
	runner := tui.New(tui.Dependencies{
		Config:         app.config,
		Profile:        app.profile,
		Memory:         app.store,
		RAG:            app.rag,
		Skills:         app.skills,
		Runtime:        app.runtime,
		RuntimeFactory: app.runtimeFactory,
		Cloud:          app.cloud,
		Router:         app.router,
		Workspace:      ".",
	})
	return runner.Run(ctx, stdin, stdout)
}

func runServe(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	addr := server.DefaultAddr
	staticDir := defaultServeStaticDir()
	frontendWatch := false
	frontendWatchSet := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--addr":
			if i+1 >= len(args) {
				return fmt.Errorf("usage: %s", serveUsage())
			}
			addr = args[i+1]
			i++
		case strings.HasPrefix(arg, "--addr="):
			addr = strings.TrimPrefix(arg, "--addr=")
		case arg == "--static":
			if i+1 >= len(args) {
				return fmt.Errorf("usage: %s", serveUsage())
			}
			staticDir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--static="):
			staticDir = strings.TrimPrefix(arg, "--static=")
		case arg == "--frontend-watch":
			frontendWatch = true
			frontendWatchSet = true
		case arg == "--no-frontend-watch":
			frontendWatch = false
			frontendWatchSet = true
		default:
			return fmt.Errorf("unknown serve option: %s", arg)
		}
	}
	if !frontendWatchSet {
		frontendWatch = frontendWatchAvailable(staticDir)
	}

	localServer := server.New(server.Dependencies{
		Config:         app.config,
		Profile:        app.profile,
		Memory:         app.store,
		RAG:            app.rag,
		Skills:         app.skills,
		Runtime:        app.runtime,
		RuntimeFactory: app.runtimeFactory,
		Cloud:          app.cloud,
		Router:         app.router,
		StaticDir:      staticDir,
		Workspace:      ".",
		FrontendWatch:  frontendWatch,
		FrontendRoot:   frontendRootForStatic(staticDir),
		DevLog:         stdout,
	})
	fmt.Fprintf(stdout, "Yemaka local web: http://%s\n", addr)
	fmt.Fprintln(stdout, "Serving the same local Go core. Press Ctrl+C to stop.")
	if frontendWatch {
		fmt.Fprintln(stdout, "Frontend watch: on (rebuilds frontend/dist and refreshes the browser after frontend changes).")
	}
	return localServer.ListenAndServe(ctx, addr)
}

func serveUsage() string {
	return "yemaka serve [--addr 127.0.0.1:7727] [--static <frontend-dist>] [--frontend-watch|--no-frontend-watch]"
}

func defaultServeStaticDir() string {
	candidates := []string{"frontend/dist"}
	if supportDir, err := config.SupportDir(); err == nil && supportDir != "" {
		candidates = append(candidates, filepath.Join(supportDir, "web", "dist"))
	}
	if executable, err := os.Executable(); err == nil && executable != "" {
		executableDir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(executableDir, "frontend", "dist"),
			filepath.Join(executableDir, "..", "share", "yemaka", "frontend", "dist"),
		)
	}
	for _, candidate := range candidates {
		if serveStaticDirReady(candidate) {
			return candidate
		}
	}
	return "frontend/dist"
}

func serveStaticDirReady(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil && !info.IsDir()
}

func frontendWatchAvailable(staticDir string) bool {
	root := frontendRootForStatic(staticDir)
	if root == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(root, "package.json")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(root, "src")); err != nil {
		return false
	}
	return true
}

func frontendRootForStatic(staticDir string) string {
	staticDir = strings.TrimSpace(staticDir)
	if staticDir == "" {
		return ""
	}
	if filepath.Base(staticDir) == "dist" {
		return filepath.Dir(staticDir)
	}
	if _, err := os.Stat("frontend"); err == nil {
		return "frontend"
	}
	return ""
}

func runRelease(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "check" {
		return fmt.Errorf("usage: yemaka release check [--json]")
	}
	jsonOutput := false
	for _, arg := range args[1:] {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		return fmt.Errorf("unknown release check option: %s", arg)
	}
	report := release.Run(ctx, release.Input{
		Config:        app.config,
		Profile:       app.profile,
		Memory:        app.store,
		Runtime:       app.runtime,
		Skills:        app.skills,
		AppBundlePath: release.DefaultAppBundlePath(),
	})
	if jsonOutput {
		return printJSON(stdout, report)
	}
	printReleaseReport(stdout, report)
	if !report.Ready {
		return fmt.Errorf("release check failed")
	}
	return nil
}

func runEval(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: yemaka eval run [--mode low-memory] [--model <installed-model>] [--skip-model] [--json] | yemaka eval report [--json]")
	}
	switch args[0] {
	case "run":
		options, jsonOutput, err := parseEvalRunArgs(args[1:])
		if err != nil {
			return err
		}
		runner := evaluation.Runner{
			Config:  app.config,
			Profile: app.profile,
			Runtime: app.runtime,
		}
		report, err := runner.Run(ctx, options)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(stdout, report)
		}
		printEvalReport(stdout, report)
		return nil
	case "report":
		jsonOutput, err := parseEvalReportArgs(args[1:])
		if err != nil {
			return err
		}
		report, err := evaluation.LoadLatest(app.profile)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(stdout, report)
		}
		printEvalReport(stdout, report)
		return nil
	default:
		return fmt.Errorf("usage: yemaka eval run [--mode low-memory] [--model <installed-model>] [--skip-model] [--json] | yemaka eval report [--json]")
	}
}

func loadApp(ctx context.Context) (*appContext, error) {
	cfg, err := config.LoadOrCreate()
	if err != nil {
		return nil, err
	}
	profile, err := profiles.Init(cfg)
	if err != nil {
		return nil, err
	}
	store, err := memory.Open(ctx, profile.Database)
	if err != nil {
		return nil, err
	}
	ragStore, err := rag.Open(ctx, profile.Database)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	skillDirs, err := skillRegistryDirs(profile)
	if err != nil {
		_ = store.Close()
		_ = ragStore.Close()
		return nil, err
	}
	skillRegistry, err := skills.LoadRegistry(skillDirs)
	if err != nil {
		_ = store.Close()
		_ = ragStore.Close()
		return nil, err
	}

	selected := cfg.Models["default"]
	if cfg.Runtime.LowMemoryMode {
		if low, ok := cfg.Models["low_memory"]; ok {
			selected = low
		}
	}

	var cloudRuntime models.Runtime
	if cfg.CloudFallback.Enabled {
		cloudRuntime = cloud.New(cfg.CloudFallback)
	}

	runtime, err := modelruntime.New(selected)
	if err != nil {
		_ = store.Close()
		_ = ragStore.Close()
		return nil, err
	}

	return &appContext{
		config:         cfg,
		profile:        profile,
		store:          store,
		rag:            ragStore,
		skills:         skillRegistry,
		runtime:        runtime,
		runtimeFactory: modelruntime.New,
		cloud:          cloudRuntime,
		router:         models.NewRouter(cfg),
	}, nil
}

func runDoctor(ctx context.Context, app *appContext, stdout io.Writer) error {
	report := diagnostics.Run(ctx, app.config, app.profile, app.store, app.runtime)
	fmt.Fprintln(stdout, "Yemaka doctor")
	fmt.Fprintf(stdout, "config: %s\n", report.ConfigPath)
	fmt.Fprintf(stdout, "profile: %s\n", report.ProfilePath)
	fmt.Fprintf(stdout, "sqlite: %s\n", diagnostics.Status(report.SQLiteOK, report.SQLiteError))
	fmt.Fprintf(stdout, "sqlite_path: %s\n", report.SQLitePath)
	fmt.Fprintf(stdout, "ollama: %s\n", diagnostics.Status(report.OllamaOK, report.OllamaError))
	fmt.Fprintf(stdout, "default_model: %s\n", report.DefaultModel)
	fmt.Fprintf(stdout, "low_memory_model: %s\n", report.LowMemoryModel)
	fmt.Fprintf(stdout, "selected_model: %s\n", report.SelectedModel)
	fmt.Fprintf(stdout, "model_ready: %t\n", report.ModelReady)
	fmt.Fprintf(stdout, "setup_complete: %t\n", report.SetupComplete)
	if report.ModelListError != "" {
		fmt.Fprintf(stdout, "model_check: not ok (%s)\n", report.ModelListError)
	}
	if len(report.ModelStatuses) > 0 {
		fmt.Fprintln(stdout, "configured_models:")
		for _, model := range report.ModelStatuses {
			state := "installed"
			if !model.Installed {
				state = "missing"
			}
			fmt.Fprintf(stdout, "  %s: %s (%s)", model.Role, model.Name, state)
			if !model.Installed {
				fmt.Fprintf(stdout, " - choose an installed model with `yemaka model set %s <name>`", model.Role)
			}
			fmt.Fprintln(stdout)
		}
	}
	fmt.Fprintf(stdout, "low_memory_mode: %t\n", report.LowMemoryMode)
	fmt.Fprintf(stdout, "rag_enabled: %t\n", report.RAGEnabled)
	fmt.Fprintf(stdout, "shell_enabled: %t\n", report.ShellEnabled)
	fmt.Fprintf(stdout, "cloud_fallback_enabled: %t\n", report.CloudFallback)
	fmt.Fprintf(stdout, "internet_enabled: %t\n", app.config.Internet.Enabled)
	fmt.Fprintf(stdout, "internet_default_mode: %s\n", app.config.Internet.DefaultMode)
	fmt.Fprintf(stdout, "search_enabled: %t\n", app.config.Internet.Search.Enabled)
	fmt.Fprintf(stdout, "search_provider: %s\n", app.config.Internet.Search.Provider)
	fmt.Fprintf(stdout, "connectors_enabled: %t\n", app.config.Connectors.Enabled)
	fmt.Fprintf(stdout, "embeddings_enabled: %t\n", app.config.RAG.Embeddings.Enabled)
	fmt.Fprintf(stdout, "scheduler_enabled: %t\n", app.config.Scheduler.Enabled)
	fmt.Fprintf(stdout, "scheduler_requires_approval: %t\n", app.config.Scheduler.RequireApprovalForNewJobs)
	fmt.Fprintf(stdout, "serve_addr: %s\n", server.DefaultAddr)
	portReady, portErr := listenAddrAvailable(server.DefaultAddr)
	if portErr != "" {
		fmt.Fprintf(stdout, "serve_port_available: %t (%s)\n", portReady, portErr)
	} else {
		fmt.Fprintf(stdout, "serve_port_available: %t\n", portReady)
	}
	fmt.Fprintf(stdout, "serve_static_dir: %s\n", defaultServeStaticDir())
	fmt.Fprintf(stdout, "local_first_defaults: %t\n", localFirstDefaultsIntact(app.config))
	fmt.Fprintf(stdout, "telemetry: %t\n", report.ConfigTelemetry)
	return nil
}

func listenAddrAvailable(addr string) (bool, string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false, err.Error()
	}
	if err := listener.Close(); err != nil {
		return true, err.Error()
	}
	return true, ""
}

func localFirstDefaultsIntact(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return !cfg.App.Telemetry &&
		!cfg.CloudFallback.Enabled &&
		!cfg.Connectors.Enabled &&
		!cfg.Internet.Enabled &&
		!cfg.Internet.Search.Enabled &&
		!cfg.RAG.Embeddings.Enabled &&
		cfg.Tools.Filesystem.WorkspaceOnly &&
		cfg.Tools.Filesystem.SnapshotBeforeWrite &&
		cfg.Tools.Filesystem.RequireConfirmation &&
		cfg.Tools.Shell.RequireConfirmationRisky &&
		cfg.Scheduler.RequireApprovalForNewJobs &&
		cfg.Security.Policy.GeneratedCanModifyCorePolicy == false
}

func runModel(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", modelUsage())
	}

	switch args[0] {
	case "profile":
		return runModelProfile(app, args[1:], stdout)
	case "providers":
		for _, provider := range models.SupportedRuntimeProviders() {
			fmt.Fprintf(stdout, "%s\t%s\n", provider, modelruntime.DefaultBaseURL(provider))
		}
		return nil
	case "list":
		models, err := app.runtime.ListModels(ctx)
		if err != nil {
			return fmt.Errorf("%w\nhint: start the configured local model runtime, then try again", err)
		}
		if len(models) == 0 {
			fmt.Fprintln(stdout, "No models reported by the configured local runtime.")
			fmt.Fprintln(stdout, "Yemaka will not pull models automatically; install or start a low-end local model if needed, then run `yemaka model set low_memory <installed-model>`.")
			return nil
		}
		for _, model := range models {
			fmt.Fprintf(stdout, "%s\t%s\t%d bytes\n", model.Name, model.ModifiedAt, model.Size)
		}
		return nil
	case "show":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka model show <name>")
		}
		detailer := modelDetailRuntime(app.runtime)
		if detailer == nil {
			return fmt.Errorf("configured runtime does not support model details")
		}
		details, err := detailer.ShowModel(ctx, strings.TrimSpace(args[1]))
		if err != nil {
			return err
		}
		printModelDetails(stdout, details)
		return nil
	case "generate":
		if len(args) < 3 || strings.TrimSpace(args[1]) == "" || strings.TrimSpace(strings.Join(args[2:], " ")) == "" {
			return fmt.Errorf("usage: yemaka model generate <name> <prompt>")
		}
		generator := generateRuntime(app.runtime)
		if generator == nil {
			return fmt.Errorf("configured runtime does not support direct generation")
		}
		name := strings.TrimSpace(args[1])
		prompt := strings.TrimSpace(strings.Join(args[2:], " "))
		err := generator.GenerateStream(ctx, models.GenerateRequest{
			Model:       name,
			Prompt:      prompt,
			Temperature: modelTemperature(app.config, name),
		}, func(event models.GenerateEvent) error {
			if event.Token != "" {
				_, err := fmt.Fprint(stdout, event.Token)
				return err
			}
			if event.Done {
				_, err := fmt.Fprintln(stdout)
				return err
			}
			return nil
		})
		return err
	case "set":
		if len(args) < 3 {
			return fmt.Errorf("usage: yemaka model set <role> <name> [--provider ollama|llamacpp|openai_compatible] [--base-url <local-url>]")
		}
		role := strings.TrimSpace(args[1])
		name := strings.TrimSpace(args[2])
		if !knownModelRole(role) {
			return fmt.Errorf("unknown model role %q\nknown roles: %s", role, strings.Join(modelRoles(), ", "))
		}
		if name == "" {
			return fmt.Errorf("usage: yemaka model set <role> <name> [--provider ollama|llamacpp|openai_compatible] [--base-url <local-url>]")
		}
		model := app.config.Models[role]
		provider := models.NormalizeProvider(model.Provider)
		if provider == "" {
			provider = models.ProviderOllama
		}
		baseURL := strings.TrimSpace(model.BaseURL)
		providerSet := false
		baseURLSet := false
		remaining := args[3:]
		for len(remaining) > 0 {
			switch remaining[0] {
			case "--provider":
				if len(remaining) < 2 || strings.TrimSpace(remaining[1]) == "" {
					return fmt.Errorf("--provider requires a value")
				}
				provider = models.NormalizeProvider(remaining[1])
				providerSet = true
				remaining = remaining[2:]
			case "--base-url":
				if len(remaining) < 2 || strings.TrimSpace(remaining[1]) == "" {
					return fmt.Errorf("--base-url requires a local URL")
				}
				baseURL = strings.TrimSpace(remaining[1])
				baseURLSet = true
				remaining = remaining[2:]
			default:
				return fmt.Errorf("unknown model set option %q", remaining[0])
			}
		}
		if !models.IsRuntimeProvider(provider) {
			return fmt.Errorf("unsupported model provider %q\nsupported providers: %s", provider, strings.Join(models.SupportedRuntimeProviders(), ", "))
		}
		if providerSet && !baseURLSet {
			baseURL = modelruntime.DefaultBaseURL(provider)
		}
		if baseURL == "" {
			baseURL = modelruntime.DefaultBaseURL(provider)
		}
		if !modelruntime.IsLocalBaseURL(baseURL) {
			return fmt.Errorf("model runtime base URL must be local: %s", baseURL)
		}
		model.Provider = provider
		model.BaseURL = baseURL
		model.Name = name
		app.config.Models[role] = model
		if err := config.Write(app.config.Path, app.config); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "configured %s model: %s (%s at %s)\n", role, name, provider, baseURL)
		return nil
	case "pull":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka model pull <name>")
		}
		name := strings.TrimSpace(args[1])
		fmt.Fprintf(stdout, "pulling: %s\n", name)
		return app.runtime.PullModel(ctx, name, func(event models.PullEvent) error {
			if event.Total > 0 {
				fmt.Fprintf(stdout, "%s %d/%d bytes\n", event.Status, event.Completed, event.Total)
				return nil
			}
			if event.Status != "" {
				fmt.Fprintln(stdout, event.Status)
			}
			return nil
		})
	default:
		return fmt.Errorf("usage: %s", modelUsage())
	}
}

func modelUsage() string {
	return "yemaka model providers | list | show <name> | generate <name> <prompt> | set <role> <name> [--provider ollama|llamacpp|openai_compatible] [--base-url <local-url>] | pull <name> | profile list|show|create|modelfile"
}

func runModelProfile(app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", modelProfileUsage())
	}
	store, err := modelProfileStore(app)
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("usage: yemaka model profile list")
		}
		profiles, err := store.List()
		if err != nil {
			return err
		}
		if len(profiles) == 0 {
			fmt.Fprintln(stdout, "No model profiles found.")
			return nil
		}
		for _, profile := range profiles {
			fmt.Fprintf(stdout, "%s\t%s", profile.Name, profile.BaseModel)
			if profile.Description != "" {
				fmt.Fprintf(stdout, "\t%s", profile.Description)
			}
			if len(profile.Tags) > 0 {
				fmt.Fprintf(stdout, "\t%s", strings.Join(profile.Tags, ","))
			}
			fmt.Fprintln(stdout)
		}
		return nil
	case "show":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka model profile show <name>")
		}
		profile, err := store.Load(strings.TrimSpace(args[1]))
		if err != nil {
			return err
		}
		printModelProfile(stdout, profile)
		return nil
	case "create":
		profile, err := parseModelProfileCreateArgs(args[1:])
		if err != nil {
			return err
		}
		path, err := store.Save(profile)
		if err != nil {
			return err
		}
		created := modelprofiles.Normalize(profile)
		fmt.Fprintf(stdout, "created model profile: %s\n", created.Name)
		fmt.Fprintf(stdout, "base_model: %s\n", created.BaseModel)
		fmt.Fprintf(stdout, "path: %s\n", path)
		return nil
	case "modelfile":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka model profile modelfile <name>")
		}
		profile, err := store.Load(strings.TrimSpace(args[1]))
		if err != nil {
			return err
		}
		rendered, err := modelprofiles.RenderValidatedModelfile(profile)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(stdout, rendered)
		return err
	default:
		return fmt.Errorf("usage: %s", modelProfileUsage())
	}
}

func modelProfileStore(app *appContext) (modelprofiles.Store, error) {
	if app == nil || app.profile == nil || strings.TrimSpace(app.profile.Root) == "" {
		return modelprofiles.Store{}, fmt.Errorf("model profile commands require an initialized profile")
	}
	return modelprofiles.NewStore(filepath.Join(app.profile.Root, "model_profiles")), nil
}

func modelProfileDir(app *appContext) string {
	if app == nil || app.profile == nil || strings.TrimSpace(app.profile.Root) == "" {
		return ""
	}
	return filepath.Join(app.profile.Root, "model_profiles")
}

func modelProfileUsage() string {
	return "yemaka model profile list | show <name> | create <name> --base <installed-or-local-model-name> [--system <text>] [--temperature <0..2>] [--num-ctx <512..32768>] [--description <text>] [--tag <tag>] [--purpose <text>] [--refined-from <label>] | modelfile <name>"
}

func parseModelProfileCreateArgs(args []string) (modelprofiles.Profile, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return modelprofiles.Profile{}, fmt.Errorf("usage: yemaka model profile create <name> --base <installed-or-local-model-name> [--system <text>] [--temperature <0..2>] [--num-ctx <512..32768>] [--description <text>] [--tag <tag>] [--purpose <text>] [--refined-from <label>]")
	}
	profile := modelprofiles.Profile{
		Name: strings.TrimSpace(args[0]),
	}

	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--base" || strings.HasPrefix(arg, "--base="):
			value, next, err := modelProfileOptionValue(args, i, "--base")
			if err != nil {
				return modelprofiles.Profile{}, err
			}
			profile.BaseModel = value
			i = next
		case arg == "--system" || strings.HasPrefix(arg, "--system="):
			value, next, err := modelProfileOptionValue(args, i, "--system")
			if err != nil {
				return modelprofiles.Profile{}, err
			}
			profile.System = value
			i = next
		case arg == "--temperature" || strings.HasPrefix(arg, "--temperature="):
			value, next, err := modelProfileOptionValue(args, i, "--temperature")
			if err != nil {
				return modelprofiles.Profile{}, err
			}
			temperature, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return modelprofiles.Profile{}, fmt.Errorf("--temperature must be a number between 0 and 2")
			}
			profile.Parameters.Temperature = temperature
			i = next
		case arg == "--num-ctx" || strings.HasPrefix(arg, "--num-ctx="):
			value, next, err := modelProfileOptionValue(args, i, "--num-ctx")
			if err != nil {
				return modelprofiles.Profile{}, err
			}
			numCtx, err := strconv.Atoi(value)
			if err != nil {
				return modelprofiles.Profile{}, fmt.Errorf("--num-ctx must be an integer between 512 and 32768")
			}
			profile.Parameters.NumCtx = numCtx
			i = next
		case arg == "--description" || strings.HasPrefix(arg, "--description="):
			value, next, err := modelProfileOptionValue(args, i, "--description")
			if err != nil {
				return modelprofiles.Profile{}, err
			}
			profile.Description = value
			i = next
		case arg == "--tag" || strings.HasPrefix(arg, "--tag="):
			value, next, err := modelProfileOptionValue(args, i, "--tag")
			if err != nil {
				return modelprofiles.Profile{}, err
			}
			profile.Tags = append(profile.Tags, value)
			i = next
		case arg == "--purpose" || strings.HasPrefix(arg, "--purpose="):
			value, next, err := modelProfileOptionValue(args, i, "--purpose")
			if err != nil {
				return modelprofiles.Profile{}, err
			}
			profile.Metadata.Purpose = value
			i = next
		case arg == "--refined-from" || strings.HasPrefix(arg, "--refined-from="):
			value, next, err := modelProfileOptionValue(args, i, "--refined-from")
			if err != nil {
				return modelprofiles.Profile{}, err
			}
			profile.Metadata.RefinedFrom = value
			i = next
		default:
			return modelprofiles.Profile{}, fmt.Errorf("unknown model profile create option %q", arg)
		}
	}
	if strings.TrimSpace(profile.BaseModel) == "" {
		return modelprofiles.Profile{}, fmt.Errorf("model profile create requires --base")
	}
	return profile, nil
}

func modelProfileOptionValue(args []string, index int, option string) (string, int, error) {
	arg := args[index]
	if strings.HasPrefix(arg, option+"=") {
		value := strings.TrimSpace(strings.TrimPrefix(arg, option+"="))
		if value == "" {
			return "", index, fmt.Errorf("%s requires a value", option)
		}
		return value, index, nil
	}
	if index+1 >= len(args) {
		return "", index, fmt.Errorf("%s requires a value", option)
	}
	value := strings.TrimSpace(args[index+1])
	if value == "" {
		return "", index, fmt.Errorf("%s requires a value", option)
	}
	return value, index + 1, nil
}

func printModelProfile(stdout io.Writer, profile modelprofiles.Profile) {
	profile = modelprofiles.Normalize(profile)
	fmt.Fprintf(stdout, "name: %s\n", profile.Name)
	if profile.Description != "" {
		fmt.Fprintf(stdout, "description: %s\n", profile.Description)
	}
	fmt.Fprintf(stdout, "base_model: %s\n", profile.BaseModel)
	fmt.Fprintf(stdout, "temperature: %s\n", strconv.FormatFloat(profile.Parameters.Temperature, 'f', -1, 64))
	fmt.Fprintf(stdout, "num_ctx: %d\n", profile.Parameters.NumCtx)
	if len(profile.Tags) > 0 {
		fmt.Fprintf(stdout, "tags: %s\n", strings.Join(profile.Tags, ", "))
	}
	if profile.Metadata.Purpose != "" {
		fmt.Fprintf(stdout, "purpose: %s\n", profile.Metadata.Purpose)
	}
	if profile.Metadata.RefinedFrom != "" {
		fmt.Fprintf(stdout, "refined_from: %s\n", profile.Metadata.RefinedFrom)
	}
	if profile.Metadata.CreatedBy != "" {
		fmt.Fprintf(stdout, "created_by: %s\n", profile.Metadata.CreatedBy)
	}
	fmt.Fprintf(stdout, "system:\n%s\n", profile.System)
}

func runCloud(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "fallback" {
		return fmt.Errorf("usage: yemaka cloud fallback status|on|off")
	}
	if len(args) < 2 {
		return fmt.Errorf("usage: yemaka cloud fallback status|on|off")
	}
	switch args[1] {
	case "status":
		printCloudFallbackStatus(stdout, app.config.CloudFallback)
		return nil
	case "off":
		app.config.CloudFallback.Enabled = false
		if err := config.Write(app.config.Path, app.config); err != nil {
			return err
		}
		app.cloud = nil
		fmt.Fprintln(stdout, "cloud fallback disabled")
		return nil
	case "on":
		cfg, err := parseCloudFallbackOnArgs(app.config.CloudFallback, args[2:])
		if err != nil {
			return err
		}
		app.config.CloudFallback = cfg
		if err := config.Write(app.config.Path, app.config); err != nil {
			return err
		}
		app.cloud = cloud.New(cfg)
		fmt.Fprintf(stdout, "cloud fallback enabled: %s %s\n", cfg.Provider, cfg.Name)
		fmt.Fprintln(stdout, "local Ollama remains primary; cloud is used only after local model failure")
		return nil
	default:
		return fmt.Errorf("usage: yemaka cloud fallback status|on|off")
	}
}

func runConnector(ctx context.Context, app *appContext, args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", connectorUsage())
	}
	switch args[0] {
	case "list", "status":
		for _, status := range connectors.List(app.config.Connectors) {
			fmt.Fprintf(stdout, "%s\tenabled=%t\tbind=%s\tsecret=%s:%s\tready=%t\thealth=%s\trate=%d/min\tendpoint=%s\n",
				status.Name,
				status.Enabled,
				status.Bind,
				status.Secret.Provider,
				status.Secret.Name,
				status.TokenReady,
				status.Health.Status,
				status.RateLimit.RequestsPerMinute,
				status.Endpoint,
			)
		}
		return nil
	case "registry":
		return printJSON(stdout, connectors.Registry(app.config.Connectors))
	case "show":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka connector show <name>")
		}
		if err := connectors.ValidateName(args[1]); err != nil {
			return err
		}
		for _, entry := range connectors.Registry(app.config.Connectors) {
			if entry.Status.Name == args[1] {
				return printJSON(stdout, entry)
			}
		}
		return fmt.Errorf("connector not found: %s", args[1])
	case "enable":
		if len(args) < 2 {
			return fmt.Errorf("usage: yemaka connector enable <name> [--token-env ENV]")
		}
		if err := connectors.ValidateName(args[1]); err != nil {
			return err
		}
		switch args[1] {
		case connectors.LocalAPI:
			tokenEnv, err := parseConnectorTokenEnv(args[2:])
			if err != nil {
				return err
			}
			if err := connectors.EnableLocalAPI(app.config, tokenEnv); err != nil {
				return err
			}
		case connectors.MCPServer:
			if len(args) > 2 {
				return fmt.Errorf("mcp_server does not accept enable options")
			}
			if err := connectors.EnableMCPServer(app.config); err != nil {
				return err
			}
		default:
			tokenEnv, err := parseConnectorTokenEnv(args[2:])
			if err != nil {
				return err
			}
			if err := connectors.EnableAdapter(app.config, args[1], tokenEnv); err != nil {
				return err
			}
		}
		if err := config.Write(app.config.Path, app.config); err != nil {
			return err
		}
		status := connectorStatus(app.config.Connectors, args[1])
		fmt.Fprintf(stdout, "connector enabled: %s\n", status.Name)
		fmt.Fprintf(stdout, "endpoint: %s\n", status.Endpoint)
		if status.TokenEnv != "" {
			fmt.Fprintf(stdout, "token_env: %s\n", status.TokenEnv)
		}
		if status.Name == connectors.LocalAPI {
			fmt.Fprintln(stdout, "run `yemaka serve` to expose the connector on localhost")
		} else if status.Name == connectors.MCPServer {
			fmt.Fprintln(stdout, "run `yemaka connector serve mcp_server` from an MCP stdio client")
		} else {
			fmt.Fprintf(stdout, "run `yemaka serve` and POST to %s/chat or %s/ask\n", status.Endpoint, status.Endpoint)
		}
		return nil
	case "disable":
		if len(args) < 2 {
			return fmt.Errorf("usage: yemaka connector disable <name>")
		}
		if err := connectors.ValidateName(args[1]); err != nil {
			return err
		}
		switch args[1] {
		case connectors.LocalAPI:
			if err := connectors.DisableLocalAPI(app.config); err != nil {
				return err
			}
		case connectors.MCPServer:
			if err := connectors.DisableMCPServer(app.config); err != nil {
				return err
			}
		default:
			if err := connectors.DisableAdapter(app.config, args[1]); err != nil {
				return err
			}
		}
		if err := config.Write(app.config.Path, app.config); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "connector disabled: %s\n", args[1])
		return nil
	case "serve":
		if len(args) != 2 || args[1] != connectors.MCPServer {
			return fmt.Errorf("usage: yemaka connector serve mcp_server")
		}
		return connectors.ServeMCP(ctx, app.config.Connectors, stdin, stdout, connectorMCPHandlers(app))
	default:
		return fmt.Errorf("usage: %s", connectorUsage())
	}
}

func connectorStatus(cfg config.ConnectorsConfig, name string) connectors.Status {
	switch name {
	case connectors.MCPServer:
		return connectors.MCPServerStatus(cfg)
	case connectors.Slack, connectors.Discord, connectors.Telegram, connectors.Email:
		return connectors.AdapterStatus(cfg, name)
	default:
		return connectors.LocalAPIStatus(cfg)
	}
}

func connectorUsage() string {
	return "yemaka connector list | registry | show <name> | enable <name> [--token-env ENV] | disable <name> | serve mcp_server"
}

func runDomainPack(app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", domainPackUsage())
	}
	root := domainPackRoot(app)
	switch args[0] {
	case "list", "status":
		statuses, err := domainpacks.List(root)
		if err != nil {
			return err
		}
		for _, status := range statuses {
			valid := "valid"
			if !status.Valid {
				valid = "invalid"
			}
			fmt.Fprintf(stdout, "%s\tenabled=%t\tinstalled=%t\t%s\tversion=%s\tcategory=%s\tskills=%s\tdir=%s\n",
				status.Name,
				status.Enabled,
				status.Installed,
				valid,
				status.Version,
				status.Category,
				formatDomainPackList(status.Skills),
				status.Dir,
			)
			if status.ValidationError != "" {
				fmt.Fprintf(stdout, "\terror=%s\n", status.ValidationError)
			}
		}
		return nil
	case "skills":
		statuses, err := domainpacks.List(root)
		if err != nil {
			return err
		}
		printDomainPackSkills(stdout, statuses)
		return nil
	case "templates":
		if len(args) != 1 {
			return fmt.Errorf("usage: yemaka domain-pack templates")
		}
		catalog, err := builtInDomainPackTemplateCatalog()
		if err != nil {
			return err
		}
		statuses, err := domainPackStatusByName(root)
		if err != nil {
			return err
		}
		printDomainPackTemplates(stdout, catalog.List(), statuses)
		return nil
	case "template":
		return runDomainPackTemplate(app, args[1:], stdout)
	case "install":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka domain-pack install <local-pack-dir>")
		}
		status, err := domainpacks.Install(root, args[1])
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "domain pack installed: %s\n", status.Name)
		fmt.Fprintln(stdout, "enabled: false")
		return nil
	case "install-template":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka domain-pack install-template <template-name>")
		}
		return installBuiltInDomainPackTemplate(app, args[1], stdout)
	case "enable":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka domain-pack enable <name>")
		}
		status, err := domainpacks.SetEnabled(root, args[1], true)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "domain pack enabled: %s\n", status.Name)
		return nil
	case "disable":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka domain-pack disable <name>")
		}
		status, err := domainpacks.SetEnabled(root, args[1], false)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "domain pack disabled: %s\n", status.Name)
		return nil
	default:
		return fmt.Errorf("usage: %s", domainPackUsage())
	}
}

func domainPackRoot(app *appContext) string {
	return filepath.Join(app.profile.Root, "domain_packs")
}

func domainPackUsage() string {
	return "yemaka domain-pack list | skills | templates | template show <name> | install <local-pack-dir> | install-template <template-name> | enable <name> | disable <name>"
}

func printDomainPackSkills(stdout io.Writer, statuses []domainpacks.PackStatus) {
	for _, status := range statuses {
		active := status.Enabled && status.Valid
		for _, ref := range status.Skills {
			fmt.Fprintf(stdout, "%s\tref=%s\tactive=%t\tenabled=%t\tvalid=%t\n",
				status.Name,
				ref,
				active,
				status.Enabled,
				status.Valid,
			)
		}
	}
}

func formatDomainPackList(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}

func runDomainPackTemplate(app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: yemaka domain-pack template list|show <name>|install <name>")
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("usage: yemaka domain-pack template list")
		}
		return runDomainPack(app, []string{"templates"}, stdout)
	case "show":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka domain-pack template show <name>")
		}
		catalog, err := builtInDomainPackTemplateCatalog()
		if err != nil {
			return err
		}
		template, ok := catalog.Get(args[1])
		if !ok {
			return fmt.Errorf("built-in domain pack template not found: %s", args[1])
		}
		statuses, err := domainPackStatusByName(domainPackRoot(app))
		if err != nil {
			return err
		}
		printDomainPackTemplate(stdout, template, statuses[template.Name])
		return nil
	case "install":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka domain-pack template install <name>")
		}
		return installBuiltInDomainPackTemplate(app, args[1], stdout)
	default:
		return fmt.Errorf("usage: yemaka domain-pack template list|show <name>|install <name>")
	}
}

func installBuiltInDomainPackTemplate(app *appContext, name string, stdout io.Writer) error {
	catalog, err := builtInDomainPackTemplateCatalog()
	if err != nil {
		return err
	}
	template, ok := catalog.Get(name)
	if !ok {
		return fmt.Errorf("built-in domain pack template not found: %s", name)
	}
	status, err := domainpacks.Install(domainPackRoot(app), template.Path)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "domain pack installed: %s\n", status.Name)
	fmt.Fprintf(stdout, "template: %s\n", template.Name)
	fmt.Fprintln(stdout, "enabled: false")
	fmt.Fprintf(stdout, "enable: yemaka domain-pack enable %s\n", status.Name)
	return nil
}

func builtInDomainPackTemplateCatalog() (workflows.TemplateCatalog, error) {
	root, err := builtInDomainPackTemplateRoot()
	if err != nil {
		return workflows.TemplateCatalog{}, err
	}
	return workflows.NewBuiltInTemplateCatalog(root)
}

func builtInDomainPackTemplateRoot() (string, error) {
	candidates := []string{}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(executable))
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Dir(file))
	}

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		root, ok := findBuiltInDomainPackTemplateRoot(candidate, seen)
		if ok {
			return root, nil
		}
	}
	return "", fmt.Errorf("built-in domain pack templates not found; run from a Yemaka checkout or install a local pack directory")
}

func findBuiltInDomainPackTemplateRoot(start string, seen map[string]struct{}) (string, bool) {
	start = strings.TrimSpace(start)
	if start == "" {
		return "", false
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if _, exists := seen[dir]; exists {
			return "", false
		}
		seen[dir] = struct{}{}
		if hasBuiltInDomainPackTemplates(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func hasBuiltInDomainPackTemplates(root string) bool {
	info, err := os.Stat(filepath.Join(root, "packs", "templates"))
	return err == nil && info.IsDir()
}

func domainPackStatusByName(root string) (map[string]domainpacks.PackStatus, error) {
	statuses, err := domainpacks.List(root)
	if err != nil {
		return nil, err
	}
	byName := map[string]domainpacks.PackStatus{}
	for _, status := range statuses {
		byName[status.Name] = status
	}
	return byName, nil
}

func printDomainPackTemplates(stdout io.Writer, templates []workflows.Template, statuses map[string]domainpacks.PackStatus) {
	for _, template := range templates {
		status, installed := statuses[template.Name]
		enabled := installed && status.Enabled
		fmt.Fprintf(stdout, "%s\tinstalled=%t\tenabled=%t\tsource=%s\tversion=%s\tcategory=%s\tworkflows=%s\trequired_tools=%s\tpath=%s\n",
			template.Name,
			installed,
			enabled,
			workflows.SourceDomainPackTemplate,
			template.Version,
			template.Category,
			formatWorkflowNames(template.Workflows),
			formatDomainPackList(template.RequiredTools),
			domainPackTemplateRef(template),
		)
		if installed && !status.Valid {
			fmt.Fprintf(stdout, "\terror=%s\n", status.ValidationError)
		}
	}
}

func printDomainPackTemplate(stdout io.Writer, template workflows.Template, status domainpacks.PackStatus) {
	installed := status.Name != ""
	fmt.Fprintf(stdout, "template: %s\n", template.Name)
	fmt.Fprintf(stdout, "description: %s\n", template.Description)
	fmt.Fprintf(stdout, "version: %s\n", template.Version)
	fmt.Fprintf(stdout, "category: %s\n", template.Category)
	fmt.Fprintf(stdout, "installed: %t\n", installed)
	fmt.Fprintf(stdout, "enabled: %t\n", installed && status.Enabled)
	fmt.Fprintf(stdout, "source: %s\n", workflows.SourceDomainPackTemplate)
	fmt.Fprintf(stdout, "path: %s\n", domainPackTemplateRef(template))
	fmt.Fprintf(stdout, "required_tools: %s\n", formatDomainPackList(template.RequiredTools))
	fmt.Fprintln(stdout, "workflows:")
	for _, workflow := range template.Workflows {
		fmt.Fprintf(stdout, " - %s\tskill=%s\tintent=%s\ttools=%s\n",
			workflow.Name,
			workflow.SkillName,
			workflow.Intent,
			formatDomainPackList(workflow.RequiredTools),
		)
	}
	fmt.Fprintf(stdout, "install: yemaka domain-pack install-template %s\n", template.Name)
	fmt.Fprintf(stdout, "enable: yemaka domain-pack enable %s\n", template.Name)
}

func domainPackTemplateRef(template workflows.Template) string {
	if template.Metadata != nil {
		if ref := strings.TrimSpace(template.Metadata["template_ref"]); ref != "" {
			return ref
		}
	}
	return template.Path
}

func formatWorkflowNames(items []workflows.Workflow) string {
	if len(items) == 0 {
		return "-"
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return strings.Join(names, ",")
}

func runWorkflow(app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", workflowUsage())
	}
	switch args[0] {
	case "list":
		options, positionals, err := parseWorkflowFlags(args[1:])
		if err != nil {
			return err
		}
		if len(positionals) != 0 {
			return fmt.Errorf("usage: yemaka workflow list [--include-disabled] [--json]")
		}
		library, err := loadWorkflowLibrary(app)
		if err != nil {
			return err
		}
		items := filterWorkflows(library.List(), options.includeDisabled)
		if options.json {
			return printJSON(stdout, items)
		}
		for _, workflow := range items {
			fmt.Fprintf(stdout, "%s\tenabled=%t\tsource=%s\tpack=%s\tskill=%s\tintent=%s\ttools=%s\tdescription=%s\n",
				workflow.Name,
				workflow.Enabled,
				workflow.Source,
				workflow.PackName,
				workflow.SkillName,
				workflow.Intent,
				formatDomainPackList(workflow.RequiredTools),
				workflow.Description,
			)
		}
		return nil
	case "match":
		options, positionals, err := parseWorkflowFlags(args[1:])
		if err != nil {
			return err
		}
		prompt := strings.TrimSpace(strings.Join(positionals, " "))
		if prompt == "" {
			return fmt.Errorf("usage: yemaka workflow match <prompt> [--include-disabled] [--json]")
		}
		library, err := loadWorkflowLibrary(app)
		if err != nil {
			return err
		}
		match, ok := library.Match(prompt, workflows.MatchOptions{IncludeDisabled: options.includeDisabled})
		result := workflowMatchResult{Matched: ok}
		if ok {
			result.Workflow = &match
		}
		if options.json {
			return printJSON(stdout, result)
		}
		if !ok {
			fmt.Fprintln(stdout, "no workflow match")
			return nil
		}
		fmt.Fprintf(stdout, "match: %s\tenabled=%t\tsource=%s\tpack=%s\tskill=%s\tintent=%s\ttools=%s\n",
			match.Name,
			match.Enabled,
			match.Source,
			match.PackName,
			match.SkillName,
			match.Intent,
			formatDomainPackList(match.RequiredTools),
		)
		return nil
	default:
		return fmt.Errorf("usage: %s", workflowUsage())
	}
}

type workflowCLIOptions struct {
	includeDisabled bool
	json            bool
}

type workflowMatchResult struct {
	Matched  bool                `json:"matched"`
	Workflow *workflows.Workflow `json:"workflow,omitempty"`
}

func workflowUsage() string {
	return "yemaka workflow list [--include-disabled] [--json] | match <prompt> [--include-disabled] [--json]"
}

func parseWorkflowFlags(args []string) (workflowCLIOptions, []string, error) {
	var options workflowCLIOptions
	positionals := []string{}
	for _, arg := range args {
		switch arg {
		case "--include-disabled":
			options.includeDisabled = true
		case "--json":
			options.json = true
		default:
			if strings.HasPrefix(arg, "-") {
				return workflowCLIOptions{}, nil, fmt.Errorf("unknown workflow option: %s", arg)
			}
			positionals = append(positionals, arg)
		}
	}
	return options, positionals, nil
}

func loadWorkflowLibrary(app *appContext) (workflows.Library, error) {
	library, err := workflows.NewLibrary()
	if err != nil {
		return workflows.Library{}, err
	}
	statuses, err := domainpacks.List(domainPackRoot(app))
	if err != nil {
		return workflows.Library{}, err
	}
	for _, status := range statuses {
		if !status.Valid {
			continue
		}
		manifest, err := domainpacks.Load(status.Dir)
		if err != nil {
			return workflows.Library{}, fmt.Errorf("load workflow pack %s: %w", status.Name, err)
		}
		for _, ref := range manifest.Skills {
			skill, err := skills.Load(filepath.Join(status.Dir, filepath.FromSlash(ref)))
			if err != nil {
				return workflows.Library{}, fmt.Errorf("load workflow skill %s/%s: %w", status.Name, ref, err)
			}
			workflow, err := workflows.FromDomainPackSkill(manifest, skill)
			if err != nil {
				return workflows.Library{}, fmt.Errorf("build workflow %s/%s: %w", status.Name, skill.Name, err)
			}
			workflow.Enabled = status.Enabled && !skill.Disabled
			if err := library.Add(workflow); err != nil {
				return workflows.Library{}, err
			}
		}
	}
	return library, nil
}

func filterWorkflows(items []workflows.Workflow, includeDisabled bool) []workflows.Workflow {
	if includeDisabled {
		return items
	}
	filtered := make([]workflows.Workflow, 0, len(items))
	for _, item := range items {
		if item.Enabled {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func runPolicy(app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "status" {
		return printJSON(stdout, app.config.Security.Policy)
	}
	if len(args) != 2 || args[0] != "mode" {
		return fmt.Errorf("usage: yemaka policy status | policy mode safe | policy mode full_access")
	}
	requested := strings.ToLower(strings.TrimSpace(args[1]))
	if requested != safety.PolicyModeSafe && requested != safety.PolicyModeFullAccess {
		return fmt.Errorf("unknown policy mode %q; use safe or full_access", args[1])
	}
	mode := safety.NormalizePolicyMode(requested)
	app.config.Security.Policy.Mode = mode
	if mode == safety.PolicyModeFullAccess {
		app.config.Tools.Shell.RequireConfirmationRisky = false
	}
	if err := config.Write(app.config.Path, app.config); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "policy mode: %s\n", mode)
	if mode == safety.PolicyModeFullAccess {
		fmt.Fprintln(stdout, "full access mode disables policy confirmation gates for this profile")
	}
	return nil
}

func runLearn(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", learnUsage())
	}
	switch args[0] {
	case "report":
		report, err := learning.BuildReport(ctx, app.store, extensionStore(app))
		if err != nil {
			return err
		}
		return printJSON(stdout, report)
	case "export":
		conversationID, outPath, err := parseLearnExportArgs(args[1:])
		if err != nil {
			return err
		}
		trajectory, err := learning.ExportTrajectory(ctx, app.store, conversationID)
		if err != nil {
			return err
		}
		if outPath == "" {
			return printJSON(stdout, trajectory)
		}
		if looksRemote(outPath) {
			return fmt.Errorf("trajectory export path must be local")
		}
		data, err := json.MarshalIndent(trajectory, "", "  ")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outPath, append(data, '\n'), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "trajectory exported: %s\n", outPath)
		return nil
	case "correction":
		conversationID, correction, err := parseLearnCorrectionArgs(args[1:])
		if err != nil {
			return err
		}
		item, err := learning.SaveCorrection(ctx, app.store, conversationID, correction)
		if err != nil {
			return err
		}
		return printJSON(stdout, item)
	case "route-correction", "route-corrections":
		return runLearnRouteCorrection(ctx, app, args[1:], stdout)
	case "regression-artifacts":
		tracePath, outPath, err := parseLearnRegressionArtifactArgs(args[1:])
		if err != nil {
			return err
		}
		if looksRemote(tracePath) || looksRemote(outPath) {
			return fmt.Errorf("regression artifact paths must be local")
		}
		data, err := os.ReadFile(tracePath)
		if err != nil {
			return fmt.Errorf("read replay trace: %w", err)
		}
		var trace replay.Trace
		if err := json.Unmarshal(data, &trace); err != nil {
			return fmt.Errorf("parse replay trace: %w", err)
		}
		artifacts, err := learning.SaveRegressionArtifactsFromTrace(outPath, trace.Normalize())
		if err != nil {
			return err
		}
		return printJSON(stdout, artifacts)
	default:
		return fmt.Errorf("usage: %s", learnUsage())
	}
}

func runLearnRouteCorrection(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", learnUsage())
	}
	switch args[0] {
	case "list":
		corrections, err := learning.ListRouteCorrections(ctx, app.store, 200)
		if err != nil {
			return err
		}
		return printJSON(stdout, corrections)
	case "add":
		correction, err := parseLearnRouteCorrectionAddArgs(args[1:])
		if err != nil {
			return err
		}
		saved, err := learning.SaveRouteCorrection(ctx, app.store, correction)
		if err != nil {
			return err
		}
		return printJSON(stdout, saved)
	case "approve":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka learn route-correction approve <id>")
		}
		correction, err := learning.ApproveRouteCorrection(ctx, app.store, args[1])
		if err != nil {
			return err
		}
		return printJSON(stdout, correction)
	case "disable":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka learn route-correction disable <id>")
		}
		correction, err := learning.DisableRouteCorrection(ctx, app.store, args[1])
		if err != nil {
			return err
		}
		return printJSON(stdout, correction)
	default:
		return fmt.Errorf("usage: %s", learnUsage())
	}
}

func runReplay(app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", replayUsage())
	}
	store, err := replayStore(app)
	if err != nil {
		return err
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("usage: %s", replayUsage())
		}
		files, err := store.List()
		if err != nil {
			return err
		}
		return printJSON(stdout, files)
	case "show":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka replay show <trace_id>")
		}
		trace, err := store.Load(args[1])
		if err != nil {
			return err
		}
		return printJSON(stdout, trace)
	case "explain":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka replay explain <trace_id>")
		}
		trace, err := store.Load(args[1])
		if err != nil {
			return err
		}
		return printJSON(stdout, replay.ExplainTrace(trace))
	case "save-artifacts":
		traceID, outPath, err := parseReplaySaveArtifactsArgs(args[1:])
		if err != nil {
			return err
		}
		if looksRemote(outPath) {
			return fmt.Errorf("replay artifact output path must be local")
		}
		trace, err := store.Load(traceID)
		if err != nil {
			return err
		}
		artifacts, err := learning.SaveRegressionArtifactsFromTrace(outPath, trace)
		if err != nil {
			return err
		}
		return printJSON(stdout, artifacts)
	default:
		return fmt.Errorf("usage: %s", replayUsage())
	}
}

func runNotification(app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", notificationUsage())
	}
	store := notifications.NewStore(app.profile.Notifications)
	switch args[0] {
	case "list", "inbox":
		limit, includeDismissed, err := parseNotificationListArgs(args[1:])
		if err != nil {
			return err
		}
		items, err := store.List(limit, includeDismissed)
		if err != nil {
			return err
		}
		return printJSON(stdout, items)
	case "add":
		item, err := parseNotificationAddArgs(args[1:])
		if err != nil {
			return err
		}
		created, err := store.Record(item)
		if err != nil {
			return err
		}
		return printJSON(stdout, created)
	case "read":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka notification read <id>")
		}
		item, err := store.MarkRead(args[1])
		if err != nil {
			return err
		}
		return printJSON(stdout, item)
	case "dismiss":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka notification dismiss <id>")
		}
		item, err := store.Dismiss(args[1])
		if err != nil {
			return err
		}
		return printJSON(stdout, item)
	default:
		return fmt.Errorf("usage: %s", notificationUsage())
	}
}

func notificationUsage() string {
	return "yemaka notification list [--limit N] [--all] | add --title <title> [--message <text>] [--type <type>] [--severity info|success|warning|error] [--source <source>] [--action-required] | read <id> | dismiss <id>"
}

func parseNotificationListArgs(args []string) (int, bool, error) {
	limit := 20
	includeDismissed := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--all":
			includeDismissed = true
		case arg == "--limit":
			if i+1 >= len(args) {
				return 0, false, fmt.Errorf("usage: %s", notificationUsage())
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil || value < 0 {
				return 0, false, fmt.Errorf("notification limit must be a non-negative integer")
			}
			limit = value
			i++
		case strings.HasPrefix(arg, "--limit="):
			value, err := strconv.Atoi(strings.TrimPrefix(arg, "--limit="))
			if err != nil || value < 0 {
				return 0, false, fmt.Errorf("notification limit must be a non-negative integer")
			}
			limit = value
		default:
			return 0, false, fmt.Errorf("unknown notification list option: %s", arg)
		}
	}
	return limit, includeDismissed, nil
}

func parseNotificationAddArgs(args []string) (notifications.Notification, error) {
	item := notifications.Notification{Severity: notifications.SeverityInfo}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--title":
			if i+1 >= len(args) {
				return notifications.Notification{}, fmt.Errorf("usage: %s", notificationUsage())
			}
			item.Title = args[i+1]
			i++
		case strings.HasPrefix(arg, "--title="):
			item.Title = strings.TrimPrefix(arg, "--title=")
		case arg == "--message":
			if i+1 >= len(args) {
				return notifications.Notification{}, fmt.Errorf("usage: %s", notificationUsage())
			}
			item.Message = args[i+1]
			i++
		case strings.HasPrefix(arg, "--message="):
			item.Message = strings.TrimPrefix(arg, "--message=")
		case arg == "--type":
			if i+1 >= len(args) {
				return notifications.Notification{}, fmt.Errorf("usage: %s", notificationUsage())
			}
			item.Type = args[i+1]
			i++
		case strings.HasPrefix(arg, "--type="):
			item.Type = strings.TrimPrefix(arg, "--type=")
		case arg == "--source":
			if i+1 >= len(args) {
				return notifications.Notification{}, fmt.Errorf("usage: %s", notificationUsage())
			}
			item.Source = args[i+1]
			i++
		case strings.HasPrefix(arg, "--source="):
			item.Source = strings.TrimPrefix(arg, "--source=")
		case arg == "--severity":
			if i+1 >= len(args) {
				return notifications.Notification{}, fmt.Errorf("usage: %s", notificationUsage())
			}
			item.Severity = notifications.Severity(strings.TrimSpace(args[i+1]))
			i++
		case strings.HasPrefix(arg, "--severity="):
			item.Severity = notifications.Severity(strings.TrimSpace(strings.TrimPrefix(arg, "--severity=")))
		case arg == "--action-required":
			item.ActionRequired = true
		default:
			return notifications.Notification{}, fmt.Errorf("unknown notification add option: %s", arg)
		}
	}
	if strings.TrimSpace(item.Title) == "" {
		return notifications.Notification{}, fmt.Errorf("notification title is required")
	}
	return item, nil
}

func replayStore(app *appContext) (replay.Store, error) {
	if app == nil || app.profile == nil || strings.TrimSpace(app.profile.Root) == "" {
		return replay.Store{}, fmt.Errorf("replay command requires an initialized profile")
	}
	return replay.NewStore(filepath.Join(app.profile.Root, "replay")), nil
}

func agentReplayStore(app *appContext) agent.ReplaySaver {
	store, err := replayStore(app)
	if err != nil {
		return nil
	}
	return store
}

func replayUsage() string {
	return "yemaka replay list | show <trace_id> | explain <trace_id> | save-artifacts <trace_id> --out ./regressions"
}

func parseReplaySaveArtifactsArgs(args []string) (string, string, error) {
	traceID := ""
	outPath := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--trace":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("usage: %s", replayUsage())
			}
			traceID = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--trace="):
			traceID = strings.TrimSpace(strings.TrimPrefix(arg, "--trace="))
		case arg == "--out":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("usage: %s", replayUsage())
			}
			outPath = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--out="):
			outPath = strings.TrimSpace(strings.TrimPrefix(arg, "--out="))
		case strings.HasPrefix(arg, "--"):
			return "", "", fmt.Errorf("unknown replay save-artifacts option: %s", arg)
		case traceID == "":
			traceID = strings.TrimSpace(arg)
		default:
			return "", "", fmt.Errorf("unexpected replay trace id argument: %s", arg)
		}
	}
	if traceID == "" {
		return "", "", fmt.Errorf("replay save-artifacts requires <trace_id>")
	}
	if outPath == "" {
		return "", "", fmt.Errorf("replay save-artifacts requires --out <directory>")
	}
	return traceID, outPath, nil
}

func learnUsage() string {
	return "yemaka learn report | export --conversation <conversation_id> [--out ./trajectory.json] | correction --conversation <conversation_id> \"what to remember\" | route-correction add/list/approve/disable [--tag source:local_documents] | regression-artifacts --trace ./trace.json --out ./regressions"
}

func parseLearnExportArgs(args []string) (string, string, error) {
	conversationID := ""
	outPath := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--conversation":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("usage: %s", learnUsage())
			}
			conversationID = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--conversation="):
			conversationID = strings.TrimSpace(strings.TrimPrefix(arg, "--conversation="))
		case arg == "--out":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("usage: %s", learnUsage())
			}
			outPath = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--out="):
			outPath = strings.TrimSpace(strings.TrimPrefix(arg, "--out="))
		default:
			return "", "", fmt.Errorf("unknown learn export option: %s", arg)
		}
	}
	if conversationID == "" {
		return "", "", fmt.Errorf("learn export requires --conversation <conversation_id>")
	}
	return conversationID, outPath, nil
}

func parseLearnCorrectionArgs(args []string) (string, string, error) {
	conversationID := ""
	correctionParts := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--conversation":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("usage: %s", learnUsage())
			}
			conversationID = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--conversation="):
			conversationID = strings.TrimSpace(strings.TrimPrefix(arg, "--conversation="))
		default:
			correctionParts = append(correctionParts, arg)
		}
	}
	if conversationID == "" {
		return "", "", fmt.Errorf("learn correction requires --conversation <conversation_id>")
	}
	correction := strings.TrimSpace(strings.Join(correctionParts, " "))
	if correction == "" {
		return "", "", fmt.Errorf("learn correction requires correction text")
	}
	return conversationID, correction, nil
}

func parseLearnRouteCorrectionAddArgs(args []string) (routing.RouteCorrection, error) {
	correction := routing.RouteCorrection{ApprovalStatus: routing.RouteCorrectionStatusPending}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--conversation":
			if i+1 >= len(args) {
				return routing.RouteCorrection{}, fmt.Errorf("usage: %s", learnUsage())
			}
			correction.SourceConversationID = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--conversation="):
			correction.SourceConversationID = strings.TrimSpace(strings.TrimPrefix(arg, "--conversation="))
		case arg == "--pattern":
			if i+1 >= len(args) {
				return routing.RouteCorrection{}, fmt.Errorf("usage: %s", learnUsage())
			}
			correction.Pattern = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--pattern="):
			correction.Pattern = strings.TrimSpace(strings.TrimPrefix(arg, "--pattern="))
		case arg == "--route":
			if i+1 >= len(args) {
				return routing.RouteCorrection{}, fmt.Errorf("usage: %s", learnUsage())
			}
			correction.IntendedRouteCategory = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--route="):
			correction.IntendedRouteCategory = strings.TrimSpace(strings.TrimPrefix(arg, "--route="))
		case arg == "--task":
			if i+1 >= len(args) {
				return routing.RouteCorrection{}, fmt.Errorf("usage: %s", learnUsage())
			}
			correction.IntendedTaskType = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--task="):
			correction.IntendedTaskType = strings.TrimSpace(strings.TrimPrefix(arg, "--task="))
		case arg == "--capability":
			if i+1 >= len(args) {
				return routing.RouteCorrection{}, fmt.Errorf("usage: %s", learnUsage())
			}
			correction.IntendedCapability = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--capability="):
			correction.IntendedCapability = strings.TrimSpace(strings.TrimPrefix(arg, "--capability="))
		case arg == "--tool" || arg == "--required-tool":
			if i+1 >= len(args) {
				return routing.RouteCorrection{}, fmt.Errorf("usage: %s", learnUsage())
			}
			correction.RequiredTools = append(correction.RequiredTools, strings.TrimSpace(args[i+1]))
			i++
		case strings.HasPrefix(arg, "--tool="):
			correction.RequiredTools = append(correction.RequiredTools, strings.TrimSpace(strings.TrimPrefix(arg, "--tool=")))
		case strings.HasPrefix(arg, "--required-tool="):
			correction.RequiredTools = append(correction.RequiredTools, strings.TrimSpace(strings.TrimPrefix(arg, "--required-tool=")))
		case arg == "--forbid" || arg == "--forbidden-tool":
			if i+1 >= len(args) {
				return routing.RouteCorrection{}, fmt.Errorf("usage: %s", learnUsage())
			}
			correction.ForbiddenTools = append(correction.ForbiddenTools, strings.TrimSpace(args[i+1]))
			i++
		case strings.HasPrefix(arg, "--forbid="):
			correction.ForbiddenTools = append(correction.ForbiddenTools, strings.TrimSpace(strings.TrimPrefix(arg, "--forbid=")))
		case strings.HasPrefix(arg, "--forbidden-tool="):
			correction.ForbiddenTools = append(correction.ForbiddenTools, strings.TrimSpace(strings.TrimPrefix(arg, "--forbidden-tool=")))
		case arg == "--tag":
			if i+1 >= len(args) {
				return routing.RouteCorrection{}, fmt.Errorf("usage: %s", learnUsage())
			}
			correction.Tags = append(correction.Tags, strings.TrimSpace(args[i+1]))
			i++
		case strings.HasPrefix(arg, "--tag="):
			correction.Tags = append(correction.Tags, strings.TrimSpace(strings.TrimPrefix(arg, "--tag=")))
		case arg == "--original-prompt":
			if i+1 >= len(args) {
				return routing.RouteCorrection{}, fmt.Errorf("usage: %s", learnUsage())
			}
			correction.OriginalPrompt = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--original-prompt="):
			correction.OriginalPrompt = strings.TrimSpace(strings.TrimPrefix(arg, "--original-prompt="))
		case arg == "--correction":
			if i+1 >= len(args) {
				return routing.RouteCorrection{}, fmt.Errorf("usage: %s", learnUsage())
			}
			correction.CorrectionText = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--correction="):
			correction.CorrectionText = strings.TrimSpace(strings.TrimPrefix(arg, "--correction="))
		case arg == "--clarify":
			if i+1 >= len(args) {
				return routing.RouteCorrection{}, fmt.Errorf("usage: %s", learnUsage())
			}
			correction.ClarificationQuestion = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--clarify="):
			correction.ClarificationQuestion = strings.TrimSpace(strings.TrimPrefix(arg, "--clarify="))
		case arg == "--approve":
			correction.ApprovalStatus = routing.RouteCorrectionStatusApproved
		default:
			return routing.RouteCorrection{}, fmt.Errorf("unknown route-correction add option: %s", arg)
		}
	}
	if strings.TrimSpace(correction.Pattern) == "" {
		return routing.RouteCorrection{}, fmt.Errorf("route-correction add requires --pattern")
	}
	if strings.TrimSpace(correction.IntendedRouteCategory) == "" {
		return routing.RouteCorrection{}, fmt.Errorf("route-correction add requires --route")
	}
	return correction, nil
}

func parseLearnRegressionArtifactArgs(args []string) (string, string, error) {
	tracePath := ""
	outPath := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--trace":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("usage: %s", learnUsage())
			}
			tracePath = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--trace="):
			tracePath = strings.TrimSpace(strings.TrimPrefix(arg, "--trace="))
		case arg == "--out":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("usage: %s", learnUsage())
			}
			outPath = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--out="):
			outPath = strings.TrimSpace(strings.TrimPrefix(arg, "--out="))
		default:
			return "", "", fmt.Errorf("unknown learn regression-artifacts option: %s", arg)
		}
	}
	if tracePath == "" {
		return "", "", fmt.Errorf("learn regression-artifacts requires --trace <trace.json>")
	}
	if outPath == "" {
		return "", "", fmt.Errorf("learn regression-artifacts requires --out <directory>")
	}
	return tracePath, outPath, nil
}

func looksRemote(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "git@")
}

func runCapability(app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: yemaka capability propose \"missing capability\" | generate \"missing capability\" --yes [--name <name>] [--synthesize | --draft-json <file>] [--run --input-json '{}']")
	}
	switch args[0] {
	case "propose":
		request := strings.TrimSpace(strings.Join(args[1:], " "))
		if request == "" {
			return fmt.Errorf("usage: yemaka capability propose \"missing capability\"")
		}
		result, err := agent.NewCapabilityGapRouter(app.config, extensionStore(app)).ProposeRequest(request)
		if err != nil {
			return err
		}
		return printJSON(stdout, result)
	case "generate":
		command, err := parseCapabilityGenerateArgs(args[1:], app.config.Extensions.MaxRuntimeSeconds)
		if err != nil {
			return err
		}
		input := command.Generation
		if input.SynthesizeDraft {
			draftModel, err := agent.SelectCapabilityDraftModel(app.config)
			if err != nil {
				return err
			}
			input.DraftModel = draftModel
			input.DraftRuntimeFactory = app.runtimeFactory
		}
		input.RunOptions = extensions.RunOptions{
			Input:             input.RunInput,
			MaxRuntimeSeconds: app.config.Extensions.MaxRuntimeSeconds,
			Internet:          internetService(app),
		}
		router := agent.NewCapabilityGapRouter(app.config, extensionStore(app))
		result, err := router.Generate(context.Background(), extensionStore(app), input)
		if err != nil {
			return err
		}
		if command.Schedule.Enabled {
			if err := attachCapabilityJobCLI(context.Background(), app, &result, command.Schedule, input); err != nil {
				return err
			}
		}
		if app.store != nil {
			_, _ = app.store.SaveMemory(context.Background(), memory.Memory{
				Kind:       "capability_generation",
				Content:    result.Generation.SkillCandidate,
				Importance: 4,
				Source:     "capability_generator",
			})
		}
		return printJSON(stdout, result)
	default:
		return fmt.Errorf("usage: yemaka capability propose \"missing capability\" | generate \"missing capability\" --yes [--name <name>] [--synthesize | --draft-json <file>] [--run --input-json '{}']")
	}
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

func parseCapabilityGenerateArgs(args []string, maxRuntimeSeconds int) (capabilityGenerateCommand, error) {
	if len(args) == 0 {
		return capabilityGenerateCommand{}, fmt.Errorf("usage: yemaka capability generate \"missing capability\" --yes [--name <name>] [--synthesize | --draft-json <file>] [--repair-attempts <n>] [--run --input-json '{}'] [--job --every 1h]")
	}
	approved := false
	name := ""
	runAfterGenerate := false
	runInput := map[string]any(nil)
	draftPath := ""
	synthesizeDraft := false
	draftRepairAttempts := 0
	schedule := capabilityGenerateSchedule{}
	requestParts := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
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
			if index+1 >= len(args) {
				return capabilityGenerateCommand{}, fmt.Errorf("--name requires a value")
			}
			index++
			name = args[index]
		case "--draft-json":
			if index+1 >= len(args) {
				return capabilityGenerateCommand{}, fmt.Errorf("--draft-json requires a file path")
			}
			index++
			draftPath = strings.TrimSpace(args[index])
		case "--repair-attempts", "--draft-repair-attempts":
			if index+1 >= len(args) {
				return capabilityGenerateCommand{}, fmt.Errorf("%s requires a number", arg)
			}
			value, err := strconv.Atoi(args[index+1])
			if err != nil {
				return capabilityGenerateCommand{}, fmt.Errorf("%s must be a number", arg)
			}
			index++
			draftRepairAttempts = value
		case "--job-name", "--schedule-name":
			if index+1 >= len(args) {
				return capabilityGenerateCommand{}, fmt.Errorf("%s requires a value", arg)
			}
			schedule.Enabled = true
			index++
			schedule.Name = args[index]
		case "--schedule", "--schedule-type":
			if index+1 >= len(args) {
				return capabilityGenerateCommand{}, fmt.Errorf("%s requires a value", arg)
			}
			schedule.Enabled = true
			index++
			schedule.ScheduleType = args[index]
		case "--every":
			if index+1 >= len(args) {
				return capabilityGenerateCommand{}, fmt.Errorf("--every requires a duration")
			}
			schedule.Enabled = true
			schedule.ScheduleType = scheduler.ScheduleInterval
			index++
			schedule.ScheduleExpr = args[index]
		case "--cron":
			if index+1 >= len(args) {
				return capabilityGenerateCommand{}, fmt.Errorf("--cron requires a cron expression")
			}
			schedule.Enabled = true
			schedule.ScheduleType = scheduler.ScheduleCron
			index++
			schedule.ScheduleExpr = args[index]
		case "--at":
			if index+1 >= len(args) {
				return capabilityGenerateCommand{}, fmt.Errorf("--at requires an RFC3339 timestamp")
			}
			schedule.Enabled = true
			schedule.ScheduleType = scheduler.ScheduleOneTime
			index++
			schedule.ScheduleExpr = args[index]
		case "--input-json":
			if index+1 >= len(args) {
				return capabilityGenerateCommand{}, fmt.Errorf("--input-json requires a JSON object")
			}
			parsed, err := parseCapabilityRunInput(args[index+1])
			if err != nil {
				return capabilityGenerateCommand{}, err
			}
			runInput = parsed
			index++
		case "--job-input-json":
			if index+1 >= len(args) {
				return capabilityGenerateCommand{}, fmt.Errorf("--job-input-json requires a JSON object")
			}
			parsed, err := parseCapabilityRunInput(args[index+1])
			if err != nil {
				return capabilityGenerateCommand{}, err
			}
			schedule.Enabled = true
			schedule.Input = parsed
			index++
		default:
			if strings.HasPrefix(arg, "--input-json=") {
				parsed, err := parseCapabilityRunInput(strings.TrimPrefix(arg, "--input-json="))
				if err != nil {
					return capabilityGenerateCommand{}, err
				}
				runInput = parsed
			} else if strings.HasPrefix(arg, "--draft-json=") {
				draftPath = strings.TrimSpace(strings.TrimPrefix(arg, "--draft-json="))
			} else if strings.HasPrefix(arg, "--repair-attempts=") {
				value, err := strconv.Atoi(strings.TrimPrefix(arg, "--repair-attempts="))
				if err != nil {
					return capabilityGenerateCommand{}, fmt.Errorf("--repair-attempts must be a number")
				}
				draftRepairAttempts = value
			} else if strings.HasPrefix(arg, "--draft-repair-attempts=") {
				value, err := strconv.Atoi(strings.TrimPrefix(arg, "--draft-repair-attempts="))
				if err != nil {
					return capabilityGenerateCommand{}, fmt.Errorf("--draft-repair-attempts must be a number")
				}
				draftRepairAttempts = value
			} else if strings.HasPrefix(arg, "--job-input-json=") {
				parsed, err := parseCapabilityRunInput(strings.TrimPrefix(arg, "--job-input-json="))
				if err != nil {
					return capabilityGenerateCommand{}, err
				}
				schedule.Enabled = true
				schedule.Input = parsed
			} else if strings.HasPrefix(arg, "--job-name=") {
				schedule.Enabled = true
				schedule.Name = strings.TrimPrefix(arg, "--job-name=")
			} else if strings.HasPrefix(arg, "--schedule-name=") {
				schedule.Enabled = true
				schedule.Name = strings.TrimPrefix(arg, "--schedule-name=")
			} else if strings.HasPrefix(arg, "--schedule=") {
				schedule.Enabled = true
				schedule.ScheduleType = strings.TrimPrefix(arg, "--schedule=")
			} else if strings.HasPrefix(arg, "--schedule-type=") {
				schedule.Enabled = true
				schedule.ScheduleType = strings.TrimPrefix(arg, "--schedule-type=")
			} else if strings.HasPrefix(arg, "--every=") {
				schedule.Enabled = true
				schedule.ScheduleType = scheduler.ScheduleInterval
				schedule.ScheduleExpr = strings.TrimPrefix(arg, "--every=")
			} else if strings.HasPrefix(arg, "--cron=") {
				schedule.Enabled = true
				schedule.ScheduleType = scheduler.ScheduleCron
				schedule.ScheduleExpr = strings.TrimPrefix(arg, "--cron=")
			} else if strings.HasPrefix(arg, "--at=") {
				schedule.Enabled = true
				schedule.ScheduleType = scheduler.ScheduleOneTime
				schedule.ScheduleExpr = strings.TrimPrefix(arg, "--at=")
			} else {
				requestParts = append(requestParts, arg)
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
	draft, err := loadExtensionDraft(draftPath)
	if err != nil {
		return capabilityGenerateCommand{}, err
	}
	if synthesizeDraft && draft != nil {
		return capabilityGenerateCommand{}, fmt.Errorf("--synthesize cannot be combined with --draft-json")
	}
	return capabilityGenerateCommand{
		Generation: agent.CapabilityGenerationInput{
			Request:             request,
			Approved:            approved,
			Name:                name,
			MaxRuntimeSeconds:   maxRuntimeSeconds,
			RunAfterGenerate:    runAfterGenerate,
			RunInput:            runInput,
			Draft:               draft,
			SynthesizeDraft:     synthesizeDraft,
			DraftRepairAttempts: draftRepairAttempts,
		},
		Schedule: schedule,
	}, nil
}

func attachCapabilityJobCLI(ctx context.Context, app *appContext, result *agent.CapabilityGenerationResult, schedule capabilityGenerateSchedule, generation agent.CapabilityGenerationInput) error {
	if result == nil {
		return fmt.Errorf("capability result is required")
	}
	extensionName := strings.TrimSpace(result.Generation.Extension.Name)
	if extensionName == "" {
		return fmt.Errorf("generated extension name is required before scheduling")
	}
	store, err := scheduler.Open(ctx, app.profile.Database)
	if err != nil {
		return err
	}
	defer store.Close()
	input := schedule.Input
	if input == nil {
		input = generation.RunInput
	}
	if input == nil {
		input = map[string]any{}
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
	result.NextSteps = append([]string{capabilityJobNextStepCLI(job)}, result.NextSteps...)
	if job.Enabled {
		result.Message = "capability generated, tested, registered, and scheduled after explicit approval"
	} else {
		result.Message = "capability generated, tested, registered, and attached to an approved disabled job"
	}
	return nil
}

func capabilityJobNextStepCLI(job scheduler.Job) string {
	if job.Enabled {
		return "Scheduler job `" + job.ID + "` is enabled and will run when due."
	}
	if job.ScheduleType == scheduler.ScheduleManual {
		return "Run the approved manual job when ready with `yemaka job run " + job.ID + "`."
	}
	return "Enable the approved job when ready with `yemaka job enable " + job.ID + "`."
}

func parseCapabilityRunInput(raw string) (map[string]any, error) {
	var input map[string]any
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return nil, fmt.Errorf("input-json must be a JSON object: %w", err)
	}
	if input == nil {
		input = map[string]any{}
	}
	return input, nil
}

func runExtension(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", extensionUsage())
	}
	store := extensionStore(app)
	switch args[0] {
	case "propose":
		request := strings.TrimSpace(strings.Join(args[1:], " "))
		if request == "" {
			return fmt.Errorf("usage: yemaka extension propose \"missing capability\"")
		}
		proposal, err := store.Propose(request)
		if err != nil {
			return err
		}
		return printJSON(stdout, proposal)
	case "generate":
		input, err := parseExtensionGenerateArgs(args[1:], app.config.Extensions.MaxRuntimeSeconds)
		if err != nil {
			return err
		}
		result, err := store.Generate(ctx, input)
		if err != nil {
			return err
		}
		if app.store != nil {
			_, _ = app.store.SaveMemory(ctx, memory.Memory{
				Kind:       "extension_generation",
				Content:    result.SkillCandidate,
				Importance: 4,
				Source:     "extension_generator",
			})
		}
		return printJSON(stdout, result)
	case "list":
		items, err := store.List()
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Fprintln(stdout, "No generated extensions registered yet.")
			return nil
		}
		for _, item := range items {
			state := "enabled"
			if !item.Enabled {
				state = "disabled"
			}
			valid := "valid"
			if !item.Valid {
				valid = "invalid"
			}
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\n", item.Name, item.Version, item.Type, state, valid, item.Description)
		}
		return nil
	case "show":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka extension show <name>")
		}
		detail, err := store.Show(args[1])
		if err != nil {
			return err
		}
		printExtensionDetail(stdout, detail)
		return nil
	case "inspect":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka extension inspect <name>")
		}
		inspection, err := store.InspectPackage(args[1])
		if err != nil {
			return err
		}
		return printJSON(stdout, inspection)
	case "review":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka extension review <name>")
		}
		review, err := store.Review(args[1])
		if err != nil {
			return err
		}
		return printJSON(stdout, review)
	case "validate":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka extension validate <name>")
		}
		result, err := store.Validate(args[1])
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s: %s\n", result.Extension.Name, result.Message)
		return nil
	case "test":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka extension test <name>")
		}
		result, err := store.Test(ctx, args[1], extensions.TestOptions{})
		if err != nil {
			return err
		}
		return printJSON(stdout, result)
	case "register":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka extension register <name>")
		}
		result, err := store.RegisterGenerated(ctx, args[1], extensions.TestOptions{})
		if err != nil {
			return err
		}
		return printJSON(stdout, result)
	case "enable":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka extension enable <name>")
		}
		result, err := store.SetEnabled(args[1], true)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s: %s\n", result.Extension.Name, result.Message)
		return nil
	case "disable":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka extension disable <name>")
		}
		result, err := store.SetEnabled(args[1], false)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s: %s\n", result.Extension.Name, result.Message)
		return nil
	case "delete":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka extension delete <name>")
		}
		result, err := store.Delete(args[1])
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s: %s\n", result.Extension.Name, result.Message)
		return nil
	case "rollback":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka extension rollback <name-or-snapshot-id>")
		}
		result, err := store.RollbackGeneration(args[1])
		if err != nil {
			return err
		}
		return printJSON(stdout, result)
	case "run":
		if len(args) < 2 || len(args) > 3 {
			return fmt.Errorf("usage: yemaka extension run <name> [input-json]")
		}
		input := map[string]any{}
		if len(args) == 3 {
			if err := json.Unmarshal([]byte(args[2]), &input); err != nil {
				return fmt.Errorf("input-json must be a JSON object: %w", err)
			}
		}
		result, err := store.Run(ctx, args[1], extensions.RunOptions{
			Input:             input,
			MaxRuntimeSeconds: app.config.Extensions.MaxRuntimeSeconds,
			Internet:          internetService(app),
		})
		if err != nil {
			return err
		}
		return printJSON(stdout, result)
	case "failures":
		failures, err := store.Failures(20)
		if err != nil {
			return err
		}
		return printJSON(stdout, failures)
	default:
		return fmt.Errorf("usage: %s", extensionUsage())
	}
}

func extensionStore(app *appContext) *extensions.Store {
	store := extensions.NewStore(app.profile.GeneratedExtensions, app.profile.Logs)
	store.PolicyMode = policyMode(app.config)
	return store
}

func extensionUsage() string {
	return "yemaka extension propose <request> | generate <name> <description> --yes [--brokered-network] [--allow-domain <domain>] [--draft-json <file>] | list | show <name> | inspect <name> | review <name> | validate <name> | test <name> | register <name> | enable <name> | disable <name> | delete <name> | rollback <name-or-snapshot-id> | run <name> [input-json] | failures"
}

func parseExtensionGenerateArgs(args []string, maxRuntimeSeconds int) (extensions.GenerateInput, error) {
	if len(args) < 3 {
		return extensions.GenerateInput{}, fmt.Errorf("usage: yemaka extension generate <name> <description> --yes [--brokered-network] [--allow-domain <domain>] [--draft-json <file>]")
	}
	name := strings.TrimSpace(args[0])
	approved := false
	brokeredNetwork := false
	allowedDomains := []string{}
	draftPath := ""
	descriptionParts := make([]string, 0, len(args)-1)
	for index := 1; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--yes", "--approved":
			approved = true
		case "--brokered-network", "--core-broker-network":
			brokeredNetwork = true
		case "--allow-domain", "--domain":
			if index+1 >= len(args) {
				return extensions.GenerateInput{}, fmt.Errorf("%s requires a domain", arg)
			}
			index++
			allowedDomains = append(allowedDomains, args[index])
		case "--draft-json":
			if index+1 >= len(args) {
				return extensions.GenerateInput{}, fmt.Errorf("%s requires a file path", arg)
			}
			index++
			draftPath = strings.TrimSpace(args[index])
		default:
			descriptionParts = append(descriptionParts, arg)
		}
	}
	description := strings.TrimSpace(strings.Join(descriptionParts, " "))
	if description == "" {
		return extensions.GenerateInput{}, fmt.Errorf("extension description is required")
	}
	if !approved {
		return extensions.GenerateInput{}, fmt.Errorf("use --yes to approve local extension file generation")
	}
	draft, err := loadExtensionDraft(draftPath)
	if err != nil {
		return extensions.GenerateInput{}, err
	}
	if draft != nil && brokeredNetwork {
		return extensions.GenerateInput{}, fmt.Errorf("--draft-json cannot be combined with --brokered-network; declare brokered network permissions in the draft manifest")
	}
	return extensions.GenerateInput{
		Name:              name,
		Description:       description,
		Approved:          approved,
		MaxRuntimeSeconds: maxRuntimeSeconds,
		BrokeredNetwork:   brokeredNetwork,
		AllowedDomains:    allowedDomains,
		Draft:             draft,
	}, nil
}

func loadExtensionDraft(path string) (*extensions.PackageDraft, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read extension draft: %w", err)
	}
	var draft extensions.PackageDraft
	if err := json.Unmarshal(data, &draft); err != nil {
		return nil, fmt.Errorf("decode extension draft JSON: %w", err)
	}
	return &draft, nil
}

func printExtensionDetail(stdout io.Writer, detail extensions.Detail) {
	status := detail.Status
	fmt.Fprintf(stdout, "name: %s\n", status.Name)
	fmt.Fprintf(stdout, "version: %s\n", status.Version)
	fmt.Fprintf(stdout, "type: %s\n", status.Type)
	fmt.Fprintf(stdout, "description: %s\n", status.Description)
	fmt.Fprintf(stdout, "enabled: %t\n", status.Enabled)
	fmt.Fprintf(stdout, "valid: %t\n", status.Valid)
	fmt.Fprintf(stdout, "callable: %t\n", status.Callable)
	fmt.Fprintf(stdout, "dir: %s\n", status.Dir)
	fmt.Fprintf(stdout, "manifest: %s\n", status.ManifestPath)
	if status.ValidationError != "" {
		fmt.Fprintf(stdout, "validation_error: %s\n", status.ValidationError)
	}
	if status.RunBlockedReason != "" {
		fmt.Fprintf(stdout, "run_blocked_reason: %s\n", status.RunBlockedReason)
	}
	if detail.Manifest.Name != "" {
		fmt.Fprintf(stdout, "entrypoint: %s %s\n", detail.Manifest.Entrypoint.Type, detail.Manifest.Entrypoint.Command)
		fmt.Fprintf(stdout, "tests: %s\n", strings.Join(detail.Manifest.Tests, ", "))
	}
	if detail.Inspection.Name != "" {
		fmt.Fprintf(stdout, "package_status: %s\n", detail.Inspection.Status)
		fmt.Fprintf(stdout, "package_files: %d\n", detail.Inspection.FileCount)
		fmt.Fprintf(stdout, "package_bytes: %d\n", detail.Inspection.TotalBytes)
	}
}

func runInternet(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", internetUsage())
	}
	service := internetService(app)
	switch args[0] {
	case "status":
		return printJSON(stdout, service.Status())
	case "on", "enable":
		app.config.Internet.Enabled = true
		app.config.Internet.DefaultMode = "profile_enabled"
		if err := config.Write(app.config.Path, app.config); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "internet enabled for this profile")
		return nil
	case "off", "disable":
		app.config.Internet.Enabled = false
		app.config.Internet.DefaultMode = "ask_each_time"
		if err := config.Write(app.config.Path, app.config); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "internet disabled")
		return nil
	case "fetch":
		input, err := parseInternetFetchArgs(args[1:], false)
		if err != nil {
			return err
		}
		result, err := service.Fetch(ctx, input)
		if err != nil {
			return err
		}
		return printJSON(stdout, result)
	case "head":
		input, err := parseInternetFetchArgs(args[1:], true)
		if err != nil {
			return err
		}
		result, err := service.Head(ctx, input)
		if err != nil {
			return err
		}
		return printJSON(stdout, result)
	case "search":
		input, err := parseInternetSearchArgs(args[1:])
		if err != nil {
			return err
		}
		result, err := service.Search(ctx, input)
		if err != nil {
			return err
		}
		return printJSON(stdout, result)
	case "search-provider", "provider":
		return runInternetSearchProvider(app, args[1:], stdout)
	case "cache":
		if len(args) != 2 || args[1] != "list" {
			return fmt.Errorf("usage: yemaka internet cache list")
		}
		items, err := service.CacheList()
		if err != nil {
			return err
		}
		return printJSON(stdout, items)
	case "requests":
		limit := 20
		if len(args) == 2 {
			parsed, err := strconv.Atoi(args[1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("usage: yemaka internet requests [limit]")
			}
			limit = parsed
		} else if len(args) > 2 {
			return fmt.Errorf("usage: yemaka internet requests [limit]")
		}
		items, err := service.Requests(limit)
		if err != nil {
			return err
		}
		return printJSON(stdout, items)
	default:
		return fmt.Errorf("usage: %s", internetUsage())
	}
}

func runInternetSearchProvider(app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "status" {
		printInternetSearchProviderStatus(stdout, app.config)
		return nil
	}
	switch args[0] {
	case "off", "disable", "none":
		app.config.Internet.Search.Enabled = false
		app.config.Internet.Search.Provider = "none"
		app.config.Internet.Search.Endpoint = ""
		app.config.Internet.Search.APIKeyEnv = ""
		if err := config.Write(app.config.Path, app.config); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "internet search provider disabled")
		return nil
	case "auto", "fallback", "auto_fallback":
		opts, err := parseSearchProviderOptions(args[1:])
		if err != nil {
			return err
		}
		app.config.Internet.Search.Enabled = true
		app.config.Internet.Search.Provider = "auto"
		app.config.Internet.Search.Endpoint = ""
		app.config.Internet.Search.APIKeyEnv = ""
		applySearchProviderOptions(app.config, opts)
		return writeSearchProviderConfig(app, stdout, "auto")
	case "searx", "searxng":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka internet search-provider searxng <endpoint> [--max-results 5] [--enable-internet]")
		}
		opts, err := parseSearchProviderOptions(args[2:])
		if err != nil {
			return err
		}
		endpoint := strings.TrimSpace(args[1])
		if !hasHTTPPrefix(endpoint) {
			return fmt.Errorf("searxng endpoint must start with http:// or https://")
		}
		app.config.Internet.Search.Enabled = true
		app.config.Internet.Search.Provider = "searxng"
		app.config.Internet.Search.Endpoint = endpoint
		app.config.Internet.Search.APIKeyEnv = ""
		applySearchProviderOptions(app.config, opts)
		return writeSearchProviderConfig(app, stdout, "searxng")
	case "tavily", "serper", "firecrawl", "mojeek":
		provider := strings.ToLower(strings.TrimSpace(args[0]))
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka internet search-provider %s <api-key-env> [--endpoint <url>] [--max-results 5] [--enable-internet]", provider)
		}
		opts, err := parseSearchProviderOptions(args[2:])
		if err != nil {
			return err
		}
		if opts.endpoint != "" && !hasHTTPPrefix(opts.endpoint) {
			return fmt.Errorf("%s endpoint must start with http:// or https://", provider)
		}
		app.config.Internet.Search.Enabled = true
		app.config.Internet.Search.Provider = provider
		app.config.Internet.Search.APIKeyEnv = strings.TrimSpace(args[1])
		app.config.Internet.Search.Endpoint = opts.endpoint
		applySearchProviderOptions(app.config, opts)
		return writeSearchProviderConfig(app, stdout, provider)
	case "brave", "brave_search":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka internet search-provider brave <api-key-env> [--endpoint https://api.search.brave.com/res/v1/web/search] [--max-results 5] [--enable-internet]")
		}
		opts, err := parseSearchProviderOptions(args[2:])
		if err != nil {
			return err
		}
		app.config.Internet.Search.Enabled = true
		app.config.Internet.Search.Provider = "brave"
		app.config.Internet.Search.APIKeyEnv = strings.TrimSpace(args[1])
		if opts.endpoint != "" {
			if !hasHTTPPrefix(opts.endpoint) {
				return fmt.Errorf("brave endpoint must start with http:// or https://")
			}
			app.config.Internet.Search.Endpoint = opts.endpoint
		} else {
			app.config.Internet.Search.Endpoint = ""
		}
		applySearchProviderOptions(app.config, opts)
		return writeSearchProviderConfig(app, stdout, "brave")
	case "wikimedia", "wikipedia", "duckduckgo", "ddg":
		provider := strings.ToLower(strings.TrimSpace(args[0]))
		if provider == "wikipedia" {
			provider = "wikimedia"
		}
		if provider == "ddg" {
			provider = "duckduckgo"
		}
		opts, err := parseSearchProviderOptions(args[1:])
		if err != nil {
			return err
		}
		if opts.endpoint != "" && !hasHTTPPrefix(opts.endpoint) {
			return fmt.Errorf("%s endpoint must start with http:// or https://", provider)
		}
		app.config.Internet.Search.Enabled = true
		app.config.Internet.Search.Provider = provider
		app.config.Internet.Search.APIKeyEnv = ""
		app.config.Internet.Search.Endpoint = opts.endpoint
		applySearchProviderOptions(app.config, opts)
		return writeSearchProviderConfig(app, stdout, provider)
	case "custom", "custom_search", "yemaka_custom", "yemaka_custom_search":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka internet search-provider custom <endpoint> [--max-results 5] [--enable-internet]")
		}
		opts, err := parseSearchProviderOptions(args[2:])
		if err != nil {
			return err
		}
		endpoint := strings.TrimSpace(args[1])
		if !hasHTTPPrefix(endpoint) {
			return fmt.Errorf("custom search endpoint must start with http:// or https://")
		}
		app.config.Internet.Search.Enabled = true
		app.config.Internet.Search.Provider = "yemaka_custom"
		app.config.Internet.Search.Endpoint = endpoint
		app.config.Internet.Search.APIKeyEnv = ""
		applySearchProviderOptions(app.config, opts)
		return writeSearchProviderConfig(app, stdout, "yemaka_custom")
	default:
		return fmt.Errorf("usage: %s", internetSearchProviderUsage())
	}
}

type searchProviderOptions struct {
	endpoint       string
	maxResults     int
	enableInternet bool
}

func parseSearchProviderOptions(args []string) (searchProviderOptions, error) {
	var opts searchProviderOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--enable-internet":
			opts.enableInternet = true
		case arg == "--endpoint":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--endpoint requires a URL")
			}
			opts.endpoint = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--endpoint="):
			opts.endpoint = strings.TrimSpace(strings.TrimPrefix(arg, "--endpoint="))
		case arg == "--max-results":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--max-results requires a number")
			}
			limit, err := parsePositiveInt(args[i+1])
			if err != nil {
				return opts, fmt.Errorf("--max-results requires a positive number")
			}
			opts.maxResults = limit
			i++
		case strings.HasPrefix(arg, "--max-results="):
			limit, err := parsePositiveInt(strings.TrimPrefix(arg, "--max-results="))
			if err != nil {
				return opts, fmt.Errorf("--max-results requires a positive number")
			}
			opts.maxResults = limit
		default:
			return opts, fmt.Errorf("unknown search provider option: %s", arg)
		}
	}
	return opts, nil
}

func applySearchProviderOptions(cfg *config.Config, opts searchProviderOptions) {
	if opts.maxResults > 0 {
		cfg.Internet.Search.MaxResults = opts.maxResults
	}
	cfg.Internet.Search.SafeSearch = true
	if opts.enableInternet {
		cfg.Internet.Enabled = true
		cfg.Internet.DefaultMode = "profile_enabled"
	}
}

func writeSearchProviderConfig(app *appContext, stdout io.Writer, provider string) error {
	if err := config.Write(app.config.Path, app.config); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "internet search provider configured: %s\n", provider)
	if !app.config.Internet.Enabled {
		fmt.Fprintln(stdout, "internet remains disabled for this profile; run `yemaka internet on` or pass --enable-internet when configuring the provider.")
	}
	return nil
}

func printInternetSearchProviderStatus(stdout io.Writer, cfg *config.Config) {
	status := internet.Status{}
	if cfg != nil {
		status = internet.New(cfg.Internet, ".", "").Status()
	} else {
		fmt.Fprintln(stdout, "internet_enabled: false")
		fmt.Fprintln(stdout, "internet_mode: ")
		fmt.Fprintln(stdout, "search_enabled: false")
		fmt.Fprintln(stdout, "provider: none")
		fmt.Fprintln(stdout, "provider_label: ")
		fmt.Fprintln(stdout, "endpoint: ")
		fmt.Fprintln(stdout, "api_key_env: ")
		fmt.Fprintln(stdout, "max_results: 0")
		fmt.Fprintln(stdout, "safe_search: false")
		fmt.Fprintln(stdout, "ready: false")
		fmt.Fprintln(stdout, "needs_config: true")
		fmt.Fprintln(stdout, "needs_auth: false")
		fmt.Fprintln(stdout, "status: config is not loaded")
		return
	}
	fmt.Fprintf(stdout, "internet_enabled: %t\n", cfg.Internet.Enabled)
	fmt.Fprintf(stdout, "internet_mode: %s\n", cfg.Internet.DefaultMode)
	fmt.Fprintf(stdout, "search_enabled: %t\n", cfg.Internet.Search.Enabled)
	fmt.Fprintf(stdout, "provider: %s\n", cfg.Internet.Search.Provider)
	fmt.Fprintf(stdout, "provider_label: %s\n", status.SearchProviderLabel)
	fmt.Fprintf(stdout, "endpoint: %s\n", cfg.Internet.Search.Endpoint)
	fmt.Fprintf(stdout, "api_key_env: %s\n", cfg.Internet.Search.APIKeyEnv)
	fmt.Fprintf(stdout, "max_results: %d\n", cfg.Internet.Search.MaxResults)
	fmt.Fprintf(stdout, "safe_search: %t\n", cfg.Internet.Search.SafeSearch)
	fmt.Fprintf(stdout, "ready: %t\n", status.SearchProviderReady)
	fmt.Fprintf(stdout, "needs_config: %t\n", status.SearchProviderNeedsConfig)
	fmt.Fprintf(stdout, "needs_auth: %t\n", status.SearchProviderNeedsAuth)
	fmt.Fprintf(stdout, "status: %s\n", status.SearchStatus)
	fmt.Fprintf(stdout, "fallback_providers: %s\n", strings.Join(status.SearchFallbackProviders, ","))
}

func internetSearchProviderReady(cfg *config.Config) bool {
	return cfg != nil && internet.New(cfg.Internet, ".", "").Status().SearchProviderReady
}

func hasHTTPPrefix(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func internetService(app *appContext) *internet.Service {
	service := internet.New(app.config.Internet, app.profile.Root, app.profile.Logs)
	service.PolicyMode = policyMode(app.config)
	return service
}

func parseInternetFetchArgs(args []string, head bool) (internet.FetchInput, error) {
	if len(args) == 0 {
		return internet.FetchInput{}, fmt.Errorf("usage: yemaka internet fetch <url> [--yes] [--extract-text] [--domain example.com]")
	}
	input := internet.FetchInput{
		URL:    strings.TrimSpace(args[0]),
		Method: "GET",
		Caller: "cli",
	}
	if head {
		input.Method = "HEAD"
	}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--yes" || arg == "--task-approved":
			input.TaskApproved = true
		case arg == "--extract-text":
			input.ExtractText = true
		case arg == "--domain":
			if i+1 >= len(args) {
				return internet.FetchInput{}, fmt.Errorf("--domain requires a domain")
			}
			input.AllowedDomains = append(input.AllowedDomains, args[i+1])
			i++
		case strings.HasPrefix(arg, "--domain="):
			input.AllowedDomains = append(input.AllowedDomains, strings.TrimPrefix(arg, "--domain="))
		default:
			return internet.FetchInput{}, fmt.Errorf("unknown internet fetch option: %s", arg)
		}
	}
	return input, nil
}

func parseInternetSearchArgs(args []string) (internet.SearchInput, error) {
	if len(args) == 0 {
		return internet.SearchInput{}, fmt.Errorf("usage: yemaka internet search <query> [--yes] [--limit 5]")
	}
	input := internet.SearchInput{
		Caller: "cli",
	}
	queryParts := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--yes" || arg == "--task-approved":
			input.TaskApproved = true
		case arg == "--limit":
			if i+1 >= len(args) {
				return internet.SearchInput{}, fmt.Errorf("--limit requires a number")
			}
			limit, err := strconv.Atoi(args[i+1])
			if err != nil || limit <= 0 {
				return internet.SearchInput{}, fmt.Errorf("--limit requires a positive number")
			}
			input.MaxResults = limit
			i++
		case strings.HasPrefix(arg, "--limit="):
			limit, err := strconv.Atoi(strings.TrimPrefix(arg, "--limit="))
			if err != nil || limit <= 0 {
				return internet.SearchInput{}, fmt.Errorf("--limit requires a positive number")
			}
			input.MaxResults = limit
		default:
			queryParts = append(queryParts, arg)
		}
	}
	input.Query = strings.TrimSpace(strings.Join(queryParts, " "))
	if input.Query == "" {
		return internet.SearchInput{}, fmt.Errorf("usage: yemaka internet search <query> [--yes] [--limit 5]")
	}
	return input, nil
}

func internetUsage() string {
	return "yemaka internet status | on | off | search-provider status|auto|searxng|tavily|serper|brave|firecrawl|wikimedia|duckduckgo|mojeek|custom|off | fetch <url> [--yes] [--extract-text] [--domain example.com] | head <url> [--yes] | search <query> [--yes] [--limit 5] | cache list | requests [limit]"
}

func internetSearchProviderUsage() string {
	return "yemaka internet search-provider status | auto [--max-results 5] [--enable-internet] | searxng <endpoint> [--max-results 5] [--enable-internet] | tavily|serper|brave|firecrawl|mojeek <api-key-env> [--endpoint <url>] [--max-results 5] [--enable-internet] | wikimedia|duckduckgo [--endpoint <url>] [--max-results 5] [--enable-internet] | custom <endpoint> [--max-results 5] [--enable-internet] | off"
}

func runJob(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", jobUsage())
	}
	store, err := scheduler.Open(ctx, app.profile.Database)
	if err != nil {
		return err
	}
	defer store.Close()

	switch args[0] {
	case "list":
		jobs, err := store.List(ctx)
		if err != nil {
			return err
		}
		jobs, _ = annotateSchedulerJobs(app, jobs)
		if len(jobs) == 0 {
			fmt.Fprintln(stdout, "No jobs registered.")
			return nil
		}
		for _, job := range jobs {
			enabled := "disabled"
			if job.Enabled {
				enabled = "enabled"
			}
			approved := "unapproved"
			if job.Approved {
				approved = "approved"
			}
			detail := ""
			if job.InputStatus == scheduler.InputStatusInvalid {
				detail = "\tinput_invalid: " + oneLine(job.InputValidationError)
			} else if job.InputStatus != "" {
				detail = "\tinput_" + job.InputStatus
			}
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\t%s%s\n", job.ID, enabled, approved, job.ScheduleType, job.ScheduleExpr, job.TargetType, job.TargetName, detail)
		}
		return nil
	case "create":
		input, err := parseJobCreateArgs(args[1:], app.config.Scheduler.RequireApprovalForNewJobs)
		if err != nil {
			return err
		}
		if input.TargetType == scheduler.TargetExtension {
			if err := extensionStore(app).ValidateKnownRunInput(input.TargetName, input.Input); err != nil {
				return err
			}
		}
		job, err := store.Create(ctx, input)
		if err != nil {
			return err
		}
		return printJSON(stdout, job)
	case "update-input":
		input, err := parseJobUpdateInputArgs(args[1:], true)
		if err != nil {
			return err
		}
		existing, err := store.Get(ctx, input.ID)
		if err != nil {
			return err
		}
		if existing.TargetType == scheduler.TargetExtension {
			if err := extensionStore(app).ValidateKnownRunInput(existing.TargetName, input.Input); err != nil {
				return err
			}
		}
		job, err := store.UpdateInput(ctx, input)
		if err != nil {
			return err
		}
		annotated, _ := annotateSchedulerJobs(app, []scheduler.Job{job})
		if len(annotated) == 1 {
			job = annotated[0]
		}
		return printJSON(stdout, job)
	case "approve":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka job approve <job_id>")
		}
		job, err := store.Approve(ctx, args[1])
		if err != nil {
			return err
		}
		return printJSON(stdout, job)
	case "enable":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka job enable <job_id>")
		}
		existing, err := store.Get(ctx, args[1])
		if err != nil {
			return err
		}
		if existing.TargetType == scheduler.TargetExtension {
			if err := extensionStore(app).ValidateKnownRunInput(existing.TargetName, existing.Input); err != nil {
				return err
			}
		}
		job, err := store.SetEnabled(ctx, args[1], true)
		if err != nil {
			return err
		}
		return printJSON(stdout, job)
	case "disable":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka job disable <job_id>")
		}
		job, err := store.SetEnabled(ctx, args[1], false)
		if err != nil {
			return err
		}
		return printJSON(stdout, job)
	case "archive":
		if len(args) != 3 || args[2] != "--yes" {
			return fmt.Errorf("usage: yemaka job archive <job_id> --yes")
		}
		job, err := store.Archive(ctx, args[1])
		if err != nil {
			return err
		}
		return printJSON(stdout, job)
	case "run":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka job run <job_id>")
		}
		job, jobErr := store.Get(ctx, args[1])
		if jobErr == nil && job.TargetType == scheduler.TargetExtension {
			if err := extensionStore(app).ValidateKnownRunInput(job.TargetName, job.Input); err != nil {
				return err
			}
		}
		run, err := store.Run(ctx, args[1], jobRunner(app), jobRunOptions(app))
		if jobErr == nil {
			recordJobRunNotification(app, job, run)
		}
		if jobErr == nil && app.store != nil {
			if _, recordErr := agent.RecordJobRunResult(ctx, app.store, job, run, ""); recordErr != nil && err == nil {
				return recordErr
			}
		}
		if err != nil {
			_ = printJSON(stdout, run)
			return err
		}
		return printJSON(stdout, run)
	case "runs":
		limit := 20
		if len(args) == 2 {
			parsed, err := strconv.Atoi(args[1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("usage: yemaka job runs [limit]")
			}
			limit = parsed
		} else if len(args) > 2 {
			return fmt.Errorf("usage: yemaka job runs [limit]")
		}
		runs, err := store.LastRuns(ctx, limit)
		if err != nil {
			return err
		}
		return printJSON(stdout, runs)
	case "tick":
		if len(args) != 1 {
			return fmt.Errorf("usage: yemaka job tick")
		}
		loop := scheduler.Loop{
			Store:   store,
			Runner:  jobRunner(app),
			Options: jobRunOptions(app),
		}
		result, err := loop.Tick(ctx)
		if err != nil {
			return err
		}
		if err := recordTickJobRuns(ctx, app, store, result.Runs); err != nil {
			return err
		}
		return printJSON(stdout, result)
	case "loop":
		interval, err := parseLoopInterval(args[1:], time.Minute, "yemaka job loop [--interval 1m]")
		if err != nil {
			return err
		}
		return runJobLoop(ctx, app, store, interval, stdout)
	case "status":
		status, err := store.Status(ctx, app.config.Scheduler.Enabled, schedulerMaxParallel(app))
		if err != nil {
			return err
		}
		if jobs, listErr := store.List(ctx); listErr == nil {
			_, invalid := annotateSchedulerJobs(app, jobs)
			status.InvalidInputJobs = invalid
		}
		return printJSON(stdout, status)
	default:
		return fmt.Errorf("usage: %s", jobUsage())
	}
}

func runHeartbeat(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", heartbeatUsage())
	}
	store, err := heartbeat.Open(ctx, app.profile.Database)
	if err != nil {
		return err
	}
	defer store.Close()

	switch args[0] {
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("usage: yemaka heartbeat status")
		}
		report, err := heartbeatReport(ctx, app)
		if err != nil {
			return err
		}
		previous, _ := store.Latest(ctx)
		if err := store.Record(ctx, report); err != nil {
			return err
		}
		recordHeartbeatTransitionNotification(app, previous, report)
		return printJSON(stdout, report)
	case "loop":
		defaultInterval := time.Duration(app.config.Heartbeat.IntervalSeconds) * time.Second
		if defaultInterval <= 0 {
			defaultInterval = time.Minute
		}
		interval, err := parseLoopInterval(args[1:], defaultInterval, "yemaka heartbeat loop [--interval 60s]")
		if err != nil {
			return err
		}
		return runHeartbeatLoop(ctx, app, store, interval, stdout)
	default:
		return fmt.Errorf("usage: %s", heartbeatUsage())
	}
}

func runJobLoop(ctx context.Context, app *appContext, store *scheduler.Store, interval time.Duration, stdout io.Writer) error {
	loop := scheduler.Loop{
		Store:    store,
		Runner:   jobRunner(app),
		Options:  jobRunOptions(app),
		Interval: interval,
	}
	for {
		result, err := loop.Tick(ctx)
		if err != nil {
			return err
		}
		if err := recordTickJobRuns(ctx, app, store, result.Runs); err != nil {
			return err
		}
		if err := printJSON(stdout, result); err != nil {
			return err
		}
		if err := waitLoopInterval(ctx, interval); err != nil {
			return err
		}
	}
}

func recordTickJobRuns(ctx context.Context, app *appContext, store *scheduler.Store, runs []scheduler.JobRun) error {
	for _, run := range runs {
		if strings.TrimSpace(run.ID) == "" {
			continue
		}
		job, err := store.Get(ctx, run.JobID)
		if err == nil {
			recordJobRunNotification(app, job, run)
		}
		if app.store == nil {
			continue
		}
		if _, err := agent.RecordJobRunResultFromStore(ctx, app.store, store, run, ""); err != nil {
			return err
		}
	}
	return nil
}

func recordJobRunNotification(app *appContext, job scheduler.Job, run scheduler.JobRun) {
	if app == nil || app.profile == nil {
		return
	}
	_, _, _ = notifications.RecordJobRunNotification(notifications.NewStore(app.profile.Notifications), job, run)
}

func runHeartbeatLoop(ctx context.Context, app *appContext, store *heartbeat.Store, interval time.Duration, stdout io.Writer) error {
	loop := heartbeat.Loop{
		Enabled: app.config.Heartbeat.Enabled,
		Store:   store,
		Reporter: heartbeat.ReporterFunc(func(ctx context.Context) (heartbeat.Report, error) {
			return heartbeatReport(ctx, app)
		}),
		Interval: interval,
	}
	for {
		previous, _ := store.Latest(ctx)
		report, err := loop.Tick(ctx)
		if err != nil {
			return err
		}
		recordHeartbeatTransitionNotification(app, previous, report)
		if err := printJSON(stdout, report); err != nil {
			return err
		}
		if err := waitLoopInterval(ctx, interval); err != nil {
			return err
		}
	}
}

func recordHeartbeatTransitionNotification(app *appContext, previous heartbeat.Report, report heartbeat.Report) {
	if app == nil || app.profile == nil {
		return
	}
	_, _ = notifications.RecordHeartbeatTransitionNotifications(notifications.NewStore(app.profile.Notifications), previous, report)
}

func waitLoopInterval(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func parseLoopInterval(args []string, fallback time.Duration, usage string) (time.Duration, error) {
	if len(args) == 0 {
		if fallback <= 0 {
			return time.Minute, nil
		}
		return fallback, nil
	}
	if len(args) == 1 && strings.HasPrefix(args[0], "--interval=") {
		return parsePositiveDuration(strings.TrimPrefix(args[0], "--interval="), usage)
	}
	if len(args) == 2 && args[0] == "--interval" {
		return parsePositiveDuration(args[1], usage)
	}
	return 0, fmt.Errorf("usage: %s", usage)
}

func parsePositiveDuration(value string, usage string) (time.Duration, error) {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("usage: %s", usage)
	}
	return duration, nil
}

func parseJobCreateArgs(args []string, requireApproval bool) (scheduler.CreateInput, error) {
	if len(args) < 2 {
		return scheduler.CreateInput{}, fmt.Errorf("usage: yemaka job create <manual|interval|cron|one_time> <target_name> [--every 1h] [--cron \"*/5 * * * *\"] [--at RFC3339] [--target-type extension|heartbeat] [--input-json '{}'] [--enable] --yes")
	}
	input := scheduler.CreateInput{
		ScheduleType: strings.TrimSpace(args[0]),
		TargetName:   strings.TrimSpace(args[1]),
		TargetType:   scheduler.TargetExtension,
		Input:        map[string]any{},
	}
	if input.TargetName == scheduler.TargetHeartbeat {
		input.TargetType = scheduler.TargetHeartbeat
	}
	for i := 2; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--yes" || arg == "--approved":
			input.Approved = true
		case arg == "--enable":
			input.Enabled = true
		case arg == "--name":
			if i+1 >= len(args) {
				return scheduler.CreateInput{}, fmt.Errorf("--name requires a value")
			}
			input.Name = args[i+1]
			i++
		case strings.HasPrefix(arg, "--name="):
			input.Name = strings.TrimPrefix(arg, "--name=")
		case arg == "--target-type":
			if i+1 >= len(args) {
				return scheduler.CreateInput{}, fmt.Errorf("--target-type requires a value")
			}
			input.TargetType = args[i+1]
			i++
		case strings.HasPrefix(arg, "--target-type="):
			input.TargetType = strings.TrimPrefix(arg, "--target-type=")
		case arg == "--every":
			if i+1 >= len(args) {
				return scheduler.CreateInput{}, fmt.Errorf("--every requires a duration")
			}
			input.ScheduleExpr = args[i+1]
			i++
		case strings.HasPrefix(arg, "--every="):
			input.ScheduleExpr = strings.TrimPrefix(arg, "--every=")
		case arg == "--cron":
			if i+1 >= len(args) {
				return scheduler.CreateInput{}, fmt.Errorf("--cron requires a cron expression")
			}
			input.ScheduleExpr = args[i+1]
			i++
		case strings.HasPrefix(arg, "--cron="):
			input.ScheduleExpr = strings.TrimPrefix(arg, "--cron=")
		case arg == "--at":
			if i+1 >= len(args) {
				return scheduler.CreateInput{}, fmt.Errorf("--at requires an RFC3339 timestamp")
			}
			input.ScheduleExpr = args[i+1]
			i++
		case strings.HasPrefix(arg, "--at="):
			input.ScheduleExpr = strings.TrimPrefix(arg, "--at=")
		case arg == "--input-json":
			if i+1 >= len(args) {
				return scheduler.CreateInput{}, fmt.Errorf("--input-json requires a JSON object")
			}
			if err := json.Unmarshal([]byte(args[i+1]), &input.Input); err != nil {
				return scheduler.CreateInput{}, fmt.Errorf("input-json must be a JSON object: %w", err)
			}
			i++
		case strings.HasPrefix(arg, "--input-json="):
			if err := json.Unmarshal([]byte(strings.TrimPrefix(arg, "--input-json=")), &input.Input); err != nil {
				return scheduler.CreateInput{}, fmt.Errorf("input-json must be a JSON object: %w", err)
			}
		default:
			return scheduler.CreateInput{}, fmt.Errorf("unknown job create option: %s", arg)
		}
	}
	if input.ScheduleType == scheduler.ScheduleManual && input.ScheduleExpr == "" {
		input.ScheduleExpr = ""
	}
	if requireApproval && !input.Approved {
		return scheduler.CreateInput{}, fmt.Errorf("use --yes to approve creating a local scheduled job")
	}
	if !requireApproval {
		input.Approved = true
	}
	return input, nil
}

func parseJobUpdateInputArgs(args []string, requireApproval bool) (scheduler.UpdateInput, error) {
	if len(args) < 1 {
		return scheduler.UpdateInput{}, fmt.Errorf("usage: yemaka job update-input <job_id> --input-json '{}' --yes")
	}
	input := scheduler.UpdateInput{
		ID:    strings.TrimSpace(args[0]),
		Input: map[string]any{},
	}
	approved := false
	seenInput := false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--yes" || arg == "--approved":
			approved = true
		case arg == "--input-json":
			if i+1 >= len(args) {
				return scheduler.UpdateInput{}, fmt.Errorf("--input-json requires a JSON object")
			}
			if err := json.Unmarshal([]byte(args[i+1]), &input.Input); err != nil {
				return scheduler.UpdateInput{}, fmt.Errorf("input-json must be a JSON object: %w", err)
			}
			seenInput = true
			i++
		case strings.HasPrefix(arg, "--input-json="):
			if err := json.Unmarshal([]byte(strings.TrimPrefix(arg, "--input-json=")), &input.Input); err != nil {
				return scheduler.UpdateInput{}, fmt.Errorf("input-json must be a JSON object: %w", err)
			}
			seenInput = true
		default:
			return scheduler.UpdateInput{}, fmt.Errorf("unknown job update-input option: %s", arg)
		}
	}
	if strings.TrimSpace(input.ID) == "" {
		return scheduler.UpdateInput{}, fmt.Errorf("job id is required")
	}
	if !seenInput {
		return scheduler.UpdateInput{}, fmt.Errorf("--input-json is required")
	}
	if requireApproval && !approved {
		return scheduler.UpdateInput{}, fmt.Errorf("use --yes to approve updating the local scheduled job input")
	}
	return input, nil
}

func annotateSchedulerJobs(app *appContext, jobs []scheduler.Job) ([]scheduler.Job, int) {
	return scheduler.AnnotateJobInputStatuses(jobs, func(job scheduler.Job) (bool, error) {
		if strings.TrimSpace(job.TargetType) != scheduler.TargetExtension {
			return false, nil
		}
		err := extensionStore(app).ValidateRunInput(job.TargetName, job.Input)
		if errors.Is(err, extensions.ErrExtensionNotFound) {
			return false, nil
		}
		return true, err
	})
}

func jobRunner(app *appContext) scheduler.Runner {
	return scheduler.RunnerFunc(func(ctx context.Context, job scheduler.Job) (any, error) {
		if err := scheduler.ValidateTarget(job.TargetType, job.TargetName); err != nil {
			return nil, err
		}
		switch job.TargetType {
		case scheduler.TargetExtension:
			if err := extensionStore(app).ValidateRunInput(job.TargetName, job.Input); err != nil {
				return nil, err
			}
			result, err := extensionStore(app).Run(ctx, job.TargetName, extensions.RunOptions{
				Input:             job.Input,
				MaxRuntimeSeconds: app.config.Scheduler.DefaultJobTimeoutSeconds,
				Internet:          internetService(app),
			})
			if err != nil {
				return result, err
			}
			return result, nil
		case scheduler.TargetHeartbeat:
			report, err := heartbeatReport(ctx, app)
			if err != nil {
				return report, err
			}
			return report, nil
		default:
			return nil, fmt.Errorf("unsupported job target type: %s", job.TargetType)
		}
	})
}

func heartbeatReport(ctx context.Context, app *appContext) (heartbeat.Report, error) {
	schedule, err := scheduler.Open(ctx, app.profile.Database)
	if err != nil {
		return heartbeat.Report{}, err
	}
	defer schedule.Close()
	schedulerStatus, err := schedule.Status(ctx, app.config.Scheduler.Enabled, schedulerMaxParallel(app))
	if err != nil {
		return heartbeat.Report{}, err
	}
	return heartbeat.Run(ctx, heartbeat.Input{
		Config:          app.config,
		Profile:         app.profile,
		Memory:          app.store,
		Runtime:         app.runtime,
		SchedulerStatus: &schedulerStatus,
		ExtensionStore:  extensionStore(app),
	}), nil
}

func jobRunOptions(app *appContext) scheduler.RunOptions {
	return scheduler.RunOptions{
		Enabled:        app.config.Scheduler.Enabled,
		TimeoutSeconds: app.config.Scheduler.DefaultJobTimeoutSeconds,
		MaxParallel:    schedulerMaxParallel(app),
		LowMemoryMode:  app.config.Runtime.LowMemoryMode,
		PolicyMode:     policyMode(app.config),
	}
}

func schedulerMaxParallel(app *appContext) int {
	maxParallel := app.config.Scheduler.MaxParallelJobs
	if app.config.Runtime.LowMemoryMode && app.config.Scheduler.LowMemoryMaxParallelJobs > 0 {
		maxParallel = app.config.Scheduler.LowMemoryMaxParallelJobs
	}
	if maxParallel <= 0 {
		return 1
	}
	return maxParallel
}

func jobUsage() string {
	return "yemaka job list | create <manual|interval|cron|one_time> <target_name> [--every 1h] [--target-type extension|heartbeat] --yes | update-input <job_id> --input-json '{}' --yes | approve <job_id> | enable <job_id> | disable <job_id> | archive <job_id> --yes | run <job_id> | runs [limit] | tick | loop [--interval 1m] | status"
}

func heartbeatUsage() string {
	return "yemaka heartbeat status | loop [--interval 60s]"
}

func connectorMCPHandlers(app *appContext) connectors.MCPHandlers {
	return connectors.MCPHandlers{
		Chat: func(ctx context.Context, content string) (connectors.CoreResult, error) {
			return connectorChat(ctx, app, content)
		},
		Ask: func(ctx context.Context, content string, skill string) (connectors.CoreResult, error) {
			return connectorAsk(ctx, app, content, skill)
		},
		MemorySearch: func(ctx context.Context, query string, limit int) ([]connectors.MemoryResult, error) {
			memories, err := app.store.SearchMemories(ctx, query, limit)
			if err != nil {
				return nil, err
			}
			result := make([]connectors.MemoryResult, 0, len(memories))
			for _, item := range memories {
				result = append(result, connectors.MemoryResult{
					ID:         item.ID,
					Kind:       item.Kind,
					Content:    item.Content,
					Snippet:    item.Snippet,
					Importance: item.Importance,
					Source:     item.Source,
					Pinned:     item.Pinned,
					CreatedAt:  item.CreatedAt,
				})
			}
			return result, nil
		},
	}
}

func connectorChat(ctx context.Context, app *appContext, content string) (connectors.CoreResult, error) {
	result := connectors.CoreResult{}
	service := &agent.Service{
		Router:           app.router,
		Runtime:          app.runtime,
		RuntimeFactory:   app.runtimeFactory,
		Memory:           app.store,
		CloudFallback:    cloudFallback(app.config, app.cloud),
		ToolExecutor:     cliToolExecutor(app),
		KnowledgeContext: agent.NewLocalKnowledgeContextProvider(agent.LocalKnowledgeContextConfig{Config: app.config}),
		CapabilityGap:    agent.NewCapabilityGapRouter(app.config, extensionStore(app)),
		ReplayStore:      agentReplayStore(app),
		ModelProfileDir:  modelProfileDir(app),
		PromptOptions:    agent.PromptOptionsFromConfig(app.config),
		PolicyMode:       policyMode(app.config),
		InternetTools:    agent.InternetToolLoopEnabled(app.config, policyMode(app.config)),
		InternetSearch:   agent.InternetSearchLoopEnabled(app.config, policyMode(app.config)),
	}
	err := service.Chat(ctx, agent.ChatInput{Content: content}, func(event agent.Event) error {
		switch event.Type {
		case agent.EventModelSelected:
			result.Model = event.Data["model"]
		case agent.EventModelToken:
			result.Text += event.Token
		case agent.EventAgentCompleted:
			result.ConversationID = event.Data["conversation_id"]
		case agent.EventAgentError:
			return errors.New(event.Message)
		}
		return nil
	})
	return result, err
}

func connectorAsk(ctx context.Context, app *appContext, content string, skillName string) (connectors.CoreResult, error) {
	selectedSkill, hasSkill, err := selectSkill(app, skillName, content)
	if err != nil {
		return connectors.CoreResult{}, err
	}
	skillInstructions := ""
	if hasSkill {
		skillInstructions = skills.PromptBlock(selectedSkill)
	}
	result := connectors.CoreResult{}
	if hasSkill {
		result.Skill = selectedSkill.Name
	}
	service := &agent.Service{
		Router:           app.router,
		Runtime:          app.runtime,
		RuntimeFactory:   app.runtimeFactory,
		Memory:           app.store,
		CloudFallback:    cloudFallback(app.config, app.cloud),
		ToolExecutor:     cliToolExecutor(app),
		Retriever:        cliRetriever(app),
		KnowledgeContext: agent.NewLocalKnowledgeContextProvider(agent.LocalKnowledgeContextConfig{Config: app.config}),
		CapabilityGap:    agent.NewCapabilityGapRouter(app.config, extensionStore(app)),
		ReplayStore:      agentReplayStore(app),
		ModelProfileDir:  modelProfileDir(app),
		PromptOptions:    agent.PromptOptionsFromConfig(app.config),
		PolicyMode:       policyMode(app.config),
		InternetTools:    agent.InternetToolLoopEnabled(app.config, policyMode(app.config)),
		InternetSearch:   agent.InternetSearchLoopEnabled(app.config, policyMode(app.config)),
	}
	err = service.Ask(ctx, agent.AskInput{
		Content:            content,
		SkillName:          selectedSkill.Name,
		SkillVersion:       selectedSkill.Version,
		SkillRequiredTools: selectedSkill.RequiredTools,
		SkillInstructions:  skillInstructions,
	}, func(event agent.Event) error {
		switch event.Type {
		case agent.EventModelSelected:
			result.Model = event.Data["model"]
		case agent.EventWorkspaceUsed:
			result.Sources = splitEventSources(event.Data["sources"])
			result.SourceKind = event.Data["source_kind"]
		case agent.EventModelToken:
			result.Text += event.Token
		case agent.EventAgentCompleted:
			result.ConversationID = event.Data["conversation_id"]
		case agent.EventAgentError:
			return errors.New(event.Message)
		}
		return nil
	})
	return result, err
}

func parseConnectorTokenEnv(args []string) (string, error) {
	tokenEnv := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--token-env":
			if i+1 >= len(args) {
				return "", fmt.Errorf("--token-env requires an environment variable name")
			}
			tokenEnv = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--token-env="):
			tokenEnv = strings.TrimSpace(strings.TrimPrefix(arg, "--token-env="))
		default:
			return "", fmt.Errorf("unknown connector option: %s", arg)
		}
	}
	if secrets.LooksLikeSecretValue(tokenEnv) {
		return "", fmt.Errorf("--token-env must be an environment variable name, not a secret value")
	}
	return tokenEnv, nil
}

func parseCloudFallbackOnArgs(current config.CloudFallbackConfig, args []string) (config.CloudFallbackConfig, error) {
	if len(args) < 2 {
		return config.CloudFallbackConfig{}, fmt.Errorf("usage: yemaka cloud fallback on <base-url> <model> [--provider openai_compatible] [--key-env ENV_NAME]")
	}
	cfg := current
	cfg.Enabled = true
	cfg.BaseURL = strings.TrimSpace(args[0])
	cfg.Name = strings.TrimSpace(args[1])
	if cfg.Provider == "" {
		cfg.Provider = "openai_compatible"
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.2
	}
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 45
	}
	if cfg.BaseURL == "" || cfg.Name == "" {
		return config.CloudFallbackConfig{}, fmt.Errorf("base-url and model are required")
	}
	for i := 2; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--provider":
			if i+1 >= len(args) {
				return config.CloudFallbackConfig{}, fmt.Errorf("--provider requires a value")
			}
			cfg.Provider = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--provider="):
			cfg.Provider = strings.TrimSpace(strings.TrimPrefix(arg, "--provider="))
		case arg == "--key-env":
			if i+1 >= len(args) {
				return config.CloudFallbackConfig{}, fmt.Errorf("--key-env requires a value")
			}
			cfg.APIKeyEnv = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--key-env="):
			cfg.APIKeyEnv = strings.TrimSpace(strings.TrimPrefix(arg, "--key-env="))
		case arg == "--timeout":
			if i+1 >= len(args) {
				return config.CloudFallbackConfig{}, fmt.Errorf("--timeout requires seconds")
			}
			timeout, err := parsePositiveInt(args[i+1])
			if err != nil {
				return config.CloudFallbackConfig{}, fmt.Errorf("invalid timeout: %w", err)
			}
			cfg.TimeoutSeconds = timeout
			i++
		case strings.HasPrefix(arg, "--timeout="):
			timeout, err := parsePositiveInt(strings.TrimPrefix(arg, "--timeout="))
			if err != nil {
				return config.CloudFallbackConfig{}, fmt.Errorf("invalid timeout: %w", err)
			}
			cfg.TimeoutSeconds = timeout
		default:
			return config.CloudFallbackConfig{}, fmt.Errorf("unknown cloud fallback option: %s", arg)
		}
	}
	if cfg.Provider != "openai_compatible" && cfg.Provider != "litellm" {
		return config.CloudFallbackConfig{}, fmt.Errorf("provider must be openai_compatible or litellm")
	}
	return cfg, nil
}

func runChat(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	content := strings.TrimSpace(strings.Join(args, " "))
	if content == "" {
		return fmt.Errorf("usage: yemaka chat \"Hello\"")
	}

	service := &agent.Service{
		Router:           app.router,
		Runtime:          app.runtime,
		RuntimeFactory:   app.runtimeFactory,
		Memory:           app.store,
		CloudFallback:    cloudFallback(app.config, app.cloud),
		ToolExecutor:     cliToolExecutor(app),
		KnowledgeContext: agent.NewLocalKnowledgeContextProvider(agent.LocalKnowledgeContextConfig{Config: app.config}),
		CapabilityGap:    agent.NewCapabilityGapRouter(app.config, extensionStore(app)),
		ReplayStore:      agentReplayStore(app),
		ModelProfileDir:  modelProfileDir(app),
		PromptOptions:    agent.PromptOptionsFromConfig(app.config),
		PolicyMode:       policyMode(app.config),
		InternetTools:    agent.InternetToolLoopEnabled(app.config, policyMode(app.config)),
		InternetSearch:   agent.InternetSearchLoopEnabled(app.config, policyMode(app.config)),
	}
	return service.Chat(ctx, agent.ChatInput{Content: content}, func(event agent.Event) error {
		switch event.Type {
		case agent.EventModelSelected:
			fmt.Fprintf(stdout, "model: %s\n\n", event.Data["model"])
		case agent.EventModelToken:
			fmt.Fprint(stdout, event.Token)
		case agent.EventAgentCompleted:
			fmt.Fprintln(stdout)
			fmt.Fprintf(stdout, "\nconversation: %s\n", event.Data["conversation_id"])
		case agent.EventAgentError:
			return errors.New(event.Message)
		}
		return nil
	})
}

func runMemory(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: yemaka memory search|write|list|pin|delete")
	}
	switch args[0] {
	case "search":
		query := strings.TrimSpace(strings.Join(args[1:], " "))
		if query == "" {
			return fmt.Errorf("usage: yemaka memory search \"topic\"")
		}
		memories, err := app.store.SearchMemories(ctx, query, app.config.Memory.MaxRelevantMemories)
		if err != nil {
			return err
		}
		messages, err := app.store.SearchMessages(ctx, query, app.config.Memory.MaxRelevantMemories)
		if err != nil {
			return err
		}
		if len(memories) == 0 && len(messages) == 0 {
			fmt.Fprintln(stdout, "No memory matches yet.")
			return nil
		}
		for _, result := range memories {
			fmt.Fprintf(stdout, "%s\t%s\timportance=%d\t%s\n", result.ID, result.Kind, result.Importance, result.Snippet)
		}
		for _, result := range messages {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", result.CreatedAt, result.Role, result.Snippet)
		}
		return nil
	case "write":
		if len(args) < 3 {
			return fmt.Errorf("usage: yemaka memory write <kind> <content> [--importance N]")
		}
		kind := strings.TrimSpace(args[1])
		contentArgs, importance, err := parseMemoryWriteArgs(args[2:])
		if err != nil {
			return err
		}
		content := strings.TrimSpace(strings.Join(contentArgs, " "))
		item, err := app.store.SaveMemory(ctx, memory.Memory{
			Kind:       kind,
			Content:    content,
			Importance: importance,
			Source:     "cli",
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s\t%s\timportance=%d\n", item.ID, item.Kind, item.Importance)
		return nil
	case "list":
		memories, err := app.store.ListMemories(ctx, app.config.Memory.MaxRelevantMemories)
		if err != nil {
			return err
		}
		if len(memories) == 0 {
			fmt.Fprintln(stdout, "No explicit memories yet.")
			return nil
		}
		for _, result := range memories {
			pinned := ""
			if result.Pinned {
				pinned = "\tpinned"
			}
			fmt.Fprintf(stdout, "%s\t%s\timportance=%d%s\t%s\n", result.ID, result.Kind, result.Importance, pinned, result.Content)
		}
		return nil
	case "pin":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka memory pin <memory_id>")
		}
		if err := app.store.PinMemory(ctx, args[1], true); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "pinned: %s\n", args[1])
		return nil
	case "delete":
		if len(args) != 2 {
			return fmt.Errorf("usage: yemaka memory delete <memory_id>")
		}
		if err := app.store.DisableMemory(ctx, args[1]); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "deleted: %s\n", args[1])
		return nil
	default:
		return fmt.Errorf("usage: yemaka memory search|write|list|pin|delete")
	}
}

func parseMemoryWriteArgs(args []string) ([]string, int, error) {
	importance := 3
	content := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--importance" {
			if index+1 >= len(args) {
				return nil, 0, fmt.Errorf("--importance requires a value")
			}
			value, err := strconv.Atoi(args[index+1])
			if err != nil {
				return nil, 0, fmt.Errorf("importance must be a number: %w", err)
			}
			importance = value
			index++
			continue
		}
		content = append(content, arg)
	}
	if strings.TrimSpace(strings.Join(content, " ")) == "" {
		return nil, 0, fmt.Errorf("memory content is required")
	}
	return content, importance, nil
}

func runWorkspace(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: yemaka workspace scan [path] | summary [path] | grant <path> [label] | grants | revoke <id-or-path>")
	}
	limits := workspaceLimits(app.config.Workspace)

	switch args[0] {
	case "scan":
		root := workspaceArg(args)
		access, err := beginWorkspaceAccess(app, root)
		if err != nil {
			return err
		}
		defer access.Close()
		result, err := workspace.Scan(root, limits)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "workspace: %s\n", result.Root)
		fmt.Fprintf(stdout, "files_indexed: %d\n", len(result.Files))
		fmt.Fprintf(stdout, "files_skipped: %d\n", len(result.Skipped))
		fmt.Fprintf(stdout, "indexed_bytes: %d\n", result.TotalIndexedSize)
		fmt.Fprintf(stdout, "limit_reached: %t\n", result.LimitReached)
		if len(result.LanguageCounts) > 0 {
			fmt.Fprintln(stdout, "languages:")
			for _, line := range languageLines(result.LanguageCounts) {
				fmt.Fprintf(stdout, "  %s\n", line)
			}
		}
		return nil
	case "summary":
		root := workspaceArg(args)
		access, err := beginWorkspaceAccess(app, root)
		if err != nil {
			return err
		}
		defer access.Close()
		result, err := workspace.Scan(root, limits)
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, workspace.Summary(result))
		return nil
	case "grant":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka workspace grant <path> [label]")
		}
		label := strings.TrimSpace(strings.Join(args[2:], " "))
		grant, err := workspaceGrantStore(app).Grant(args[1], label, "cli")
		if err != nil {
			_, _ = logToolRun(ctx, app, "workspace_grant", map[string]any{"path": args[1]}, map[string]any{"error": err.Error()}, "failed", "low")
			return err
		}
		_, _ = logToolRun(ctx, app, "workspace_grant", map[string]any{"path": grant.Path, "label": grant.Label}, map[string]any{"id": grant.ID}, "completed", "low")
		fmt.Fprintf(stdout, "granted: %s\n", grant.ID)
		fmt.Fprintf(stdout, "path: %s\n", grant.Path)
		if grant.Label != "" {
			fmt.Fprintf(stdout, "label: %s\n", grant.Label)
		}
		return nil
	case "grants":
		grants, err := workspaceGrantStore(app).List()
		if err != nil {
			return err
		}
		if len(grants) == 0 {
			fmt.Fprintln(stdout, "No workspace grants.")
			return nil
		}
		for _, grant := range grants {
			label := grant.Label
			if label == "" {
				label = "-"
			}
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", grant.ID, grant.Path, label, grant.CreatedAt)
		}
		return nil
	case "revoke":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka workspace revoke <id-or-path>")
		}
		grant, ok, err := workspaceGrantStore(app).Revoke(args[1])
		if err != nil {
			_, _ = logToolRun(ctx, app, "workspace_revoke", map[string]any{"match": args[1]}, map[string]any{"error": err.Error()}, "failed", "low")
			return err
		}
		if !ok {
			return fmt.Errorf("workspace grant not found: %s", args[1])
		}
		_, _ = logToolRun(ctx, app, "workspace_revoke", map[string]any{"match": args[1]}, map[string]any{"id": grant.ID, "path": grant.Path}, "completed", "low")
		fmt.Fprintf(stdout, "revoked: %s\n", grant.ID)
		fmt.Fprintf(stdout, "path: %s\n", grant.Path)
		return nil
	default:
		return fmt.Errorf("usage: yemaka workspace scan [path] | summary [path] | grant <path> [label] | grants | revoke <id-or-path>")
	}
}

func runFile(ctx context.Context, app *appContext, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: yemaka file read <path> | yemaka file search <query> | yemaka file write <path> [content] | yemaka file edit <path> [content]")
	}
	limits := workspaceLimits(app.config.Workspace)

	switch args[0] {
	case "read":
		path := strings.TrimSpace(strings.Join(args[1:], " "))
		if path == "" {
			return fmt.Errorf("usage: yemaka file read <path>")
		}
		result, err := workspace.ReadFile(ctx, ".", path, limits)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "# %s (%d bytes)\n", result.Path, result.Size)
		fmt.Fprint(stdout, result.Content)
		if !strings.HasSuffix(result.Content, "\n") {
			fmt.Fprintln(stdout)
		}
		return nil
	case "search":
		query := strings.TrimSpace(strings.Join(args[1:], " "))
		if query == "" {
			return fmt.Errorf("usage: yemaka file search <query>")
		}
		matches, err := workspace.SearchFiles(ctx, ".", query, limits)
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			fmt.Fprintln(stdout, "No file matches.")
			return nil
		}
		for _, match := range matches {
			fmt.Fprintf(stdout, "%s:%d: %s\n", match.Path, match.Line, match.Text)
		}
		return nil
	case "write", "edit":
		return runFileWrite(ctx, app, args, stdout, stderr)
	default:
		return fmt.Errorf("usage: yemaka file read <path> | yemaka file search <query> | yemaka file write <path> [content] | yemaka file edit <path> [content]")
	}
}

func runFileWrite(ctx context.Context, app *appContext, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: yemaka file %s <path> [content] [--yes]", args[0])
	}
	path := args[1]
	rest := args[2:]
	yes := false
	filtered := rest[:0]
	for _, arg := range rest {
		if arg == "--yes" || arg == "-y" {
			yes = true
			continue
		}
		filtered = append(filtered, arg)
	}
	content := strings.Join(filtered, " ")
	if len(filtered) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		content = string(data)
	}

	options := writeOptions(app)
	plan, err := workspace.PlanWrite(ctx, ".", path, content, options)
	if err != nil {
		_, _ = logToolRun(ctx, app, args[0]+"_file", map[string]any{"path": path}, map[string]any{"error": err.Error()}, "blocked", "medium")
		return err
	}
	fmt.Fprint(stdout, plan.Diff)
	if !strings.HasSuffix(plan.Diff, "\n") {
		fmt.Fprintln(stdout)
	}
	if !plan.Changed {
		fmt.Fprintln(stdout, "No changes to apply.")
		return nil
	}
	if app.config.Tools.Filesystem.RequireConfirmation && !yes {
		approved, err := confirm(stderr, os.Stdin, "Apply this file change? Type yes to continue: ")
		if err != nil {
			return err
		}
		if !approved {
			_, _ = logToolRun(ctx, app, args[0]+"_file", map[string]any{"path": path}, map[string]any{"changed": false}, "rejected", "medium")
			fmt.Fprintln(stdout, "Change rejected.")
			return nil
		}
	}

	applied, err := workspace.ApplyWrite(ctx, ".", plan, options)
	if err != nil {
		_, _ = logToolRun(ctx, app, args[0]+"_file", map[string]any{"path": path}, map[string]any{"error": err.Error()}, "failed", "medium")
		return err
	}
	verification := workspace.VerifyAppliedWrite(ctx, ".", applied, options)
	writeStatus := "completed"
	if verification.Status != "pass" {
		writeStatus = verification.Status
	}
	_, _ = logToolRun(ctx, app, args[0]+"_file", map[string]any{"path": applied.Path, "bytes": applied.TargetBytes}, map[string]any{"snapshot_id": applied.SnapshotID, "verification_status": verification.Status}, writeStatus, "medium")
	_, _ = logToolRun(ctx, app, "write_verifier", map[string]any{"path": applied.Path, "snapshot_id": applied.SnapshotID}, verification, verification.Status, "medium")
	if applied.SnapshotID != "" {
		fmt.Fprintf(stdout, "snapshot: %s\n", applied.SnapshotID)
	}
	fmt.Fprintf(stdout, "verification: %s\n", verification.Status)
	fmt.Fprintf(stdout, "updated: %s\n", applied.Path)
	return nil
}

func runCoreTool(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: yemaka tool doctor-status | project-map [path] | patch-preview <path> [content] | symbol-search <query> [path] | secret-scan [path]")
	}
	limits := workspaceLimits(app.config.Workspace)
	switch args[0] {
	case "doctor-status":
		text, err := tools.DoctorStatus(ctx, tools.DoctorStatusInput{
			Config:  app.config,
			Profile: app.profile,
			Memory:  app.store,
			Runtime: app.runtime,
		})
		if err != nil {
			_, _ = logToolRun(ctx, app, "doctor_status", map[string]any{}, map[string]any{"error": err.Error()}, "failed", "low")
			return err
		}
		_, _ = logToolRun(ctx, app, "doctor_status", map[string]any{}, map[string]any{"completed": true}, "completed", "low")
		fmt.Fprintln(stdout, text)
		return nil
	case "project-map":
		root := "."
		if len(args) > 1 && strings.TrimSpace(args[1]) != "" {
			root = args[1]
		}
		access, err := beginWorkspaceAccess(app, root)
		if err != nil {
			return err
		}
		defer access.Close()
		result, err := tools.ProjectMap(root, limits)
		if err != nil {
			_, _ = logToolRun(ctx, app, "project_map", map[string]any{"root": root}, map[string]any{"error": err.Error()}, "failed", "low")
			return err
		}
		_, _ = logToolRun(ctx, app, "project_map", map[string]any{"root": root}, map[string]any{"files_indexed": result.FilesIndexed, "files_skipped": result.FilesSkipped, "limit_reached": result.LimitReached}, "completed", "low")
		fmt.Fprintln(stdout, tools.FormatProjectMap(result))
		return nil
	case "patch-preview":
		if len(args) < 2 {
			return fmt.Errorf("usage: yemaka tool patch-preview <path> [content]")
		}
		path := args[1]
		content := strings.Join(args[2:], " ")
		if len(args) == 2 {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			content = string(data)
		}
		diff, err := tools.PatchPreview(ctx, ".", path, content, writeOptions(app))
		if err != nil {
			_, _ = logToolRun(ctx, app, "patch_preview", map[string]any{"path": path}, map[string]any{"error": err.Error()}, "failed", "low")
			return err
		}
		_, _ = logToolRun(ctx, app, "patch_preview", map[string]any{"path": path}, map[string]any{"diff_bytes": len(diff), "changed": false}, "completed", "low")
		fmt.Fprintln(stdout, diff)
		return nil
	case "symbol-search":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka tool symbol-search <query> [path]")
		}
		root := "."
		if len(args) > 2 && strings.TrimSpace(args[2]) != "" {
			root = args[2]
		}
		access, err := beginWorkspaceAccess(app, root)
		if err != nil {
			return err
		}
		defer access.Close()
		matches, err := tools.SymbolSearch(ctx, root, args[1], limits)
		if err != nil {
			_, _ = logToolRun(ctx, app, "symbol_search", map[string]any{"query": args[1], "root": root}, map[string]any{"error": err.Error()}, "failed", "low")
			return err
		}
		_, _ = logToolRun(ctx, app, "symbol_search", map[string]any{"query": args[1], "root": root}, map[string]any{"matches": len(matches)}, "completed", "low")
		if len(matches) == 0 {
			fmt.Fprintln(stdout, "No symbols matched.")
			return nil
		}
		for _, match := range matches {
			fmt.Fprintf(stdout, "%s:%d\t%s\t%s\t%s\n", match.Path, match.Line, match.Kind, match.Name, match.Source)
		}
		return nil
	case "secret-scan":
		root := "."
		if len(args) > 1 && strings.TrimSpace(args[1]) != "" {
			root = args[1]
		}
		access, err := beginWorkspaceAccess(app, root)
		if err != nil {
			return err
		}
		defer access.Close()
		findings, err := tools.SecretScan(ctx, root, limits)
		if err != nil {
			_, _ = logToolRun(ctx, app, "secret_scan", map[string]any{"root": root}, map[string]any{"error": err.Error()}, "failed", "low")
			return err
		}
		_, _ = logToolRun(ctx, app, "secret_scan", map[string]any{"root": root}, map[string]any{"findings": len(findings), "values_hidden": true}, "completed", "low")
		fmt.Fprintln(stdout, tools.FormatSecretFindings(findings))
		return nil
	default:
		return fmt.Errorf("usage: yemaka tool doctor-status | project-map [path] | patch-preview <path> [content] | symbol-search <query> [path] | secret-scan [path]")
	}
}

func runAsk(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	explicitSkill, askArgs, err := parseAskArgs(args)
	if err != nil {
		return err
	}
	content := strings.TrimSpace(strings.Join(askArgs, " "))
	if content == "" {
		return fmt.Errorf("usage: yemaka ask \"Explain this project\"")
	}
	selectedSkill, hasSkill, err := selectSkill(app, explicitSkill, content)
	if err != nil {
		return err
	}
	skillInstructions := ""
	if hasSkill {
		skillInstructions = skills.PromptBlock(selectedSkill)
	}

	service := &agent.Service{
		Router:           app.router,
		Runtime:          app.runtime,
		RuntimeFactory:   app.runtimeFactory,
		Memory:           app.store,
		CloudFallback:    cloudFallback(app.config, app.cloud),
		ToolExecutor:     cliToolExecutor(app),
		Retriever:        cliRetriever(app),
		KnowledgeContext: agent.NewLocalKnowledgeContextProvider(agent.LocalKnowledgeContextConfig{Config: app.config}),
		CapabilityGap:    agent.NewCapabilityGapRouter(app.config, extensionStore(app)),
		ReplayStore:      agentReplayStore(app),
		ModelProfileDir:  modelProfileDir(app),
		PromptOptions:    agent.PromptOptionsFromConfig(app.config),
		PolicyMode:       policyMode(app.config),
		InternetTools:    agent.InternetToolLoopEnabled(app.config, policyMode(app.config)),
		InternetSearch:   agent.InternetSearchLoopEnabled(app.config, policyMode(app.config)),
	}
	return service.Ask(ctx, agent.AskInput{
		Content:            content,
		SkillName:          selectedSkill.Name,
		SkillVersion:       selectedSkill.Version,
		SkillRequiredTools: selectedSkill.RequiredTools,
		SkillInstructions:  skillInstructions,
	}, func(event agent.Event) error {
		switch event.Type {
		case agent.EventModelSelected:
			fmt.Fprintf(stdout, "model: %s\n", event.Data["model"])
		case agent.EventWorkspaceUsed:
			if event.Data["sources"] != "" {
				fmt.Fprintf(stdout, "%s_sources: %s\n\n", event.Data["source_kind"], event.Data["sources"])
			} else {
				fmt.Fprintln(stdout)
			}
		case agent.EventSkillSelected:
			fmt.Fprintf(stdout, "skill: %s@%s\n", event.Data["name"], event.Data["version"])
		case agent.EventModelToken:
			fmt.Fprint(stdout, event.Token)
		case agent.EventAgentCompleted:
			fmt.Fprintln(stdout)
			fmt.Fprintf(stdout, "\nconversation: %s\n", event.Data["conversation_id"])
		case agent.EventAgentError:
			return errors.New(event.Message)
		}
		return nil
	})
}

func runIngest(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	root := "."
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		root = args[0]
	}
	access, err := beginWorkspaceAccess(app, root)
	if err != nil {
		return err
	}
	defer access.Close()
	result, err := app.rag.IngestPath(ctx, app.profile.Name, root, ragConfig(app.config.RAG), workspaceLimits(app.config.Workspace))
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, rag.FormatIngestResult(result))
	return nil
}

func runSkill(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", skillUsage())
	}
	switch args[0] {
	case "list":
		for _, status := range app.skills.Statuses() {
			state := "enabled"
			if !status.Enabled {
				state = "disabled"
			}
			valid := "valid"
			if !status.Valid {
				valid = "invalid"
			}
			source := skillSourceLabel(app, status.Source, status.Dir)
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\tsource=%s\t%s\n", status.Name, status.Version, state, valid, source, status.Description)
		}
		return nil
	case "show":
		if len(args) < 2 {
			return fmt.Errorf("usage: yemaka skill show <name>")
		}
		skill, ok := app.skills.Get(args[1])
		if !ok {
			return fmt.Errorf("skill not found: %s", args[1])
		}
		fmt.Fprintf(stdout, "name: %s\n", skill.Name)
		fmt.Fprintf(stdout, "version: %s\n", skill.Version)
		fmt.Fprintf(stdout, "description: %s\n", skill.Description)
		fmt.Fprintf(stdout, "enabled: %t\n", !skill.Disabled)
		fmt.Fprintf(stdout, "source: %s\n", skillSourceLabel(app, skill.Source, skill.Dir))
		fmt.Fprintf(stdout, "required_tools: %s\n", strings.Join(skill.RequiredTools, ", "))
		fmt.Fprintln(stdout, "instructions:")
		fmt.Fprintln(stdout, skill.Instructions)
		return nil
	case "validate":
		if len(args) < 2 {
			return fmt.Errorf("usage: yemaka skill validate <name>")
		}
		skill, ok := app.skills.Get(args[1])
		if !ok {
			return fmt.Errorf("skill not found: %s", args[1])
		}
		if err := skills.Validate(skill); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "valid: %s@%s\n", skill.Name, skill.Version)
		return nil
	case "create-from-session":
		if len(args) < 2 {
			return fmt.Errorf("usage: yemaka skill create-from-session <conversation_id>")
		}
		created, err := createSkillFromSession(ctx, app, args[1])
		if err != nil {
			return err
		}
		if err := reloadSkills(app); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "created: %s@%s\n", created.Name, created.Version)
		return nil
	case "improve-from-session":
		if len(args) < 3 {
			return fmt.Errorf("usage: yemaka skill improve-from-session <skill_name> <conversation_id>")
		}
		improved, err := improveSkillFromSession(ctx, app, args[1], args[2])
		if err != nil {
			return err
		}
		if err := reloadSkills(app); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "improved: %s@%s\n", improved.Name, improved.Version)
		return nil
	case "disable":
		if len(args) < 2 {
			return fmt.Errorf("usage: yemaka skill disable <name>")
		}
		updated, err := setSkillEnabled(app, args[1], false)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "disabled: %s@%s\n", updated.Name, updated.Version)
		return nil
	case "enable":
		if len(args) < 2 {
			return fmt.Errorf("usage: yemaka skill enable <name>")
		}
		updated, err := setSkillEnabled(app, args[1], true)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "enabled: %s@%s\n", updated.Name, updated.Version)
		return nil
	case "import":
		if len(args) < 2 {
			return fmt.Errorf("usage: yemaka skill import <folder>")
		}
		imported, err := skills.Import(app.profile.Skills, args[1])
		if err != nil {
			return err
		}
		if err := reloadSkills(app); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "imported: %s@%s\n", imported.Name, imported.Version)
		return nil
	case "export":
		if len(args) < 3 {
			return fmt.Errorf("usage: yemaka skill export <name> <folder>")
		}
		skill, ok := app.skills.Get(args[1])
		if !ok {
			return fmt.Errorf("skill not found: %s", args[1])
		}
		path, err := skills.Export(skill, args[2])
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "exported: %s\n", path)
		return nil
	default:
		return fmt.Errorf("usage: %s", skillUsage())
	}
}

func skillUsage() string {
	return "yemaka skill list | show <name> | validate <name> | create-from-session <conversation_id> | improve-from-session <skill_name> <conversation_id> | enable <name> | disable <name> | import <folder> | export <name> <folder>"
}

func reloadSkills(app *appContext) error {
	dirs, err := skillRegistryDirs(app.profile)
	if err != nil {
		return err
	}
	registry, err := skills.LoadRegistry(dirs)
	if err != nil {
		return err
	}
	app.skills = registry
	return nil
}

func skillRegistryDirs(profile *profiles.Profile) ([]string, error) {
	if profile == nil {
		return nil, fmt.Errorf("profile is required")
	}
	packDirs, err := domainpacks.EnabledSkillDirs(filepath.Join(profile.Root, "domain_packs"))
	if err != nil {
		return nil, err
	}
	return skills.ProfileRegistryDirs("skills/default", profile.Skills, packDirs), nil
}

func skillSourceLabel(app *appContext, source string, dir string) string {
	if packName, ok := domainPackNameForSkillDir(app, dir); ok {
		return "domain-pack:" + packName
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return "local"
	}
	return source
}

func domainPackNameForSkillDir(app *appContext, dir string) (string, bool) {
	if app == nil || app.profile == nil || strings.TrimSpace(dir) == "" {
		return "", false
	}
	root, err := filepath.Abs(filepath.Join(app.profile.Root, "domain_packs"))
	if err != nil {
		return "", false
	}
	skillDir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(root, skillDir)
	if err != nil || rel == "." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return "", false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 3 || parts[1] != domainpacks.PackSkillsDir {
		return "", false
	}
	return parts[0], true
}

func setSkillEnabled(app *appContext, name string, enabled bool) (skills.Skill, error) {
	skill, ok := app.skills.Get(name)
	if !ok {
		return skills.Skill{}, fmt.Errorf("skill not found: %s", name)
	}
	updated, err := skills.SetEnabled(app.profile.Skills, skill, enabled)
	if err != nil {
		return skills.Skill{}, err
	}
	if err := reloadSkills(app); err != nil {
		return skills.Skill{}, err
	}
	return updated, nil
}

func createSkillFromSession(ctx context.Context, app *appContext, conversationID string) (skills.Skill, error) {
	messages, err := app.store.ListConversationMessages(ctx, conversationID, app.config.Memory.SummarizeAfter)
	if err != nil {
		return skills.Skill{}, err
	}
	toolRuns, err := app.store.ListToolRunsForConversation(ctx, conversationID, 50)
	if err != nil {
		return skills.Skill{}, err
	}
	transcript := make([]skills.TranscriptMessage, 0, len(messages))
	for _, msg := range messages {
		transcript = append(transcript, skills.TranscriptMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	runs := make([]skills.ToolRunSummary, 0, len(toolRuns))
	for _, run := range toolRuns {
		runs = append(runs, skills.ToolRunSummary{
			ToolName: run.ToolName,
			Status:   run.Status,
		})
	}
	return skills.CreateFromSession(skills.GenerateInput{
		ProfileSkillsDir: app.profile.Skills,
		ConversationID:   conversationID,
		Messages:         transcript,
		ToolRuns:         runs,
	})
}

func improveSkillFromSession(ctx context.Context, app *appContext, skillName string, conversationID string) (skills.Skill, error) {
	skill, ok := app.skills.Get(strings.TrimSpace(skillName))
	if !ok {
		return skills.Skill{}, fmt.Errorf("skill not found: %s", skillName)
	}
	messages, err := app.store.ListConversationMessages(ctx, conversationID, app.config.Memory.SummarizeAfter)
	if err != nil {
		return skills.Skill{}, err
	}
	toolRuns, err := app.store.ListToolRunsForConversation(ctx, conversationID, 50)
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
		ProfileSkillsDir: app.profile.Skills,
		ConversationID:   conversationID,
		Skill:            skill,
		Messages:         transcript,
		ToolRuns:         runs,
	})
}

func runRAG(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", ragUsage())
	}
	switch args[0] {
	case "search":
		query := strings.TrimSpace(strings.Join(args[1:], " "))
		if query == "" {
			return fmt.Errorf("usage: yemaka rag search <query>")
		}
		results, err := app.rag.SearchWithConfig(ctx, query, ragConfig(app.config.RAG), embeddingRuntime(app.runtime))
		if err != nil {
			return err
		}
		if len(results) == 0 {
			fmt.Fprintln(stdout, "No RAG matches.")
			return nil
		}
		for _, result := range results {
			source := result.Source
			if source == "" {
				source = "fts"
			}
			fmt.Fprintf(stdout, "%s\t%.2f\t%s", source, result.Score, result.Path)
			if len(result.Explanation) > 0 {
				fmt.Fprintf(stdout, "\t%s", strings.Join(result.Explanation, ","))
			}
			fmt.Fprintf(stdout, "\t%s\n", oneLine(result.Content))
		}
		return nil
	case "inventory", "list":
		return runRAGInventory(ctx, app, args[1:], stdout)
	case "prune-missing":
		return runRAGPruneMissing(ctx, app, args[1:], stdout)
	case "embeddings":
		return runRAGEmbeddings(ctx, app, args[1:], stdout)
	case "vector":
		return runRAGVector(ctx, app, args[1:], stdout)
	default:
		return fmt.Errorf("usage: %s", ragUsage())
	}
}

func runRAGInventory(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	jsonOutput := false
	limit := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--limit":
			i++
			if i >= len(args) {
				return fmt.Errorf("usage: yemaka rag inventory [--limit N] [--json]")
			}
			parsed, err := strconv.Atoi(args[i])
			if err != nil {
				return fmt.Errorf("invalid rag inventory limit: %w", err)
			}
			limit = parsed
		default:
			return fmt.Errorf("unknown rag inventory option: %s", args[i])
		}
	}
	items, err := app.rag.ListDocuments(ctx, limit)
	if err != nil {
		return err
	}
	if jsonOutput {
		return printJSON(stdout, items)
	}
	if len(items) == 0 {
		fmt.Fprintln(stdout, "No indexed RAG documents.")
		return nil
	}
	for _, item := range items {
		fmt.Fprintf(stdout, "%s\t%s\tchunks:%d\tembeddings:%d\t%s", item.Status, item.ManagedSource, item.ChunkCount, item.EmbeddingCount, item.Path)
		if item.Reason != "" {
			fmt.Fprintf(stdout, "\t%s", item.Reason)
		}
		fmt.Fprintln(stdout)
	}
	return nil
}

func runRAGPruneMissing(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	jsonOutput := false
	yes := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		case "--dry-run":
			yes = false
		case "--yes":
			yes = true
		default:
			return fmt.Errorf("unknown rag prune-missing option: %s", arg)
		}
	}
	result, err := app.rag.PruneMissingDocuments(ctx, !yes)
	if err != nil {
		return err
	}
	if jsonOutput {
		return printJSON(stdout, result)
	}
	printRAGPruneResult(stdout, result)
	if result.DryRun && result.DocumentsMatched > 0 {
		fmt.Fprintln(stdout, "rerun with `yemaka rag prune-missing --yes` to remove missing documents")
	}
	return nil
}

func printRAGPruneResult(stdout io.Writer, result rag.DocumentPruneResult) {
	fmt.Fprintf(stdout, "dry_run: %t\n", result.DryRun)
	fmt.Fprintf(stdout, "documents_matched: %d\n", result.DocumentsMatched)
	fmt.Fprintf(stdout, "chunks_matched: %d\n", result.ChunksMatched)
	fmt.Fprintf(stdout, "fts_rows_matched: %d\n", result.FTSRowsMatched)
	fmt.Fprintf(stdout, "embeddings_matched: %d\n", result.EmbeddingsMatched)
	fmt.Fprintf(stdout, "documents_removed: %d\n", result.DocumentsRemoved)
	fmt.Fprintf(stdout, "chunks_removed: %d\n", result.ChunksRemoved)
	fmt.Fprintf(stdout, "fts_rows_removed: %d\n", result.FTSRowsRemoved)
	fmt.Fprintf(stdout, "embeddings_removed: %d\n", result.EmbeddingsRemoved)
}

func runRAGEmbeddings(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: yemaka rag embeddings status|on <installed-model>|off|index")
	}
	switch args[0] {
	case "status":
		printEmbeddingStatus(stdout, app.config.RAG.Embeddings)
		return nil
	case "on":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: yemaka rag embeddings on <installed-model>")
		}
		model := strings.TrimSpace(args[1])
		if err := verifyInstalledLocalModel(ctx, app.runtime, model); err != nil {
			return err
		}
		app.config.RAG.Embeddings.Enabled = true
		app.config.RAG.Embeddings.Provider = "ollama"
		app.config.RAG.Embeddings.Model = model
		if err := config.Write(app.config.Path, app.config); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "rag embeddings enabled: %s\n", model)
		fmt.Fprintln(stdout, "run `yemaka rag embeddings index` to embed existing chunks")
		return nil
	case "off":
		app.config.RAG.Embeddings.Enabled = false
		if err := config.Write(app.config.Path, app.config); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "rag embeddings disabled; SQLite FTS5 remains active")
		return nil
	case "index":
		cfg := ragConfig(app.config.RAG)
		if !cfg.Embeddings.Enabled {
			return fmt.Errorf("embeddings are disabled; run `yemaka rag embeddings on <installed-model>` first")
		}
		result, err := app.rag.EnsureEmbeddings(ctx, cfg, embeddingRuntime(app.runtime))
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "enabled: %t\n", result.Enabled)
		fmt.Fprintf(stdout, "provider: %s\n", result.Provider)
		fmt.Fprintf(stdout, "model: %s\n", result.Model)
		fmt.Fprintf(stdout, "chunks_embedded: %d\n", result.ChunksEmbedded)
		fmt.Fprintf(stdout, "chunks_skipped: %d\n", result.ChunksSkipped)
		fmt.Fprintf(stdout, "dimensions: %d\n", result.Dimensions)
		return nil
	default:
		return fmt.Errorf("usage: yemaka rag embeddings status|on <installed-model>|off|index")
	}
}

func runRAGVector(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: yemaka rag vector status|evaluate [--json]")
	}
	command := args[0]
	jsonOutput := false
	for _, arg := range args[1:] {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		return fmt.Errorf("unknown rag vector option: %s", arg)
	}
	if command != "status" && command != "evaluate" {
		return fmt.Errorf("usage: yemaka rag vector status|evaluate [--json]")
	}
	report, err := app.rag.EvaluateVectorDB(ctx, ragConfig(app.config.RAG))
	if err != nil {
		return err
	}
	if jsonOutput {
		return printJSON(stdout, report)
	}
	if command == "status" {
		printVectorStatus(stdout, report)
		return nil
	}
	printVectorReport(stdout, report)
	return nil
}

func ragUsage() string {
	return "yemaka rag search <query> | inventory [--limit N] [--json] | prune-missing [--dry-run|--yes] [--json] | embeddings status|on|off|index | vector status|evaluate [--json]"
}

func runSnapshot(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("usage: yemaka snapshot create <path> [path...]")
	}
	paths := args[1:]
	if len(paths) == 0 {
		return fmt.Errorf("usage: yemaka snapshot create <path> [path...]")
	}
	manager := safety.NewSnapshotManager(app.profile.Snapshots)
	manifest, err := manager.Create(ctx, ".", paths, "manual snapshot", safety.PathPolicy{})
	if err != nil {
		_, _ = logToolRun(ctx, app, "snapshot_create", map[string]any{"paths": paths}, map[string]any{"error": err.Error()}, "failed", "low")
		return err
	}
	_, _ = logToolRun(ctx, app, "snapshot_create", map[string]any{"paths": paths}, map[string]any{"snapshot_id": manifest.ID}, "completed", "low")
	fmt.Fprintf(stdout, "snapshot: %s\n", manifest.ID)
	fmt.Fprintf(stdout, "files: %d\n", len(manifest.Files))
	return nil
}

func runRollback(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	id := "last"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		id = strings.TrimSpace(args[0])
	}
	manager := safety.NewSnapshotManager(app.profile.Snapshots)
	manifest, err := manager.Rollback(ctx, id)
	if err != nil {
		_, _ = logToolRun(ctx, app, "rollback", map[string]any{"snapshot_id": id}, map[string]any{"error": err.Error()}, "failed", "high")
		return err
	}
	_, _ = logToolRun(ctx, app, "rollback", map[string]any{"snapshot_id": id}, map[string]any{"restored_snapshot_id": manifest.ID}, "completed", "high")
	fmt.Fprintf(stdout, "rolled_back: %s\n", manifest.ID)
	fmt.Fprintf(stdout, "files: %d\n", len(manifest.Files))
	return nil
}

func runGit(ctx context.Context, app *appContext, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: yemaka git status | yemaka git diff [--stat]")
	}
	var command []string
	switch args[0] {
	case "status":
		command = []string{"git", "status", "--short"}
	case "diff":
		if len(args) > 1 && args[1] == "--stat" {
			command = []string{"git", "diff", "--stat"}
		} else {
			command = []string{"git", "diff"}
		}
	default:
		return fmt.Errorf("usage: yemaka git status | yemaka git diff [--stat]")
	}
	result, err := runAllowedCommand(ctx, app, command, "git_"+args[0], stdout)
	if err != nil {
		return err
	}
	printCommandResult(stdout, result)
	return nil
}

func runTest(ctx context.Context, app *appContext, stdout io.Writer) error {
	command, err := tools.DetectTestCommand(".")
	if err != nil {
		return err
	}
	result, err := runAllowedCommand(ctx, app, command, "run_tests", stdout)
	if err != nil {
		return err
	}
	printCommandResult(stdout, result)
	return nil
}

func runShell(ctx context.Context, app *appContext, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: yemaka shell \"go test ./...\"")
	}
	yes := false
	filtered := args[:0]
	for _, arg := range args {
		if arg == "--yes" || arg == "-y" {
			yes = true
			continue
		}
		filtered = append(filtered, arg)
	}
	command := strings.TrimSpace(strings.Join(filtered, " "))
	if command == "" {
		return fmt.Errorf("usage: yemaka shell \"go test ./...\"")
	}
	policy := commandPolicy(app)
	analysis := tools.AnalyzeCommand(command, policy)
	if !analysis.Allowed {
		_, _ = logToolRun(ctx, app, "run_shell_safe", map[string]any{"command": command}, map[string]any{"reason": analysis.Reason}, "blocked", analysis.RiskLevel)
		return fmt.Errorf("command blocked: %s", analysis.Reason)
	}
	if analysis.RequiresConfirmation && !yes {
		approved, err := confirm(stderr, os.Stdin, fmt.Sprintf("Run `%s`? Type yes to continue: ", analysis.Display))
		if err != nil {
			return err
		}
		if !approved {
			_, _ = logToolRun(ctx, app, "run_shell_safe", map[string]any{"command": command}, map[string]any{"approved": false}, "rejected", analysis.RiskLevel)
			fmt.Fprintln(stdout, "Command rejected.")
			return nil
		}
	}
	result, err := tools.RunCommand(ctx, analysis, policy)
	status := "completed"
	if result.ExitCode != 0 {
		status = "failed"
	}
	if err != nil {
		status = "failed"
	}
	_, _ = logToolRun(ctx, app, "run_shell_safe", map[string]any{"command": command}, toolRunOutput(result, err), status, analysis.RiskLevel)
	printCommandResult(stdout, result)
	return err
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "Yemaka local-first AI agent")
	fmt.Fprintln(w, "An AI agent from SiloTabs optimized for local-first operation on low-resource systems, https://silotabs.com")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  yemaka doctor")
	fmt.Fprintln(w, "  yemaka model providers")
	fmt.Fprintln(w, "  yemaka model list")
	fmt.Fprintln(w, "  yemaka model show <installed-model>")
	fmt.Fprintln(w, "  yemaka model generate <installed-model> \"Say hello briefly\"")
	fmt.Fprintln(w, "  yemaka model set low_memory <installed-model>")
	fmt.Fprintln(w, "  yemaka model set low_memory <installed-model> --provider llamacpp --base-url http://127.0.0.1:8080/v1")
	fmt.Fprintln(w, "  yemaka model profile list")
	fmt.Fprintln(w, "  yemaka model profile create coding-helper --base qwen2.5-coder:3b")
	fmt.Fprintln(w, "  yemaka model profile modelfile coding-helper")
	fmt.Fprintln(w, "  yemaka cloud fallback status")
	fmt.Fprintln(w, "  yemaka connector list")
	fmt.Fprintln(w, "  yemaka connector registry")
	fmt.Fprintln(w, "  yemaka connector show local_api")
	fmt.Fprintln(w, "  yemaka connector enable local_api --token-env YEMAKA_CONNECTOR_TOKEN")
	fmt.Fprintln(w, "  yemaka connector enable mcp_server")
	fmt.Fprintln(w, "  yemaka connector enable slack --token-env YEMAKA_SLACK_CONNECTOR_TOKEN")
	fmt.Fprintln(w, "  yemaka connector enable discord --token-env YEMAKA_DISCORD_CONNECTOR_TOKEN")
	fmt.Fprintln(w, "  yemaka connector enable telegram --token-env YEMAKA_TELEGRAM_CONNECTOR_TOKEN")
	fmt.Fprintln(w, "  yemaka connector enable email --token-env YEMAKA_EMAIL_CONNECTOR_TOKEN")
	fmt.Fprintln(w, "  yemaka connector disable local_api")
	fmt.Fprintln(w, "  yemaka connector disable mcp_server")
	fmt.Fprintln(w, "  yemaka connector serve mcp_server")
	fmt.Fprintln(w, "  yemaka domain-pack list")
	fmt.Fprintln(w, "  yemaka domain-pack skills")
	fmt.Fprintln(w, "  yemaka domain-pack templates")
	fmt.Fprintln(w, "  yemaka domain-pack install-template personal_productivity")
	fmt.Fprintln(w, "  yemaka domain-pack install ./my_pack")
	fmt.Fprintln(w, "  yemaka domain-pack enable my_pack")
	fmt.Fprintln(w, "  yemaka domain-pack disable my_pack")
	fmt.Fprintln(w, "  yemaka workflow list")
	fmt.Fprintln(w, "  yemaka workflow match \"triage this request\"")
	fmt.Fprintln(w, "  yemaka policy status")
	fmt.Fprintln(w, "  yemaka policy mode full_access")
	fmt.Fprintln(w, "  yemaka policy mode safe")
	fmt.Fprintln(w, "  yemaka learn report")
	fmt.Fprintln(w, "  yemaka learn export --conversation <conversation_id>")
	fmt.Fprintln(w, "  yemaka learn correction --conversation <conversation_id> \"Prefer X next time\"")
	fmt.Fprintln(w, "  yemaka learn regression-artifacts --trace ./trace.json --out ./regressions")
	fmt.Fprintln(w, "  yemaka replay list")
	fmt.Fprintln(w, "  yemaka replay show <trace_id>")
	fmt.Fprintln(w, "  yemaka replay explain <trace_id>")
	fmt.Fprintln(w, "  yemaka replay save-artifacts <trace_id> --out ./regressions")
	fmt.Fprintln(w, "  yemaka capability propose \"post daily summary to Slack\"")
	fmt.Fprintln(w, "  yemaka capability generate \"build a tool to check https://example.com\" --yes")
	fmt.Fprintln(w, "  yemaka capability generate \"build a custom local tool\" --yes --name custom_tool --draft-json ./draft.json")
	fmt.Fprintln(w, "  yemaka capability generate \"build a small CSV cleanup tool\" --yes --name csv_cleanup --synthesize --repair-attempts 1")
	fmt.Fprintln(w, "  yemaka capability generate \"build a CSV cleanup tool\" --yes --name csv_cleanup --run --input-json '{\"task\":\"normalize rows\"}'")
	fmt.Fprintln(w, "  yemaka extension propose \"monitor a local folder\"")
	fmt.Fprintln(w, "  yemaka extension generate folder_monitor \"Monitor a local folder\" --yes")
	fmt.Fprintln(w, "  yemaka extension list")
	fmt.Fprintln(w, "  yemaka extension show <name>")
	fmt.Fprintln(w, "  yemaka extension inspect <name>")
	fmt.Fprintln(w, "  yemaka extension validate <name>")
	fmt.Fprintln(w, "  yemaka extension test <name>")
	fmt.Fprintln(w, "  yemaka extension register <name>")
	fmt.Fprintln(w, "  yemaka extension disable <name>")
	fmt.Fprintln(w, "  yemaka extension delete <name>")
	fmt.Fprintln(w, "  yemaka extension rollback <name-or-snapshot-id>")
	fmt.Fprintln(w, "  yemaka extension run <name> '{\"task\":\"summarize notes\"}'")
	fmt.Fprintln(w, "  yemaka extension failures")
	fmt.Fprintln(w, "  yemaka internet status")
	fmt.Fprintln(w, "  yemaka internet search-provider auto --enable-internet")
	fmt.Fprintln(w, "  yemaka internet search-provider tavily TAVILY_API_KEY --enable-internet")
	fmt.Fprintln(w, "  yemaka internet search-provider serper SERPER_API_KEY --enable-internet")
	fmt.Fprintln(w, "  yemaka internet search-provider brave BRAVE_SEARCH_API_KEY --enable-internet")
	fmt.Fprintln(w, "  yemaka internet search-provider searxng https://search.example/search --enable-internet")
	fmt.Fprintln(w, "  yemaka internet fetch https://example.com --yes --extract-text --domain example.com")
	fmt.Fprintln(w, "  yemaka internet head https://example.com --yes --domain example.com")
	fmt.Fprintln(w, "  yemaka internet search \"local agents\" --yes")
	fmt.Fprintln(w, "  yemaka internet cache list")
	fmt.Fprintln(w, "  yemaka internet requests")
	fmt.Fprintln(w, "  yemaka job list")
	fmt.Fprintln(w, "  yemaka job create interval website_monitor --every 1h --yes")
	fmt.Fprintln(w, "  yemaka job update-input <job_id> --input-json '{\"task\":\"scheduled cleanup\"}' --yes")
	fmt.Fprintln(w, "  yemaka job enable <job_id>")
	fmt.Fprintln(w, "  yemaka job disable <job_id>")
	fmt.Fprintln(w, "  yemaka job archive <job_id> --yes")
	fmt.Fprintln(w, "  yemaka job run <job_id>")
	fmt.Fprintln(w, "  yemaka job tick")
	fmt.Fprintln(w, "  yemaka job loop --interval 1m")
	fmt.Fprintln(w, "  yemaka heartbeat status")
	fmt.Fprintln(w, "  yemaka heartbeat loop --interval 60s")
	fmt.Fprintln(w, "  yemaka notification list")
	fmt.Fprintln(w, "  yemaka notification add --title \"Review ready\"")
	fmt.Fprintln(w, "  yemaka chat \"Hello\"")
	fmt.Fprintln(w, "  yemaka workspace scan .")
	fmt.Fprintln(w, "  yemaka workspace summary .")
	fmt.Fprintln(w, "  yemaka workspace grant /path/to/project")
	fmt.Fprintln(w, "  yemaka workspace grants")
	fmt.Fprintln(w, "  yemaka workspace revoke <id-or-path>")
	fmt.Fprintln(w, "  yemaka file read README.md")
	fmt.Fprintln(w, "  yemaka file search \"sqlite\"")
	fmt.Fprintln(w, "  yemaka file write notes.md \"hello\"")
	fmt.Fprintln(w, "  yemaka file edit README.md < new-content.md")
	fmt.Fprintln(w, "  yemaka tool doctor-status")
	fmt.Fprintln(w, "  yemaka tool project-map .")
	fmt.Fprintln(w, "  yemaka tool patch-preview notes.md \"hello\"")
	fmt.Fprintln(w, "  yemaka tool symbol-search Run")
	fmt.Fprintln(w, "  yemaka tool secret-scan .")
	fmt.Fprintln(w, "  yemaka snapshot create README.md")
	fmt.Fprintln(w, "  yemaka rollback last")
	fmt.Fprintln(w, "  yemaka git status")
	fmt.Fprintln(w, "  yemaka git diff")
	fmt.Fprintln(w, "  yemaka test")
	fmt.Fprintln(w, "  yemaka shell \"go test ./...\"")
	fmt.Fprintln(w, "  yemaka skill list")
	fmt.Fprintln(w, "  yemaka skill show project_explainer")
	fmt.Fprintln(w, "  yemaka skill validate project_explainer")
	fmt.Fprintln(w, "  yemaka skill create-from-session <conversation_id>")
	fmt.Fprintln(w, "  yemaka skill improve-from-session project_explainer <conversation_id>")
	fmt.Fprintln(w, "  yemaka skill disable project_explainer")
	fmt.Fprintln(w, "  yemaka skill enable project_explainer")
	fmt.Fprintln(w, "  yemaka skill import ./my_skill")
	fmt.Fprintln(w, "  yemaka skill export project_explainer ./exports")
	fmt.Fprintln(w, "  yemaka ask --skill project_explainer \"Explain this project\"")
	fmt.Fprintln(w, "  yemaka eval run --mode low-memory")
	fmt.Fprintln(w, "  yemaka eval run --mode low-memory --skip-model")
	fmt.Fprintln(w, "  yemaka eval report")
	fmt.Fprintln(w, "  yemaka release check")
	fmt.Fprintln(w, "  yemaka serve")
	fmt.Fprintln(w, "  yemaka serve --no-frontend-watch")
	fmt.Fprintln(w, "  yemaka tui")
	fmt.Fprintln(w, "  yemaka ingest ./docs")
	fmt.Fprintln(w, "  yemaka rag search \"model routing\"")
	fmt.Fprintln(w, "  yemaka rag inventory")
	fmt.Fprintln(w, "  yemaka rag prune-missing --dry-run")
	fmt.Fprintln(w, "  yemaka rag embeddings status")
	fmt.Fprintln(w, "  yemaka rag vector evaluate")
	fmt.Fprintln(w, "  yemaka ask \"Explain this project\"")
	fmt.Fprintln(w, "  yemaka memory search \"topic\"")
	fmt.Fprintln(w, "  yemaka memory write preference \"Prefer concise answers\"")
	fmt.Fprintln(w, "  yemaka memory list")
}

func knownModelRole(role string) bool {
	for _, known := range modelRoles() {
		if role == known {
			return true
		}
	}
	return false
}

func modelRoles() []string {
	return []string{"default", "low_memory", "coding", "reasoning", "stronger_local"}
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

func workspaceArg(args []string) string {
	if len(args) > 1 && strings.TrimSpace(args[1]) != "" {
		return args[1]
	}
	return "."
}

func workspaceGrantStore(app *appContext) safety.WorkspaceGrantStore {
	return safety.NewWorkspaceGrantStore(safety.WorkspaceGrantsPath(app.profile.Permissions))
}

func ensureWorkspaceAccess(app *appContext, root string) (string, error) {
	return safety.RequireWorkspaceAccess(".", root, workspaceGrantStore(app))
}

func beginWorkspaceAccess(app *appContext, root string) (*safety.WorkspaceAccess, error) {
	return safety.RequireWorkspaceAccessWithBookmark(".", root, workspaceGrantStore(app))
}

func writeOptions(app *appContext) workspace.WriteOptions {
	return workspace.WriteOptions{
		SnapshotsRoot:       app.profile.Snapshots,
		SnapshotBeforeWrite: app.config.Tools.Filesystem.SnapshotBeforeWrite,
		FollowSymlinks:      app.config.Workspace.FollowSymlinks,
		IncludeHidden:       app.config.Workspace.IncludeHidden,
		MaxEditFileBytes:    app.config.Tools.Filesystem.MaxEditFileBytes,
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
		Rerank: rag.RerankConfig{
			CandidateLimit:  cfg.Rerank.CandidateLimit,
			SourceDiversity: cfg.Rerank.SourceDiversity,
		},
		Embeddings: rag.EmbeddingConfig{
			Enabled:        cfg.Embeddings.Enabled,
			Provider:       cfg.Embeddings.Provider,
			Model:          cfg.Embeddings.Model,
			BatchSize:      cfg.Embeddings.BatchSize,
			MaxTextChars:   cfg.Embeddings.MaxTextChars,
			CandidateLimit: cfg.Embeddings.CandidateLimit,
		},
		VectorDB: rag.VectorDBConfig{
			Enabled:                cfg.VectorDB.Enabled,
			Provider:               cfg.VectorDB.Provider,
			ResearchOnly:           cfg.VectorDB.ResearchOnly,
			LocalOnly:              cfg.VectorDB.LocalOnly,
			AllowBackgroundService: cfg.VectorDB.AllowBackgroundService,
			MaxRAMMB:               cfg.VectorDB.MaxRAMMB,
			MaxStorageMB:           cfg.VectorDB.MaxStorageMB,
			MinBenefitPercent:      cfg.VectorDB.MinBenefitPercent,
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

func generateRuntime(runtime models.Runtime) models.GenerateRuntime {
	generator, ok := runtime.(models.GenerateRuntime)
	if !ok {
		return nil
	}
	return generator
}

func modelDetailRuntime(runtime models.Runtime) models.ModelDetailRuntime {
	detailer, ok := runtime.(models.ModelDetailRuntime)
	if !ok {
		return nil
	}
	return detailer
}

func modelTemperature(cfg *config.Config, name string) float64 {
	if cfg == nil {
		return 0.2
	}
	name = strings.TrimSpace(name)
	for _, role := range modelRoles() {
		model := cfg.Models[role]
		if strings.EqualFold(strings.TrimSpace(model.Name), name) && model.Temperature > 0 {
			return model.Temperature
		}
	}
	if model := cfg.Models["default"]; model.Temperature > 0 {
		return model.Temperature
	}
	return 0.2
}

func printEmbeddingStatus(stdout io.Writer, cfg config.EmbeddingConfig) {
	fmt.Fprintf(stdout, "enabled: %t\n", cfg.Enabled)
	fmt.Fprintf(stdout, "provider: %s\n", cfg.Provider)
	fmt.Fprintf(stdout, "model: %s\n", cfg.Model)
	fmt.Fprintf(stdout, "batch_size: %d\n", cfg.BatchSize)
	fmt.Fprintf(stdout, "max_text_chars: %d\n", cfg.MaxTextChars)
	fmt.Fprintf(stdout, "candidate_limit: %d\n", cfg.CandidateLimit)
}

func printVectorStatus(stdout io.Writer, report rag.VectorDBResearchReport) {
	fmt.Fprintf(stdout, "enabled: %t\n", report.Enabled)
	fmt.Fprintf(stdout, "provider: %s\n", report.Provider)
	fmt.Fprintf(stdout, "research_only: %t\n", report.ResearchOnly)
	fmt.Fprintf(stdout, "local_only: %t\n", report.LocalOnly)
	fmt.Fprintf(stdout, "allow_background_service: %t\n", report.AllowBackgroundService)
	fmt.Fprintf(stdout, "sqlite_fts_active: %t\n", report.SQLiteFTSActive)
	fmt.Fprintf(stdout, "sqlite_embeddings_active: %t\n", report.SQLiteEmbeddingsActive)
	fmt.Fprintf(stdout, "approved: %t\n", report.Approved)
	fmt.Fprintf(stdout, "recommendation: %s\n", report.Recommendation)
	fmt.Fprintf(stdout, "documents: %d\n", report.Stats.Documents)
	fmt.Fprintf(stdout, "chunks: %d\n", report.Stats.Chunks)
	fmt.Fprintf(stdout, "embeddings: %d\n", report.Stats.Embeddings)
}

func printVectorReport(stdout io.Writer, report rag.VectorDBResearchReport) {
	printVectorStatus(stdout, report)
	fmt.Fprintln(stdout, "candidates:")
	for _, candidate := range report.Candidates {
		fmt.Fprintf(
			stdout,
			"  %s\t%s\tram_mb=%d\tstorage_mb=%d\t%s\n",
			candidate.Name,
			candidate.Status,
			candidate.EstimatedRAMMB,
			candidate.EstimatedStorageMB,
			candidate.Reason,
		)
	}
	fmt.Fprintln(stdout, "reasons:")
	for _, reason := range report.Reasons {
		fmt.Fprintf(stdout, "  - %s\n", reason)
	}
}

func printModelDetails(stdout io.Writer, details models.ModelDetails) {
	fmt.Fprintf(stdout, "name: %s\n", details.Name)
	if details.ModifiedAt != "" {
		fmt.Fprintf(stdout, "modified_at: %s\n", details.ModifiedAt)
	}
	if details.Size > 0 {
		fmt.Fprintf(stdout, "size_bytes: %d\n", details.Size)
	}
	if details.Digest != "" {
		fmt.Fprintf(stdout, "digest: %s\n", details.Digest)
	}
	if details.Family != "" {
		fmt.Fprintf(stdout, "family: %s\n", details.Family)
	}
	if details.Format != "" {
		fmt.Fprintf(stdout, "format: %s\n", details.Format)
	}
	if details.ParameterSize != "" {
		fmt.Fprintf(stdout, "parameter_size: %s\n", details.ParameterSize)
	}
	if details.QuantizationLevel != "" {
		fmt.Fprintf(stdout, "quantization: %s\n", details.QuantizationLevel)
	}
	if details.ContextLength > 0 {
		fmt.Fprintf(stdout, "context_length: %d\n", details.ContextLength)
	}
	if details.Parameters != "" {
		fmt.Fprintf(stdout, "parameters: %s\n", compactSingleLine(details.Parameters, 240))
	}
	if details.Template != "" {
		fmt.Fprintf(stdout, "template: %s\n", compactSingleLine(details.Template, 240))
	}
}

func compactSingleLine(value string, maxLen int) string {
	value = strings.Join(strings.Fields(value), " ")
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return value[:maxLen]
	}
	return value[:maxLen-3] + "..."
}

func printCloudFallbackStatus(stdout io.Writer, cfg config.CloudFallbackConfig) {
	fmt.Fprintf(stdout, "enabled: %t\n", cfg.Enabled)
	fmt.Fprintf(stdout, "provider: %s\n", cfg.Provider)
	fmt.Fprintf(stdout, "model: %s\n", cfg.Name)
	fmt.Fprintf(stdout, "base_url: %s\n", cfg.BaseURL)
	fmt.Fprintf(stdout, "api_key_env: %s\n", cfg.APIKeyEnv)
	fmt.Fprintf(stdout, "timeout_seconds: %d\n", cfg.TimeoutSeconds)
	fmt.Fprintf(stdout, "temperature: %.2f\n", cfg.Temperature)
}

func parsePositiveInt(input string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return 0, err
	}
	if value <= 0 {
		return 0, fmt.Errorf("must be greater than zero")
	}
	return value, nil
}

func cloudFallback(cfg *config.Config, runtime models.Runtime) *agent.CloudFallback {
	if cfg == nil || !cfg.CloudFallback.Enabled || runtime == nil {
		return nil
	}
	return &agent.CloudFallback{
		Config:  cfg.CloudFallback,
		Runtime: runtime,
	}
}

func cliToolExecutor(app *appContext) agent.ToolExecutor {
	if app == nil || app.config == nil {
		return nil
	}
	internetService := internetService(app)
	return agent.NewSafeToolExecutor(agent.SafeToolConfig{
		WorkspaceRoot:            ".",
		TimeoutSeconds:           app.config.Tools.Shell.TimeoutSeconds,
		MaxOutputBytes:           app.config.Tools.Shell.MaxOutputBytes,
		MaxContextChars:          app.config.Workspace.MaxContextChars,
		RequireConfirmationRisky: app.config.Tools.Shell.RequireConfirmationRisky,
		FullAccess:               safety.IsFullAccessMode(policyMode(app.config)),
		DisableShellCommands:     !app.config.Tools.Shell.Enabled,
		Config:                   app.config,
		Profile:                  app.profile,
		Memory:                   app.store,
		RAG:                      app.rag,
		Runtime:                  app.runtime,
		Skills:                   app.skills,
		Internet:                 internetService,
		WorkspaceGrants:          workspaceGrantStore(app),
		UseShellHelper:           true,
		ShellHelperPath:          tools.DefaultShellHelperPath(),
	})
}

func cliRetriever(app *appContext) agent.Retriever {
	if app == nil || app.config == nil {
		return nil
	}
	return agent.NewLocalRetriever(agent.LocalRetrieverConfig{
		Config:        app.config,
		WorkspaceRoot: ".",
		RAG:           app.rag,
		Runtime:       app.runtime,
	})
}

func splitEventSources(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ", ")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			clean = append(clean, part)
		}
	}
	return clean
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

func parseAskArgs(args []string) (string, []string, error) {
	var skill string
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--skill" {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("usage: yemaka ask --skill <name> \"request\"")
			}
			skill = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--skill=") {
			skill = strings.TrimPrefix(arg, "--skill=")
			continue
		}
		rest = append(rest, arg)
	}
	return skill, rest, nil
}

func parseEvalRunArgs(args []string) (evaluation.RunOptions, bool, error) {
	options := evaluation.RunOptions{Mode: "low-memory"}
	jsonOutput := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOutput = true
		case arg == "--skip-model":
			options.SkipModel = true
		case arg == "--mode":
			if i+1 >= len(args) {
				return evaluation.RunOptions{}, false, fmt.Errorf("usage: yemaka eval run --mode <low-memory> [--model <installed-model>] [--skip-model] [--json]")
			}
			options.Mode = args[i+1]
			i++
		case strings.HasPrefix(arg, "--mode="):
			options.Mode = strings.TrimPrefix(arg, "--mode=")
		case arg == "--model":
			if i+1 >= len(args) {
				return evaluation.RunOptions{}, false, fmt.Errorf("usage: yemaka eval run --mode <low-memory> --model <installed-model> [--skip-model] [--json]")
			}
			options.Model = args[i+1]
			i++
		case strings.HasPrefix(arg, "--model="):
			options.Model = strings.TrimPrefix(arg, "--model=")
		default:
			return evaluation.RunOptions{}, false, fmt.Errorf("unknown eval run option: %s", arg)
		}
	}
	if options.SkipModel && strings.TrimSpace(options.Model) != "" {
		return evaluation.RunOptions{}, false, fmt.Errorf("--skip-model cannot be combined with --model")
	}
	return options, jsonOutput, nil
}

func parseEvalReportArgs(args []string) (bool, error) {
	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		return false, fmt.Errorf("unknown eval report option: %s", arg)
	}
	return jsonOutput, nil
}

func printEvalReport(stdout io.Writer, report evaluation.Report) {
	fmt.Fprintln(stdout, "Yemaka eval")
	fmt.Fprintf(stdout, "report: %s\n", report.Path)
	fmt.Fprintf(stdout, "mode: %s\n", report.Mode)
	fmt.Fprintf(stdout, "model: %s\n", report.Model)
	fmt.Fprintf(stdout, "duration_ms: %d\n", report.DurationMS)
	fmt.Fprintf(stdout, "passed: %d\n", report.Summary.Passed)
	fmt.Fprintf(stdout, "failed: %d\n", report.Summary.Failed)
	fmt.Fprintf(stdout, "skipped: %d\n", report.Summary.Skipped)
	if len(report.Metrics) > 0 {
		fmt.Fprintln(stdout, "metrics:")
		keys := make([]string, 0, len(report.Metrics))
		for key := range report.Metrics {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(stdout, "  %s: %s\n", key, metricValueString(report.Metrics[key]))
		}
	}
	fmt.Fprintln(stdout, "tasks:")
	for _, task := range report.Tasks {
		fmt.Fprintf(stdout, "  %s\t%s\t%dms", task.Status, task.Name, task.DurationMS)
		if task.Details != "" {
			fmt.Fprintf(stdout, "\t%s", task.Details)
		}
		fmt.Fprintln(stdout)
	}
}

func metricValueString(value any) string {
	switch typed := value.(type) {
	case float64:
		asInt := int64(typed)
		if typed == float64(asInt) {
			return strconv.FormatInt(asInt, 10)
		}
		return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(typed, 'f', 4, 64), "0"), ".")
	case float32:
		return metricValueString(float64(typed))
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case string:
		return typed
	default:
		return fmt.Sprint(value)
	}
}

func printReleaseReport(stdout io.Writer, report release.Report) {
	fmt.Fprintln(stdout, "Yemaka release check")
	fmt.Fprintf(stdout, "version: %s\n", report.Version)
	fmt.Fprintf(stdout, "ready: %t\n", report.Ready)
	fmt.Fprintln(stdout, "checks:")
	for _, check := range report.Checks {
		fmt.Fprintf(stdout, "  %s\t%s", check.Status, check.Name)
		if check.Details != "" {
			fmt.Fprintf(stdout, "\t%s", check.Details)
		}
		fmt.Fprintln(stdout)
	}
}

func printJSON(stdout io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, string(data))
	return nil
}

func selectSkill(app *appContext, explicit string, content string) (skills.Skill, bool, error) {
	available := availableSkillTools()
	if explicit != "" {
		skill, ok := app.skills.Get(explicit)
		if !ok {
			return skills.Skill{}, false, fmt.Errorf("skill not found: %s", explicit)
		}
		if skill.Disabled {
			return skills.Skill{}, false, fmt.Errorf("skill is disabled: %s", explicit)
		}
		for _, tool := range skill.RequiredTools {
			if !available[tool] {
				return skills.Skill{}, false, fmt.Errorf("skill %s requires unavailable tool: %s", explicit, tool)
			}
		}
		return skill, true, nil
	}
	skill, ok := app.skills.Select(content, available)
	return skill, ok, nil
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

func languageLines(counts map[string]int) []string {
	lines := make([]string, 0, len(counts))
	for language, count := range counts {
		lines = append(lines, fmt.Sprintf("%s: %d", language, count))
	}
	for i := 0; i < len(lines); i++ {
		for j := i + 1; j < len(lines); j++ {
			if lines[j] < lines[i] {
				lines[i], lines[j] = lines[j], lines[i]
			}
		}
	}
	return lines
}

func commandPolicy(app *appContext) tools.CommandPolicy {
	timeout := time.Duration(app.config.Tools.Shell.TimeoutSeconds) * time.Second
	return tools.CommandPolicy{
		WorkspaceRoot:            ".",
		Timeout:                  timeout,
		MaxOutputBytes:           app.config.Tools.Shell.MaxOutputBytes,
		RequireConfirmationRisky: app.config.Tools.Shell.RequireConfirmationRisky,
		FullAccess:               safety.IsFullAccessMode(policyMode(app.config)),
		UseHelper:                true,
		HelperPath:               tools.DefaultShellHelperPath(),
	}
}

func policyMode(cfg *config.Config) string {
	if cfg == nil {
		return safety.PolicyModeSafe
	}
	return safety.NormalizePolicyMode(cfg.Security.Policy.Mode)
}

func runAllowedCommand(ctx context.Context, app *appContext, command []string, toolName string, stdout io.Writer) (tools.CommandResult, error) {
	policy := commandPolicy(app)
	analysis := tools.AnalyzeArgs(command, policy)
	if !analysis.Allowed {
		_, _ = logToolRun(ctx, app, toolName, map[string]any{"command": strings.Join(command, " ")}, map[string]any{"reason": analysis.Reason}, "blocked", analysis.RiskLevel)
		return tools.CommandResult{Command: analysis.Display, Args: analysis.Args, ExitCode: -1}, fmt.Errorf("command blocked: %s", analysis.Reason)
	}
	result, err := tools.RunCommand(ctx, analysis, policy)
	status := "completed"
	if result.ExitCode != 0 {
		status = "failed"
	}
	if err != nil {
		status = "failed"
	}
	_, _ = logToolRun(ctx, app, toolName, map[string]any{"command": analysis.Display}, toolRunOutput(result, err), status, analysis.RiskLevel)
	if stdout != io.Discard {
		fmt.Fprintf(stdout, "command: %s\n", analysis.Display)
	}
	return result, err
}

func printCommandResult(stdout io.Writer, result tools.CommandResult) {
	fmt.Fprintf(stdout, "exit_code: %d\n", result.ExitCode)
	fmt.Fprintf(stdout, "duration_ms: %d\n", result.Duration.Milliseconds())
	if result.TimedOut {
		fmt.Fprintln(stdout, "timed_out: true")
	}
	if result.Truncated {
		fmt.Fprintln(stdout, "truncated: true")
	}
	if result.Helper {
		fmt.Fprintln(stdout, "helper_process: true")
	}
	if strings.TrimSpace(result.Stdout) != "" {
		fmt.Fprintln(stdout, "stdout:")
		fmt.Fprintln(stdout, strings.TrimRight(result.Stdout, "\n"))
	}
	if strings.TrimSpace(result.Stderr) != "" {
		fmt.Fprintln(stdout, "stderr:")
		fmt.Fprintln(stdout, strings.TrimRight(result.Stderr, "\n"))
	}
}

func toolRunOutput(result tools.CommandResult, err error) map[string]any {
	output := map[string]any{
		"exit_code": result.ExitCode,
		"stdout":    result.Stdout,
		"stderr":    result.Stderr,
		"truncated": result.Truncated,
		"timed_out": result.TimedOut,
		"helper":    result.Helper,
	}
	if err != nil {
		output["error"] = err.Error()
	}
	return output
}

func confirm(w io.Writer, r io.Reader, prompt string) (bool, error) {
	fmt.Fprint(w, prompt)
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return answer == "yes", nil
}

func logToolRun(ctx context.Context, app *appContext, name string, input any, output any, status string, risk string) (memory.ToolRun, error) {
	return app.store.SaveToolRun(ctx, memory.ToolRun{
		ToolName:  name,
		Input:     input,
		Output:    output,
		Status:    status,
		RiskLevel: risk,
	})
}
