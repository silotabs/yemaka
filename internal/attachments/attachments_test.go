package attachments

import (
	"context"
	"strings"
	"testing"

	"yemaka/internal/memory"
	"yemaka/internal/workspace"
)

func TestProcessUploadExtractsLocalTextContext(t *testing.T) {
	attachment, err := ProcessUpload(context.Background(), UploadInput{
		FileName:    "/Users/example/Documents/project.txt",
		ContentType: "text/plain",
		Data:        []byte("Project alpha notes for this chat only."),
		Retention:   "conversation",
	}, workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("ProcessUpload() error = %v", err)
	}
	if attachment.FileName != "project.txt" {
		t.Fatalf("FileName = %q, want basename", attachment.FileName)
	}
	if attachment.Status != "ready" {
		t.Fatalf("Status = %q, want ready", attachment.Status)
	}
	if !strings.Contains(attachment.Content, "Project alpha") {
		t.Fatalf("Content = %q, want uploaded text", attachment.Content)
	}

	contextText, sources := BuildContext([]memory.ChatAttachment{attachment}, 0)
	if !strings.Contains(contextText, "CHAT ATTACHMENTS") || !strings.Contains(contextText, "Project alpha") {
		t.Fatalf("context = %q, want attachment context", contextText)
	}
	if len(sources) != 1 || sources[0] != "project.txt" {
		t.Fatalf("sources = %#v, want project.txt", sources)
	}
}

func TestProcessUploadKeepsImagesMetadataOnly(t *testing.T) {
	attachment, err := ProcessUpload(context.Background(), UploadInput{
		FileName:    "diagram.png",
		ContentType: "image/png",
		Data:        []byte{1, 2, 3},
	}, workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("ProcessUpload() error = %v", err)
	}
	if attachment.Status != "metadata_only" {
		t.Fatalf("Status = %q, want metadata_only", attachment.Status)
	}
	if !strings.Contains(attachment.Summary, "Image attached locally") {
		t.Fatalf("Summary = %q, want image metadata summary", attachment.Summary)
	}
}
