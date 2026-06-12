package notifications

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const InboxFile = "inbox.json"

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeveritySuccess Severity = "success"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type Notification struct {
	ID             string            `json:"id"`
	CreatedAt      string            `json:"createdAt"`
	Type           string            `json:"type"`
	Severity       Severity          `json:"severity"`
	Title          string            `json:"title"`
	Message        string            `json:"message,omitempty"`
	Source         string            `json:"source,omitempty"`
	ActionRequired bool              `json:"actionRequired,omitempty"`
	Read           bool              `json:"read"`
	Dismissed      bool              `json:"dismissed"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type Store struct {
	Dir string
	Now func() time.Time
}

func NewStore(dir string) Store {
	return Store{Dir: strings.TrimSpace(dir), Now: time.Now}
}

func (s Store) Record(input Notification) (Notification, error) {
	if strings.TrimSpace(s.Dir) == "" {
		return Notification{}, fmt.Errorf("notification store directory is required")
	}
	items, err := s.readAll()
	if err != nil {
		return Notification{}, err
	}
	item := sanitize(input)
	if item.Title == "" {
		return Notification{}, fmt.Errorf("notification title is required")
	}
	now := s.now().UTC()
	if item.ID == "" {
		item.ID = fmt.Sprintf("note_%d", now.UnixNano())
	}
	if item.CreatedAt == "" {
		item.CreatedAt = now.Format(time.RFC3339)
	}
	if item.Severity == "" {
		item.Severity = SeverityInfo
	}
	if hasID(items, item.ID) {
		return Notification{}, fmt.Errorf("notification already exists: %s", item.ID)
	}
	items = append(items, item)
	sortNotifications(items)
	if err := s.writeAll(items); err != nil {
		return Notification{}, err
	}
	return item, nil
}

func (s Store) List(limit int, includeDismissed bool) ([]Notification, error) {
	items, err := s.readAll()
	if err != nil {
		return nil, err
	}
	sortNotifications(items)
	out := make([]Notification, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if item.Dismissed && !includeDismissed {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s Store) MarkRead(id string) (Notification, error) {
	return s.update(id, func(item *Notification) {
		item.Read = true
	})
}

func (s Store) Dismiss(id string) (Notification, error) {
	return s.update(id, func(item *Notification) {
		item.Read = true
		item.Dismissed = true
	})
}

func (s Store) update(id string, update func(*Notification)) (Notification, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Notification{}, fmt.Errorf("notification id is required")
	}
	items, err := s.readAll()
	if err != nil {
		return Notification{}, err
	}
	for i := range items {
		if items[i].ID != id {
			continue
		}
		update(&items[i])
		items[i] = sanitize(items[i])
		if err := s.writeAll(items); err != nil {
			return Notification{}, err
		}
		return items[i], nil
	}
	return Notification{}, fmt.Errorf("notification not found: %s", id)
}

func (s Store) readAll() ([]Notification, error) {
	path := s.path()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []Notification{}, nil
	}
	if err != nil {
		return nil, err
	}
	var items []Notification
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("parse notification inbox: %w", err)
	}
	out := make([]Notification, 0, len(items))
	for _, item := range items {
		item = sanitize(item)
		if item.ID == "" || item.Title == "" {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s Store) writeAll(items []Notification) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	sortNotifications(items)
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path() + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path())
}

func (s Store) path() string {
	return filepath.Join(s.Dir, InboxFile)
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func sanitize(item Notification) Notification {
	item.ID = sanitizeToken(item.ID)
	item.CreatedAt = strings.TrimSpace(item.CreatedAt)
	item.Type = sanitizeToken(item.Type)
	item.Title = redact(strings.TrimSpace(item.Title))
	item.Message = redact(strings.TrimSpace(item.Message))
	item.Source = sanitizeToken(item.Source)
	switch item.Severity {
	case SeverityInfo, SeveritySuccess, SeverityWarning, SeverityError:
	default:
		item.Severity = SeverityInfo
	}
	if item.Metadata != nil {
		cleaned := map[string]string{}
		for key, value := range item.Metadata {
			key = sanitizeToken(key)
			value = redact(strings.TrimSpace(value))
			if key == "" || value == "" {
				continue
			}
			cleaned[key] = value
		}
		if len(cleaned) > 0 {
			item.Metadata = cleaned
		} else {
			item.Metadata = nil
		}
	}
	return item
}

func sanitizeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = tokenPattern.ReplaceAllString(value, "_")
	return strings.Trim(value, "_")
}

func hasID(items []Notification, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func sortNotifications(items []Notification) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt == items[j].CreatedAt {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt < items[j].CreatedAt
	})
}

func redact(value string) string {
	for _, pattern := range secretPatterns {
		value = pattern.ReplaceAllString(value, "[redacted]")
	}
	return value
}

var (
	tokenPattern   = regexp.MustCompile(`[^a-z0-9_\-]+`)
	secretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bsk-[a-z0-9_\-]{20,}`),
		regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{20,}`),
		regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9\-]{20,}`),
		regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	}
)
