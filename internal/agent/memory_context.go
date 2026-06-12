package agent

import (
	"context"
	"fmt"
	"strings"

	"yemaka/internal/learning"
	"yemaka/internal/memory"
)

const (
	defaultRelevantMemories = 6
	defaultRouteCorrections = 8
	memoryListScanLimit     = 40
	maxProfileMemoryChars   = 700
	maxTaskMemoryChars      = 1200
	maxMemoryLineChars      = 260
)

func (s *Service) attachMemoryContext(ctx context.Context, input *PlanInput, emit EventHandler) error {
	if s == nil || s.Memory == nil || input == nil {
		return nil
	}
	limit := s.maxRelevantMemories()
	if limit <= 0 {
		return nil
	}
	corrections, err := learning.ApprovedRouteCorrections(ctx, s.Memory, input.Content, defaultRouteCorrections)
	if err != nil {
		return err
	}
	input.RouteCorrections = append(input.RouteCorrections, corrections...)
	results, err := s.relevantMemories(ctx, input.Content, limit)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return nil
	}

	profile, task, sources := splitMemoryContext(results)
	input.ProfileMemory = profile
	input.TaskMemory = task
	input.MemorySources = sources

	if emit == nil {
		return nil
	}
	return emit(Event{
		Type: EventMemoryUsed,
		Data: map[string]string{
			"source_count": fmt.Sprintf("%d", len(sources)),
			"sources":      strings.Join(sources, ", "),
		},
	})
}

func (s *Service) relevantMemories(ctx context.Context, query string, limit int) ([]memory.MemorySearchResult, error) {
	if limit <= 0 {
		return nil, nil
	}
	seen := map[string]bool{}
	results := make([]memory.MemorySearchResult, 0, limit)
	add := func(item memory.MemorySearchResult) {
		if len(results) >= limit || item.ID == "" || seen[item.ID] {
			return
		}
		seen[item.ID] = true
		results = append(results, item)
	}

	listLimit := memoryListScanLimit
	if listLimit < limit*4 {
		listLimit = limit * 4
	}
	listed, err := s.Memory.ListMemories(ctx, listLimit)
	if err != nil {
		return nil, err
	}
	for _, item := range listed {
		if item.Pinned || (item.Kind == "preference" && item.Importance >= 4) {
			add(item)
		}
	}

	matches, err := s.Memory.SearchMemories(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	for _, item := range matches {
		add(item)
	}
	return results, nil
}

func splitMemoryContext(results []memory.MemorySearchResult) (string, string, []string) {
	var profileLines []string
	var taskLines []string
	var sources []string
	profileChars := 0
	taskChars := 0

	for _, result := range results {
		if result.Kind == learning.KindRouteCorrection {
			continue
		}
		if isNoisyWorkflowMemory(result) {
			continue
		}
		line := memoryLine(result)
		if line == "" {
			continue
		}
		source := result.Source
		if source == "" {
			source = result.Kind
		}
		sources = append(sources, source)

		if result.Kind == "preference" {
			if profileChars+len(line) > maxProfileMemoryChars {
				continue
			}
			profileLines = append(profileLines, line)
			profileChars += len(line)
			continue
		}
		if taskChars+len(line) > maxTaskMemoryChars {
			continue
		}
		taskLines = append(taskLines, line)
		taskChars += len(line)
	}
	return strings.Join(profileLines, "\n"), strings.Join(taskLines, "\n"), sources
}

func isNoisyWorkflowMemory(result memory.MemorySearchResult) bool {
	if result.Kind != "workflow_success" {
		return false
	}
	content := strings.ToLower(result.Content)
	if !strings.Contains(content, "tools:") {
		return true
	}
	toolLine := content
	if index := strings.Index(content, "tools:"); index >= 0 {
		toolLine = content[index:]
		if end := strings.Index(toolLine, "\n"); end >= 0 {
			toolLine = toolLine[:end]
		}
	}
	toolLine = strings.TrimSpace(strings.TrimPrefix(toolLine, "tools:"))
	if toolLine == "" {
		return true
	}
	for _, tool := range strings.Split(toolLine, ",") {
		name := strings.TrimSpace(tool)
		if name == "" || strings.HasPrefix(name, "agent_verifier(") {
			continue
		}
		return false
	}
	return true
}

func memoryLine(result memory.MemorySearchResult) string {
	content := compactMemoryText(result.Content, maxMemoryLineChars)
	if content == "" {
		return ""
	}
	source := strings.TrimSpace(result.Source)
	if source == "" {
		source = "local"
	}
	return fmt.Sprintf("- [%s, importance %d, source %s] %s", result.Kind, result.Importance, source, content)
}

func compactMemoryText(input string, maxChars int) string {
	text := strings.Join(strings.Fields(input), " ")
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	if maxChars <= 3 {
		return text[:maxChars]
	}
	return strings.TrimSpace(text[:maxChars-3]) + "..."
}

func (s *Service) maxRelevantMemories() int {
	if s != nil && s.Router != nil && s.Router.Config != nil && s.Router.Config.Memory.MaxRelevantMemories > 0 {
		return s.Router.Config.Memory.MaxRelevantMemories
	}
	return defaultRelevantMemories
}
