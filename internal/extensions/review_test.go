package extensions

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreReviewIncludesSafeFilePreviewsAndReuseGuidance(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "website_monitor")
	writeExtensionManifest(t, dir, validManifestYAML())

	store := NewStore(generated, filepath.Join(root, "logs"))
	review, err := store.Review("website_monitor")
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if review.Detail.Status.Name != "website_monitor" {
		t.Fatalf("status name = %q, want website_monitor", review.Detail.Status.Name)
	}
	if review.CanRun {
		t.Fatalf("CanRun = true, want false before registration")
	}
	if !review.CanRegister {
		t.Fatalf("CanRegister = false, want valid runnable package to be registerable")
	}
	if review.ReuseCommand != `yemaka extension run website_monitor '{"url":"https://example.com"}'` {
		t.Fatalf("ReuseCommand = %q", review.ReuseCommand)
	}
	if review.SampleInputJSON != `{"url":"https://example.com"}` || review.SampleInput["url"] != "https://example.com" {
		t.Fatalf("sample input = %q / %+v, want url sample", review.SampleInputJSON, review.SampleInput)
	}
	if len(review.SuggestedActions) < 2 || !strings.Contains(strings.Join(review.SuggestedActions, "\n"), "Run tests and register") {
		t.Fatalf("SuggestedActions = %+v, want test/register guidance", review.SuggestedActions)
	}
	var manifestPreview string
	for _, file := range review.Files {
		if file.Path == ManifestFile {
			manifestPreview = file.Content
			break
		}
	}
	if !strings.Contains(manifestPreview, "name: website_monitor") {
		t.Fatalf("manifest preview = %q, want extension manifest content", manifestPreview)
	}
}
