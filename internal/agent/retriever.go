package agent

import (
	"context"
	"strings"

	"yemaka/internal/config"
	"yemaka/internal/models"
	"yemaka/internal/rag"
	"yemaka/internal/routing"
	"yemaka/internal/workspace"
)

type RetrievalResult struct {
	Context    string
	Sources    []string
	SourceKind string
}

type Retriever interface {
	Retrieve(ctx context.Context, content string) (RetrievalResult, error)
}

type RetrieverFunc func(ctx context.Context, content string) (RetrievalResult, error)

func (fn RetrieverFunc) Retrieve(ctx context.Context, content string) (RetrievalResult, error) {
	return fn(ctx, content)
}

type LocalRetrieverConfig struct {
	Config        *config.Config
	WorkspaceRoot string
	RAG           *rag.Store
	Runtime       models.Runtime
}

func NewLocalRetriever(cfg LocalRetrieverConfig) Retriever {
	return RetrieverFunc(func(ctx context.Context, content string) (RetrievalResult, error) {
		content = strings.TrimSpace(content)
		if content == "" {
			return RetrievalResult{}, nil
		}
		appCfg := cfg.Config
		if shouldUseRAG(appCfg, content) && cfg.RAG != nil {
			ragContext, err := cfg.RAG.BuildContextWithConfig(ctx, content, retrieverRAGConfig(appCfg), embeddingRuntime(cfg.Runtime), retrieverMaxContextChars(appCfg))
			if err != nil {
				return RetrievalResult{}, err
			}
			if strings.TrimSpace(ragContext.Text) != "" {
				return RetrievalResult{Context: ragContext.Text, Sources: ragContext.Sources, SourceKind: "rag"}, nil
			}
		}

		root := strings.TrimSpace(cfg.WorkspaceRoot)
		if root == "" {
			root = "."
		}
		questionContext, err := workspace.BuildQuestionContext(ctx, root, content, retrieverWorkspaceLimits(appCfg))
		if err != nil {
			return RetrievalResult{}, err
		}
		return RetrievalResult{Context: questionContext.Text, Sources: questionContext.Sources, SourceKind: "workspace"}, nil
	})
}

func shouldRetrieveLocalContext(input PlanInput) bool {
	return routing.ShouldRetrieveLocalContext(routing.Request{
		Content:          input.Content,
		SkillName:        input.SkillName,
		WorkspaceContext: input.WorkspaceContext,
		ProfileMemory:    input.ProfileMemory,
		TaskMemory:       input.TaskMemory,
		Sources:          input.Sources,
		MemorySources:    input.MemorySources,
		SessionContract:  input.SessionContract,
	})
}

func hasExplicitLocalRetrievalIntent(content string) bool {
	return routing.ShouldRetrieveLocalContext(routing.Request{Content: content})
}

func looksLocalContextToolTask(content string) bool {
	return routing.LooksLocalContextToolTask(content)
}

func containsLocalContextTerm(content string) bool {
	return routing.ContainsLocalContextTerm(content)
}

func shouldUseRAG(cfg *config.Config, content string) bool {
	enabled := true
	if cfg != nil && !cfg.RAG.Enabled {
		enabled = false
	}
	return routing.ShouldUseRAG(enabled, content)
}

func retrieverMaxContextChars(cfg *config.Config) int {
	if cfg == nil || cfg.Workspace.MaxContextChars <= 0 {
		return workspace.DefaultLimits().MaxContextChars
	}
	return cfg.Workspace.MaxContextChars
}

func retrieverWorkspaceLimits(cfg *config.Config) workspace.Limits {
	if cfg == nil {
		return workspace.DefaultLimits()
	}
	return workspace.Limits{
		MaxFilesScanned:   cfg.Workspace.MaxFilesScanned,
		MaxFileBytes:      cfg.Workspace.MaxFileBytes,
		MaxTotalScanBytes: cfg.Workspace.MaxTotalScanBytes,
		MaxSearchResults:  cfg.Workspace.MaxSearchResults,
		MaxContextFiles:   cfg.Workspace.MaxContextFiles,
		MaxContextChars:   cfg.Workspace.MaxContextChars,
		IncludeHidden:     cfg.Workspace.IncludeHidden,
		FollowSymlinks:    cfg.Workspace.FollowSymlinks,
	}
}

func retrieverRAGConfig(cfg *config.Config) rag.Config {
	if cfg == nil {
		return rag.DefaultConfig()
	}
	return ragConfigFromAgentConfig(cfg)
}
