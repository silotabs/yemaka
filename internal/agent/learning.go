package agent

import (
	"context"

	"yemaka/internal/learning"
)

func (s *Service) recordWorkflowMemory(ctx context.Context, conversationID string) error {
	if s == nil || s.Memory == nil {
		return nil
	}
	_, err := learning.RecordWorkflow(ctx, s.Memory, conversationID)
	return err
}
