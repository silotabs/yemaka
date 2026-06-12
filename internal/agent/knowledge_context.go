package agent

import (
	"context"
	"fmt"
	"strings"

	"yemaka/internal/config"
	"yemaka/internal/knowledge"
)

type KnowledgeContext struct {
	Text        string
	Sources     []string
	EntityCount int
	EdgeCount   int
}

type KnowledgeContextProvider interface {
	RetrieveKnowledgeContext(ctx context.Context, query string) (KnowledgeContext, error)
}

type KnowledgeContextProviderFunc func(ctx context.Context, query string) (KnowledgeContext, error)

func (fn KnowledgeContextProviderFunc) RetrieveKnowledgeContext(ctx context.Context, query string) (KnowledgeContext, error) {
	return fn(ctx, query)
}

type LocalKnowledgeContextConfig struct {
	Config *config.Config
}

func NewLocalKnowledgeContextProvider(input LocalKnowledgeContextConfig) KnowledgeContextProvider {
	return KnowledgeContextProviderFunc(func(ctx context.Context, query string) (KnowledgeContext, error) {
		cfg := input.Config
		if cfg == nil || !cfg.Knowledge.Enabled || !cfg.Knowledge.InfluenceEnabled {
			return KnowledgeContext{}, nil
		}
		if strings.TrimSpace(cfg.Memory.Database) == "" {
			return KnowledgeContext{}, fmt.Errorf("knowledge graph database is not configured")
		}
		store, err := knowledge.OpenWithOptions(ctx, cfg.Memory.Database, knowledge.Options{
			MaxEvidenceChars: cfg.Knowledge.MaxEvidenceChars,
		})
		if err != nil {
			return KnowledgeContext{}, err
		}
		defer store.Close()
		result, err := store.BuildInfluenceContext(ctx, query, knowledge.InfluenceOptions{
			Limit:    cfg.Knowledge.MaxInfluenceEntities,
			MaxChars: cfg.Knowledge.MaxInfluenceChars,
		})
		if err != nil {
			return KnowledgeContext{}, err
		}
		return KnowledgeContext{
			Text:        result.Text,
			Sources:     append([]string{}, result.Sources...),
			EntityCount: result.EntityCount,
			EdgeCount:   result.EdgeCount,
		}, nil
	})
}

func (s *Service) attachKnowledgeContext(ctx context.Context, input *PlanInput, emit EventHandler) error {
	if s == nil || s.KnowledgeContext == nil || input == nil {
		return nil
	}
	result, err := s.KnowledgeContext.RetrieveKnowledgeContext(ctx, input.Content)
	if err != nil {
		return err
	}
	if strings.TrimSpace(result.Text) == "" {
		return nil
	}
	input.KnowledgeContext = result.Text
	input.KnowledgeSources = appendUniqueStrings(input.KnowledgeSources, result.Sources)
	if emit == nil {
		return nil
	}
	return emit(Event{
		Type: EventKnowledgeUsed,
		Data: map[string]string{
			"source_count": fmt.Sprintf("%d", len(result.Sources)),
			"entity_count": fmt.Sprintf("%d", result.EntityCount),
			"edge_count":   fmt.Sprintf("%d", result.EdgeCount),
			"sources":      strings.Join(result.Sources, ", "),
		},
	})
}
