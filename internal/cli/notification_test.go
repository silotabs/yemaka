package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunNotificationLifecycleRedactsSecrets(t *testing.T) {
	app := newExtensionTestApp(t)
	var out bytes.Buffer

	if err := runNotification(app, []string{
		"add",
		"--title", "Review ready sk-abcdefghijklmnopqrstuvwxyz123456",
		"--message", "Open the local report",
		"--type", "job completed",
		"--severity", "success",
		"--source", "scheduler",
		"--action-required",
	}, &out); err != nil {
		t.Fatalf("runNotification(add) error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, `"title"`) || !strings.Contains(output, "[redacted]") {
		t.Fatalf("add output = %q, want redacted notification JSON", output)
	}
	if strings.Contains(output, "sk-abcdefghijklmnopqrstuvwxyz123456") {
		t.Fatalf("add output leaked secret: %q", output)
	}

	out.Reset()
	if err := runNotification(app, []string{"list", "--limit", "5"}, &out); err != nil {
		t.Fatalf("runNotification(list) error = %v", err)
	}
	if !strings.Contains(out.String(), "Review ready [redacted]") {
		t.Fatalf("list output = %q, want saved redacted notification", out.String())
	}

	id := jsonFieldValueForTest(t, out.String(), "id")
	out.Reset()
	if err := runNotification(app, []string{"dismiss", id}, &out); err != nil {
		t.Fatalf("runNotification(dismiss) error = %v", err)
	}
	if !strings.Contains(out.String(), `"dismissed": true`) {
		t.Fatalf("dismiss output = %q, want dismissed", out.String())
	}

	out.Reset()
	if err := runNotification(app, []string{"list"}, &out); err != nil {
		t.Fatalf("runNotification(list after dismiss) error = %v", err)
	}
	if strings.Contains(out.String(), id) {
		t.Fatalf("dismissed notification visible without --all: %q", out.String())
	}
}

func jsonFieldValueForTest(t *testing.T, data string, field string) string {
	t.Helper()
	needle := `"` + field + `": "`
	start := strings.Index(data, needle)
	if start < 0 {
		t.Fatalf("field %q not found in %s", field, data)
	}
	start += len(needle)
	end := strings.Index(data[start:], `"`)
	if end < 0 {
		t.Fatalf("unterminated field %q in %s", field, data)
	}
	return data[start : start+end]
}
