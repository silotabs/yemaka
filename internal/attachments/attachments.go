package attachments

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"yemaka/internal/memory"
	"yemaka/internal/rag"
	"yemaka/internal/workspace"
)

const (
	RetentionConversation = "conversation"
	SourceKind            = "attachment"
	defaultContextChars   = 12000
	previewChars          = 700
	summaryChars          = 180
)

type UploadInput struct {
	FileName    string
	ContentType string
	Data        []byte
	Retention   string
}

func ProcessUpload(ctx context.Context, input UploadInput, limits workspace.Limits) (memory.ChatAttachment, error) {
	name := cleanDisplayName(input.FileName)
	if name == "" {
		return memory.ChatAttachment{}, fmt.Errorf("attachment file name is required")
	}
	if limits.MaxFileBytes > 0 && int64(len(input.Data)) > limits.MaxFileBytes {
		return memory.ChatAttachment{}, fmt.Errorf("attachment exceeds max read size")
	}

	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	attachment := memory.ChatAttachment{
		ID:          newAttachmentID(),
		FileName:    name,
		ContentType: contentType,
		SizeBytes:   int64(len(input.Data)),
		Status:      "ready",
		SourceKind:  SourceKind,
		Retention:   normalizeRetention(input.Retention),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}

	if isImageAttachment(name, contentType) {
		attachment.Status = "metadata_only"
		attachment.Summary = "Image attached locally. Visual analysis requires an image-capable local model or OCR path."
		attachment.Preview = attachment.Summary
		attachment.Sources = []string{name}
		attachment.Content = attachment.Summary
		return attachment, nil
	}

	extracted, err := rag.ExtractUploadedFile(ctx, name, input.Data, limits)
	if err != nil {
		attachment.Status = "metadata_only"
		attachment.Summary = fmt.Sprintf("Attached locally, but no readable text was extracted: %s", compactText(err.Error(), summaryChars))
		attachment.Preview = attachment.Summary
		attachment.Sources = []string{name}
		attachment.Content = attachment.Summary
		return attachment, nil
	}
	attachment.ContentType = firstNonEmpty(extracted.MimeType, contentType)
	attachment.SizeBytes = extracted.Size
	attachment.Sources = []string{firstNonEmpty(extracted.Path, name)}
	attachment.Content = strings.TrimSpace(extracted.Content)
	attachment.Preview = compactText(attachment.Content, previewChars)
	attachment.Summary = attachmentSummary(name, attachment.Content)
	return attachment, nil
}

func BuildContext(items []memory.ChatAttachment, maxChars int) (string, []string) {
	if maxChars <= 0 {
		maxChars = defaultContextChars
	}
	var builder strings.Builder
	var sources []string
	for _, item := range items {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		source := firstNonEmpty(firstString(item.Sources), item.FileName)
		if source != "" {
			sources = appendUnique(sources, source)
		}
		body := strings.TrimSpace(item.Content)
		if body == "" {
			body = strings.TrimSpace(item.Preview)
		}
		if body == "" {
			continue
		}
		if builder.Len() == 0 {
			builder.WriteString("CHAT ATTACHMENTS (local, session-scoped; not permanently indexed unless the user explicitly ingests them):\n")
		}
		entry := fmt.Sprintf("\n[%s]\nStatus: %s\nType: %s\nExcerpt:\n%s\n", item.FileName, firstNonEmpty(item.Status, "ready"), item.ContentType, body)
		if builder.Len()+len(entry) > maxChars {
			remaining := maxChars - builder.Len()
			if remaining <= 80 {
				break
			}
			entry = compactText(entry, remaining)
		}
		builder.WriteString(entry)
		if builder.Len() >= maxChars {
			break
		}
	}
	return strings.TrimSpace(builder.String()), sources
}

func cleanDisplayName(name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(filepath.ToSlash(name), "\\", "/"))
	if name == "" {
		return ""
	}
	name = filepath.Base(name)
	name = strings.TrimSpace(name)
	if name == "." || name == "/" {
		return ""
	}
	return name
}

func normalizeRetention(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "conversation", "session":
		return RetentionConversation
	default:
		return RetentionConversation
	}
}

func isImageAttachment(name string, contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if strings.HasPrefix(contentType, "image/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tiff", ".heic", ".svg":
		return true
	default:
		return false
	}
}

func attachmentSummary(name string, content string) string {
	content = strings.Join(strings.Fields(content), " ")
	if content == "" {
		return "Attached locally."
	}
	return fmt.Sprintf("%s ready for this conversation: %s", name, compactText(content, summaryChars))
}

func compactText(input string, maxChars int) string {
	text := strings.Join(strings.Fields(strings.TrimSpace(input)), " ")
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	if maxChars <= 3 {
		return text[:maxChars]
	}
	return strings.TrimSpace(text[:maxChars-3]) + "..."
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return values
		}
	}
	return append(values, value)
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func newAttachmentID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "att_" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("att_%d", time.Now().UnixNano())
}
