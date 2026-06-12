package notifications

import (
	"strings"
	"testing"
	"time"
)

func TestStoreRecordsListsAndDismissesNotifications(t *testing.T) {
	store := NewStore(t.TempDir())
	store.Now = func() time.Time { return time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC) }

	created, err := store.Record(Notification{
		Type:           "job completed",
		Severity:       SeveritySuccess,
		Title:          "Website monitor completed",
		Message:        "No title change detected.",
		Source:         "scheduler",
		ActionRequired: false,
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if created.ID == "" || created.CreatedAt == "" {
		t.Fatalf("created notification missing id/time: %#v", created)
	}

	items, err := store.List(10, false)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 || items[0].Title != "Website monitor completed" {
		t.Fatalf("List() = %#v, want recorded item", items)
	}

	dismissed, err := store.Dismiss(created.ID)
	if err != nil {
		t.Fatalf("Dismiss() error = %v", err)
	}
	if !dismissed.Read || !dismissed.Dismissed {
		t.Fatalf("Dismiss() = %#v, want read+dismissed", dismissed)
	}
	items, err = store.List(10, false)
	if err != nil {
		t.Fatalf("List(after dismiss) error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("List(after dismiss) = %#v, want dismissed hidden", items)
	}
}

func TestStoreRedactsSecretLikeNotificationContent(t *testing.T) {
	store := NewStore(t.TempDir())
	created, err := store.Record(Notification{
		Title:   "token sk-abcdefghijklmnopqrstuvwxyz123456",
		Message: "github ghp_abcdefghijklmnopqrstuvwxyz123456 should not persist",
		Metadata: map[string]string{
			"api_key": "AKIA1234567890ABCDEF",
		},
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	joined := created.Title + "\n" + created.Message + "\n" + created.Metadata["api_key"]
	for _, leaked := range []string{"sk-abcdefghijklmnopqrstuvwxyz123456", "ghp_abcdefghijklmnopqrstuvwxyz123456", "AKIA1234567890ABCDEF"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("notification leaked secret %q in %#v", leaked, created)
		}
	}
}
