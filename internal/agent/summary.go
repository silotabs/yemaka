package agent

import (
	"context"

	"yemaka/internal/memory"
)

const maxConversationSummaryChars = 1000

func (s *Service) maybeSaveConversationSummary(ctx context.Context, conversationID string) error {
	if s == nil || s.Memory == nil || !s.autoSummarizeEnabled() {
		return nil
	}
	threshold := s.summaryThreshold()
	count, err := s.Memory.CountConversationMessages(ctx, conversationID)
	if err != nil {
		return err
	}
	if count < threshold {
		return nil
	}
	messages, err := s.Memory.ListConversationMessages(ctx, conversationID, threshold)
	if err != nil {
		return err
	}
	summary := memory.BuildConversationSummary(messages, maxConversationSummaryChars)
	if summary == "" {
		return nil
	}
	if _, err := s.Memory.SaveConversationSummary(ctx, conversationID, summary); err != nil {
		return err
	}
	_, err = s.Memory.PruneDuplicateMemories(ctx)
	return err
}

func (s *Service) autoSummarizeEnabled() bool {
	return s != nil &&
		s.Router != nil &&
		s.Router.Config != nil &&
		s.Router.Config.Runtime.AutoSummarize
}

func (s *Service) summaryThreshold() int {
	if s != nil &&
		s.Router != nil &&
		s.Router.Config != nil &&
		s.Router.Config.Memory.SummarizeAfter > 0 {
		return s.Router.Config.Memory.SummarizeAfter
	}
	return 20
}
